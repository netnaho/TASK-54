package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	authhandler "careops/clinic/internal/modules/auth/handler"
	"careops/clinic/internal/modules/exams/domain"
	examrepo "careops/clinic/internal/modules/exams/repository"
	examsvc "careops/clinic/internal/modules/exams/service"
	usersrepo "careops/clinic/internal/modules/users/repository"
	"careops/clinic/internal/platform/idempotency"
	"careops/clinic/internal/shared/httpx"
	"github.com/gofiber/fiber/v2"
)

// ExamHandler handles all exam scheduler routes.
type ExamHandler struct {
	svc          *examsvc.ExamScheduler
	templateRepo *examrepo.TemplateRepo
	sessionRepo  *examrepo.SessionRepo
	conflictRepo *examrepo.ConflictRepo
	userRepo     *usersrepo.UserRepo
	idempotency  *idempotency.Service
	tz           *time.Location
	log          *slog.Logger
}

// NewExamHandler constructs the handler.
func NewExamHandler(
	svc *examsvc.ExamScheduler,
	templateRepo *examrepo.TemplateRepo,
	sessionRepo *examrepo.SessionRepo,
	conflictRepo *examrepo.ConflictRepo,
	userRepo *usersrepo.UserRepo,
	idp *idempotency.Service,
	tz *time.Location,
	log *slog.Logger,
) *ExamHandler {
	if tz == nil {
		tz = time.UTC
	}
	return &ExamHandler{
		svc:          svc,
		templateRepo: templateRepo,
		sessionRepo:  sessionRepo,
		conflictRepo: conflictRepo,
		userRepo:     userRepo,
		idempotency:  idp,
		tz:           tz,
		log:          log,
	}
}

// Index renders the exam overview: templates list + recent sessions.
func (h *ExamHandler) Index(c *fiber.Ctx) error {
	templates, err := h.templateRepo.List()
	if err != nil {
		return fmt.Errorf("exams.Index templates: %w", err)
	}
	sessions, err := h.sessionRepo.List(20)
	if err != nil {
		return fmt.Errorf("exams.Index sessions: %w", err)
	}
	return httpx.RenderPage(c, "exams/index", httpx.PageData{
		Title: "Competency Exams",
		User:  authhandler.CurrentUser(c),
		Data: fiber.Map{
			"Templates": templates,
			"Sessions":  sessions,
		},
	})
}

// TemplateNew renders the new template form.
func (h *ExamHandler) TemplateNew(c *fiber.Ctx) error {
	return httpx.RenderPage(c, "exams/template_form", httpx.PageData{
		Title: "New Exam Template",
		User:  authhandler.CurrentUser(c),
		Data:  fiber.Map{"FormError": "", "Values": fiber.Map{}},
	})
}

// TemplateCreate processes new-template form submission.
func (h *ExamHandler) TemplateCreate(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)

	if idemKey != "" {
		if entry, err := h.idempotency.Check(idemKey); err == nil && entry != nil {
			return httpx.RedirectWithFlash(c, "/exams/templates/"+entry.ResponseBody, "Template already created.")
		}
	}

	cmd := domain.CreateTemplate{
		Title:           strings.TrimSpace(c.FormValue("title")),
		Description:     strings.TrimSpace(c.FormValue("description")),
		RoleScope:       strings.TrimSpace(c.FormValue("role_scope")),
		PassingScore:    parseIntForm(c.FormValue("passing_score"), 80),
		DurationMinutes: parseIntForm(c.FormValue("duration_minutes"), 60),
		WindowStart:     strings.TrimSpace(c.FormValue("window_start")),
		WindowEnd:       strings.TrimSpace(c.FormValue("window_end")),
	}
	if err := cmd.Validate(); err != nil {
		return c.Status(http.StatusBadRequest).Render("exams/template_form", httpx.PageData{
			Title: "New Exam Template",
			User:  cu,
			Data:  fiber.Map{"FormError": err.Error(), "Values": formValues(c)},
		}, "layouts/base")
	}
	tmpl, err := h.svc.CreateTemplate(cmd, cu.ID, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		return fmt.Errorf("exams.TemplateCreate: %w", err)
	}
	if idemKey != "" {
		_ = h.idempotency.Store(idemKey, http.StatusFound, tmpl.ID)
	}
	return httpx.RedirectWithFlash(c, "/exams/templates/"+tmpl.ID, "Template created.")
}

// TemplateDetail renders a template's detail page with an edit form.
func (h *ExamHandler) TemplateDetail(c *fiber.Ctx) error {
	id := c.Params("id")
	tmpl, err := h.templateRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "Template not found.")
		}
		return err
	}
	return httpx.RenderPage(c, "exams/template_detail", httpx.PageData{
		Title: "Exam Template — " + tmpl.Title,
		User:  authhandler.CurrentUser(c),
		Data:  fiber.Map{"Template": tmpl, "FormError": ""},
	})
}

// TemplateUpdate processes template edit form submission.
func (h *ExamHandler) TemplateUpdate(c *fiber.Ctx) error {
	id := c.Params("id")
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)

	if idemKey != "" {
		if entry, err := h.idempotency.Check(idemKey); err == nil && entry != nil {
			return httpx.RedirectWithFlash(c, "/exams/templates/"+id, "Template already updated.")
		}
	}

	tmpl, err := h.templateRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "Template not found.")
		}
		return err
	}
	cmd := domain.UpdateTemplate{
		Title:           strings.TrimSpace(c.FormValue("title")),
		Description:     strings.TrimSpace(c.FormValue("description")),
		DurationMinutes: parseIntForm(c.FormValue("duration_minutes"), 60),
		WindowStart:     strings.TrimSpace(c.FormValue("window_start")),
		WindowEnd:       strings.TrimSpace(c.FormValue("window_end")),
		PassingScore:    parseIntForm(c.FormValue("passing_score"), 80),
	}
	currentVersion := parseIntForm(c.FormValue("template_version"), tmpl.TemplateVersion)
	if err := cmd.Validate(); err != nil {
		return c.Status(http.StatusBadRequest).Render("exams/template_detail", httpx.PageData{
			Title: "Exam Template — " + tmpl.Title,
			User:  cu,
			Data:  fiber.Map{"Template": tmpl, "FormError": err.Error()},
		}, "layouts/base")
	}
	updated, err := h.svc.UpdateTemplate(id, cmd, currentVersion, cu.ID, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		if errors.Is(err, examsvc.ErrVersionConflict) {
			return fiber.NewError(fiber.StatusConflict,
				"Template was modified by another request. Reload and try again.")
		}
		return c.Status(http.StatusBadRequest).Render("exams/template_detail", httpx.PageData{
			Title: "Exam Template — " + tmpl.Title,
			User:  cu,
			Data:  fiber.Map{"Template": tmpl, "FormError": err.Error()},
		}, "layouts/base")
	}
	if idemKey != "" {
		_ = h.idempotency.Store(idemKey, http.StatusFound, updated.ID)
	}
	return httpx.RedirectWithFlash(c, "/exams/templates/"+updated.ID,
		fmt.Sprintf("Template updated (now version %d).", updated.TemplateVersion))
}

// SessionNew renders the new session form.
func (h *ExamHandler) SessionNew(c *fiber.Ctx) error {
	templates, err := h.templateRepo.List()
	if err != nil {
		return fmt.Errorf("exams.SessionNew: %w", err)
	}
	users, err := h.userRepo.ListActive()
	if err != nil {
		return fmt.Errorf("exams.SessionNew users: %w", err)
	}
	return httpx.RenderPage(c, "exams/session_form", httpx.PageData{
		Title: "Generate Exam Session",
		User:  authhandler.CurrentUser(c),
		Data: fiber.Map{
			"Templates": templates,
			"Users":     users,
			"FormError": "",
			"Values":    fiber.Map{},
		},
	})
}

// SessionCreate processes the new session form.
func (h *ExamHandler) SessionCreate(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)

	if idemKey != "" {
		if entry, err := h.idempotency.Check(idemKey); err == nil && entry != nil {
			return httpx.RedirectWithFlash(c, "/exams/sessions/"+entry.ResponseBody,
				"Session already generated. Review conflicts before publishing.")
		}
	}

	templateID := strings.TrimSpace(c.FormValue("template_id"))
	plannedStartRaw := strings.TrimSpace(c.FormValue("planned_start"))
	room := strings.TrimSpace(c.FormValue("room"))
	proctorID := strings.TrimSpace(c.FormValue("proctor_id"))
	candidateIDs := c.Request().PostArgs().PeekMulti("candidate_ids")

	plannedStart, parseErr := parseFacilityDatetime(plannedStartRaw, h.tz)

	templates, _ := h.templateRepo.List()
	users, _ := h.userRepo.ListActive()

	rerenderForm := func(msg string) error {
		return c.Status(http.StatusBadRequest).Render("exams/session_form", httpx.PageData{
			Title: "Generate Exam Session",
			User:  cu,
			Data: fiber.Map{
				"Templates": templates,
				"Users":     users,
				"FormError": msg,
				"Values":    formValues(c),
			},
		}, "layouts/base")
	}

	if templateID == "" {
		return rerenderForm("template is required")
	}
	if room == "" {
		return rerenderForm("room is required")
	}
	if parseErr != nil {
		return rerenderForm("planned start time is required and must be a valid date/time")
	}

	var candIDs []string
	for _, b := range candidateIDs {
		if s := strings.TrimSpace(string(b)); s != "" {
			candIDs = append(candIDs, s)
		}
	}

	cmd := domain.GenerateSessionCmd{
		TemplateID:   templateID,
		PlannedStart: plannedStart,
		Room:         room,
		ProctorID:    proctorID,
		CandidateIDs: candIDs,
	}
	sess, _, err := h.svc.GenerateSession(cmd, cu.ID, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		return rerenderForm(err.Error())
	}
	if idemKey != "" {
		_ = h.idempotency.Store(idemKey, http.StatusFound, sess.ID)
	}
	return httpx.RedirectWithFlash(c, "/exams/sessions/"+sess.ID,
		"Session generated. Review conflicts before publishing.")
}

// SessionDetail renders the session detail page.
func (h *ExamHandler) SessionDetail(c *fiber.Ctx) error {
	id := c.Params("id")
	sess, err := h.loadSessionFull(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "Session not found.")
		}
		return err
	}
	users, _ := h.userRepo.ListActive()
	return httpx.RenderPage(c, "exams/session_detail", httpx.PageData{
		Title: "Exam Session — " + sess.TemplateTitle,
		User:  authhandler.CurrentUser(c),
		Data: fiber.Map{
			"Session":   sess,
			"Users":     users,
			"FormError": "",
		},
	})
}

// SessionBlockUpdate moves the session's time block and re-runs conflict detection.
// On HTMX requests returns the session block partial; otherwise redirects.
func (h *ExamHandler) SessionBlockUpdate(c *fiber.Ctx) error {
	id := c.Params("id")
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)

	if idemKey != "" {
		if entry, err := h.idempotency.Check(idemKey); err == nil && entry != nil {
			return httpx.RedirectWithFlash(c, "/exams/sessions/"+id, "Block already updated.")
		}
	}

	plannedStartRaw := strings.TrimSpace(c.FormValue("planned_start"))
	room := strings.TrimSpace(c.FormValue("room"))
	proctorID := strings.TrimSpace(c.FormValue("proctor_id"))
	rowVersion := parseIntForm(c.FormValue("row_version"), 0)

	plannedStart, parseErr := parseFacilityDatetime(plannedStartRaw, h.tz)

	if room == "" {
		return errorResponse(c, http.StatusBadRequest, "room is required", id)
	}
	if parseErr != nil {
		return errorResponse(c, http.StatusBadRequest, "planned start must be a valid date/time", id)
	}

	cmd := domain.UpdateSessionBlockCmd{
		SessionID:    id,
		PlannedStart: plannedStart,
		Room:         room,
		ProctorID:    proctorID,
		RowVersion:   rowVersion,
	}
	_, _, err := h.svc.UpdateSessionBlock(cmd, cu.ID, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		if errors.Is(err, examsvc.ErrVersionConflict) {
			return fiber.NewError(fiber.StatusConflict,
				"Session was modified by another request. Reload and try again.")
		}
		return errorResponse(c, http.StatusBadRequest, err.Error(), id)
	}

	sessFull, err := h.loadSessionFull(id)
	if err != nil {
		return err
	}

	if idemKey != "" {
		_ = h.idempotency.Store(idemKey, http.StatusFound, id)
	}

	if c.Get("HX-Request") == "true" {
		return httpx.RenderPartial(c, "exams/session_block_partial", fiber.Map{
			"Session": sessFull,
		})
	}
	return httpx.RedirectWithFlash(c, "/exams/sessions/"+id, "Time block updated.")
}

// SessionPublish publishes a draft session after validating no blocking conflicts remain.
func (h *ExamHandler) SessionPublish(c *fiber.Ctx) error {
	id := c.Params("id")
	cu := authhandler.CurrentUser(c)
	rowVersion := parseIntForm(c.FormValue("row_version"), 0)
	idempotencyKey := idempotency.ExtractKey(c)

	err := h.svc.PublishSession(id, cu.ID, rowVersion, idempotencyKey, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		switch {
		case errors.Is(err, examsvc.ErrBlockingConflicts):
			return fiber.NewError(fiber.StatusConflict, err.Error())
		case errors.Is(err, examsvc.ErrVersionConflict):
			return fiber.NewError(fiber.StatusConflict,
				"Session was modified by another request. Reload and try again.")
		case errors.Is(err, examsvc.ErrAlreadyPublished):
			return httpx.RedirectWithFlash(c, "/exams/sessions/"+id, "Session is already published.")
		default:
			return fmt.Errorf("exams.Publish: %w", err)
		}
	}
	return httpx.RedirectWithFlash(c, "/exams/sessions/"+id, "Session published.")
}

// SessionAddCandidate adds a candidate to a draft session.
func (h *ExamHandler) SessionAddCandidate(c *fiber.Ctx) error {
	id := c.Params("id")
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)

	if idemKey != "" {
		if entry, err := h.idempotency.Check(idemKey); err == nil && entry != nil {
			return c.Redirect("/exams/sessions/"+id, http.StatusSeeOther)
		}
	}

	userID := strings.TrimSpace(c.FormValue("user_id"))
	if userID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "user_id is required")
	}
	if err := h.svc.AddCandidate(id, userID, cu.ID, c.IP(), c.Get("X-Request-ID")); err != nil {
		return fmt.Errorf("exams.AddCandidate: %w", err)
	}

	if idemKey != "" {
		_ = h.idempotency.Store(idemKey, http.StatusSeeOther, id+":"+userID)
	}

	if c.Get("HX-Request") == "true" {
		sessFull, err := h.loadSessionFull(id)
		if err != nil {
			return err
		}
		return httpx.RenderPartial(c, "exams/conflicts_partial", fiber.Map{"Session": sessFull})
	}
	return c.Redirect("/exams/sessions/"+id, http.StatusSeeOther)
}

// SessionRemoveCandidate removes a candidate from a draft session.
func (h *ExamHandler) SessionRemoveCandidate(c *fiber.Ctx) error {
	id := c.Params("id")
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)

	if idemKey != "" {
		if entry, err := h.idempotency.Check(idemKey); err == nil && entry != nil {
			return c.Redirect("/exams/sessions/"+id, http.StatusSeeOther)
		}
	}

	userID := strings.TrimSpace(c.FormValue("user_id"))
	if err := h.svc.RemoveCandidate(id, userID, cu.ID, c.IP(), c.Get("X-Request-ID")); err != nil {
		return fmt.Errorf("exams.RemoveCandidate: %w", err)
	}

	if idemKey != "" {
		_ = h.idempotency.Store(idemKey, http.StatusSeeOther, id+":remove:"+userID)
	}

	if c.Get("HX-Request") == "true" {
		sessFull, err := h.loadSessionFull(id)
		if err != nil {
			return err
		}
		return httpx.RenderPartial(c, "exams/conflicts_partial", fiber.Map{"Session": sessFull})
	}
	return c.Redirect("/exams/sessions/"+id, http.StatusSeeOther)
}

// ConflictResolve marks a conflict as resolved with optional notes.
func (h *ExamHandler) ConflictResolve(c *fiber.Ctx) error {
	conflictID := c.Params("id")
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)

	if idemKey != "" {
		if entry, err := h.idempotency.Check(idemKey); err == nil && entry != nil {
			sessionID := c.FormValue("session_id")
			return c.Redirect("/exams/sessions/"+sessionID, http.StatusSeeOther)
		}
	}

	notes := strings.TrimSpace(c.FormValue("resolution_notes"))
	if err := h.svc.ResolveConflict(conflictID, notes, cu.ID, c.IP(), c.Get("X-Request-ID")); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "Conflict not found.")
		}
		return fmt.Errorf("exams.ConflictResolve: %w", err)
	}

	if idemKey != "" {
		_ = h.idempotency.Store(idemKey, http.StatusSeeOther, conflictID)
	}

	if c.Get("HX-Request") == "true" {
		conflict, err := h.conflictRepo.FindByID(conflictID)
		if err != nil {
			return err
		}
		return httpx.RenderPartial(c, "exams/conflict_row_partial", fiber.Map{"Conflict": conflict})
	}
	sessionID := c.FormValue("session_id")
	return c.Redirect("/exams/sessions/"+sessionID, http.StatusSeeOther)
}

// Scheduler renders the day-view scheduling timeline.
func (h *ExamHandler) Scheduler(c *fiber.Ctx) error {
	dateStr := c.Query("date")
	var base time.Time
	if dateStr != "" {
		parsed, err := time.ParseInLocation("2006-01-02", dateStr, h.tz)
		if err == nil {
			base = parsed
		}
	}
	if base.IsZero() {
		base = time.Now().In(h.tz)
	}
	dayStart := time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, h.tz).UTC()

	sessions, err := h.sessionRepo.ListForDate(dayStart)
	if err != nil {
		return fmt.Errorf("exams.Scheduler: %w", err)
	}

	type SessionWithConflicts struct {
		Session   domain.ExamSession
		Conflicts []domain.ExamConflict
	}
	var swc []SessionWithConflicts
	for _, s := range sessions {
		conflicts, _ := h.conflictRepo.ListForSession(s.ID)
		swc = append(swc, SessionWithConflicts{Session: s, Conflicts: conflicts})
	}

	localDate := base.In(h.tz)
	prevDate := localDate.AddDate(0, 0, -1).Format("2006-01-02")
	nextDate := localDate.AddDate(0, 0, 1).Format("2006-01-02")
	currentDate := localDate.Format("2006-01-02")

	return httpx.RenderPage(c, "exams/scheduler", httpx.PageData{
		Title: "Exam Scheduler — " + localDate.Format("Jan 2, 2006"),
		User:  authhandler.CurrentUser(c),
		Data: fiber.Map{
			"Date":                  dayStart,
			"CurrentDate":           currentDate,
			"PrevDate":              prevDate,
			"NextDate":              nextDate,
			"SessionsWithConflicts": swc,
			"DayStartHour":          8,
			"DayMinutes":            600,
		},
	})
}

// — internal helpers —

func (h *ExamHandler) loadSessionFull(id string) (*domain.ExamSession, error) {
	sess, err := h.sessionRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	candidates, err := h.sessionRepo.ListCandidates(id)
	if err != nil {
		return nil, err
	}
	conflicts, err := h.conflictRepo.ListForSession(id)
	if err != nil {
		return nil, err
	}
	sess.Candidates = candidates
	sess.Conflicts = conflicts
	return sess, nil
}

func parseFacilityDatetime(s string, tz *time.Location) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty datetime")
	}
	t, err := time.ParseInLocation("2006-01-02T15:04", s, tz)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid datetime %q: %w", s, err)
	}
	return t.UTC(), nil
}

func parseIntForm(s string, def int) int {
	n := def
	if s != "" {
		fmt.Sscanf(s, "%d", &n)
	}
	return n
}

func formValues(c *fiber.Ctx) fiber.Map {
	m := fiber.Map{}
	c.Request().PostArgs().VisitAll(func(k, v []byte) {
		m[string(k)] = string(v)
	})
	return m
}

func errorResponse(c *fiber.Ctx, status int, msg, sessionID string) error {
	if c.Get("HX-Request") == "true" {
		return c.Status(status).JSON(fiber.Map{"error": msg})
	}
	return httpx.RedirectWithFlash(c, "/exams/sessions/"+sessionID, "Error: "+msg)
}
