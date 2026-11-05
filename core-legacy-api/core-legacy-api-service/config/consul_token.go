package config

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/hashicorp/consul/api"
	consul "github.com/netcracker/qubership-core-lib-go-rest-utils/v2/consul-propertysource"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
)

const consulACLDisabledMessage = "ACL support disabled"

type ConsulToken interface {
	Login() error
	SecretID() string
}

func NewConsulTokenFromConfig(ctx context.Context) (ConsulToken, error) {
	mode, err := consulAuthMode()
	if err != nil {
		return nil, err
	}
	if mode != consul.AuthModeK8s {
		staticToken := configloader.GetOrDefaultString("consul.token", "")
		if mode == consul.AuthModeLegacy || staticToken != "" {
			logger.InfoC(ctx, "Consul auth mode is '%s': using the token from the config-server-consul-token secret", mode)
			return staticConsulToken(staticToken), nil
		}
	}
	logger.InfoC(ctx, "Consul auth mode is '%s': logging in to Consul with the service account token", mode)
	return newLoginConsulToken(ctx, configloader.GetOrDefaultString("consul.url", "")), nil
}

func consulAuthMode() (consul.AuthMode, error) {
	if mode := configloader.GetOrDefaultString("consul.auth.mode", ""); mode != "" {
		switch consul.AuthMode(mode) {
		case consul.AuthModeLegacy, consul.AuthModeHybrid, consul.AuthModeK8s:
			return consul.AuthMode(mode), nil
		default:
			return "", fmt.Errorf("unknown value '%s' of property 'consul.auth.mode': expected one of '%s', '%s', '%s'",
				mode, consul.AuthModeLegacy, consul.AuthModeHybrid, consul.AuthModeK8s)
		}
	}
	m2mMode, err := security.ReadM2MAuthMode()
	if err != nil {
		return "", fmt.Errorf("property 'consul.auth.mode' is not set, so the Consul auth mode is taken from %s: %w",
			security.M2MAuthModeEnv, err)
	}
	return consul.AuthMode(m2mMode), nil
}

type staticConsulToken string

func (t staticConsulToken) Login() error {
	return nil
}

func (t staticConsulToken) SecretID() string {
	return string(t)
}

type loginConsulToken struct {
	ctx      context.Context
	client   *consul.Client
	loggedIn atomic.Bool
}

func newLoginConsulToken(ctx context.Context, address string) *loginConsulToken {
	return &loginConsulToken{
		ctx: ctx,
		client: consul.NewClient(consul.ClientConfig{
			Address: address,
			Ctx:     ctx,
			Mode:    consul.AuthModeK8s,
		}),
	}
}

func (t *loginConsulToken) Login() error {
	if t.loggedIn.Load() {
		return nil
	}
	if err := t.client.Login(); err != nil {
		if !isConsulACLDisabled(err) {
			return err
		}
		logger.InfoC(t.ctx, "Consul ACLs are disabled: working without a Consul token")
		return nil
	}
	t.loggedIn.Store(true)
	return nil
}

func (t *loginConsulToken) SecretID() string {
	if !t.loggedIn.Load() {
		return ""
	}
	return t.client.SecretId()
}

func isConsulACLDisabled(err error) bool {
	var statusErr api.StatusError
	return errors.As(err, &statusErr) &&
		statusErr.Code == http.StatusUnauthorized &&
		strings.TrimSpace(statusErr.Body) == consulACLDisabledMessage
}
