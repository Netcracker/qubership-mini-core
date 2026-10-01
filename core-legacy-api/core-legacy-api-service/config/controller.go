package config

import (
	"context"
	"net/http"
	"strings"

	"github.com/Netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/model"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"

	"github.com/gofiber/fiber/v2"
)

type ConfigService interface {
	FindAll(ctx context.Context) ([]model.ConfigProfile, error)
	FindByApplicationAndProfile(ctx context.Context, serviceName string, profile string) (model.ConfigProfile, error)
	AddProperties(ctx context.Context, application string, profile string, properties map[string]string) error
	DeleteProfile(ctx context.Context, application string, profile string) error
	DeleteProperties(ctx context.Context, application string, profile string, propertiesToDelete []string) error
}

type ConfigController struct {
	sv     ConfigService
	logger logging.Logger
}

func NewConfigPropertiesController(configService ConfigService) *ConfigController {
	return &ConfigController{configService, logging.GetLogger("ConfigController")}
}

// GetApplicationsAndProfiles godoc
// @Summary Get applications list
// @Description Returns list of configured applications with their profiles
// @Tags Config Properties
// @Produce application/json
// @Success 200 {array} model.ApplicationWithProfiles "Returned successfully list of configured applications with their profiles"
// @Failure 500 {object} map[string]string
// @Router /applications [get]
func (ctrl *ConfigController) GetApplicationsAndProfiles(c *fiber.Ctx) error {
	ctx := c.UserContext()
	ctrl.logger.InfoC(ctx, "Getting applications")

	configProfiles, err := ctrl.sv.FindAll(ctx)
	if err != nil {
		ctrl.logger.ErrorC(ctx, "Failed to get applications: %s", err.Error())
		return RespondWithError(c, http.StatusInternalServerError, "Internal Server Error")
	}

	result := toApplicationResponses(groupProfilesByApplication(configProfiles))
	ctrl.logger.DebugC(ctx, "Found %d applications", len(result))

	return ResponseOk(c, result)
}

// FindOne godoc
// @Tags Config Properties
// @Param name path string true "name"
// @Param profiles path string true "profiles"
// @Param label path string true "label"
// @Produce application/json
// @Success 200 {object} model.Environment
// @Failure 500 {object} map[string]string
// @Router /{name}/{profiles} [get]
// @Router /{name}/{profiles}/{label} [get]
func (ctrl *ConfigController) FindOne(c *fiber.Ctx) error {
	ctx := c.UserContext()

	application := GetFiberParam(c, "application")
	activeProfiles := strings.Split(GetFiberParam(c, "profile"), ",")
	label := optional(GetFiberParam(c, "label"), "")

	ctrl.logger.InfoC(ctx, "Finding config for application=%s, profiles=%s", application, activeProfiles)

	properties, err := ctrl.loadMergedProperties(ctx, application, activeProfiles)
	if err != nil {
		ctrl.logger.ErrorC(ctx, "Failed to load merged properties for application=%s: %s", application, err.Error())
		return RespondWithError(c, http.StatusInternalServerError, "Internal Server Error")
	}

	environment := buildEnvironment(
		application,
		[]string{GetFiberParam(c, "profile")},
		label,
		properties,
	)
	return ResponseOk(c, environment)
}

// FindOneJSON godoc
// @Tags Config Properties
// @Param name path string true "name"
// @Param profiles path string true "profiles"
// @Param label path string true "label"
// @Param resolvePlaceholders query boolean false " " default(true)
// @Produce text/plain
// @Success 200 {string} string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /{name}-{profiles}.json [get]
// @Router /{label}/{name}-{profiles}.json [get]
func (ctrl *ConfigController) FindOneJSON(c *fiber.Ctx) error {
	ctx := c.UserContext()
	nameAndProfiles := GetFiberParam(c, "nameAndProfiles")
	application, profiles, err := splitApplicationAndProfiles(nameAndProfiles)
	if err != nil {
		return RespondWithError(c, http.StatusNotFound, "Not Found")
	}
	activeProfiles := strings.Split(profiles, ",")
	resolvePlaceholders := c.QueryBool("resolvePlaceholders", true)

	ctrl.logger.InfoC(ctx, "Finding JSON config for application=%s, profiles=%s", application, activeProfiles)

	properties, err := ctrl.loadMergedProperties(ctx, application, activeProfiles)
	if err != nil {
		ctrl.logger.ErrorC(ctx, "Failed to load merged properties for application=%s: %s", application, err.Error())

		return RespondWithError(c, http.StatusInternalServerError, "Internal Server Error")
	}

	if resolvePlaceholders {
		var err error
		err = resolveProperties(properties)
		if err != nil {
			return RespondWithError(c, http.StatusBadRequest, "Bad Request")
		}
	}
	result := buildNestedProperties(properties)
	return ResponseOk(c, result)
}

// FindOneProperties godoc
// @Tags Config Properties
// @Param name path string true "application"
// @Param profiles path string true "profiles"
// @Param label path string true "label"
// @Param resolvePlaceholders query boolean false " " default(true)
// @Produce text/plain
// @Success 200 {string} string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /{name}-{profiles}.properties [get]
// @Router /{label}/{name}-{profiles}.properties [get]
func (ctrl *ConfigController) FindOneProperties(c *fiber.Ctx) error {
	ctx := c.UserContext()
	nameAndProfiles := GetFiberParam(c, "nameAndProfiles")
	application, profiles, err := splitApplicationAndProfiles(nameAndProfiles)
	if err != nil {
		return RespondWithError(c, http.StatusNotFound, "Not Found")
	}
	activeProfiles := strings.Split(profiles, ",")
	resolvePlaceholders := c.QueryBool("resolvePlaceholders", true)

	ctrl.logger.InfoC(ctx, "Finding properties-format config for application=%s, profiles=%s", application, activeProfiles)

	properties, err := ctrl.loadMergedProperties(ctx, application, activeProfiles)
	if err != nil {
		ctrl.logger.ErrorC(ctx, "Failed to load merged properties for application=%s: %s", application, err.Error())
		return RespondWithError(c, http.StatusInternalServerError, "Internal Server Error")
	}

	if resolvePlaceholders {
		var err error
		err = resolveProperties(properties)
		if err != nil {
			return RespondWithError(c, http.StatusBadRequest, "Bad Request")
		}
	}
	c.Set("Content-Type", "text/plain")

	return RespondWithProperties(c, http.StatusOK, buildPropertiesText(properties))
}

// FindOneYaml godoc
// @Description Returns YAML formatted configuration for application and profiles
// @Tags Config Properties
// @Param name path string true "application"
// @Param profiles path string true "profiles"
// @Param label path string true "label"
// @Param resolvePlaceholders query boolean false " " default(true)
// @Produce text/plain
// @Success 200 {string} string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /{name}-{profiles}.yml [get]
// @Router /{label}/{name}-{profiles}.yml [get]
// @Router /{name}-{profiles}.yaml [get]
// @Router /{label}/{name}-{profiles}.yaml [get]
func (ctrl *ConfigController) FindOneYaml(c *fiber.Ctx) error {
	ctx := c.UserContext()

	nameAndProfiles := GetFiberParam(c, "nameAndProfiles")
	application, profiles, err := splitApplicationAndProfiles(nameAndProfiles)
	if err != nil {
		return RespondWithError(c, http.StatusNotFound, "Not Found")
	}
	activeProfiles := strings.Split(profiles, ",")
	resolvePlaceholders := c.QueryBool("resolvePlaceholders", true)

	ctrl.logger.InfoC(ctx, "Finding YAML config for application=%s, profiles=%s", application, activeProfiles)

	properties, err := ctrl.loadMergedProperties(ctx, application, activeProfiles)
	if err != nil {
		ctrl.logger.ErrorC(ctx, "Failed to load merged properties for application=%s: %s", application, err.Error())
		return RespondWithError(c, http.StatusInternalServerError, "Internal Server Error")
	}

	if resolvePlaceholders {
		var err error
		err = resolveProperties(properties)
		if err != nil {
			return RespondWithError(c, http.StatusBadRequest, "Bad Request")
		}
	}

	nested := buildNestedProperties(properties)

	yamlBytes, err := marshalWithSingleQuotes(nested)
	if err != nil {
		return RespondWithError(c, http.StatusInternalServerError, "Internal Server Error")
	}
	return RespondWithBytes(c, http.StatusOK, yamlBytes)
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
		ctrl.logger.ErrorC(ctx, "Failed to find global default profile: %s", err.Error())
		return nil, err
	}

	mergeConfigProperties(merged, globalProfile.GetPropertiesAsMap())

	for _, profile := range activeProfiles {
		configProfile, err := ctrl.sv.FindByApplicationAndProfile(ctx, application, profile)
		if err != nil {
			ctrl.logger.ErrorC(ctx, "Failed to find application=%s profile=%s: %s", application, profile, err.Error())
			return nil, err
		}
		mergeConfigProperties(merged, configProfile.GetPropertiesAsMap())
	}
	return merged, nil
}

// AddProperties godoc
// @Summary Add new property
// @Description Adds new property for defined application and profile. If property is already existed then replace it with new one.
// @Tags Config Properties
// @Param application path string true "application"
// @Param profile path string true "profile"
// @Accept application/json
// @Param properties body object true "JSON mapping key string to value string"
// @Success 201 {string} string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /{application}/{profile} [post]
// @Router /{application}/{profile} [put]
func (ctrl *ConfigController) AddProperties(c *fiber.Ctx) error {
	ctx := c.UserContext()

	application := GetFiberParam(c, "application")
	profile := GetFiberParam(c, "profile")

	newProperties := make(map[string]string)
	ctrl.logger.InfoC(ctx, "Adding properties for application=%s, profile=%s", application, profile)
	err := c.BodyParser(&newProperties)
	if err != nil {
		ctrl.logger.ErrorC(ctx, "Failed to parse request body: %s", err.Error())
		return RespondWithError(c, http.StatusBadRequest, "Bad Request")
	}
	err = ctrl.sv.AddProperties(ctx, application, profile, newProperties)
	if err != nil {
		ctrl.logger.ErrorC(ctx, "Failed to add properties for application=%s, profile=%s: %s", application, profile, err.Error())
		return RespondWithError(c, http.StatusInternalServerError, "Internal Server Error")
	}
	ctrl.logger.InfoC(ctx, "Added %d properties for application=%s, profile=%s", len(newProperties), application, profile)
	return ResponseCreated(c)

}

// DeleteProperties godoc
// @Summary Delete properties
// @Description Deletes properties for defined application and profile. If properties was not specified would delete all of them.
// @Tags Config Properties
// @Param application path string true "application"
// @Param profile path string true "profile"
// @Accept application/json
// @Param properties body []string false "JSON list with names of properties"
// @Success 200 "Properties successfully deleted"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /{application}/{profile}/properties-delete [post]
func (ctrl *ConfigController) DeleteProperties(c *fiber.Ctx) error {
	ctx := c.UserContext()

	application := GetFiberParam(c, "application")
	profile := GetFiberParam(c, "profile")
	var properties []string
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&properties); err != nil {
			ctrl.logger.ErrorC(ctx, "Failed to parse request body: %s", err.Error())
			return RespondWithError(c, http.StatusBadRequest, "Bad Request")
		}
	}
	var err error
	if properties != nil {
		ctrl.logger.InfoC(ctx, "Deleting %d properties for application=%s, profile=%s", len(properties), application, profile)
		err = ctrl.sv.DeleteProperties(ctx, application, profile, properties)

	} else {
		ctrl.logger.InfoC(ctx, "Deleting entire profile for application=%s, profile=%s", application, profile)
		err = ctrl.sv.DeleteProfile(ctx, application, profile)

	}
	if err != nil {
		ctrl.logger.ErrorC(ctx, "Failed to delete properties/profile for application=%s, profile=%s: %s", application, profile, err.Error())
		return RespondWithError(c, http.StatusInternalServerError, "Internal Server Error")
	}
	ctrl.logger.InfoC(ctx, "Delete succeeded for application=%s, profile=%s", application, profile)
	return ResponseOk(c, nil)
}
