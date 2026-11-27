package tenantmanager

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/config"
)

type Handler struct {
	path   string
	logger logging.Logger
}

func NewHandler(path string) *Handler {
	return &Handler{path: path, logger: logging.GetLogger("TenantManager")}
}

func (h *Handler) List(c *fiber.Ctx) error {
	tenants, err := loadTenants(h.path)
	if err != nil {
		return config.LogError(h.logger, c.UserContext(), "list tenants: %w", err)
	}

	result := make([]tenantResponse, 0, len(tenants))
	if matchesStatus(c.Query("search")) {
		for _, t := range tenants {
			result = append(result, h.toResponse(t))
		}
	}
	return config.ResponseOk(c, result)
}

func (h *Handler) Get(c *fiber.Ctx) error {
	id := config.GetFiberParam(c, "id")
	if _, err := uuid.Parse(id); err != nil {
		return fmt.Errorf("ObjectId = %s is not valid: %w", id, config.ErrBadRequest)
	}

	tenants, err := loadTenants(h.path)
	if err != nil {
		return config.LogError(h.logger, c.UserContext(), "get tenant: %w", err)
	}
	for _, t := range tenants {
		if t.ExternalID != nil && strings.EqualFold(*t.ExternalID, id) {
			return config.ResponseOk(c, h.toResponse(t))
		}
	}
	return fmt.Errorf("Tenant with objectId = %s was not found: %w", id, config.ErrNotFound)
}

func (h *Handler) FindByDns(c *fiber.Ctx) error {
	query := strings.ToLower(c.Query("dns"))
	if query == "" {
		return config.ErrNotFound
	}

	tenants, err := loadTenants(h.path)
	if err != nil {
		return config.LogError(h.logger, c.UserContext(), "find tenant by dns: %w", err)
	}

	// Same precedence as tenant-manager: name first, then domainName.
	for _, t := range tenants {
		if strings.ToLower(t.Name) == query {
			return respondWithExternalID(c, t)
		}
	}
	for _, t := range tenants {
		if t.DomainName != nil && strings.ToLower(*t.DomainName) == query {
			return respondWithExternalID(c, t)
		}
	}
	return config.ErrNotFound
}

func respondWithExternalID(c *fiber.Ctx, t tenant) error {
	if t.ExternalID == nil {
		return config.ErrNotFound
	}
	c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
	return c.Status(fiber.StatusOK).SendString(*t.ExternalID)
}

func (h *Handler) toResponse(t tenant) tenantResponse {
	return tenantResponse{
		ExternalID: t.ExternalID,
		Name:       t.Name,
		DomainName: t.DomainName,
		Namespace:  t.Namespace,
		Status:     activeStatus,
	}
}

// matchesStatus implements the only criterion consumers send,
// search=status=ACTIVE. Anything else is ignored.
func matchesStatus(search string) bool {
	if strings.TrimSpace(search) == "" {
		return true
	}
	for _, criterion := range strings.Split(search, ",") {
		parts := strings.FieldsFunc(criterion, func(r rune) bool { return r == '=' || r == ':' })
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "status") {
			continue
		}
		return strings.EqualFold(strings.TrimSpace(parts[1]), activeStatus)
	}
	return true
}
