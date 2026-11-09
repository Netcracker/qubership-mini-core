package config

import (
	"context"
	"time"
)

var initRetryDelay = 5 * time.Second

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
