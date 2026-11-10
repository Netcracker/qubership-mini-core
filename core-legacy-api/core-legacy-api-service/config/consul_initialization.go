package config

import (
	"context"
	"time"
)

var initRetryDelay = 5 * time.Second

// InitConsulConfiguration gets the Consul ACL token in the configured auth mode and writes the default properties,
// retrying both until they succeed. It fails only on invalid Consul auth settings.
func InitConsulConfiguration(ctx context.Context) (*ConsulService, error) {
	token, err := NewConsulACLTokenFromConfig(ctx)
	if err != nil {
		return nil, err
	}
	service := NewConsulServiceFromConfig(token.Value)
	InitializeConsul(ctx, token, service)
	return service, nil
}

func InitializeConsul(ctx context.Context, token ConsulACLToken, service ConfigService) {
	retryUntilSuccess(ctx, "Getting the Consul ACL token", token.Acquire)
	retryUntilSuccess(ctx, "Writing the default properties", func() error {
		return InitializeDefaultProperties(ctx, service)
	})
}

func retryUntilSuccess(ctx context.Context, action string, run func() error) {
	for {
		err := run()
		if err == nil {
			return
		}
		logger.ErrorC(ctx, "%s failed: %s. Next attempt in %s", action, err.Error(), initRetryDelay)
		time.Sleep(initRetryDelay)
	}
}
