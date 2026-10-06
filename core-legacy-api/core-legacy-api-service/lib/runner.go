package lib

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/netcracker/qubership-core-lib-go-actuator-common/v2/tracing"
	"github.com/netcracker/qubership-core-lib-go-rest-utils/v2/consul-propertysource"
	"github.com/netcracker/qubership-core-lib-go-rest-utils/v2/podsecrets-propertysource"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/context-propagation/baseproviders"
	"github.com/netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/config"

	"github.com/gofiber/fiber/v2"
	"github.com/hashicorp/consul/api"
	"github.com/netcracker/qubership-core-lib-go-actuator-common/v2/health"
	fiberserver "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2"
	"github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2/server"
	"github.com/netcracker/qubership-core-lib-go/v3/context-propagation/ctxmanager"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"

	"github.com/netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/docs"
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
	sources := configloader.BasePropertySources()
	sources = podsecrets.AddPodSecretsPropertySource(sources)
	configloader.InitWithSourcesArray(append(sources, consulPS))

	consul.StartWatchingForPropertiesWithRetry(ctx, consulPS, func(event interface{}, err error) {
	})

	namespace := configloader.GetOrDefaultString("microservice.namespace", "")
	consulURL := configloader.GetOrDefaultString("consul.url", "")
	consulToken := configloader.GetOrDefaultString("consul.token", "")

	u, err := url.Parse(consulURL)
	if err != nil {
		logger.Errorf("invalid consul.url %q: %v", consulURL, err)
		panic(err)
	}

	conf := api.DefaultConfig()
	conf.Address = u.Host
	conf.Scheme = u.Scheme
	conf.Token = consulToken

	consulClient, err := api.NewClient(conf)

	if err != nil {
		logger.Errorf("Couldn't create Consul client: %v", err)
		panic(err)
	}
	consulService := config.NewConsulService(consulClient, namespace)
	healthService, err := health.NewHealthService()

	if err != nil {
		logger.Error("Couldn't create healthService")
		panic(err)
	}
	healthService.AddCheck("ConsulCheck", func() health.Status {
		_, err := consulClient.Status().Leader()
		if err != nil {
			return health.Status{Name: health.StatusProblem, Details: map[string]interface{}{
				"error": err.Error(),
			}}
		}
		return health.Status{Name: health.StatusUp}
	})

	err = config.InitializeDefaultProperties(consulService, ctx)
	if err != nil {
		logger.Errorf("Couldn't initialize default properties because: %s", err.Error())
		panic(err)
	}

	app, err := fiberserver.New(fiber.Config{Network: fiber.NetworkTCP}).
		WithHealth("/health", healthService).
		WithPrometheus("/prometheus").
		WithTracer(tracing.NewZipkinTracer()).
		WithApiVersion().
		ProcessWithContext(ctx)

	if err != nil {
		logger.Errorf("Error while create app because: %s", err.Error())
		panic(err)
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

	configController := config.NewConfigPropertiesController(consulService)
	// swagger
	app.Get("/swagger-ui/swagger.json", func(ctx *fiber.Ctx) error {
		ctx.Set("Content-Type", "application/json")
		return ctx.Status(http.StatusOK).SendString(docs.SwaggerInfo.ReadDoc())
	})
	app.Get("/applications", configController.GetApplicationsAndProfiles)
	app.Get("/:label/:nameAndProfiles.json", configController.FindOneJSON)
	app.Get("/:label/:nameAndProfiles.properties", configController.FindOneProperties)
	app.Get("/:label/:nameAndProfiles.yaml", configController.FindOneYaml)
	app.Get("/:label/:nameAndProfiles.yml", configController.FindOneYaml)

	app.Get("/:nameAndProfiles.json", configController.FindOneJSON)
	app.Get("/:nameAndProfiles.properties", configController.FindOneProperties)
	app.Get("/:nameAndProfiles.yaml", configController.FindOneYaml)
	app.Get("/:nameAndProfiles.yml", configController.FindOneYaml)

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
