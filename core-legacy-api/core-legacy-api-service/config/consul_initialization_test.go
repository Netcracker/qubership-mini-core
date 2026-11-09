package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingToken struct {
	failures []error
	logins   int
}

func (t *failingToken) Login() error {
	t.logins++
	if len(t.failures) == 0 {
		return nil
	}
	err := t.failures[0]
	t.failures = t.failures[1:]
	return err
}

func (t *failingToken) SecretID() string {
	return ""
}

type memoryConfigService struct {
	ConfigService
	failures    []error
	addedByApps map[string]map[string]string
}

func (s *memoryConfigService) FindByApplicationAndProfile(_ context.Context, application string, profile string) (model.ConfigProfile, error) {
	return model.ConfigProfile{Application: application, Profile: profile}, nil
}

func (s *memoryConfigService) AddProperties(_ context.Context, application string, _ string, properties map[string]string) error {
	if len(s.failures) > 0 {
		err := s.failures[0]
		s.failures = s.failures[1:]
		return err
	}
	if s.addedByApps == nil {
		s.addedByApps = map[string]map[string]string{}
	}
	s.addedByApps[application] = properties
	return nil
}

func setInitRetryDelay(t *testing.T, delay time.Duration) {
	previous := initRetryDelay
	initRetryDelay = delay
	t.Cleanup(func() { initRetryDelay = previous })
}

func TestInitializeConsul_RetriesAFailedLoginAndThenWritesTheDefaults(t *testing.T) {
	setInitRetryDelay(t, time.Millisecond)
	token := &failingToken{failures: []error{errors.New("Permission denied")}}
	service := &memoryConfigService{}

	err := InitializeConsul(context.Background(), token, service)

	require.NoError(t, err)
	assert.Equal(t, 2, token.logins)
	assert.Equal(t, "http://internal-gateway-service:8080", service.addedByApps["global"]["apigateway.internal.url"])
}

func TestInitializeConsul_RetriesAFailedWriteOfTheDefaults(t *testing.T) {
	setInitRetryDelay(t, time.Millisecond)
	service := &memoryConfigService{failures: []error{errors.New("consul transaction rejected")}}

	err := InitializeConsul(context.Background(), staticConsulToken(""), service)

	require.NoError(t, err)
	assert.Equal(t, "http://internal-gateway-service:8080", service.addedByApps["global"]["apigateway.internal.url"])
}

func TestInitializeConsul_StopsWaitingForTheNextAttemptWhenTheServiceShutsDown(t *testing.T) {
	setInitRetryDelay(t, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	token := &failingToken{failures: []error{errors.New("Permission denied")}}

	err := InitializeConsul(ctx, token, &memoryConfigService{})

	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, token.logins)
}
