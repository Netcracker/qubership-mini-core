package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-core-lib-go/v3/security/rest"
)

var logger = logging.GetLogger("Initialization")

func InitializeDefaultProperties(ctx context.Context, s ConfigService) error {
	// Initialize the global default properties required by the system.
	if err := InitializeGlobalDefaultProperties(ctx, s); err != nil {
		return err
	}

	// Initialize baseline properties when a baseline project is configured.
	baselineProj := configloader.GetOrDefaultString("baseline.proj", "")
	if strings.TrimSpace(baselineProj) != "" {
		if err := InitializeBaselineProperties(ctx, s); err != nil {
			return err
		}
	}

	// Initialize default properties for Tenant Manager.
	if err := InitializeTenantManagerDefaultProperties(ctx, s); err != nil {
		return err
	}

	// Initialize default properties for DMP Tenant Activator.
	return InitializeDmpTenantActivatorDefaultProperties(ctx, s)
}

func addMissingProperties(ctx context.Context, s ConfigService, application string, profile string, properties map[string]string) error {
	stored, err := s.FindByApplicationAndProfile(ctx, application, profile)
	if err != nil {
		return err
	}
	storedProperties := stored.GetPropertiesAsMap()

	missingProperties := make(map[string]string)
	for key, value := range properties {
		if _, exists := storedProperties[key]; !exists {
			missingProperties[key] = value
		}
	}
	if len(missingProperties) == 0 {
		return nil
	}

	logger.InfoC(ctx, "Adding %d missing default properties for application=%s profile=%s", len(missingProperties), application, profile)
	return s.AddProperties(ctx, application, profile, missingProperties)
}

func InitializeGlobalDefaultProperties(ctx context.Context, s ConfigService) error {
	// 1. Core Cloud Infrastructure Settings
	namespace := configloader.GetOrDefaultString("microservice.namespace", "")
	cloudPublicHost := configloader.GetOrDefaultString("cloud.public.host", "")
	cloudPort := configloader.GetOrDefaultString("cloud.api.port", "6443")
	cloudProtocol := configloader.GetOrDefaultString("cloud.protocol", "https")

	cloudHost := configloader.GetOrDefaultString("cloud.api.host", cloudPublicHost)
	cloudInternalHost := configloader.GetOrDefaultString("cloud.private.host", "")
	if strings.TrimSpace(cloudInternalHost) == "" {
		cloudInternalHost = cloudPublicHost
	}

	// 2. Gateway URLs
	defaultPrivateGateway := fmt.Sprintf("%s://private-gateway-%s.%s", cloudProtocol, namespace, cloudInternalHost)
	defaultPublicGateway := fmt.Sprintf("%s://public-gateway-%s.%s", cloudProtocol, namespace, cloudPublicHost)
	defaultCloudServer := fmt.Sprintf("%s://%s:%s", cloudProtocol, cloudHost, cloudPort)

	privateGatewayUrl := configloader.GetOrDefaultString("private.gateway.url", defaultPrivateGateway)
	publicGatewayUrl := configloader.GetOrDefaultString("public.gateway.url", defaultPublicGateway)
	cloudServerUrl := configloader.GetOrDefaultString("cloud.server.url", defaultCloudServer)

	// 3. Mail & Authentication Defaults
	emailUser := configloader.GetOrDefaultString("email.user", "")
	emailPassword := configloader.GetOrDefaultString("email.password", "")
	emailAuth := true

	if strings.TrimSpace(emailUser) == "" {
		emailAuth = false
		emailUser = "admin"
	}
	if strings.TrimSpace(emailPassword) == "" {
		emailAuth = false
		emailPassword = "admin"
	}

	// 4. Property Map Assembly (Sorted Alphabetically for Scannability)
	properties := map[string]string{
		"apigateway.external.private.url":    privateGatewayUrl,
		"apigateway.external.public.url":     publicGatewayUrl,
		"apigateway.internal.url":            "http://internal-gateway-service:8080",
		"apigateway.internal.url-https":      "https://internal-gateway-service:8443",
		"apigateway.private.url":             "http://private-gateway-service:8080",
		"apigateway.private.url-https":       "https://private-gateway-service:8443",
		"apigateway.public.url":              "http://public-gateway-service:8080",
		"apigateway.public.url-https":        "https://public-gateway-service:8443",
		"apigateway.routes.registration.url": "/api/v1/routes",
		"apigateway.url":                     "http://internal-gateway-service:8080",
		"apigateway.url-https":               "https://internal-gateway-service:8443",

		"error.page.customerSpecifier": "default",
		"http.buffer.header.max.size":  configloader.GetOrDefaultString("http.buffer.header.max.size", "10240"),

		"idp.authServersCount":     "",
		"idp.authServerUrl":        "",
		"idp.clientId":             "",
		"idp.gateway.clientId":     "",
		"idp.gateway.clientSecret": "",
		"idp.gateway.route":        "",
		"idp.sslRequiredType":      "",

		"keycloak.authServersCount":     "",
		"keycloak.authServerUrl":        "",
		"keycloak.clientId":             "",
		"keycloak.gateway.clientId":     "",
		"keycloak.gateway.clientSecret": "",
		"keycloak.gateway.route":        "",
		"keycloak.sslRequiredType":      "",

		"mail.cloudAdminEmail": configloader.GetOrDefaultString("cloud.admin.email", ""),
		"mail.fromEmail":       configloader.GetOrDefaultString("email.from", ""),
		"mail.server.auth":     strconv.FormatBool(emailAuth),
		"mail.server.host":     configloader.GetOrDefaultString("email.host", ""),
		"mail.server.password": emailPassword,
		"mail.server.user":     emailUser,

		"openshift.namespace":           namespace,
		"openshift.server.internal.url": fmt.Sprintf("%s://%s:%s", cloudProtocol, cloudInternalHost, cloudPort),
		"openshift.server.url":          cloudServerUrl,
	}

	if allowedHeaders := configloader.GetOrDefaultString("allowed.headers", ""); strings.TrimSpace(allowedHeaders) != "" {
		properties["headers.allowed"] = allowedHeaders
	}

	return addMissingProperties(ctx, s, configPropertiesGlobalApplicationName, defaultProfileName, properties)
}

func InitializeBaselineProperties(ctx context.Context, s ConfigService) error {
	baselineProj := configloader.GetOrDefaultString("baseline.proj", "")

	if strings.TrimSpace(baselineProj) == "" {
		return nil
	}
	baselineFetchProperties := strings.Split(configloader.GetOrDefaultString("baseline.fetch.properties", ""), ",")

	logger.InfoC(ctx, "Starting baseline properties initialization")
	logger.InfoC(ctx, "Baseline project: %q", baselineProj)
	logger.InfoC(ctx, "Properties configured for migration: %v", baselineFetchProperties)

	// Fetch global/default from the baseline Config Server.
	baselineProps, err := getBaselineProperties(
		ctx,
		baselineProj,
		"global",
		"default",
	)
	if err != nil {
		logger.WarnC(ctx, "Skipping baseline properties: failed to fetch them from baseline project %q: %v", baselineProj, err)
		return nil
	}

	// Select only properties configured in BASELINE_FETCH_PROPERTIES.
	propertiesToMigrate := make(map[string]string)

	for _, property := range baselineFetchProperties {
		if value, exists := baselineProps[property]; exists {
			logger.DebugC(ctx, "Selected baseline property for migration: %s", property)
			propertiesToMigrate[property] = value
		}
	}

	if len(propertiesToMigrate) == 0 {
		return nil
	}

	// Add the ones missing from our global/default profile.
	return addMissingProperties(
		ctx,
		s,
		configPropertiesGlobalApplicationName,
		defaultProfileName,
		propertiesToMigrate,
	)
}

func getBaselineProperties(
	ctx context.Context,
	baselineProj string,
	app string,
	profile string,
) (map[string]string, error) {
	url := fmt.Sprintf(
		"http://config-server.%s:8080/%s/%s",
		baselineProj,
		app,
		profile,
	)

	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := rest.NewM2MRestClient().DoRequest(requestCtx, http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf(
			"baseline Config Server returned HTTP %d",
			resp.StatusCode,
		)
	}

	var response struct {
		PropertySources []struct {
			Source map[string]string `json:"source"`
		} `json:"propertySources"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode baseline response: %w", err)
	}

	logger.InfoC(
		ctx,
		"Baseline response contains %d property sources",
		len(response.PropertySources),
	)

	if len(response.PropertySources) == 0 {
		return map[string]string{}, nil
	}

	return response.PropertySources[0].Source, nil
}

func InitializeTenantManagerDefaultProperties(ctx context.Context, s ConfigService) error {
	InstallationNameRules := "[{\"namespace\":null,\"microserviceName\":null,\"tenantId\":null,\"dbClassifier\":null, \"installationName\" : \"default\"}]"
	cloudPort := configloader.GetOrDefaultString("cloud.api.port", "6443")
	cloudProtocol := configloader.GetOrDefaultString("cloud.protocol", "https")
	cloudPublicHost := configloader.GetOrDefaultString("cloud.public.host", "")
	cloudHost := configloader.GetOrDefaultString("cloud.api.host", cloudPublicHost)
	defaultCloudServer := fmt.Sprintf("%s://%s:%s", cloudProtocol, cloudHost, cloudPort)
	cloudServerUrl := configloader.GetOrDefaultString("cloud.server.url", defaultCloudServer)

	properties := map[string]string{
		"installationNameRules":                       InstallationNameRules,
		"openshift.server.url":                        cloudServerUrl,
		"tenant.registration.success_template":        "Dear %s %s your request is in progress.<br> We will send you an email all information after request approval.<br> Thank you.",
		"tenant.shoppingFrontend.templateName":        "qubership-cloud-shopping-frontend",
		"tenant.service.alias.template":               "DEFAULT",
		"default_credentials.tenant-manager.user":     "tenant",
		"default_credentials.tenant-manager.password": "tenant",
		"default_credentials.tenant-manager.auth-db":  "tenants",
		"default_credentials.dbaas.auth-db":           "",
	}
	return addMissingProperties(
		ctx,
		s,
		"tenant-manager",
		defaultProfileName,
		properties,
	)

}

func InitializeDmpTenantActivatorDefaultProperties(ctx context.Context, s ConfigService) error {
	properties := map[string]string{
		"tenant.shoppingFrontend.templateName": "qubership-cloud-shopping-frontend",
	}
	return addMissingProperties(
		ctx,
		s,
		"dmp-tenant-activator",
		defaultProfileName,
		properties,
	)
}
