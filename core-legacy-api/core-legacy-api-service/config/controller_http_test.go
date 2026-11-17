package config

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestFindOneJSON_ResolvePlaceholdersQuery(t *testing.T) {
	const resolved = "jdbc:postgresql://localhost:5432"
	const unresolved = "jdbc:postgresql://${db.host}:5432"
	tests := []struct {
		name    string
		query   string
		wantURL string
	}{
		{name: "absent", query: "", wantURL: resolved},
		{name: "empty", query: "?resolvePlaceholders=", wantURL: resolved},
		{name: "true", query: "?resolvePlaceholders=true", wantURL: resolved},
		{name: "false", query: "?resolvePlaceholders=false", wantURL: unresolved},
		{name: "0", query: "?resolvePlaceholders=0", wantURL: unresolved},
		{name: "unparsable", query: "?resolvePlaceholders=no", wantURL: resolved},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, mockService, ctrl := newTestController(t)
			defer ctrl.Finish()
			mockService.EXPECT().
				FindByApplicationAndProfile(gomock.Any(), consulGlobalApplicationName, defaultProfileName).
				Return(model.ConfigProfile{Application: consulGlobalApplicationName, Profile: defaultProfileName,
					Properties: []model.ConfigProperty{{Key: "db.host", Value: "localhost"}}}, nil)
			mockService.EXPECT().
				FindByApplicationAndProfile(gomock.Any(), "test-app", "default").
				Return(model.ConfigProfile{Application: "test-app", Profile: "default",
					Properties: []model.ConfigProperty{{Key: "connection.url", Value: unresolved}}}, nil)
			app := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler})
			app.Get("/:nameAndProfiles.json", controller.FindOneJSON)

			status, _, body := sendRequest(t, app, httptest.NewRequest(http.MethodGet, "/test-app-default.json"+tt.query, nil))

			require.Equal(t, http.StatusOK, status, "status, body %s", body)
			var result struct {
				Connection struct {
					URL string `json:"url"`
				} `json:"connection"`
			}
			require.NoError(t, json.Unmarshal([]byte(body), &result), "body %s", body)
			assert.Equal(t, tt.wantURL, result.Connection.URL, "connection.url")
		})
	}
}

func TestAddProperties_RequestContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantParsed  bool
	}{
		{name: "application/json", contentType: "application/json", wantParsed: true},
		{name: "application/json with charset", contentType: "application/json; charset=utf-8", wantParsed: true},
		{name: "text/json", contentType: "text/json", wantParsed: true},
		{name: "vendor +json type", contentType: "application/vnd.netcracker+json", wantParsed: true},
		{name: "no Content-Type", wantParsed: false},
		{name: "text/plain", contentType: "text/plain", wantParsed: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, mockService, ctrl := newTestController(t)
			defer ctrl.Finish()
			if tt.wantParsed {
				mockService.EXPECT().
					AddProperties(gomock.Any(), "test-app", "default", map[string]string{"key": "value"}).
					Return(nil)
			}
			req := httptest.NewRequest(http.MethodPost, "/test-app/default", strings.NewReader(`{"key":"value"}`))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			status, contentType, body := sendRequest(t, newAddPropertiesApp(controller), req)

			if tt.wantParsed {
				assert.Equal(t, http.StatusCreated, status, "status, body %s", body)
				return
			}
			assert.Equal(t, http.StatusBadRequest, status, "status")
			assert.Equal(t, "application/json", contentType, "Content-Type")
			assertErrorResponse(t, body, http.StatusBadRequest, "Bad Request", "/test-app/default")
		})
	}
}

func TestFiberErrorHandler_Response(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler})
	app.Get("/bad-request", func(c *fiber.Ctx) error { return ErrBadRequest })
	app.Get("/not-found", func(c *fiber.Ctx) error { return ErrNotFound })
	app.Get("/failure", func(c *fiber.Ctx) error { return io.ErrUnexpectedEOF })
	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantError  string
		wantPath   string
	}{
		{name: "ErrBadRequest", target: "/bad-request", wantStatus: http.StatusBadRequest, wantError: "Bad Request", wantPath: "/bad-request"},
		{name: "ErrNotFound", target: "/not-found", wantStatus: http.StatusNotFound, wantError: "Not Found", wantPath: "/not-found"},
		{name: "other error", target: "/failure", wantStatus: http.StatusInternalServerError, wantError: "Internal Server Error", wantPath: "/failure"},
		{name: "unknown path", target: "/unknown", wantStatus: http.StatusNotFound, wantError: "Not Found", wantPath: "/unknown"},
		{name: "unknown path with dot-dot segment", target: "/a/../unknown", wantStatus: http.StatusNotFound, wantError: "Not Found", wantPath: "/a/../unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, contentType, body := sendRequest(t, app, httptest.NewRequest(http.MethodGet, tt.target, nil))

			assert.Equal(t, tt.wantStatus, status, "status")
			assert.Equal(t, "application/json", contentType, "Content-Type")
			assertErrorResponse(t, body, tt.wantStatus, tt.wantError, tt.wantPath)
		})
	}
}

func assertErrorResponse(t *testing.T, body string, wantStatus int, wantError, wantPath string) {
	t.Helper()
	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &response), "body %s", body)
	assert.Equal(t, float64(wantStatus), response["status"], "status field, body %s", body)
	assert.Equal(t, wantError, response["error"], "error field, body %s", body)
	assert.Equal(t, wantPath, response["path"], "path field, body %s", body)
	assert.NotEmpty(t, response["timestamp"], "timestamp field, body %s", body)
	assert.Len(t, response, 4, "fields, body %s", body)
}

func sendRequest(t *testing.T, app *fiber.App, req *http.Request) (int, string, string) {
	t.Helper()
	resp, err := app.Test(req, -1)
	require.NoError(t, err, "%s %s", req.Method, req.RequestURI)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "read response body")
	return resp.StatusCode, resp.Header.Get("Content-Type"), string(body)
}
