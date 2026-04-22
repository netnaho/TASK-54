package app

import (
	"context"
	"database/sql"
	"log/slog"

	"careops/clinic/internal/config"
	auditrepo "careops/clinic/internal/modules/audit/repository"
	auditsvc "careops/clinic/internal/modules/audit/service"
	reportsrepo "careops/clinic/internal/modules/reports/repository"
	reportsched "careops/clinic/internal/modules/reports/scheduler"
	reportsvc "careops/clinic/internal/modules/reports/service"
	"careops/clinic/internal/platform/jobs"
)

// StartReportScheduler wires the report scheduler and starts it in a goroutine.
// The scheduler fires every minute, runs any active scheduled reports whose cron
// expression matches the current window, and deduplicates across restarts.
// Cancel ctx to stop the scheduler on shutdown.
func StartReportScheduler(ctx context.Context, db *sql.DB, cfg *config.Config, log *slog.Logger) {
	reportRepo := reportsrepo.NewReportRepo(db)
	jobWriter := jobs.NewWriter(db)
	auditRepo := auditrepo.NewAuditRepo(db)
	auditService := auditsvc.NewAuditService(auditRepo, log)
	svc := reportsvc.New(db, reportRepo, auditService, jobWriter, cfg.ExportsPath, log)
	sched := reportsched.New(reportRepo, svc, log)
	sched.Start(ctx)
}
