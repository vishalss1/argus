package fleet

import (
	"context"

	"github.com/vishalss1/argus/telemetry/internal/domain/severity"
)

type Stats struct {
	TotalDevices      int
	OnlineDevices     int
	ActiveIncidents   int
	WarningIncidents  int
	CriticalIncidents int
	WorstSeverity     severity.Level
}

type HealthSummary struct {
	Stats
	TopIncidentTypes []string
}

type DeviceBrief struct {
	DeviceID      string
	DeviceName    string
	OpenIncidents int
	WorstSeverity severity.Level
}

type IncidentBrief struct {
	DeviceID     string
	Metric       string
	IncidentType string
	Severity     severity.Level
}

type Service interface {
	GetStats(ctx context.Context, workspaceID string) (*Stats, error)
	GetHealthSummary(ctx context.Context, workspaceID string) (*HealthSummary, error)
	GetWorstDevices(ctx context.Context, workspaceID string, limit int) ([]DeviceBrief, error)
	GetRecentIncidents(ctx context.Context, workspaceID string, limit int) ([]IncidentBrief, error)
}

