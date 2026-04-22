package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

// RequestID injects a UUIDv4 request identifier into every request.
// If the client already provides X-Request-ID it is honoured; otherwise one is generated.
// The ID is stored in Fiber's context locals under the key "requestID".
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get(RequestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		c.Locals("requestID", id)
		c.Set(RequestIDHeader, id)
		return c.Next()
	}
}

// GetRequestID retrieves the request ID set by the RequestID middleware.
func GetRequestID(c *fiber.Ctx) string {
	if id, ok := c.Locals("requestID").(string); ok {
		return id
	}
	return ""
}
