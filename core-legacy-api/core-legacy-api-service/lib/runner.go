package lib

import (
	"context"
	"net/url"
	"os"
	"os/signal"
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
)

var (
	ctx, globalCancel = context.WithCancel(
		context.WithValue(
			context.Background(), "requestId", "",
		),
	)
	logger        = logging.GetLogger("Server")
	namespace     string
	consulURL     string
	consulToken   string
	shutdownHooks []func()
)

func RunService() {
	ctxmanager.Register(baseproviders.Get())

	consulPS := consul.NewLoggingPropertySource()
	propertySources := configloader.BasePropertySources()
	configloader.InitWithSourcesArray(append(propertySources, consulPS))
	consul.StartWatchingForPropertiesWithRetry(ctx, consulPS, func(event interface{}, err error) {
	})

	namespace = configloader.GetOrDefaultString("microservice.namespace", "")
	consulURL = configloader.GetOrDefaultString("consul.url", "")
	consulToken = configloader.GetOrDefaultString("consul.token", "")

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
