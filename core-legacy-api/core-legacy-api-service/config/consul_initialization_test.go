package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/model"
	"github.com/stretchr/testify/assert"
)

type failingACLToken struct {
	failures  []error
	acquired  int
	succeeded bool
}

func (t *failingACLToken) Acquire() error {
	t.acquired++
	if len(t.failures) == 0 {
		t.succeeded = true
		return nil
	}
	err := t.failures[0]
	t.failures = t.failures[1:]
	return err
}

func (t *failingACLToken) Value() string {
	return ""
}

type memoryConfigService struct {
	ConfigService
	token             *failingACLToken
	failures          []error
	writesBeforeToken int
	addedByApps       map[string]map[string]string
}

func (s *memoryConfigService) FindByApplicationAndProfile(_ context.Context, application string, profile string) (model.ConfigProfile, error) {
	return model.ConfigProfile{Application: application, Profile: profile}, nil
}

func (s *memoryConfigService) AddProperties(_ context.Context, application string, _ string, properties map[string]string) error {
	if s.token != nil && !s.token.succeeded {
		s.writesBeforeToken++
	}
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

func TestInitializeConsul_RetriesTheTokenAndThenWritesTheDefaults(t *testing.T) {
	setInitRetryDelay(t, time.Millisecond)
	token := &failingACLToken{failures: []error{errors.New("Permission denied")}}
	service := &memoryConfigService{token: token}

	InitializeConsul(context.Background(), token, service)

	assert.Equal(t, 2, token.acquired)
	assert.Equal(t, 0, service.writesBeforeToken)
	assert.Equal(t, "http://internal-gateway-service:8080", service.addedByApps["global"]["apigateway.internal.url"])
}

func TestInitializeConsul_RetriesAFailedWriteOfTheDefaultsWithoutAcquiringTheTokenAgain(t *testing.T) {
	setInitRetryDelay(t, time.Millisecond)
	token := &failingACLToken{}
	service := &memoryConfigService{failures: []error{errors.New("consul transaction rejected")}}

	InitializeConsul(context.Background(), token, service)

	assert.Equal(t, 1, token.acquired)
	assert.Equal(t, "http://internal-gateway-service:8080", service.addedByApps["global"]["apigateway.internal.url"])
}
