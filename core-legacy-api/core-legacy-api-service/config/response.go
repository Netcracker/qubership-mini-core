package config

import (
	"errors"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

var (
	ErrBadRequest          = errors.New("Bad Request")
	ErrNotFound            = errors.New("Not Found")
	ErrInternalServerError = errors.New("Internal Server Error")
)

type ErrorResponse struct {
	Timestamp string `json:"timestamp"`
	Status    int    `json:"status"`
	Error     string `json:"error"`
	Path      string `json:"path"`
}

func formatHTTPStatusMessage(code int) string {
	if code == 0 {
		return ErrInternalServerError.Error()
	}
	if msg := http.StatusText(code); msg != "" {
		return msg
	}
	return ErrInternalServerError.Error()
}

func FiberErrorHandler(c *fiber.Ctx, err error) error {
	statusCode := http.StatusInternalServerError
	message := ErrInternalServerError.Error()

	switch {
	case errors.Is(err, ErrBadRequest):
		statusCode = http.StatusBadRequest
		message = ErrBadRequest.Error()
	case errors.Is(err, ErrNotFound):
		statusCode = http.StatusNotFound
		message = ErrNotFound.Error()
	default:
		if fiberErr, ok := errors.AsType[*fiber.Error](err); ok {
			statusCode = fiberErr.Code
			message = formatHTTPStatusMessage(statusCode)

		}
	}

	return RespondWithError(c, statusCode, message)
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
	c.Set("Content-Type", "text/plain; charset=utf-8")
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
