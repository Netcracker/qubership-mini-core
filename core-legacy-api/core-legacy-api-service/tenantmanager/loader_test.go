package tenantmanager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTenants(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tenants.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestLoadTenants(t *testing.T) {
	path := writeTenants(t, `[{"externalId":"e1","name":"default","domainName":null,"namespace":"ns-1"}]`)

	tenants, err := loadTenants(path)
	require.NoError(t, err)
	require.Len(t, tenants, 1)
	require.NotNil(t, tenants[0].ExternalID)
	require.Equal(t, "e1", *tenants[0].ExternalID)
	require.Nil(t, tenants[0].DomainName)
	require.NotNil(t, tenants[0].Namespace)
	require.Equal(t, "ns-1", *tenants[0].Namespace)
}

func TestLoadTenantsMissingFile(t *testing.T) {
	_, err := loadTenants(filepath.Join(t.TempDir(), "absent.json"))
	require.Error(t, err)
}

func TestLoadTenantsMalformed(t *testing.T) {
	path := writeTenants(t, `{"externalId":`)
	_, err := loadTenants(path)
	require.Error(t, err)
}
