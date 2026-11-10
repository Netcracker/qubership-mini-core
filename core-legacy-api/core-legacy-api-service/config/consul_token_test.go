package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	requests []map[string]string
}

func newFakeConsulLogin(t *testing.T, status int, body string) (*fakeConsulLogin, string) {
	fake := &fakeConsulLogin{}
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
		fake.requests = append(fake.requests, request)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return fake, server.URL
}

const loginResponse = `{"SecretID":"login-token","AuthMethod":"applications-k8s-m2m"}`

var serviceAccountLogin = map[string]string{"AuthMethod": "applications-k8s-m2m", "BearerToken": projectedToken}

func initConfig(t *testing.T, env map[string]string) {
	for _, name := range []string{"CONSUL_AUTH_MODE", "M2M_AUTH_MODE", "CONSUL_TOKEN", "CONSUL_URL"} {
		t.Setenv(name, env[name])
	}
	configloader.Init(configloader.EnvPropertySource())
	t.Cleanup(func() { configloader.Init() })
}

func TestNewConsulACLTokenFromConfig_ChoosesTheTokenByAuthMode(t *testing.T) {
	cases := []struct {
		name        string
		mode        string
		secretToken string
		wantValue   string
		wantLogins  []map[string]string
	}{
		{"legacy uses the token of the secret", "legacy", "secret-token", "secret-token", nil},
		{"legacy without a secret works without a token", "legacy", "", "", nil},
		{"hybrid uses the token of the secret when bootstrap issued one", "hybrid", "secret-token", "secret-token", nil},
		{"hybrid logs in when bootstrap issued no token", "hybrid", "", "login-token", []map[string]string{serviceAccountLogin}},
		{"k8s logs in even when the secret has a token", "k8s", "secret-token", "login-token", []map[string]string{serviceAccountLogin}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			consul, address := newFakeConsulLogin(t, http.StatusOK, loginResponse)
			initConfig(t, map[string]string{"CONSUL_AUTH_MODE": tc.mode, "CONSUL_TOKEN": tc.secretToken, "CONSUL_URL": address})

			token, err := NewConsulACLTokenFromConfig(context.Background())
			require.NoError(t, err)
			require.NoError(t, token.Acquire())

			assert.Equal(t, tc.wantValue, token.Value())
			assert.Equal(t, tc.wantLogins, consul.requests)
		})
	}
}

func TestNewConsulACLTokenFromConfig_AFailedHybridLoginDoesNotFallBackToM2M(t *testing.T) {
	consul, address := newFakeConsulLogin(t, http.StatusForbidden, "Permission denied")
	initConfig(t, map[string]string{"CONSUL_AUTH_MODE": "hybrid", "CONSUL_URL": address})
	token, err := NewConsulACLTokenFromConfig(context.Background())
	require.NoError(t, err)

	assert.Error(t, token.Acquire())

	assert.Equal(t, []map[string]string{serviceAccountLogin}, consul.requests)
}

func TestNewConsulACLTokenFromConfig_RejectsAnUnknownMode(t *testing.T) {
	initConfig(t, map[string]string{"CONSUL_AUTH_MODE": "static", "CONSUL_URL": "http://consul:8500"})

	token, err := NewConsulACLTokenFromConfig(context.Background())

	assert.ErrorContains(t, err, "static")
	assert.Nil(t, token)
}

func TestServiceAccountACLToken_WorksWithoutATokenWhenConsulACLsAreDisabled(t *testing.T) {
	_, address := newFakeConsulLogin(t, http.StatusUnauthorized, "ACL support disabled")
	token := newServiceAccountACLToken(context.Background(), address)

	require.NoError(t, token.Acquire())

	assert.Equal(t, "", token.Value())
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

func TestServiceAccountACLToken_RealConsulWithACLsDisabledWorksWithoutAToken(t *testing.T) {
	address := startConsulWithACLsDisabled(t, consulImage)
	token := newServiceAccountACLToken(context.Background(), address)

	require.NoError(t, token.Acquire())

	assert.Equal(t, "", token.Value())
}
