package handler

import "github.com/gofiber/fiber/v2"

// Register mounts public auth routes on the Fiber app.
// Protected routes are registered by the respective module handlers using RequireSession.
func Register(app *fiber.App, lh *LoginHandler) {
	app.Get("/login",  lh.ShowLogin)
	app.Post("/login", lh.ProcessLogin)
	app.Post("/logout", lh.ProcessLogout)
}
