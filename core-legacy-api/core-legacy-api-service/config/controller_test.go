package config

import (
	config "core-legacy-api/config/mock-service"
	"core-legacy-api/model"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"go.uber.org/mock/gomock"
)

func newTestController(t *testing.T) (*ConfigController, *config.MockConfigService, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	mockService := config.NewMockConfigService(ctrl)
	controller := NewConfigPropertiesController(mockService)
	return controller, mockService, ctrl
}

func TestGetApplicationsAndProfiles_OK(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	profiles := []model.ConfigProfile{
		{
			Application: "app1",
			Profile:     "default",
		},
	}
	mockService.EXPECT().
		FindAll(gomock.Any()).
		Return(profiles, nil)

	expected := []model.ApplicationWithProfiles{
		{
			Name:     "app1",
			Profiles: []string{"default"},
		},
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	err = controller.GetApplicationsAndProfiles(ctx)

	if err != nil {
		t.Errorf("GetApplicationsAndProfiles() has got error = %v", err)
	}
	response := ctx.Response()
	assert.Equal(t, http.StatusOK, response.StatusCode())
	assert.Equal(t, response.Body(), expectedJSON)

}

func TestFindOne_OK(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	globalProfile := model.ConfigProfile{
		Application: consulGlobalApplicationName,
		Profile:     defaultProfileName,
		Properties: []model.ConfigProperty{
			{
				Key:   "global.key",
				Value: "global-value",
			},
		},
	}

	appProfile := model.ConfigProfile{
		Application: "test-app",
		Profile:     "default",
		Properties: []model.ConfigProperty{
			{
				Key:   "app.key",
				Value: "app-value",
			},
		},
	}

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), consulGlobalApplicationName, defaultProfileName).
		Return(globalProfile, nil)

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), "test-app", "default").
		Return(appProfile, nil)

	app := fiber.New()
	app.Get("/:application/:profile", controller.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/test-app/default", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var env model.Environment
	err = json.NewDecoder(resp.Body).Decode(&env)
	assert.NoError(t, err)

	assert.Equal(t, "test-app", env.Name)
	assert.Equal(t, []string{"default"}, env.Profiles)
	assert.Len(t, env.PropertySources, 1)
}

func TestFindOneJSON_OK(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	globalProfile := model.ConfigProfile{
		Application: consulGlobalApplicationName,
		Profile:     defaultProfileName,
		Properties: []model.ConfigProperty{
			{
				Key:   "global.key",
				Value: "global-value",
			},
		},
	}

	appProfile := model.ConfigProfile{
		Application: "test-app",
		Profile:     "default",
		Properties: []model.ConfigProperty{
			{
				Key:   "app.key",
				Value: "app-value",
			},
		},
	}

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), consulGlobalApplicationName, defaultProfileName).
		Return(globalProfile, nil)

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), "test-app", "default").
		Return(appProfile, nil)

	app := fiber.New()
	app.Get("/:name/:profiles", controller.FindOneJSON)

	req := httptest.NewRequest(http.MethodGet, "/test-app/default", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err)

	expected := map[string]any{
		"global": map[string]any{
			"key": "global-value",
		},
		"app": map[string]any{
			"key": "app-value",
		},
	}

	assert.Equal(t, expected, result)
}

func TestFindOneYaml_OK(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	globalProfile := model.ConfigProfile{
		Application: consulGlobalApplicationName,
		Profile:     defaultProfileName,
		Properties: []model.ConfigProperty{
			{
				Key:   "global.key",
				Value: "global-value",
			},
		},
	}

	appProfile := model.ConfigProfile{
		Application: "test-app",
		Profile:     "default",
		Properties: []model.ConfigProperty{
			{
				Key:   "app.key",
				Value: "app-value",
			},
		},
	}

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), consulGlobalApplicationName, defaultProfileName).
		Return(globalProfile, nil)

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), "test-app", "default").
		Return(appProfile, nil)

	app := fiber.New()
	app.Get("/:name/:profiles", controller.FindOneYaml)

	req := httptest.NewRequest(http.MethodGet, "/test-app/default", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	expected := `app:
    key: app-value
global:
    key: global-value
`
	assert.Equal(t, expected, string(body))
}

func TestAddProperties_OK(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	properties := map[string]string{
		"my.key1": "value1",
		"my.key2": "value2",
	}

	mockService.EXPECT().
		AddProperties(gomock.Any(), "test-app", "default", properties).
		Return(nil)

	app := fiber.New()
	app.Post("/:application/:profile", controller.AddProperties)

	body := `{
		"my.key1":"value1",
		"my.key2":"value2"
	}`

	req := httptest.NewRequest(http.MethodPost, "/test-app/default", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestAddProperties_BadRequest(t *testing.T) {
	controller, _, ctrl := newTestController(t)
	defer ctrl.Finish()

	app := fiber.New()
	app.Post("/:application/:profile", controller.AddProperties)

	req := httptest.NewRequest(
		http.MethodPost,
		"/test-app/default",
		strings.NewReader("{"),
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
func TestDeleteProperties_OK(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	properties := []string{
		"my.key1",
		"my.key2",
	}

	mockService.EXPECT().
		DeleteProperties(gomock.Any(), "test-app", "default", properties).
		Return(nil)

	app := fiber.New()
	app.Delete("/:application/:profile", controller.DeleteProperties)

	body := `[
		"my.key1",
		"my.key2"
	]`

	req := httptest.NewRequest(
		http.MethodDelete,
		"/test-app/default",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
func TestDeleteProfile_OK(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	mockService.EXPECT().
		DeleteProfile(gomock.Any(), "test-app", "default").
		Return(nil)

	app := fiber.New()
	app.Delete("/:application/:profile", controller.DeleteProperties)

	req := httptest.NewRequest(
		http.MethodDelete,
		"/test-app/default",
		nil,
	)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDeleteProperties_BadRequest(t *testing.T) {
	controller, _, ctrl := newTestController(t)
	defer ctrl.Finish()

	app := fiber.New()
	app.Delete("/:application/:profile", controller.DeleteProperties)

	req := httptest.NewRequest(
		http.MethodDelete,
		"/test-app/default",
		strings.NewReader("{"),
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid request body", string(body))
}

func TestGetApplicationsAndProfiles_MultipleApplications(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	profiles := []model.ConfigProfile{
		{
			Application: "app1",
			Profile:     "dev",
		},
		{
			Application: "app1",
			Profile:     "prod",
		},
		{
			Application: "app2",
			Profile:     "staging",
		},
	}
	mockService.EXPECT().
		FindAll(gomock.Any()).
		Return(profiles, nil)

	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	err := controller.GetApplicationsAndProfiles(ctx)

	assert.NoError(t, err)
	response := ctx.Response()
	assert.Equal(t, http.StatusOK, response.StatusCode())

	var result []model.ApplicationWithProfiles
	json.Unmarshal(response.Body(), &result)
	assert.GreaterOrEqual(t, len(result), 2)
}

func TestFindOne_WithMultipleProfiles(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	globalProfile := model.ConfigProfile{
		Application: consulGlobalApplicationName,
		Profile:     defaultProfileName,
		Properties: []model.ConfigProperty{
			{
				Key:   "global.key",
				Value: "global-value",
			},
		},
	}

	appProfile := model.ConfigProfile{
		Application: "test-app",
		Profile:     "dev",
		Properties: []model.ConfigProperty{
			{
				Key:   "app.key",
				Value: "app-value-dev",
			},
		},
	}

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), consulGlobalApplicationName, defaultProfileName).
		Return(globalProfile, nil)

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), "test-app", "dev").
		Return(appProfile, nil)

	app := fiber.New()
	app.Get("/:application/:profile", controller.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/test-app/dev", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var env model.Environment
	json.NewDecoder(resp.Body).Decode(&env)
	assert.Equal(t, "test-app", env.Name)
	assert.Equal(t, []string{"dev"}, env.Profiles)
}

func TestFindOne_WithoutLabel(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	globalProfile := model.ConfigProfile{
		Application: consulGlobalApplicationName,
		Profile:     defaultProfileName,
		Properties:  []model.ConfigProperty{},
	}

	appProfile := model.ConfigProfile{
		Application: "test-app",
		Profile:     "default",
		Properties:  []model.ConfigProperty{},
	}

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), consulGlobalApplicationName, defaultProfileName).
		Return(globalProfile, nil)

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), "test-app", "default").
		Return(appProfile, nil)

	app := fiber.New()
	app.Get("/:application/:profile", controller.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/test-app/default", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var env model.Environment
	json.NewDecoder(resp.Body).Decode(&env)
	// Label should be nil when not provided in URL
	assert.Nil(t, env.Label)
}

func TestFindOneJSON_WithResolvePlaceholders_False(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	globalProfile := model.ConfigProfile{
		Application: consulGlobalApplicationName,
		Profile:     defaultProfileName,
		Properties: []model.ConfigProperty{
			{
				Key:   "db.host",
				Value: "localhost",
			},
		},
	}

	appProfile := model.ConfigProfile{
		Application: "test-app",
		Profile:     "default",
		Properties: []model.ConfigProperty{
			{
				Key:   "connection.url",
				Value: "jdbc:postgresql://${db.host}:5432",
			},
		},
	}

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), consulGlobalApplicationName, defaultProfileName).
		Return(globalProfile, nil)

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), "test-app", "default").
		Return(appProfile, nil)

	app := fiber.New()
	app.Get("/:name/:profiles", controller.FindOneJSON)

	req := httptest.NewRequest(http.MethodGet, "/test-app/default?resolvePlaceholders=false", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	connection := result["connection"].(map[string]interface{})
	// Placeholder should remain unresolved
	assert.Contains(t, connection["url"], "${")
}

func TestFindOneJSON_WithResolvePlaceholders_True(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	globalProfile := model.ConfigProfile{
		Application: consulGlobalApplicationName,
		Profile:     defaultProfileName,
		Properties: []model.ConfigProperty{
			{
				Key:   "db.host",
				Value: "localhost",
			},
		},
	}

	appProfile := model.ConfigProfile{
		Application: "test-app",
		Profile:     "default",
		Properties: []model.ConfigProperty{
			{
				Key:   "connection.url",
				Value: "jdbc:postgresql://${db.host}:5432",
			},
		},
	}

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), consulGlobalApplicationName, defaultProfileName).
		Return(globalProfile, nil)

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), "test-app", "default").
		Return(appProfile, nil)

	app := fiber.New()
	app.Get("/:name/:profiles", controller.FindOneJSON)

	req := httptest.NewRequest(http.MethodGet, "/test-app/default?resolvePlaceholders=true", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	connection := result["connection"].(map[string]interface{})
	// Placeholder should be resolved
	assert.NotContains(t, connection["url"], "${")
	assert.Contains(t, connection["url"], "localhost")
}

func TestFindOneProperties_OK(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	globalProfile := model.ConfigProfile{
		Application: consulGlobalApplicationName,
		Profile:     defaultProfileName,
		Properties: []model.ConfigProperty{
			{
				Key:   "global.key",
				Value: "global-value",
			},
		},
	}

	appProfile := model.ConfigProfile{
		Application: "test-app",
		Profile:     "default",
		Properties: []model.ConfigProperty{
			{
				Key:   "app.key",
				Value: "app-value",
			},
		},
	}

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), consulGlobalApplicationName, defaultProfileName).
		Return(globalProfile, nil)

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), "test-app", "default").
		Return(appProfile, nil)

	app := fiber.New()
	app.Get("/:name/:profiles", controller.FindOneProperties)

	req := httptest.NewRequest(http.MethodGet, "/test-app/default", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(body), "global.key: global-value")
	assert.Contains(t, string(body), "app.key: app-value")
}

func TestFindOneProperties_WithResolvePlaceholders(t *testing.T) {
	controller, mockService, ctrl := newTestController(t)
	defer ctrl.Finish()

	globalProfile := model.ConfigProfile{
		Application: consulGlobalApplicationName,
		Profile:     defaultProfileName,
		Properties: []model.ConfigProperty{
			{
				Key:   "db.host",
				Value: "localhost",
			},
		},
	}

	appProfile := model.ConfigProfile{
		Application: "test-app",
		Profile:     "default",
		Properties: []model.ConfigProperty{
			{
				Key:   "connection.url",
				Value: "jdbc:postgresql://${db.host}:5432",
			},
		},
	}

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), consulGlobalApplicationName, defaultProfileName).
		Return(globalProfile, nil)

	mockService.EXPECT().
		FindByApplicationAndProfile(gomock.Any(), "test-app", "default").
		Return(appProfile, nil)

	app := fiber.New()
	app.Get("/:name/:profiles", controller.FindOneProperties)

	req := httptest.NewRequest(http.MethodGet, "/test-app/default?resolvePlaceholders=true", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	content := string(body)
	assert.Contains(t, content, "connection.url:")
	assert.NotContains(t, content, "${")
}
