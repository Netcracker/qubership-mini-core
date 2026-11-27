package config

import (
	"context"

	"github.com/netcracker/qubership-core-lib-go-rest-utils/v2/consul-propertysource"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
)

// ConsulACLToken is the ACL token that ConsulService sends with each request to Consul. Acquire gets the token and
// fails while Consul does not grant it yet; Value returns the current token once Acquire has succeeded.
type ConsulACLToken interface {
	Acquire() error
	Value() string
}

func NewConsulACLTokenFromConfig(ctx context.Context) (ConsulACLToken, error) {
	settings, err := consul.ReadAuthSettings()
	if err != nil {
		return nil, err
	}
	mode := settings.Mode
	if mode != consul.AuthModeK8s {
		token := configloader.GetOrDefaultString("consul.token", "")
		if mode == consul.AuthModeLegacy || token != "" {
			logger.InfoC(ctx, "Consul auth mode is '%s': using the token from the config-server-consul-token secret", mode)
			return secretACLToken(token), nil
		}
	}
	logger.InfoC(ctx, "Consul auth mode is '%s': logging in to Consul with the service account token", mode)
	return newServiceAccountACLToken(ctx, configloader.GetOrDefaultString("consul.url", "")), nil
}

// secretACLToken is the token that core-bootstrap stores in the config-server-consul-token secret.
type secretACLToken string

func (t secretACLToken) Acquire() error {
	return nil
}

func (t secretACLToken) Value() string {
	return string(t)
}

// serviceAccountACLToken is the token that Consul issues for a login with the projected ServiceAccount token.
type serviceAccountACLToken struct {
	client *consul.Client
}

func newServiceAccountACLToken(ctx context.Context, consulURL string) *serviceAccountACLToken {
	return &serviceAccountACLToken{
		client: consul.NewClient(consul.ClientConfig{
			Address: consulURL,
			Ctx:     ctx,
			Mode:    consul.AuthModeK8s,
		}),
	}
}

func (t *serviceAccountACLToken) Acquire() error {
	return t.client.Login()
}

func (t *serviceAccountACLToken) Value() string {
	return t.client.SecretId()
}
