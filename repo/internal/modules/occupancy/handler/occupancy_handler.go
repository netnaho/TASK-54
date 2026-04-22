package handler

import (
	"database/sql"
	"fmt"
	"time"

	authhandler "careops/clinic/internal/modules/auth/handler"
	"careops/clinic/internal/modules/occupancy/domain"
	"careops/clinic/internal/shared/httpx"
	"github.com/gofiber/fiber/v2"
)

type OccupancyHandler struct {
	db *sql.DB
}

func NewOccupancyHandler(db *sql.DB) *OccupancyHandler {
	return &OccupancyHandler{db: db}
}

// Index renders the occupancy overview with real-time counts and snapshot history.
func (h *OccupancyHandler) Index(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)

	live, err := h.loadLiveOccupancy()
	if err != nil {
		return fmt.Errorf("occupancy.Index live: %w", err)
	}

	snapshots, err := h.loadSnapshots()
	if err != nil {
		return fmt.Errorf("occupancy.Index snapshots: %w", err)
	}

	return httpx.RenderPage(c, "occupancy/index", httpx.PageData{
		Title: "Occupancy Tracking",
		User:  cu,
		Data: fiber.Map{
			"Live":      live,
			"Snapshots": snapshots,
		},
	})
}

// LiveOccupancy holds the real-time bed count derived from active admissions.
type LiveOccupancy struct {
	TotalBeds     int
	OccupiedBeds  int
	AvailableBeds int
	OccupancyPct  string
}

func (h *OccupancyHandler) loadLiveOccupancy() (LiveOccupancy, error) {
	var lo LiveOccupancy
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM beds WHERE is_active = 1`).Scan(&lo.TotalBeds); err != nil {
		return lo, fmt.Errorf("occupancy: total beds: %w", err)
	}
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM admissions WHERE discharged_at IS NULL`).Scan(&lo.OccupiedBeds); err != nil {
		return lo, fmt.Errorf("occupancy: occupied beds: %w", err)
	}
	lo.AvailableBeds = lo.TotalBeds - lo.OccupiedBeds
	if lo.TotalBeds > 0 {
		lo.OccupancyPct = fmt.Sprintf("%.1f%%", float64(lo.OccupiedBeds)/float64(lo.TotalBeds)*100)
	} else {
		lo.OccupancyPct = "0.0%"
	}
	return lo, nil
}

func (h *OccupancyHandler) loadSnapshots() ([]domain.OccupancySnapshot, error) {
	rows, err := h.db.Query(`
		SELECT id, snapshot_date, total_beds, occupied_beds, available_beds, occupancy_rate, created_at
		FROM occupancy_snapshots
		ORDER BY snapshot_date DESC
		LIMIT 30`)
	if err != nil {
		return nil, fmt.Errorf("occupancy: snapshots: %w", err)
	}
	defer rows.Close()

	var snaps []domain.OccupancySnapshot
	for rows.Next() {
		var s domain.OccupancySnapshot
		var ca string
		if err := rows.Scan(&s.ID, &s.SnapshotDate, &s.TotalBeds, &s.OccupiedBeds, &s.AvailableBeds, &s.OccupancyRate, &ca); err != nil {
			return nil, err
		}
		s.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		snaps = append(snaps, s)
	}
	return snaps, rows.Err()
}
