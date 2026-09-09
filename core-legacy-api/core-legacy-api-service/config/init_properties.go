package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
)

func InitializeDefaultProperties(s ConfigService, ctx context.Context) error {
	err := InitializeGlobalDefaultProperties(s, ctx)
	if err != nil {

		return err
	}
	baselineProj := configloader.GetOrDefaultString("baseline.proj", "")
	if strings.TrimSpace(baselineProj) != "" {
		if err := InitializeBaselineProperties(s, ctx); err != nil {
			return err
		}
	}
	err = InitializeTenantManagerDefaultProperties(s, ctx)
	if err != nil {
		return err
	}
	err = InitializeDmpTenantActivatorDefaultProperties(s, ctx)
	if err != nil {
		return err
	}
	return nil
}

func InitializeGlobalDefaultProperties(s ConfigService, ctx context.Context) error {
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

	return s.AddProperties(ctx, configPropertiesGlobalApplicationName, defaultProfileName, properties)
}

func InitializeBaselineProperties(s ConfigService, ctx context.Context) error {
	baselineProj := configloader.GetOrDefaultString("baseline.proj", "")

	if strings.TrimSpace(baselineProj) == "" {
		return nil
	}

	baselineFetchProperties := []string{"tenant.default.id", "bss.tenant.default-id"}

	// Fetch global/default from the baseline Config Server.
	baselineProps, err := getBaselineProperties(
		ctx,
		baselineProj,
		"global",
		"default",
	)
	if err != nil {
		return fmt.Errorf("failed to fetch baseline properties: %w", err)
	}

	// Select only properties configured in BASELINE_FETCH_PROPERTIES.
	propertiesToMigrate := make(map[string]string)

	for _, property := range baselineFetchProperties {
		if value, exists := baselineProps[property]; exists {
			propertiesToMigrate[property] = value
		}
	}

	if len(propertiesToMigrate) == 0 {
		return nil
	}

	// Update/insert them into our global/default profile.
	return s.AddProperties(
		ctx,
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

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
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
		return nil, err
	}

	if len(response.PropertySources) == 0 {
		return map[string]string{}, nil
	}

	return response.PropertySources[0].Source, nil
}

func InitializeTenantManagerDefaultProperties(s ConfigService, ctx context.Context) error {
	INSTALLATION_NAME_RULES := "[{\"namespace\":null,\"microserviceName\":null,\"tenantId\":null,\"dbClassifier\":null, \"installationName\" : \"default\"}]"
	cloudPort := configloader.GetOrDefaultString("cloud.api.port", "6443")
	cloudProtocol := configloader.GetOrDefaultString("cloud.protocol", "https")
	cloudPublicHost := configloader.GetOrDefaultString("cloud.public.host", "")
	cloudHost := configloader.GetOrDefaultString("cloud.api.host", cloudPublicHost)
	defaultCloudServer := fmt.Sprintf("%s://%s:%s", cloudProtocol, cloudHost, cloudPort)
	cloudServerUrl := configloader.GetOrDefaultString("cloud.server.url", defaultCloudServer)

	properties := map[string]string{
		"installationNameRules":                       INSTALLATION_NAME_RULES,
		"openshift.server.url":                        cloudServerUrl,
		"tenant.registration.success_template":        "Dear %s %s your request is in progress.<br> We will send you an email all information after request approval.<br> Thank you.",
		"tenant.shoppingFrontend.templateName":        "qubership-cloud-shopping-frontend",
		"tenant.service.alias.template":               "DEFAULT",
		"default_credentials.tenant-manager.user":     "tenant",
		"default_credentials.tenant-manager.password": "tenant",
		"default_credentials.tenant-manager.auth-db":  "tenants",
		"default_credentials.dbaas.auth-db":           "",
	}
	return s.AddProperties(
		ctx,
		"tenant-manager",
		defaultProfileName,
		properties,
	)

}

func InitializeDmpTenantActivatorDefaultProperties(s ConfigService, ctx context.Context) error {
	properties := map[string]string{
		"tenant.shoppingFrontend.templateName": "qubership-cloud-shopping-frontend",
	}
	return s.AddProperties(
		ctx,
		"dmp-tenant-activator",
		defaultProfileName,
		properties,
	)
}
