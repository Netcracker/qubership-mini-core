package tenantmanager

const activeStatus = "ACTIVE"

// tenant is one record of the tenants ConfigMap (tenants.json).
type tenant struct {
	ExternalID *string `json:"externalId"`
	Name       string  `json:"name"`
	DomainName *string `json:"domainName"`
	Namespace  *string `json:"namespace"`
}

// tenantResponse is the subset of the tenant-manager presentation served here.
type tenantResponse struct {
	ExternalID *string `json:"externalId,omitempty"`
	Name       string  `json:"name"`
	DomainName *string `json:"domainName,omitempty"`
	Namespace  *string `json:"namespace,omitempty"`
	Status     string  `json:"status"`
}
