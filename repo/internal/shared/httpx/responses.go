package httpx

import "github.com/gofiber/fiber/v2"

// PageData is the standard data envelope passed to every page template.
// Module handlers embed their specific data in the Data field.
type PageData struct {
	Title    string
	User     *CurrentUser
	Flash    string
	Data     any
}

// CurrentUser carries the fields needed by the navigation and page templates.
type CurrentUser struct {
	ID          string
	DisplayName string
	Email       string
	Roles       []string
}

// HasRole reports whether the current user holds the named role.
func (u *CurrentUser) HasRole(role string) bool {
	if u == nil {
		return false
	}
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// RenderPage renders a full page using the base layout.
func RenderPage(c *fiber.Ctx, view string, pd PageData) error {
	return c.Render(view, pd, "layouts/base")
}

// RenderPartial renders a template fragment without a layout (for HTMX swaps).
func RenderPartial(c *fiber.Ctx, view string, data any) error {
	return c.Render(view, data)
}

// RedirectWithFlash redirects to path and sets a flash message cookie.
// The flash is read by the base layout on the next request.
func RedirectWithFlash(c *fiber.Ctx, path, message string) error {
	c.Cookie(&fiber.Cookie{
		Name:     "careops_flash",
		Value:    message,
		Path:     "/",
		MaxAge:   10,
		HTTPOnly: true,
	})
	return c.Redirect(path)
}
