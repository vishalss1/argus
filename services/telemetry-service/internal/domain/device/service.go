package device

import (
	"context"
	"time"

	"github.com/vishalss1/argus/telemetry/internal/domain/fleet"
)

type Snapshot struct {
	DeviceID               string
	Status                 string
	LastSeen               time.Time
	LatestMetrics          map[string]float64
	ActiveIncidentTypes    []string
	TotalIncidentsRecorded int
}

type State struct {
	Status   string
	LastSeen time.Time
}

type Metrics struct {
	Latest map[string]float64
}

type Service interface {
	GetSnapshot(ctx context.Context, deviceID string) (*Snapshot, error)
	GetState(ctx context.Context, deviceID string) (*State, error)
	GetIncidents(ctx context.Context, deviceID string, limit int) ([]fleet.IncidentBrief, error)
	GetMetrics(ctx context.Context, deviceID string) (*Metrics, error)
}
