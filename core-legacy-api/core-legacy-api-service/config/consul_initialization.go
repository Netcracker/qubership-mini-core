package config

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/netcracker/qubership-core-lib-go-actuator-common/v2/health"
)

var (
	initRetryFirstDelay = time.Second
	initRetryMaxDelay   = time.Minute
)

var errConsulInitializationPending = errors.New("login to Consul and initialization of default properties have not completed yet")

type ConsulInitialization struct {
	mutex sync.RWMutex
	err   error
	done  chan struct{}
}

func StartConsulInitialization(ctx context.Context, token ConsulToken, service ConfigService) *ConsulInitialization {
	initialization := &ConsulInitialization{err: errConsulInitializationPending, done: make(chan struct{})}
	go initialization.run(ctx, token, service)
	return initialization
}

func (i *ConsulInitialization) run(ctx context.Context, token ConsulToken, service ConfigService) {
	defer close(i.done)
	delay := initRetryFirstDelay
	for {
		err := initializeConsul(ctx, token, service)
		i.setErr(err)
		if err == nil {
			logger.InfoC(ctx, "Consul initialization completed")
			return
		}
		logger.ErrorC(ctx, "Consul initialization failed: %s. Next attempt in %s", err.Error(), delay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = nextInitRetryDelay(delay)
	}
}

func nextInitRetryDelay(delay time.Duration) time.Duration {
	return min(delay*2, initRetryMaxDelay)
}

func initializeConsul(ctx context.Context, token ConsulToken, service ConfigService) error {
	if err := token.Login(); err != nil {
		return err
	}
	return InitializeDefaultProperties(ctx, service)
}

func (i *ConsulInitialization) setErr(err error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.err = err
}

func (i *ConsulInitialization) Done() <-chan struct{} {
	return i.done
}

func (i *ConsulInitialization) HealthCheck() health.Status {
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	if i.err != nil {
		return health.Status{Name: health.StatusProblem, Details: map[string]interface{}{
			"error": i.err.Error(),
		}}
	}
	return health.Status{Name: health.StatusUp}
}
