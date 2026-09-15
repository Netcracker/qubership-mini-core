package config

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
)

func RespondWithError(c *fiber.Ctx, code int, msg string) error {
	return RespondWithJson(c, code, map[string]string{"error": msg})
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
	return c.SendStatus(http.StatusOK)
}

func ResponseCreated(c *fiber.Ctx) error {
	return c.SendStatus(http.StatusCreated)
}
