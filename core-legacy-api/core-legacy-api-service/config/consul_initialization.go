package config

import (
	"context"
	"time"
)

var initRetryDelay = 5 * time.Second

func InitializeConsul(ctx context.Context, token ConsulToken, service ConfigService) error {
	for {
		err := token.Login()
		if err == nil {
			err = InitializeDefaultProperties(ctx, service)
		}
		if err == nil {
			return nil
		}
		logger.ErrorC(ctx, "Consul initialization failed: %s. Next attempt in %s", err.Error(), initRetryDelay)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(initRetryDelay):
		}
	}
}
