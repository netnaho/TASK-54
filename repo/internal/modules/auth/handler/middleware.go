package handler

import (
	"errors"
	"log/slog"

	"careops/clinic/internal/modules/auth/policy"
	"careops/clinic/internal/modules/auth/service"
	"careops/clinic/internal/shared/httpx"
	"github.com/gofiber/fiber/v2"
)

// RequireSession is a Fiber middleware that enforces authenticated access.
// On success it:
//   - stores *httpx.CurrentUser in Locals("currentUser")
//   - touches last_activity_at to reset the inactivity clock
//
// On failure it redirects to /login (full-page) or returns 401 JSON (HTMX).
func RequireSession(authSvc *service.AuthService, cookieName string, log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Cookies(cookieName)
		if token == "" {
			return redirectOrUnauthorized(c, cookieName)
		}

		user, err := authSvc.ResolveSessionWithRoles(token)
		if err != nil {
			if errors.Is(err, service.ErrInvalidCredentials) ||
				errors.Is(err, service.ErrSessionExpired) {
				c.ClearCookie(cookieName)
				return redirectOrUnauthorized(c, cookieName)
			}
			log.Error("session resolution failed", "err", err)
			return redirectOrUnauthorized(c, cookieName)
		}

		// Reset the inactivity clock. Errors here are non-fatal.
		if err := authSvc.TouchActivity(token); err != nil {
			log.Warn("touch activity failed", "err", err)
		}

		cu := &httpx.CurrentUser{
			ID:          user.ID,
			DisplayName: user.DisplayName,
			Email:       user.Email,
			Roles:       user.Roles,
		}
		c.Locals("currentUser", cu)
		return c.Next()
	}
}

// RequireRole returns a Fiber middleware that denies access unless the authenticated
// user holds at least one of the allowed roles.
// Must be placed in the handler chain AFTER RequireSession.
//
// Route declarations with RequireRole are self-documenting: the required roles
// appear explicitly at the point of registration, making the authorization model
// reviewable without reading individual handler code.
func RequireRole(allowed ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		cu := CurrentUser(c)
		if cu == nil {
			// Should not happen if RequireSession precedes this middleware.
			return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
		}
		if policy.HasAnyRole(cu.Roles, allowed...) {
			return c.Next()
		}
		return fiber.NewError(fiber.StatusForbidden,
			"You do not have permission to access this resource.")
	}
}

// CurrentUser retrieves the authenticated user from Fiber locals.
// Returns nil if RequireSession was not applied (should not happen on protected routes).
func CurrentUser(c *fiber.Ctx) *httpx.CurrentUser {
	cu, _ := c.Locals("currentUser").(*httpx.CurrentUser)
	return cu
}

// redirectOrUnauthorized redirects HTMX-unaware requests to /login and returns
// 401 JSON for HTMX requests so the client can handle re-auth gracefully.
func redirectOrUnauthorized(c *fiber.Ctx, cookieName string) error {
	c.ClearCookie(cookieName)
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/login")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Session expired. Please log in again.",
		})
	}
	return c.Redirect("/login")
}
