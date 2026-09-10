package lib

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/config"
	"github.com/netcracker/qubership-core-lib-go-rest-utils/v2/consul-propertysource"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/context-propagation/baseproviders"

	"github.com/gofiber/fiber/v2"
	"github.com/hashicorp/consul/api"
	"github.com/netcracker/qubership-core-lib-go-actuator-common/v2/health"
	fiberserver "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2"
	"github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2/server"
	"github.com/netcracker/qubership-core-lib-go/v3/context-propagation/ctxmanager"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"

	"github.com/Netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/docs"
)

var (
	ctx, globalCancel = context.WithCancel(
		context.WithValue(
			context.Background(), "requestId", "",
		),
	)
	logger        = logging.GetLogger("Server")
	shutdownHooks []func()
)

func RunService() {
	ctxmanager.Register(baseproviders.Get())

	consulPS := consul.NewLoggingPropertySource()
	propertySources := configloader.BasePropertySources()
	configloader.InitWithSourcesArray(append(propertySources, consulPS))
	consul.StartWatchingForPropertiesWithRetry(ctx, consulPS, func(event interface{}, err error) {
	})

	namespace := configloader.GetOrDefaultString("microservice.namespace", "")
	consulURL := configloader.GetOrDefaultString("consul.url", "")
	consulToken, err := GetConsulToken()
	if err != nil {
		logger.Errorf("%s", err.Error())
		return
	}

	healthService, err := health.NewHealthService()
	if err != nil {
		logger.Error("Couldn't create healthService")
	}

	app, err := fiberserver.New(fiber.Config{Network: fiber.NetworkTCP}).
		WithHealth("/health", healthService).
		Process()
	if err != nil {
		logger.Errorf("Error while create app because: %s", err.Error())
		return
	}
	app.Use(func(c *fiber.Ctx) error {
		requestHeaders := map[string]interface{}{}
		for key, value := range c.Request().Header.All() {
			requestHeaders[string(key)] = string(value)
		}
		var ctx = c.UserContext()
		ctx = ctxmanager.InitContext(ctx, requestHeaders)

		c.SetUserContext(ctx)
		return c.Next()
	})
	u, _ := url.Parse(consulURL)

	conf := api.DefaultConfig()
	conf.Address = u.Host
	conf.Scheme = u.Scheme
	conf.Token = consulToken

	consulClient, _ := api.NewClient(conf)
	consulService := config.NewConsulService(consulClient, namespace)
	err = config.InitializeDefaultProperties(consulService, ctx)
	if err != nil {
		logger.Errorf("Couldn't initialize default properties because: %s", err.Error())
		return
	}
	configController := config.NewConfigPropertiesController(consulService)

	app.Get("/applications", configController.GetApplicationsAndProfiles)
	app.Get("/:label/:name-:profiles.json", configController.FindOneJSON)
	app.Get("/:label/:name-:profiles.properties", configController.FindOneProperties)
	app.Get("/:label/:name-:profiles.yaml", configController.FindOneYaml)
	app.Get("/:label/:name-:profiles.yml", configController.FindOneYaml)

	app.Get("/:name-:profiles.json", configController.FindOneJSON)
	app.Get("/:name-:profiles.properties", configController.FindOneProperties)
	app.Get("/:name-:profiles.yaml", configController.FindOneYaml)
	app.Get("/:name-:profiles.yml", configController.FindOneYaml)

	app.Get("/:application/:profile", configController.FindOne)
	app.Get("/:application/:profile/:label", configController.FindOne)
	app.Post("/:application/:profile", configController.AddProperties)
	app.Put("/:application/:profile", configController.AddProperties)
	app.Post("/:application/:profile/properties-delete", configController.DeleteProperties)
	// swagger
	app.Get("/swagger-ui/swagger.json", func(ctx *fiber.Ctx) error {
		ctx.Set("Content-Type", "application/json")
		return ctx.Status(http.StatusOK).SendString(docs.SwaggerInfo.ReadDoc())
	})
	shutdownHooks = append(shutdownHooks, func() {
		logger.Info("Shutdown fiber server")
		if err := app.Shutdown(); err != nil {
			logger.ErrorC(ctx, "Error during server shutdown: %v", err)
		}
		logger.Info("Execute global cancel")
		globalCancel()
	})

	registerShutdownHooks()

	server.StartServer(app, "http.server.bind")
}

func GetConsulToken() (string, error) {
	tokenPathValue := configloader.GetOrDefault("consul.token.path", nil)
	if tokenPathValue == nil {
		return "", fmt.Errorf("Parameter %s is required but could not be found and no default value was provided", tokenPathValue)
	}
	var tokenPath string
	if s, ok := tokenPathValue.(string); ok {
		tokenPath = s
	} else {
		tokenPath = fmt.Sprintf("%v", tokenPath)
	}
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("Failed to read Consul token from file %s: %v. Consul is enabled but token file is not accessible.", tokenPath, err)
	}
	return strings.TrimSpace(string(tokenBytes)), nil
}

func registerShutdownHooks() {
	go func() {
		sigint := make(chan os.Signal, 1)

		// interrupt signal sent from terminal
		signal.Notify(sigint, os.Interrupt)
		// sigterm signal sent from kubernetes
		signal.Notify(sigint, syscall.SIGTERM)

		logger.Info("OS signal '%s' received, starting shutdown", (<-sigint).String())

		for _, hook := range shutdownHooks {
			hook()
		}
	}()
}
