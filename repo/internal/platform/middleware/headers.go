package middleware

import "github.com/gofiber/fiber/v2"

// SecurityHeaders adds browser-security response headers to every reply.
// These headers are defense-in-depth for a local-network deployment and
// are mandatory for any static-review checklist.
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Prevent the page from being framed — guards against clickjacking.
		c.Set("X-Frame-Options", "DENY")
		// Force declared MIME types to be respected — prevents MIME sniffing.
		c.Set("X-Content-Type-Options", "nosniff")
		// Disable legacy XSS filter (modern CSP supersedes it; this prevents
		// the filter from inadvertently blocking legitimate content).
		c.Set("X-XSS-Protection", "0")
		// Omit the Referer header on cross-origin navigations.
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Instruct browsers to cache only private (user-specific) responses.
		c.Set("Cache-Control", "no-store")
		return c.Next()
	}
}
