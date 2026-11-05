package config

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/netcracker/qubership-core-lib-go-actuator-common/v2/health"
	"github.com/netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scriptedToken struct {
	attempts chan chan error
}

func newScriptedToken() *scriptedToken {
	return &scriptedToken{attempts: make(chan chan error)}
}

func (t *scriptedToken) Login() error {
	result := make(chan error)
	t.attempts <- result
	return <-result
}

func (t *scriptedToken) SecretID() string {
	return ""
}

type memoryConfigService struct {
	ConfigService
	mutex       sync.Mutex
	failures    []error
	addedByApps map[string]map[string]string
}

func (s *memoryConfigService) FindByApplicationAndProfile(_ context.Context, application string, profile string) (model.ConfigProfile, error) {
	return model.ConfigProfile{Application: application, Profile: profile}, nil
}

func (s *memoryConfigService) AddProperties(_ context.Context, application string, _ string, properties map[string]string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
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

func (s *memoryConfigService) added(application string, key string) string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.addedByApps[application][key]
}

func fastInitRetries(t *testing.T) {
	first, maxDelay := initRetryFirstDelay, initRetryMaxDelay
	initRetryFirstDelay, initRetryMaxDelay = time.Millisecond, time.Millisecond
	t.Cleanup(func() { initRetryFirstDelay, initRetryMaxDelay = first, maxDelay })
}

func waitDone(t *testing.T, initialization *ConsulInitialization) {
	select {
	case <-initialization.Done():
	case <-time.After(10 * time.Second):
		t.Fatalf("Consul initialization did not complete in 10s; readiness is %+v", initialization.HealthCheck())
	}
}

func TestNextInitRetryDelay(t *testing.T) {
	cases := []struct {
		name    string
		current time.Duration
		want    time.Duration
	}{
		{"the first delay doubles", time.Second, 2 * time.Second},
		{"a delay below the cap doubles", 16 * time.Second, 32 * time.Second},
		{"a delay whose double exceeds the cap stops at the cap", 32 * time.Second, time.Minute},
		{"the cap stays the cap", time.Minute, time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, nextInitRetryDelay(tc.current), "nextInitRetryDelay(%s)", tc.current)
		})
	}
}

func TestConsulInitialization_ReadinessIsDownBeforeTheFirstAttemptCompletes(t *testing.T) {
	token := newScriptedToken()
	initialization := StartConsulInitialization(context.Background(), token, &memoryConfigService{})
	attempt := <-token.attempts

	status := initialization.HealthCheck()

	assert.Equal(t, health.StatusProblem, status.Name)
	attempt <- nil
	waitDone(t, initialization)
}

func TestConsulInitialization_AFailedLoginIsRetriedAndReportedByReadiness(t *testing.T) {
	fastInitRetries(t)
	token := newScriptedToken()
	initialization := StartConsulInitialization(context.Background(), token, &memoryConfigService{})
	(<-token.attempts) <- errors.New("Permission denied")
	secondAttempt := <-token.attempts

	status := initialization.HealthCheck()

	assert.Equal(t, health.StatusProblem, status.Name)
	assert.Equal(t, map[string]interface{}{"error": "Permission denied"}, status.Details)
	secondAttempt <- nil
	waitDone(t, initialization)
}

func TestConsulInitialization_ReadinessIsUpAndDefaultsAreWrittenAfterTheLoginSucceeds(t *testing.T) {
	fastInitRetries(t)
	token := newScriptedToken()
	service := &memoryConfigService{}
	initialization := StartConsulInitialization(context.Background(), token, service)
	(<-token.attempts) <- errors.New("Permission denied")
	(<-token.attempts) <- nil

	waitDone(t, initialization)

	assert.Equal(t, health.Status{Name: health.StatusUp}, initialization.HealthCheck())
	assert.Equal(t, "http://internal-gateway-service:8080", service.added("global", "apigateway.internal.url"))
}

func TestConsulInitialization_AFailedWriteOfDefaultsIsRetried(t *testing.T) {
	fastInitRetries(t)
	service := &memoryConfigService{failures: []error{errors.New("consul transaction rejected")}}
	initialization := StartConsulInitialization(context.Background(), staticConsulToken(""), service)

	waitDone(t, initialization)

	assert.Equal(t, health.Status{Name: health.StatusUp}, initialization.HealthCheck())
	assert.Equal(t, "http://internal-gateway-service:8080", service.added("global", "apigateway.internal.url"))
}

func TestConsulInitialization_StopsWaitingForTheNextAttemptWhenTheServiceShutsDown(t *testing.T) {
	first := initRetryFirstDelay
	initRetryFirstDelay = time.Hour
	t.Cleanup(func() { initRetryFirstDelay = first })
	ctx, cancel := context.WithCancel(context.Background())
	token := newScriptedToken()
	initialization := StartConsulInitialization(ctx, token, &memoryConfigService{})
	(<-token.attempts) <- errors.New("Permission denied")

	cancel()

	waitDone(t, initialization)
	require.Equal(t, health.StatusProblem, initialization.HealthCheck().Name)
}
