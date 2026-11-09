package config

import (
	"context"
	"errors"
	"net/http"

	"github.com/hashicorp/consul/api"
	consulpropertysource "github.com/netcracker/qubership-core-lib-go-rest-utils/v2/consul-propertysource"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
)

// TODO: remove the ACL support disabled check once rest-utils#125 is released.
const consulACLDisabledMessage = "ACL support disabled"

// ConsulACLToken is the ACL token that ConsulService sends with each request to Consul. Acquire gets the token and
// fails while Consul does not grant it yet; SecretID returns the current token once Acquire has succeeded.
type ConsulACLToken interface {
	Acquire() error
	SecretID() string
}

func NewConsulACLTokenFromConfig(ctx context.Context) (ConsulACLToken, error) {
	settings, err := consulpropertysource.ReadAuthSettings()
	if err != nil {
		return nil, err
	}
	mode := settings.Mode
	if mode != consulpropertysource.AuthModeK8s {
		token := configloader.GetOrDefaultString("consul.token", "")
		if mode == consulpropertysource.AuthModeLegacy || token != "" {
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

func (t secretACLToken) SecretID() string {
	return string(t)
}

// serviceAccountACLToken is the token that Consul issues for a login with the projected ServiceAccount token.
type serviceAccountACLToken struct {
	ctx         context.Context
	client      *consulpropertysource.Client
	aclDisabled bool
}

func newServiceAccountACLToken(ctx context.Context, consulURL string) *serviceAccountACLToken {
	return &serviceAccountACLToken{
		ctx: ctx,
		client: consulpropertysource.NewClient(consulpropertysource.ClientConfig{
			Address: consulURL,
			Ctx:     ctx,
			Mode:    consulpropertysource.AuthModeK8s,
		}),
	}
}

func (t *serviceAccountACLToken) Acquire() error {
	err := t.client.Login()
	if isConsulACLDisabled(err) {
		logger.InfoC(t.ctx, "Consul ACLs are disabled: working without a Consul token")
		t.aclDisabled = true
		return nil
	}
	return err
}

func (t *serviceAccountACLToken) SecretID() string {
	if t.aclDisabled {
		return ""
	}
	return t.client.SecretId()
}

func isConsulACLDisabled(err error) bool {
	var statusErr api.StatusError
	return errors.As(err, &statusErr) &&
		statusErr.Code == http.StatusUnauthorized &&
		statusErr.Body == consulACLDisabledMessage
}
