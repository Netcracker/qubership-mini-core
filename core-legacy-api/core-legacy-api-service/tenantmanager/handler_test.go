package tenantmanager

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/config"
	"github.com/stretchr/testify/require"
)

const testTenants = `[
  {"externalId":"11111111-1111-1111-1111-111111111111","name":"default","domainName":null,"namespace":"ns-1"},
  {"externalId":"22222222-2222-2222-2222-222222222222","name":"acme","domainName":"acme.example.com","namespace":"ns-2"}
]`

const testPendingTenant = `[{"externalId":null,"name":"pending","domainName":null,"namespace":"ns-3"}]`

func newApp(t *testing.T, path string) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: config.FiberErrorHandler})
	h := NewHandler(path)
	app.Get("/api/v4/tenant-manager/manage/tenants", h.List)
	app.Get("/api/v4/tenant-manager/manage/tenants/:id", h.Get)
	app.Get("/api/v4/tenant-manager/registration/tenants", h.FindByDns)
	return app
}

func get(t *testing.T, app *fiber.App, url string) (int, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, url, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

func TestList(t *testing.T) {
	app := newApp(t, writeTenants(t, testTenants))

	status, body := get(t, app, "/api/v4/tenant-manager/manage/tenants")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, `"externalId":"11111111-1111-1111-1111-111111111111"`)
	require.Contains(t, body, `"status":"ACTIVE"`)
	require.Contains(t, body, `"namespace":"ns-2"`)

	status, body = get(t, app, "/api/v4/tenant-manager/manage/tenants?search=status=ACTIVE")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, `"name":"acme"`)

	status, body = get(t, app, "/api/v4/tenant-manager/manage/tenants?search=status=SUSPENDED")
	require.Equal(t, http.StatusOK, status)
	require.JSONEq(t, `[]`, body)
}

func TestGet(t *testing.T) {
	app := newApp(t, writeTenants(t, testTenants))

	status, body := get(t, app, "/api/v4/tenant-manager/manage/tenants/22222222-2222-2222-2222-222222222222")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, `"name":"acme"`)

	status, _ = get(t, app, "/api/v4/tenant-manager/manage/tenants/33333333-3333-3333-3333-333333333333")
	require.Equal(t, http.StatusNotFound, status)

	status, _ = get(t, app, "/api/v4/tenant-manager/manage/tenants/not-a-uuid")
	require.Equal(t, http.StatusBadRequest, status)
}

func TestFindByDns(t *testing.T) {
	app := newApp(t, writeTenants(t, testTenants))

	for _, q := range []string{"default", "DEFAULT", "acme", "ACME", "acme.example.com"} {
		status, body := get(t, app, "/api/v4/tenant-manager/registration/tenants?dns="+q)
		require.Equal(t, http.StatusOK, status, q)
		require.NotEmpty(t, body, q)
	}

	status, _ := get(t, app, "/api/v4/tenant-manager/registration/tenants?dns=absent")
	require.Equal(t, http.StatusNotFound, status)
}

func TestListOmitsNullExternalID(t *testing.T) {
	app := newApp(t, writeTenants(t, testPendingTenant))

	status, body := get(t, app, "/api/v4/tenant-manager/manage/tenants")
	require.Equal(t, http.StatusOK, status)
	require.NotContains(t, body, `"externalId"`)
	require.Contains(t, body, `"name":"pending"`)
}

func TestFindByDnsNullExternalID(t *testing.T) {
	app := newApp(t, writeTenants(t, testPendingTenant))

	status, _ := get(t, app, "/api/v4/tenant-manager/registration/tenants?dns=pending")
	require.Equal(t, http.StatusNotFound, status)
}

func TestConfigMapMissing(t *testing.T) {
	app := newApp(t, t.TempDir()+"/absent.json")

	status, _ := get(t, app, "/api/v4/tenant-manager/manage/tenants")
	require.Equal(t, http.StatusInternalServerError, status)
}
