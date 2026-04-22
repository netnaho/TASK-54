package handler

import (
	"errors"
	"log/slog"

	auditsvc "careops/clinic/internal/modules/audit/service"
	"careops/clinic/internal/modules/auth/service"
	"careops/clinic/internal/platform/middleware"
	"github.com/gofiber/fiber/v2"
)

// LoginHandler serves the login page and processes credential submissions.
type LoginHandler struct {
	authSvc    *service.AuthService
	auditSvc   *auditsvc.AuditService
	cookieName string
	log        *slog.Logger
}

func NewLoginHandler(authSvc *service.AuthService, auditSvc *auditsvc.AuditService, cookieName string, log *slog.Logger) *LoginHandler {
	return &LoginHandler{authSvc: authSvc, auditSvc: auditSvc, cookieName: cookieName, log: log}
}

// ShowLogin renders the login form.
func (h *LoginHandler) ShowLogin(c *fiber.Ctx) error {
	flash := c.Cookies("careops_flash")
	c.ClearCookie("careops_flash")
	return c.Render("auth/login", fiber.Map{
		"Title": "Sign In",
		"Flash": flash,
	}, "layouts/auth")
}

// ProcessLogin handles the POST submission from the login form.
func (h *LoginHandler) ProcessLogin(c *fiber.Ctx) error {
	email := c.FormValue("email")
	password := c.FormValue("password")
	requestID := middleware.GetRequestID(c)

	token, userID, err := h.authSvc.Login(email, password, c.Get("User-Agent"), c.IP())
	if err != nil {
		h.log.Warn("login failed",
			"email",      email,
			"err",        err.Error(),
			"request_id", requestID,
			"ip",         c.IP(),
		)

		msg := "Invalid email or password."
		if errors.Is(err, service.ErrAccountDisabled) {
			msg = "This account has been disabled. Contact your administrator."
		}

		return c.Render("auth/login", fiber.Map{
			"Title": "Sign In",
			"Error": msg,
			"Email": email,
		}, "layouts/auth")
	}

	c.Cookie(&fiber.Cookie{
		Name:     h.cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   86400, // 24 hours — aligned with session TTL
		HTTPOnly: true,
		SameSite: "Lax",
	})

	h.log.Info("login successful", "email", email, "ip", c.IP(), "request_id", requestID)
	h.auditSvc.Record(auditsvc.Entry{
		UserID:     userID,
		Action:     "login",
		EntityType: "sessions",
		Details:    "login successful",
		IPAddress:  c.IP(),
		RequestID:  requestID,
		Outcome:    "success",
	})
	return c.Redirect("/dashboard")
}

// ProcessLogout deletes the session and clears the cookie.
func (h *LoginHandler) ProcessLogout(c *fiber.Ctx) error {
	token := c.Cookies(h.cookieName)
	requestID := middleware.GetRequestID(c)
	if token != "" {
		if err := h.authSvc.Logout(token); err != nil {
			h.log.Warn("logout error", "err", err)
		}
	}
	c.ClearCookie(h.cookieName)
	h.auditSvc.Record(auditsvc.Entry{
		Action:     "logout",
		EntityType: "sessions",
		Details:    "session terminated",
		IPAddress:  c.IP(),
		RequestID:  requestID,
		Outcome:    "success",
	})
	return c.Redirect("/login")
}
