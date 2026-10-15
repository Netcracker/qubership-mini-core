package config

import (
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
)

func (s *ConsulServiceTestSuite) TestInitDefaultProperties() {
	err := InitializeDefaultProperties(s.ctx, s.consulService)
	s.Require().NoError(err)

	expected := map[string][]string{
		"global": {"apigateway.external.private.url",
			"apigateway.external.public.url",
			"apigateway.internal.url",
			"apigateway.internal.url-https",
			"apigateway.private.url",
			"apigateway.private.url-https",
			"apigateway.public.url",
			"apigateway.public.url-https",
			"apigateway.routes.registration.url",
			"apigateway.url",
			"apigateway.url-https",
			"error.page.customerSpecifier",
			"http.buffer.header.max.size",
			"idp.authServersCount",
			"idp.authServerUrl",
			"idp.clientId",
			"idp.gateway.clientId",
			"idp.gateway.clientSecret",
			"idp.gateway.route",
			"idp.sslRequiredType",
			"keycloak.authServersCount",
			"keycloak.authServerUrl",
			"keycloak.clientId",
			"keycloak.gateway.clientId",
			"keycloak.gateway.clientSecret",
			"keycloak.gateway.route",
			"keycloak.sslRequiredType",
			"mail.cloudAdminEmail",
			"mail.fromEmail",
			"mail.server.auth",
			"mail.server.host",
			"mail.server.password",
			"mail.server.user",
			"openshift.namespace",
			"openshift.server.internal.url",
			"openshift.server.url"},

		"tenant-manager": {"installationNameRules",
			"openshift.server.url",
			"tenant.registration.success_template",
			"tenant.shoppingFrontend.templateName",
			"tenant.service.alias.template",
			"default_credentials.tenant-manager.user",
			"default_credentials.tenant-manager.password",
			"default_credentials.tenant-manager.auth-db",
			"default_credentials.dbaas.auth-db"},
		"dmp-tenant-activator": {"tenant.shoppingFrontend.templateName"},
	}

	for appName, properties := range expected {
		value, err := s.consulService.FindByApplicationAndProfile(s.ctx, appName, defaultProfileName)
		s.Require().NoError(err)
		actualKeys := make(map[string]struct{}, len(value.Properties))
		for _, p := range value.Properties {
			actualKeys[p.Key] = struct{}{}
		}
		for _, property := range properties {
			s.Contains(actualKeys, property, "application=%s missing key=%s", appName, property)
		}
	}
}

func (s *ConsulServiceTestSuite) TestInitDefaultPropertiesKeepsValuesChangedAfterFirstRun() {
	s.Require().NoError(InitializeDefaultProperties(s.ctx, s.consulService))
	s.Require().NoError(s.consulService.AddProperties(s.ctx, "tenant-manager", defaultProfileName, map[string]string{
		"default_credentials.tenant-manager.password": "changed",
	}))

	s.Require().NoError(InitializeDefaultProperties(s.ctx, s.consulService))

	profile, err := s.consulService.FindByApplicationAndProfile(s.ctx, "tenant-manager", defaultProfileName)
	s.Require().NoError(err)
	s.Equal("changed", propertiesAsMap(profile)["default_credentials.tenant-manager.password"])
}

func (s *ConsulServiceTestSuite) TestInitDefaultPropertiesKeepsValuesStoredBeforeFirstRun() {
	stored := map[string]map[string]string{
		"global": {"mail.server.user": "changed"},
		"tenant-manager": {
			"default_credentials.tenant-manager.password": "changed",
			"default_credentials.tenant-manager.auth-db":  "",
		},
		"dmp-tenant-activator": {"tenant.shoppingFrontend.templateName": "changed"},
	}
	for application, properties := range stored {
		s.Require().NoError(s.consulService.AddProperties(s.ctx, application, defaultProfileName, properties))
	}

	s.Require().NoError(InitializeDefaultProperties(s.ctx, s.consulService))

	for application, properties := range stored {
		profile, err := s.consulService.FindByApplicationAndProfile(s.ctx, application, defaultProfileName)
		s.Require().NoError(err)
		actual := propertiesAsMap(profile)
		for key, value := range properties {
			s.Equal(value, actual[key], "application=%s key=%s", application, key)
		}
	}
	profile, err := s.consulService.FindByApplicationAndProfile(s.ctx, "tenant-manager", defaultProfileName)
	s.Require().NoError(err)
	s.Equal("tenant", propertiesAsMap(profile)["default_credentials.tenant-manager.user"])
}

func (s *ConsulServiceTestSuite) TestInitDefaultPropertiesRestoresMissingDefaults() {
	s.Require().NoError(InitializeDefaultProperties(s.ctx, s.consulService))
	s.Require().NoError(s.consulService.DeleteProperties(s.ctx, "tenant-manager", defaultProfileName, []string{
		"default_credentials.tenant-manager.user",
	}))

	s.Require().NoError(InitializeDefaultProperties(s.ctx, s.consulService))

	profile, err := s.consulService.FindByApplicationAndProfile(s.ctx, "tenant-manager", defaultProfileName)
	s.Require().NoError(err)
	s.Equal("tenant", propertiesAsMap(profile)["default_credentials.tenant-manager.user"])
}

func (s *ConsulServiceTestSuite) TestInitDefaultPropertiesSkipsUnreachableBaseline() {
	serviceloader.Register(1, &security.DummyToken{})
	s.T().Setenv("BASELINE_PROJ", "invalid")
	configloader.Init(configloader.EnvPropertySource())
	defer configloader.Init()

	s.Require().NoError(InitializeDefaultProperties(s.ctx, s.consulService))

	profile, err := s.consulService.FindByApplicationAndProfile(s.ctx, "dmp-tenant-activator", defaultProfileName)
	s.Require().NoError(err)
	s.Equal("qubership-cloud-shopping-frontend", propertiesAsMap(profile)["tenant.shoppingFrontend.templateName"])
}
