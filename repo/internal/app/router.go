package app

import (
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"careops/clinic/internal/config"
	mwpkg "careops/clinic/internal/platform/middleware"
	"careops/clinic/internal/platform/idempotency"
	"careops/clinic/internal/platform/jobs"
	"careops/clinic/internal/shared/crypto"
	"careops/clinic/internal/shared/httpx"
	"careops/clinic/internal/shared/validation"

	authhandler "careops/clinic/internal/modules/auth/handler"
	authpolicy "careops/clinic/internal/modules/auth/policy"
	authrepo "careops/clinic/internal/modules/auth/repository"
	authsvc "careops/clinic/internal/modules/auth/service"

	admissionsrepo "careops/clinic/internal/modules/admissions/repository"
	auditrepo "careops/clinic/internal/modules/audit/repository"
	auditsvc "careops/clinic/internal/modules/audit/service"
	bedsrepo "careops/clinic/internal/modules/beds/repository"
	cfgvrepo "careops/clinic/internal/modules/config_versions/repository"
	cfgvsvc "careops/clinic/internal/modules/config_versions/service"
	diagsvc "careops/clinic/internal/modules/diagnostics/service"
	reportsrepo "careops/clinic/internal/modules/reports/repository"
	reportsvc "careops/clinic/internal/modules/reports/service"
	examrepo "careops/clinic/internal/modules/exams/repository"
	examsvc "careops/clinic/internal/modules/exams/service"
	exrepo "careops/clinic/internal/modules/exercise_library/repository"
	financerepo "careops/clinic/internal/modules/finance/repository"
	financesvc "careops/clinic/internal/modules/finance/service"
	sdrepo "careops/clinic/internal/modules/service_delivery/repository"
	usersrepo "careops/clinic/internal/modules/users/repository"
	userssvc "careops/clinic/internal/modules/users/service"

	admissionshandler "careops/clinic/internal/modules/admissions/handler"
	audithandler "careops/clinic/internal/modules/audit/handler"
	bedshandler "careops/clinic/internal/modules/beds/handler"
	cfgvhandler "careops/clinic/internal/modules/config_versions/handler"
	diaghandler "careops/clinic/internal/modules/diagnostics/handler"
	examshandler "careops/clinic/internal/modules/exams/handler"
	exercisehandler "careops/clinic/internal/modules/exercise_library/handler"
	financehandler "careops/clinic/internal/modules/finance/handler"
	jobshandler "careops/clinic/internal/modules/jobs/handler"
	occupancyhandler "careops/clinic/internal/modules/occupancy/handler"
	reportshandler "careops/clinic/internal/modules/reports/handler"
	sdhandler "careops/clinic/internal/modules/service_delivery/handler"
	usershandler "careops/clinic/internal/modules/users/handler"

	"github.com/gofiber/fiber/v2"
)

func registerMiddleware(app *fiber.App, cfg *config.Config, log *slog.Logger) {
	app.Use(mwpkg.SecurityHeaders())
	app.Use(mwpkg.Recovery(log))
	app.Use(mwpkg.RequestID())
	app.Use(mwpkg.AccessLog(log))
}

// registerRoutes wires all routes with their explicit per-route middleware chains.
//
// Authorization model (deny-by-default, statically reviewable):
//
//	Every protected route is registered as:
//	    app.Method(path, requireSession, RequireRole(allowed...), handler)
//
//	The allowed roles appear inline at the registration call site so the full
//	auth model is visible in this single file without opening handler code.
//
// Role matrix (source of truth — kept in sync with policy/rbac.go constants):
//
//	/dashboard          any authenticated user
//	/users              admin
//	/beds               admin, nurse, front_desk
//	/admissions         admin, nurse, front_desk
//	/occupancy          admin, nurse, front_desk
//	/service-delivery   admin, nurse, therapist, aide, front_desk
//	/exercise-library   admin, therapist, training_coordinator, nurse, aide
//	/exams              admin, training_coordinator
//	/finance            finance_clerk, admin   (sensitive: payer data)
//	/reports            admin, auditor
//	/audit              admin, auditor         (immutable read view)
//	/jobs               admin
//	/config-versions    admin                  (privileged: config rollback)
//	/diagnostics        admin                  (privileged: DB stats & exports)
func registerRoutes(app *fiber.App, cfg *config.Config, db *sql.DB, log *slog.Logger) {
	app.Static("/static", cfg.StaticPath)

	// Health probes — always public
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	app.Get("/readyz", func(c *fiber.Ctx) error {
		if err := db.Ping(); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "db_unreachable"})
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})
	app.Get("/", func(c *fiber.Ctx) error { return c.Redirect("/dashboard") })

	// — Audit service (constructed early; needed by login handler for auth auditing) —
	auditRepo := auditrepo.NewAuditRepo(db)
	auditService := auditsvc.NewAuditService(auditRepo, log)

	// — Auth (public) —
	sessionRepo := authrepo.NewSessionRepo(db)
	userRepo := usersrepo.NewUserRepo(db)
	authService := authsvc.NewAuthService(
		sessionRepo, userRepo,
		cfg.SessionTTLHours, cfg.InactivityTimeoutMinutes,
		log,
	)
	loginHandler := authhandler.NewLoginHandler(authService, auditService, cfg.CookieName, log)
	authhandler.Register(app, loginHandler)

	// Middleware aliases for readability at registration sites.
	rs := authhandler.RequireSession(authService, cfg.CookieName, log)
	rr := authhandler.RequireRole // rr("admin", "nurse") → RequireRole middleware

	// — Dashboard — any authenticated user —
	app.Get("/dashboard", rs, buildDashboardHandler(db, log))

	// — Users — admin only —
	userIdp := idempotency.NewService(db)
	uh := usershandler.NewUserHandler(userssvc.NewUserService(userRepo), auditService, userIdp)
	usershandler.Register(app, uh, rs, rr(authpolicy.RoleAdmin))

	// — Beds — admin, nurse, front_desk —
	bedRoles := []string{authpolicy.RoleAdmin, authpolicy.RoleNurse, authpolicy.RoleFrontDesk}
	bh := bedshandler.NewBedHandler(bedsrepo.NewBedRepo(db))
	app.Get("/beds", rs, rr(bedRoles...), bh.Index)
	app.Get("/beds/:id", rs, rr(bedRoles...), bh.Detail)

	// — Admissions — admin, nurse, front_desk —
	admRepo := admissionsrepo.NewAdmissionRepo(db)
	ah := admissionshandler.NewAdmissionHandler(admRepo)
	admissionshandler.Register(app, ah, rs, rr(bedRoles...))

	// — Occupancy — admin, nurse, front_desk —
	oh := occupancyhandler.NewOccupancyHandler(db)
	app.Get("/occupancy", rs, rr(bedRoles...), oh.Index)

	// — Job writer (shared across modules that emit job_history rows) —
	jobWriter := jobs.NewWriter(db)

	// — Service delivery — admin, nurse, therapist, aide, front_desk —
	sdRoles := []string{authpolicy.RoleAdmin, authpolicy.RoleNurse, authpolicy.RoleTherapist, authpolicy.RoleAide, authpolicy.RoleFrontDesk}
	sdIdp := idempotency.NewService(db)
	sdh := sdhandler.NewServiceDeliveryHandler(
		sdrepo.NewServiceOrderRepo(db),
		sdrepo.NewExecutionRecordRepo(db),
		sdrepo.NewAlertRepo(db),
		sdrepo.NewCareCheckpointRepo(db),
		admRepo,
		sdIdp,
		auditService,
		log,
	)
	app.Get("/service-delivery", rs, rr(sdRoles...), sdh.Index)
	app.Get("/service-delivery/orders/:id", rs, rr(sdRoles...), sdh.OrderDetail)
	app.Get("/service-delivery/alerts/new", rs, rr(sdRoles...), sdh.NewAlertForm)
	app.Post("/service-delivery/alerts", rs, rr(sdRoles...), sdh.CreateAlert)
	app.Get("/service-delivery/checkpoints/new", rs, rr(sdRoles...), sdh.NewCheckpointForm)
	app.Post("/service-delivery/checkpoints", rs, rr(sdRoles...), sdh.CreateCheckpoint)

	// — Exercise library — admin, therapist, training_coordinator, nurse, aide —
	exRoles := []string{authpolicy.RoleAdmin, authpolicy.RoleTherapist, authpolicy.RoleTrainingCoordinator, authpolicy.RoleNurse, authpolicy.RoleAide}
	elh := exercisehandler.NewExerciseLibraryHandler(exrepo.NewExerciseRepo(db))
	app.Get("/exercise-library", rs, rr(exRoles...), elh.Index)
	app.Get("/exercise-library/:id", rs, rr(exRoles...), elh.Detail)
	app.Post("/exercise-library/:id/favorite", rs, rr(exRoles...), elh.ToggleFavorite)

	// — Local media — authenticated only; path-safe serving from MediaPath —
	app.Get("/media/:filename", rs, func(c *fiber.Ctx) error {
		filename := c.Params("filename")
		if err := validation.PathSegment("filename", filename); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid filename")
		}
		return c.SendFile(filepath.Join(cfg.MediaPath, filename))
	})

	// — Exams — admin, training_coordinator —
	examRoles := []string{authpolicy.RoleAdmin, authpolicy.RoleTrainingCoordinator}
	examIdp := idempotency.NewService(db)
	examTmplRepo := examrepo.NewTemplateRepo(db)
	examSessRepo := examrepo.NewSessionRepo(db)
	examConflictRepo := examrepo.NewConflictRepo(db)

	examTZ, tzErr := time.LoadLocation(cfg.FacilityTimezone)
	if tzErr != nil {
		examTZ = time.UTC
	}

	examScheduler := examsvc.NewExamScheduler(
		examTmplRepo, examSessRepo, examConflictRepo,
		auditService, examIdp, examTZ, log,
	)
	exh := examshandler.NewExamHandler(
		examScheduler, examTmplRepo, examSessRepo, examConflictRepo,
		userRepo, examIdp, examTZ, log,
	)
	// All exam routes gated to admin + training_coordinator.
	// NOTE: app.Group("", ...) bleeds middleware onto all subsequent routes in Fiber v2;
	// pass middleware explicitly to the Register helper instead.
	examshandler.Register(app, exh, rs, rr(examRoles...))

	// — Finance — finance_clerk, admin ONLY (sensitive: payer reference numbers) —
	financeRoles := []string{authpolicy.RoleFinanceClerk, authpolicy.RoleAdmin}
	cipher, cipherErr := crypto.LoadFromEnv()
	if cipherErr != nil {
		log.Warn("field encryption not configured; sensitive reference numbers will not be encrypted", "err", cipherErr)
		cipher = nil
	}
	financeIdem := idempotency.NewService(db)
	fSvc := financesvc.New(
		financerepo.NewPaymentRepo(db),
		financerepo.NewRefundRepo(db),
		financerepo.NewShiftRepo(db),
		financerepo.NewBatchRepo(db),
		auditService,
		financeIdem,
		cipher,
		cfg.Env == "production",
		cfg.ExportsPath,
		jobWriter,
		log,
	)
	fh := financehandler.NewFinanceHandler(
		fSvc,
		financerepo.NewPaymentRepo(db),
		financerepo.NewRefundRepo(db),
		financerepo.NewShiftRepo(db),
		financerepo.NewBatchRepo(db),
		financeIdem,
		cfg.ExportsPath,
		db,
		log,
	)
	financehandler.Register(app, fh, rs, rr(financeRoles...))

	// — Reports — admin, auditor —
	reportRoles := []string{authpolicy.RoleAdmin, authpolicy.RoleAuditor}
	reportRepo := reportsrepo.NewReportRepo(db)
	reportSvc := reportsvc.New(db, reportRepo, auditService, jobWriter, cfg.ExportsPath, log)
	reportIdem := idempotency.NewService(db)
	rph := reportshandler.NewReportsHandler(reportSvc, reportIdem)
	reportshandler.Register(app, rph, rs, rr(reportRoles...))

	// — Audit log — admin, auditor (immutable read view) —
	audh := audithandler.NewAuditHandler(auditService)
	app.Get("/audit", rs, rr(authpolicy.RoleAdmin, authpolicy.RoleAuditor), audh.Index)

	// — Jobs — admin only —
	jh := jobshandler.NewJobsHandler(db)
	app.Get("/jobs", rs, rr(authpolicy.RoleAdmin), jh.Index)

	// — Config versions — admin ONLY (privileged: config rollback) —
	cfgvSvc := cfgvsvc.New(
		cfgvrepo.NewConfigRepo(db),
		auditService,
		jobWriter,
		cfg.ConfigSnapPath,
		log,
	)
	cfgvIdem := idempotency.NewService(db)
	cvh := cfgvhandler.NewConfigVersionsHandler(cfgvSvc, cfgvIdem)
	cfgvhandler.Register(app, cvh, rs, rr(authpolicy.RoleAdmin))

	// — Diagnostics — admin ONLY (privileged: DB stats, diagnostic exports) —
	diagIdem := idempotency.NewService(db)
	dSvc := diagsvc.New(db, auditService, jobWriter, cfg.DiagnosticsPath, cfg.ConfigSnapPath, log).
		WithLogPath(cfg.LogPath)
	dh := diaghandler.NewDiagnosticsHandler(dSvc, diagIdem, cfg.DiagnosticsPath)
	diaghandler.Register(app, dh, rs, rr(authpolicy.RoleAdmin))
}

func buildDashboardHandler(db *sql.DB, log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		counts, err := loadDashboardCounts(db)
		if err != nil {
			log.Error("dashboard counts failed", "err", err)
			counts = map[string]int{}
		}
		cu, _ := c.Locals("currentUser").(*httpx.CurrentUser)
		return c.Render("dashboard/index", httpx.PageData{
			Title: "Dashboard",
			User:  cu,
			Data:  counts,
		}, "layouts/base")
	}
}

func loadDashboardCounts(db *sql.DB) (map[string]int, error) {
	queries := []struct {
		key   string
		query string
	}{
		{"TotalBeds", "SELECT COUNT(*) FROM beds WHERE is_active=1"},
		{"OccupiedBeds", "SELECT COUNT(*) FROM admissions WHERE discharged_at IS NULL"},
		{"TotalResidents", "SELECT COUNT(*) FROM residents"},
		{"ActiveOrders", "SELECT COUNT(*) FROM service_orders WHERE status='active'"},
		{"OpenAlerts", "SELECT COUNT(*) FROM alert_events WHERE is_resolved=0"},
		{"TotalUsers", "SELECT COUNT(*) FROM users WHERE is_active=1"},
		{"ExerciseItems", "SELECT COUNT(*) FROM exercise_items WHERE is_active=1"},
		{"ExamTemplates", "SELECT COUNT(*) FROM competency_exam_templates WHERE is_active=1"},
		{"TotalAuditLogs", "SELECT COUNT(*) FROM audit_logs"},
	}

	counts := make(map[string]int, len(queries))
	for _, q := range queries {
		var n int
		if err := db.QueryRow(q.query).Scan(&n); err != nil {
			return nil, fmt.Errorf("dashboard: %s: %w", q.key, err)
		}
		counts[q.key] = n
	}
	return counts, nil
}
