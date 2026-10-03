package config

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

type ErrorResponse struct {
	Timestamp string `json:"timestamp"`
	Status    int    `json:"status"`
	Error     string `json:"error"`
	Path      string `json:"path"`
}

func RespondWithError(c *fiber.Ctx, code int, msg string) error {
	return RespondWithJson(c, code, ErrorResponse{
		Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Status:    code,
		Error:     msg,
		Path:      c.Path(),
	})
}

func RespondWithJson(c *fiber.Ctx, code int, payload interface{}) error {
	return c.Status(code).JSON(payload)
}
func RespondWithProperties(c *fiber.Ctx, code int, payload string) error {
	return c.Status(code).SendString(payload)
}

func RespondWithBytes(c *fiber.Ctx, code int, data []byte) error {
	return c.Status(code).Send(data)
}

func ResponseOk(c *fiber.Ctx, payload interface{}) error {
	if payload != nil {
		return RespondWithJson(c, http.StatusOK, payload)
	}
	c.Status(http.StatusOK)
	return nil
}

func ResponseCreated(c *fiber.Ctx) error {
	c.Status(http.StatusCreated)
	return nil
}
