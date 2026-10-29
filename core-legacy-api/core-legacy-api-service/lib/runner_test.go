package lib

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-lib-go-actuator-common/v2/health"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newProbeTestApp(t *testing.T) *fiber.App {
	healthService, err := health.NewHealthService()
	require.NoError(t, err)
	readinessService, err := health.NewHealthService()
	require.NoError(t, err)
	readinessService.AddCheck("ConsulCheck", func() health.Status {
		return health.Status{Name: health.StatusProblem}
	})

	app := fiber.New()
	registerProbeEndpoints(app, healthService, readinessService)
	app.Get("/:application/:profile", func(c *fiber.Ctx) error {
		return c.JSON(map[string]interface{}{"name": c.Params("application")})
	})
	return app
}

func getProbe(t *testing.T, app *fiber.App, path string) (int, map[string]interface{}) {
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &payload), "GET %s returned %s", path, body)
	return resp.StatusCode, payload
}

func TestOpenAPISpecEndpointsAreNotCaughtByPropertyRoutes(t *testing.T) {
	app := fiber.New()
	registerOpenAPISpecEndpoints(app)
	app.Get("/:label/:nameAndProfiles.json", func(c *fiber.Ctx) error {
		return c.Status(http.StatusNotFound).JSON(map[string]string{"error": "caught"})
	})
	app.Get("/:application/:profile", func(c *fiber.Ctx) error {
		return c.Status(http.StatusNotFound).JSON(map[string]string{"error": "caught"})
	})

	for _, path := range []string{"/v3/api-docs", "/swagger-ui/doc.json", "/doc", "/swagger-ui/swagger.json"} {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode, path)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"swagger"`, path)
	}
}

func TestProbeEndpoints(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		wantCode   int
		wantStatus string
	}{
		{"liveness is up", "/health", http.StatusOK, "UP"},
		{"Actuator liveness path answers as liveness", "/health/liveness", http.StatusOK, "UP"},
		{"readiness reports the failed Consul check", "/readiness", http.StatusServiceUnavailable, "PROBLEM"},
		{"Actuator readiness path answers as readiness", "/health/readiness", http.StatusServiceUnavailable, "PROBLEM"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, payload := getProbe(t, newProbeTestApp(t), tc.path)
			assert.Equal(t, tc.wantCode, code, "GET %s", tc.path)
			assert.Equal(t, tc.wantStatus, payload["status"], "GET %s", tc.path)
		})
	}
}
