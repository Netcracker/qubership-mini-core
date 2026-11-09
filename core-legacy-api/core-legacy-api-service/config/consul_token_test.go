package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/security/tokensource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const projectedToken = "projected-service-account-token"

func TestMain(m *testing.M) {
	tokensDir, err := os.MkdirTemp("", "core-legacy-api-tokens")
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Join(tokensDir, "netcracker"), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(tokensDir, "netcracker", "token"), []byte(projectedToken), 0o644); err != nil {
		panic(err)
	}
	tokensource.DefaultAudienceTokensDir = tokensDir
	code := m.Run()
	_ = os.RemoveAll(tokensDir)
	os.Exit(code)
}

type fakeConsulLogin struct {
	mutex    sync.Mutex
	requests []map[string]string
	status   int
	body     string
}

func newFakeConsulLogin(t *testing.T, status int, body string) (*fakeConsulLogin, string) {
	fake := &fakeConsulLogin{status: status, body: body}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/acl/login" {
			t.Errorf("unexpected request to Consul: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var request map[string]string
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("login request body is not JSON: %v", err)
		}
		fake.mutex.Lock()
		defer fake.mutex.Unlock()
		fake.requests = append(fake.requests, request)
		w.WriteHeader(fake.status)
		_, _ = w.Write([]byte(fake.body))
	}))
	t.Cleanup(server.Close)
	return fake, server.URL
}

func (f *fakeConsulLogin) respond(status int, body string) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.status = status
	f.body = body
}

func (f *fakeConsulLogin) loginRequests() []map[string]string {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return append([]map[string]string(nil), f.requests...)
}

const loginResponse = `{"SecretID":"login-token","AuthMethod":"applications-k8s-m2m"}`

func initConfig(t *testing.T, env map[string]string) {
	for _, name := range []string{"CONSUL_AUTH_MODE", "M2M_AUTH_MODE", "CONSUL_TOKEN", "CONSUL_URL"} {
		t.Setenv(name, env[name])
	}
	configloader.Init(configloader.EnvPropertySource())
	t.Cleanup(func() { configloader.Init() })
}

func TestNewConsulTokenFromConfig_ChoosesTheTokenByAuthMode(t *testing.T) {
	cases := []struct {
		name           string
		consulAuthMode string
		m2mAuthMode    string
		secretToken    string
		wantSecretID   string
		wantLogins     int
	}{
		{"legacy uses the token of the secret", "legacy", "", "secret-token", "secret-token", 0},
		{"legacy without a secret works without a token", "legacy", "", "", "", 0},
		{"hybrid uses the token of the secret when bootstrap issued one", "hybrid", "", "secret-token", "secret-token", 0},
		{"hybrid logs in when bootstrap issued no token", "hybrid", "", "", "login-token", 1},
		{"k8s logs in even when the secret has a token", "k8s", "", "secret-token", "login-token", 1},
		{"an unset consul auth mode follows M2M_AUTH_MODE", "", "k8s", "secret-token", "login-token", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			consul, address := newFakeConsulLogin(t, http.StatusOK, loginResponse)
			initConfig(t, map[string]string{
				"CONSUL_AUTH_MODE": tc.consulAuthMode,
				"M2M_AUTH_MODE":    tc.m2mAuthMode,
				"CONSUL_TOKEN":     tc.secretToken,
				"CONSUL_URL":       address,
			})

			token, err := NewConsulTokenFromConfig(context.Background())
			require.NoError(t, err)
			require.NoError(t, token.Login())

			assert.Equal(t, tc.wantSecretID, token.SecretID())
			assert.Len(t, consul.loginRequests(), tc.wantLogins)
		})
	}
}

func TestNewConsulTokenFromConfig_RejectsAnUnknownMode(t *testing.T) {
	initConfig(t, map[string]string{"CONSUL_AUTH_MODE": "static", "CONSUL_URL": "http://consul:8500"})

	token, err := NewConsulTokenFromConfig(context.Background())

	assert.ErrorContains(t, err, "static")
	assert.Nil(t, token)
}

func TestLoginConsulToken_SendsTheProjectedTokenToTheAuthMethod(t *testing.T) {
	consul, address := newFakeConsulLogin(t, http.StatusOK, loginResponse)
	initConfig(t, map[string]string{"CONSUL_URL": address})

	require.NoError(t, newLoginConsulToken(context.Background(), address).Login())

	assert.Equal(t, []map[string]string{{"AuthMethod": "applications-k8s-m2m", "BearerToken": projectedToken}},
		consul.loginRequests())
}

func TestLoginConsulToken_ASecondLoginAfterASuccessfulOneSendsNoRequest(t *testing.T) {
	consul, address := newFakeConsulLogin(t, http.StatusOK, loginResponse)
	initConfig(t, map[string]string{"CONSUL_URL": address})
	token := newLoginConsulToken(context.Background(), address)
	require.NoError(t, token.Login())

	require.NoError(t, token.Login())

	assert.Len(t, consul.loginRequests(), 1)
}

func TestLoginConsulToken_WorksWithoutATokenWhenConsulACLsAreDisabled(t *testing.T) {
	_, address := newFakeConsulLogin(t, http.StatusUnauthorized, "ACL support disabled")
	initConfig(t, map[string]string{"CONSUL_URL": address})
	token := newLoginConsulToken(context.Background(), address)

	require.NoError(t, token.Login())

	assert.Equal(t, "", token.SecretID())
}

func TestLoginConsulToken_ReturnsOtherLoginErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"no binding rule matches yet", http.StatusForbidden, "Permission denied"},
		{"unauthorized for another reason", http.StatusUnauthorized, "ACL not found"},
		{"Consul fails", http.StatusInternalServerError, "rpc error"},
		{"ACL support disabled with another status", http.StatusForbidden, "ACL support disabled"},
		{"401 whose body adds words to the ACL disabled text", http.StatusUnauthorized, "ACL support disabled for this token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, address := newFakeConsulLogin(t, tc.status, tc.body)
			initConfig(t, map[string]string{"CONSUL_URL": address})
			token := newLoginConsulToken(context.Background(), address)

			err := token.Login()

			assert.Error(t, err)
		})
	}
}

func TestLoginConsulToken_ALoginThatFailedSucceedsOnTheNextCall(t *testing.T) {
	consul, address := newFakeConsulLogin(t, http.StatusForbidden, "Permission denied")
	initConfig(t, map[string]string{"CONSUL_URL": address})
	token := newLoginConsulToken(context.Background(), address)
	require.Error(t, token.Login())
	consul.respond(http.StatusOK, loginResponse)

	require.NoError(t, token.Login())

	assert.Equal(t, "login-token", token.SecretID())
}

func startConsulWithACLsDisabled(t *testing.T, image string) string {
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        image,
			ExposedPorts: []string{"8500/tcp"},
			Cmd:          []string{"agent", "-dev", "-client", "0.0.0.0"},
			WaitingFor:   wait.ForListeningPort("8500/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "8500")
	require.NoError(t, err)
	return fmt.Sprintf("http://%s:%s", host, port.Port())
}

func TestLoginConsulToken_RealConsulWithACLsDisabledWorksWithoutAToken(t *testing.T) {
	address := startConsulWithACLsDisabled(t, consulImage)
	token := newLoginConsulToken(context.Background(), address)

	require.NoError(t, token.Login())

	assert.Equal(t, "", token.SecretID())
}
