package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	authhandler "careops/clinic/internal/modules/auth/handler"
	auditsvc "careops/clinic/internal/modules/audit/service"
	"careops/clinic/internal/modules/users/domain"
	"careops/clinic/internal/modules/users/service"
	audithelper "careops/clinic/internal/shared/audit"
	"careops/clinic/internal/shared/apperr"
	"careops/clinic/internal/shared/httpx"
	"careops/clinic/internal/platform/idempotency"
	"github.com/gofiber/fiber/v2"
)

type UserHandler struct {
	svc      *service.UserService
	auditSvc *auditsvc.AuditService
	idem     *idempotency.Service
}

func NewUserHandler(svc *service.UserService, auditSvc *auditsvc.AuditService, idem *idempotency.Service) *UserHandler {
	return &UserHandler{svc: svc, auditSvc: auditSvc, idem: idem}
}

// Index renders the user list page with users, roles and a create link.
func (h *UserHandler) Index(c *fiber.Ctx) error {
	users, err := h.svc.ListWithRoles()
	if err != nil {
		return err
	}
	roles, err := h.svc.ListRoles()
	if err != nil {
		return err
	}

	return httpx.RenderPage(c, "users/index", httpx.PageData{
		Title: "Staff Users",
		User:  authhandler.CurrentUser(c),
		Flash: c.Query("flash"),
		Data: fiber.Map{
			"Users": users,
			"Roles": roles,
		},
	})
}

// NewForm renders GET /users/new — user create form.
func (h *UserHandler) NewForm(c *fiber.Ctx) error {
	roles, err := h.svc.ListRoles()
	if err != nil {
		return err
	}
	return httpx.RenderPage(c, "users/create", httpx.PageData{
		Title: "Create Staff User",
		User:  authhandler.CurrentUser(c),
		Data:  fiber.Map{"Roles": roles, "FormError": "", "Values": fiber.Map{}},
	})
}

// Create handles POST /users — creates a new staff user.
func (h *UserHandler) Create(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	roles, _ := h.svc.ListRoles()

	idemKey := idempotency.ExtractKey(c)
	if idemKey != "" {
		if entry, err := h.idem.Check(idemKey); err == nil && entry != nil {
			return c.Redirect("/users?flash=User+already+created.", http.StatusSeeOther)
		}
	}

	cmd := domain.CreateUserCmd{
		Email:       strings.TrimSpace(c.FormValue("email")),
		DisplayName: strings.TrimSpace(c.FormValue("display_name")),
		Password:    c.FormValue("password"),
		RoleID:      c.FormValue("role"),
	}

	rerender := func(errMsg string) error {
		return c.Status(http.StatusOK).Render("users/create", httpx.PageData{
			Title: "Create Staff User",
			User:  cu,
			Data: fiber.Map{
				"Roles":     roles,
				"FormError": errMsg,
				"Values": fiber.Map{
					"email":        cmd.Email,
					"display_name": cmd.DisplayName,
					"role":         cmd.RoleID,
				},
			},
		}, "layouts/base")
	}

	newUser, err := h.svc.CreateUser(cmd)
	if err != nil {
		return rerender(err.Error())
	}

	h.auditSvc.Record(auditsvc.Entry{
		UserID:        cu.ID,
		Action:        "user.create",
		EntityType:    "users",
		EntityID:      newUser.ID,
		Details:       fmt.Sprintf("created user %q with role %q", newUser.Email, cmd.RoleID),
		AfterSnapshot: audithelper.ToJSON(fiber.Map{"id": newUser.ID, "email": newUser.Email, "display_name": newUser.DisplayName}),
		IPAddress:     c.IP(),
		RequestID:     c.Get("X-Request-ID"),
		Outcome:       "success",
	})

	if idemKey != "" {
		_ = h.idem.Store(idemKey, http.StatusSeeOther, newUser.ID)
	}

	return c.Redirect("/users?flash=User+"+newUser.DisplayName+"+created+successfully.", http.StatusSeeOther)
}

// ResetPasswordForm renders GET /users/:id/reset-password.
func (h *UserHandler) ResetPasswordForm(c *fiber.Ctx) error {
	id := c.Params("id")
	user, err := h.svc.FindByID(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "User not found.")
	}
	return httpx.RenderPage(c, "users/reset_password", httpx.PageData{
		Title: "Reset Password — " + user.DisplayName,
		User:  authhandler.CurrentUser(c),
		Data:  fiber.Map{"TargetUser": user, "FormError": ""},
	})
}

// ResetPassword handles POST /users/:id/reset-password.
// The form must include a row_version hidden field (populated by ResetPasswordForm)
// to enforce optimistic concurrency control. A stale version returns HTTP 409.
func (h *UserHandler) ResetPassword(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	id := c.Params("id")

	idemKey := idempotency.ExtractKey(c)
	if idemKey != "" {
		if entry, err := h.idem.Check(idemKey); err == nil && entry != nil {
			return c.Redirect("/users?flash=Password+already+reset.", http.StatusSeeOther)
		}
	}

	user, err := h.svc.FindByID(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "User not found.")
	}

	rowVersion, _ := strconv.Atoi(c.FormValue("row_version"))
	cmd := domain.ResetPasswordCmd{
		UserID:     id,
		Password:   c.FormValue("password"),
		RowVersion: rowVersion,
	}
	if err := h.svc.ResetPassword(cmd); err != nil {
		if errors.Is(err, apperr.ErrConflict) {
			h.auditSvc.Record(auditsvc.Entry{
				UserID:     cu.ID,
				Action:     "user.reset_password",
				EntityType: "users",
				EntityID:   id,
				Details:    fmt.Sprintf("password reset conflict for user %q (stale row_version %d)", user.Email, rowVersion),
				IPAddress:  c.IP(),
				RequestID:  c.Get("X-Request-ID"),
				Outcome:    "failure",
			})
			return fiber.NewError(fiber.StatusConflict,
				"Record was modified by another request. Reload the page and try again.")
		}
		return c.Status(http.StatusOK).Render("users/reset_password", httpx.PageData{
			Title: "Reset Password — " + user.DisplayName,
			User:  cu,
			Data:  fiber.Map{"TargetUser": user, "FormError": err.Error()},
		}, "layouts/base")
	}

	h.auditSvc.Record(auditsvc.Entry{
		UserID:     cu.ID,
		Action:     "user.reset_password",
		EntityType: "users",
		EntityID:   id,
		Details:    fmt.Sprintf("reset password for user %q", user.Email),
		IPAddress:  c.IP(),
		RequestID:  c.Get("X-Request-ID"),
		Outcome:    "success",
	})

	if idemKey != "" {
		_ = h.idem.Store(idemKey, http.StatusSeeOther, id)
	}

	return c.Redirect("/users?flash=Password+reset+for+"+user.DisplayName+".", http.StatusSeeOther)
}
