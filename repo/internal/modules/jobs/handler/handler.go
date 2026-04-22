package handler

import (
	"database/sql"
	"fmt"
	"time"

	authhandler "careops/clinic/internal/modules/auth/handler"
	"careops/clinic/internal/modules/jobs/domain"
	"careops/clinic/internal/shared/httpx"
	"github.com/gofiber/fiber/v2"
)

type JobsHandler struct {
	db *sql.DB
}

func NewJobsHandler(db *sql.DB) *JobsHandler {
	return &JobsHandler{db: db}
}

func (h *JobsHandler) Index(c *fiber.Ctx) error {
	jobs, err := h.loadJobs()
	if err != nil {
		return err
	}

	return httpx.RenderPage(c, "jobs/index", httpx.PageData{
		Title: "Background Jobs",
		User:  authhandler.CurrentUser(c),
		Data: fiber.Map{
			"Jobs": jobs,
		},
	})
}

func (h *JobsHandler) loadJobs() ([]domain.JobHistory, error) {
	rows, err := h.db.Query(`
		SELECT id, job_name, job_type, status, duration_ms, detail,
		       actor, correlation_id, ran_at, created_at
		FROM job_history ORDER BY ran_at DESC LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("jobs: query: %w", err)
	}
	defer rows.Close()

	var jobs []domain.JobHistory
	for rows.Next() {
		var j domain.JobHistory
		var ranAt, ca string
		if err := rows.Scan(
			&j.ID, &j.JobName, &j.JobType, &j.Status, &j.DurationMs, &j.Detail,
			&j.Actor, &j.CorrelationID, &ranAt, &ca,
		); err != nil {
			return nil, err
		}
		j.RanAt, _ = time.Parse(time.RFC3339, ranAt)
		j.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
