// Package domain defines the finance module's core types, value objects,
// and input-command structs.  All money is stored as integer cents to avoid
// floating-point rounding.
package domain

import (
	"fmt"
	"strings"
	"time"
)

// — Payment methods ———————————————————————————————————————————————————————————

const (
	MethodCash            = "cash"
	MethodCheck           = "check"
	MethodFacilityAccount = "facility_account"
	MethodCard            = "card"
	MethodInsurance       = "insurance"
	MethodBankTransfer    = "bank_transfer"
)

// SensitiveMethods lists payment methods whose reference numbers contain
// personally identifiable financial data (card/check numbers) and must be
// encrypted at rest and masked in non-finance views.
var SensitiveMethods = map[string]bool{
	MethodCard:  true,
	MethodCheck: true,
}

// ShiftNames are the three named settlement windows per day.
const (
	ShiftMorning   = "morning"   // 07:00–15:00
	ShiftAfternoon = "afternoon" // 15:00–23:00
	ShiftNight     = "night"     // 23:00–07:00
)

// ValidShiftNames for validation.
var ValidShiftNames = map[string]bool{
	ShiftMorning:   true,
	ShiftAfternoon: true,
	ShiftNight:     true,
}

// — Payment ——————————————————————————————————————————————————————————————————

// Payment records a single financial transaction for a resident.
type Payment struct {
	ID                  string
	ResidentID          string
	ShiftID             string
	BatchID             string
	PaymentMethod       string
	AmountCents         int
	Currency            string
	ReferenceNumber     string // masked display value (last-4 for sensitive methods)
	EncryptedReference  string // AES-GCM encrypted raw value; empty if not set
	Description         string
	RecordedBy          string
	Status              string // "pending" | "completed" | "reversed"
	RowVersion          int
	RecordedAt          time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time

	// Joined from related tables.
	ResidentName   string
	RecordedByName string
	TotalRefunded  int // sum of refund amounts for this payment
}

// AmountDollars converts stored cents to a "$X.XX" string.
func (p Payment) AmountDollars() string {
	return centsToDollars(p.AmountCents)
}

// NetCents returns the payment amount minus any refunds.
func (p Payment) NetCents() int { return p.AmountCents - p.TotalRefunded }

// NetDollars returns the net payment amount (after refunds) as a "$X.XX" string.
func (p Payment) NetDollars() string { return centsToDollars(p.NetCents()) }

// IsSensitive reports whether the payment method requires reference encryption.
func (p Payment) IsSensitive() bool { return SensitiveMethods[p.PaymentMethod] }

// — Refund ———————————————————————————————————————————————————————————————————

// Refund records a partial or full reversal of a payment.
type Refund struct {
	ID          string
	PaymentID   string
	AmountCents int
	Reason      string
	ProcessedBy string
	ProcessedAt time.Time
	CreatedAt   time.Time

	// Joined
	ProcessedByName string
	ResidentName    string
}

// AmountDollars converts stored cents to a "$X.XX" string.
func (r Refund) AmountDollars() string { return centsToDollars(r.AmountCents) }

// — CardBatch ————————————————————————————————————————————————————————————————

// CardBatch tracks a card-terminal batch file import.
type CardBatch struct {
	ID           string
	FileName     string
	FilePath     string // stored on disk
	ShiftID      string
	ImportedBy   string
	RecordCount  int
	SuccessCount int
	ErrorCount   int
	Status       string // "pending"|"processing"|"complete"|"failed"
	ErrorDetail  string
	ImportedAt   time.Time
	CreatedAt    time.Time

	// Joined
	ImportedByName string
	ShiftDate      string
}

// — SettlementShift —————————————————————————————————————————————————————————

// SettlementShift is one of the three named settlement windows per day.
type SettlementShift struct {
	ID                  string
	ShiftDate           string // YYYY-MM-DD
	ShiftName           string // "morning"|"afternoon"|"night"
	OpenedBy            string
	ClosedBy            string
	OpenedAt            time.Time
	ClosedAt            *time.Time
	Status              string // "open"|"closed"|"reconciled"
	Notes               string
	TotalAmountCents    int
	ReconciliationNotes string
	CreatedAt           time.Time

	// Joined
	OpenedByName  string
	ClosedByName  string
	TotalPayments int
}

// TotalDollars returns the cached reconciled total as a "$X.XX" string.
func (s SettlementShift) TotalDollars() string { return centsToDollars(s.TotalAmountCents) }

// ShiftLabel returns a human-readable label for the shift window.
func (s SettlementShift) ShiftLabel() string {
	switch s.ShiftName {
	case ShiftMorning:
		return "Morning (07:00–15:00)"
	case ShiftAfternoon:
		return "Afternoon (15:00–23:00)"
	case ShiftNight:
		return "Night (23:00–07:00)"
	}
	return s.ShiftName
}

// — Input commands —————————————————————————————————————————————————————————

// CreatePayment is the form submission for recording a new payment.
type CreatePayment struct {
	ResidentID      string
	ShiftID         string
	PaymentMethod   string
	AmountDollarsIn string // raw string from form; converted to cents on validate
	AmountCents     int    // set by Validate
	Currency        string
	ReferenceNumber string
	Description     string
}

// Validate parses and checks all fields, converting AmountDollarsIn → AmountCents.
func (c *CreatePayment) Validate() error {
	if strings.TrimSpace(c.ResidentID) == "" {
		return fmt.Errorf("resident is required")
	}
	switch c.PaymentMethod {
	case MethodCash, MethodCheck, MethodFacilityAccount, MethodCard, MethodInsurance, MethodBankTransfer:
	default:
		return fmt.Errorf("invalid payment method %q", c.PaymentMethod)
	}
	cents, err := parseDollars(c.AmountDollarsIn)
	if err != nil {
		return err
	}
	if cents <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}
	c.AmountCents = cents
	if strings.TrimSpace(c.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if c.Currency == "" {
		c.Currency = "USD"
	}
	// Card and check payments should provide a reference number (not required
	// for cash/facility_account/insurance).
	if SensitiveMethods[c.PaymentMethod] && strings.TrimSpace(c.ReferenceNumber) == "" {
		return fmt.Errorf("reference number is required for %s payments", c.PaymentMethod)
	}
	return nil
}

// ShouldEncryptReference reports whether the reference number should be
// encrypted at rest for this payment method.
func (c *CreatePayment) ShouldEncryptReference() bool {
	return SensitiveMethods[c.PaymentMethod] && c.ReferenceNumber != ""
}

// CreateRefund is the form submission for processing a refund.
type CreateRefund struct {
	PaymentID       string
	AmountDollarsIn string
	AmountCents     int
	Reason          string
}

// AmountDollarsDisplay converts stored cents to a "$X.XX" string (display only).
func (c CreateRefund) AmountDollarsDisplay() string { return centsToDollars(c.AmountCents) }

// Validate checks all fields.
func (c *CreateRefund) Validate() error {
	if strings.TrimSpace(c.PaymentID) == "" {
		return fmt.Errorf("payment_id is required")
	}
	cents, err := parseDollars(c.AmountDollarsIn)
	if err != nil {
		return err
	}
	if cents <= 0 {
		return fmt.Errorf("refund amount must be greater than zero")
	}
	c.AmountCents = cents
	if strings.TrimSpace(c.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	return nil
}

// CloseShiftCmd carries the parameters for the shift-close action.
type CloseShiftCmd struct {
	ShiftID              string
	ReconciliationNotes  string
}

// ExportParams controls what is exported and in which format.
type ExportParams struct {
	ReportType string // "finance" | "occupancy"
	DateFrom   string // YYYY-MM-DD
	DateTo     string // YYYY-MM-DD
	ShiftID    string // optional: limit to one shift
	Format     string // "csv" | "xlsx"
}

// Validate checks the export parameters.
func (p *ExportParams) Validate() error {
	switch p.ReportType {
	case "finance", "occupancy":
	default:
		return fmt.Errorf("invalid report_type %q", p.ReportType)
	}
	switch p.Format {
	case "csv", "xlsx":
	default:
		return fmt.Errorf("invalid format %q; must be csv or xlsx", p.Format)
	}
	if p.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", p.DateFrom); err != nil {
			return fmt.Errorf("date_from must be YYYY-MM-DD")
		}
	}
	if p.DateTo != "" {
		if _, err := time.Parse("2006-01-02", p.DateTo); err != nil {
			return fmt.Errorf("date_to must be YYYY-MM-DD")
		}
	}
	return nil
}

// — helpers ——————————————————————————————————————————————————————————————————

func centsToDollars(cents int) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}

// parseDollars converts a user-supplied dollar string ("12.50", "12", "12.5")
// to integer cents. Accepts optional leading '$'.
func parseDollars(s string) (int, error) {
	s = strings.TrimSpace(strings.TrimPrefix(s, "$"))
	if s == "" {
		return 0, fmt.Errorf("amount is required")
	}
	parts := strings.SplitN(s, ".", 2)
	var dollars, cents int
	if _, err := fmt.Sscanf(parts[0], "%d", &dollars); err != nil {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > 2 {
			frac = frac[:2]
		}
		for len(frac) < 2 {
			frac += "0"
		}
		if _, err := fmt.Sscanf(frac, "%d", &cents); err != nil {
			return 0, fmt.Errorf("invalid amount %q", s)
		}
	}
	return dollars*100 + cents, nil
}
