package config

import (
	"context"
	"core-legacy-api/model"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
)

func init() {
	logger = logging.GetLogger("ConfigController")
}

type ConfigService interface {
	FindAll(ctx context.Context) ([]model.ConfigProfile, error)
	FindByApplicationAndProfile(ctx context.Context, serviceName string, profile string) (model.ConfigProfile, error)
	AddProperties(ctx context.Context, application string, profile string, properties map[string]string) error
	DeleteProfile(ctx context.Context, application string, profile string) error
	DeleteProperties(ctx context.Context, application string, profile string, propertiesToDelete []string) error
}

type ConfigController struct {
	sv ConfigService
}

func NewConfigPropertiesController(configService ConfigService) *ConfigController {
	return &ConfigController{configService}
}

func (ctrl *ConfigController) GetApplicationsAndProfiles(c *fiber.Ctx) error {
	ctx := c.UserContext()
	logger.InfoC(ctx, "Getting applications")

	configProfiles, err := ctrl.sv.FindAll(ctx)
	if err != nil {
		logger.ErrorC(ctx, "Failed to get applications: %s", err.Error())
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	result := toApplicationResponses(groupProfilesByApplication(configProfiles))
	logger.DebugC(ctx, "Found %d applications", len(result))

	return c.Status(fiber.StatusOK).JSON(result)
}

func (ctrl *ConfigController) FindOne(c *fiber.Ctx) error {
	ctx := c.UserContext()

	application := c.Params("application")
	activeProfiles := strings.Split(c.Params("profile"), ",")
	label := optional(c.Params("label"), "")

	logger.InfoC(ctx, "Finding config for application=%s, profiles=%s", application, activeProfiles)

	properties, err := ctrl.loadMergedProperties(ctx, application, activeProfiles)
	if err != nil {
		logger.ErrorC(ctx, "Failed to load merged properties for application=%s: %s", application, err.Error())
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	environment := buildEnvironment(
		application,
		[]string{c.Params("profile")},
		label,
		properties,
	)
	return c.Status(fiber.StatusOK).JSON(environment)
}

func (ctrl *ConfigController) FindOneJSON(c *fiber.Ctx) error {
	ctx := c.UserContext()

	application := c.Params("name")
	activeProfiles := strings.Split(c.Params("profiles"), ",")
	resolvePlaceholders := c.QueryBool("resolvePlaceholders", true)

	logger.InfoC(ctx, "Finding JSON config for application=%s, profiles=%s", application, activeProfiles)

	properties, err := ctrl.loadMergedProperties(ctx, application, activeProfiles)
	if err != nil {
		logger.ErrorC(ctx, "Failed to load merged properties for application=%s: %s", application, err.Error())
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	if resolvePlaceholders {
		var err error
		err = resolveProperties(properties)
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}
	}
	result := buildNestedProperties(properties)
	return c.Status(fiber.StatusOK).JSON(result)
}

func (ctrl *ConfigController) FindOneProperties(c *fiber.Ctx) error {
	ctx := c.UserContext()

	application := c.Params("name")
	activeProfiles := strings.Split(c.Params("profiles"), ",")
	resolvePlaceholders := c.QueryBool("resolvePlaceholders", true)

	logger.InfoC(ctx, "Finding properties-format config for application=%s, profiles=%s", application, activeProfiles)

	properties, err := ctrl.loadMergedProperties(ctx, application, activeProfiles)
	if err != nil {
		logger.ErrorC(ctx, "Failed to load merged properties for application=%s: %s", application, err.Error())
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	if resolvePlaceholders {
		var err error
		err = resolveProperties(properties)
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}
	}
	c.Set("Content-Type", "text/plain")

	return c.Status(fiber.StatusOK).SendString(buildPropertiesText(properties))
}

func (ctrl *ConfigController) FindOneYaml(c *fiber.Ctx) error {
	ctx := c.UserContext()

	application := c.Params("name") // TODO: copy?
	activeProfiles := strings.Split(c.Params("profiles"), ",")
	resolvePlaceholders := c.QueryBool("resolvePlaceholders", true)

	logger.InfoC(ctx, "Finding YAML config for application=%s, profiles=%s", application, activeProfiles)

	properties, err := ctrl.loadMergedProperties(ctx, application, activeProfiles)
	if err != nil {
		logger.ErrorC(ctx, "Failed to load merged properties for application=%s: %s", application, err.Error())
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	if resolvePlaceholders {
		var err error
		err = resolveProperties(properties)
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}
	}

	nested := buildNestedProperties(properties)

	yamlBytes, err := marshalWithSingleQuotes(nested)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return c.Status(fiber.StatusOK).Send(yamlBytes)
}

// merges properties from global,default profile with properties from specified application,profile
func (ctrl *ConfigController) loadMergedProperties(
	ctx context.Context,
	application string,
	activeProfiles []string,
) (map[string]model.ConfigProperty, error) {

	merged := make(map[string]model.ConfigProperty)

	globalProfile, err := ctrl.sv.FindByApplicationAndProfile(
		ctx,
		consulGlobalApplicationName,
		defaultProfileName,
	)
	if err != nil {
		logger.ErrorC(ctx, "Failed to find global default profile: %s", err.Error())
		return nil, err
	}

	mergeConfigProperties(merged, globalProfile.GetPropertiesAsMap())

	for _, profile := range activeProfiles {
		configProfile, err := ctrl.sv.FindByApplicationAndProfile(ctx, application, profile)
		if err != nil {
			logger.ErrorC(ctx, "Failed to find application=%s profile=%s: %s", application, profile, err.Error())
			return nil, err
		}
		mergeConfigProperties(merged, configProfile.GetPropertiesAsMap())
	}
	return merged, nil
}

func (ctrl *ConfigController) AddProperties(c *fiber.Ctx) error {
	ctx := c.UserContext()

	application := c.Params("application")
	profile := c.Params("profile")

	newProperties := make(map[string]string)
	logger.InfoC(ctx, "Adding properties for application=%s, profile=%s", application, profile)
	err := c.BodyParser(&newProperties)
	if err != nil {
		logger.ErrorC(ctx, "Failed to parse request body: %s", err.Error())
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}
	err = ctrl.sv.AddProperties(ctx, application, profile, newProperties)
	if err != nil {
		logger.ErrorC(ctx, "Failed to add properties for application=%s, profile=%s: %s", application, profile, err.Error())
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	logger.InfoC(ctx, "Added %d properties for application=%s, profile=%s", len(newProperties), application, profile)
	return c.SendStatus(fiber.StatusCreated)

}

func (ctrl *ConfigController) DeleteProperties(c *fiber.Ctx) error {
	ctx := c.UserContext()

	application := c.Params("application")
	profile := c.Params("profile")
	var properties []string
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&properties); err != nil {
			logger.ErrorC(ctx, "Failed to parse request body: %s", err.Error())
			return c.Status(fiber.StatusBadRequest).SendString("Invalid request body")
		}
	}
	var err error
	if properties != nil {
		logger.InfoC(ctx, "Deleting %d properties for application=%s, profile=%s", len(properties), application, profile)
		err = ctrl.sv.DeleteProperties(ctx, application, profile, properties)

	} else {
		logger.InfoC(ctx, "Deleting entire profile for application=%s, profile=%s", application, profile)
		err = ctrl.sv.DeleteProfile(ctx, application, profile)

	}
	if err != nil {
		logger.ErrorC(ctx, "Failed to delete properties/profile for application=%s, profile=%s: %s", application, profile, err.Error())
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	logger.InfoC(ctx, "Delete succeeded for application=%s, profile=%s", application, profile)
	return c.SendStatus(fiber.StatusOK)
}
