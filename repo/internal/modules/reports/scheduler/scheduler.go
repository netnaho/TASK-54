package scheduler

import (
	"context"
	"log/slog"
	"time"

	"careops/clinic/internal/modules/reports/repository"
	reportsvc "careops/clinic/internal/modules/reports/service"
)

// Scheduler evaluates active scheduled report definitions once per minute and
// runs any that are due, deduplicating via HasRunForWindow so restarts and
// overlapping ticks never produce a double-run within the same minute window.
type Scheduler struct {
	repo *repository.ReportRepo
	svc  *reportsvc.ReportService
	log  *slog.Logger
}

// New constructs a Scheduler.
func New(repo *repository.ReportRepo, svc *reportsvc.ReportService, log *slog.Logger) *Scheduler {
	return &Scheduler{repo: repo, svc: svc, log: log}
}

// Start launches the scheduler goroutine. It runs until ctx is cancelled.
// It fires once immediately at startup to catch any reports due at launch time,
// then ticks every minute on the clock boundary.
func (s *Scheduler) Start(ctx context.Context) {
	go s.run(ctx)
}

func (s *Scheduler) run(ctx context.Context) {
	s.tick(time.Now())

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case t := <-ticker.C:
			s.tick(t)
		case <-ctx.Done():
			return
		}
	}
}

// TickForTest exposes the tick logic for use in unit/integration tests only.
func (s *Scheduler) TickForTest(now time.Time) { s.tick(now) }

func (s *Scheduler) tick(now time.Time) {
	reports, err := s.repo.ListActiveScheduled()
	if err != nil {
		s.log.Error("report scheduler: list active reports", "err", err)
		return
	}

	windowStart := now.UTC().Truncate(time.Minute)
	windowEnd := windowStart.Add(time.Minute)

	for _, sr := range reports {
		if !MatchesCron(sr.ScheduleCron, windowStart) {
			continue
		}
		already, err := s.repo.HasRunForWindow(sr.ID, windowStart, windowEnd)
		if err != nil {
			s.log.Error("report scheduler: check window", "report_id", sr.ID, "err", err)
			continue
		}
		if already {
			continue
		}
		s.log.Info("report scheduler: running due report",
			"report_id", sr.ID, "name", sr.Name, "cron", sr.ScheduleCron)
		if _, err := s.svc.RunReport(sr.ID, "scheduler", "", ""); err != nil {
			s.log.Error("report scheduler: run failed", "report_id", sr.ID, "name", sr.Name, "err", err)
		}
	}
}
