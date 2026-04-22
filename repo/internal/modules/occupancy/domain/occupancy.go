package domain

import (
	"fmt"
	"time"
)

// OccupancySnapshot is a point-in-time record of bed utilisation.
type OccupancySnapshot struct {
	ID            string
	SnapshotDate  string
	TotalBeds     int
	OccupiedBeds  int
	AvailableBeds int
	OccupancyRate float64
	CreatedAt     time.Time
}

func (s OccupancySnapshot) OccupancyPct() string {
	return fmt.Sprintf("%.1f%%", s.OccupancyRate*100)
}
