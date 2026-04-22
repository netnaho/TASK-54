package handler

import "github.com/gofiber/fiber/v2"

// Register wires all finance routes onto r with the provided middleware chain.
func Register(r fiber.Router, h *FinanceHandler, middleware ...fiber.Handler) {
	mw := middleware

	r.Get("/finance", append(mw, h.Index)...)

	// Payments
	r.Get("/finance/payments/new", append(mw, h.PaymentNew)...)
	r.Post("/finance/payments", append(mw, h.PaymentCreate)...)
	r.Get("/finance/payments/:id", append(mw, h.PaymentDetail)...)
	r.Get("/finance/payments/:id/refund", append(mw, h.RefundNew)...)
	r.Post("/finance/payments/:id/refund", append(mw, h.RefundCreate)...)

	// Shifts
	r.Get("/finance/shifts", append(mw, h.ShiftList)...)
	r.Post("/finance/shifts", append(mw, h.ShiftOpen)...)
	r.Get("/finance/shifts/:id", append(mw, h.ShiftDetail)...)
	r.Post("/finance/shifts/:id/close", append(mw, h.ShiftClose)...)

	// Card-terminal batch imports
	r.Get("/finance/batches/new", append(mw, h.BatchImportNew)...)
	r.Post("/finance/batches", append(mw, h.BatchImportSubmit)...)
	r.Get("/finance/batches/:id", append(mw, h.BatchDetail)...)

	// Exports
	r.Get("/finance/exports/new", append(mw, h.ExportNew)...)
	r.Post("/finance/exports", append(mw, h.ExportCreate)...)
	r.Get("/finance/exports/download", append(mw, h.ExportDownload)...)
}
