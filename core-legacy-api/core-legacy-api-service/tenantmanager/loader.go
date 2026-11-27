package tenantmanager

import (
	"encoding/json"
	"fmt"
	"os"
)

// loadTenants reads the file on every call so kubelet's ConfigMap refresh
// becomes visible without a restart.
func loadTenants(path string) ([]tenant, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tenants config %s: %w", path, err)
	}

	var tenants []tenant
	if err := json.Unmarshal(data, &tenants); err != nil {
		return nil, fmt.Errorf("parse tenants config %s: %w", path, err)
	}
	return tenants, nil
}
