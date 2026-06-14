package query

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/vishalss1/argus/telemetry/internal/ai/operations"
	ctxdomain "github.com/vishalss1/argus/telemetry/internal/domain/context"
	"github.com/vishalss1/argus/telemetry/internal/domain/device"
	"github.com/vishalss1/argus/telemetry/internal/domain/telemetry"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/postgres"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/redis"
)

type SnapshotBuilder struct {
	deviceRepo  device.Repository
	eventRepo   *postgres.EventRepository
	contextRepo *postgres.ContextRepository
	redisClient *redis.Client
}

func NewSnapshotBuilder(
	deviceRepo device.Repository,
	eventRepo *postgres.EventRepository,
	contextRepo *postgres.ContextRepository,
	redisClient *redis.Client,
) *SnapshotBuilder {
	return &SnapshotBuilder{
		deviceRepo:  deviceRepo,
		eventRepo:   eventRepo,
		contextRepo: contextRepo,
		redisClient: redisClient,
	}
}

func (b *SnapshotBuilder) Build(ctx context.Context, deviceID string) (*operations.Snapshot, error) {
	target, err := b.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("get device: %w", err)
	}

	snapshot := &operations.Snapshot{
		Device:           *target,
		LatestTelemetry:  make(map[string]any),
		TelemetryWindows: make(map[string][]operations.TelemetryPoint),
		FirmwareInfo: map[string]any{
			"version": target.FirmwareVersion,
		},
	}

	if raw, err := b.redisClient.Client().Get(ctx, fmt.Sprintf("device:%s:latest", target.ID)).Bytes(); err == nil {
		var latest telemetry.Telemetry
		if json.Unmarshal(raw, &latest) == nil {
			_ = json.Unmarshal(latest.Metrics, &snapshot.LatestTelemetry)
			snapshot.TelemetryRecordedAt = &latest.RecordedAt
		}
	}

	workspaceID := ""
	if target.WorkspaceID != nil {
		workspaceID = *target.WorkspaceID
	}
	sessionID, _ := b.redisClient.Client().Get(ctx, fmt.Sprintf("workspace:%s:active_session", workspaceID)).Result()
	if sessionID != "" {
		snapshot.TelemetryWindows = b.loadTelemetryWindows(ctx, sessionID, target.ID)
		snapshot.TelemetryTrends = b.loadTrends(ctx, sessionID, target.ID, snapshot.LatestTelemetry)
		snapshot.IncidentHistory = append(snapshot.IncidentHistory, b.loadActiveIncidents(ctx, sessionID, target.ID)...)
		snapshot.IncidentHistory = append(snapshot.IncidentHistory, b.loadClosedIncidents(ctx, sessionID, target.ID)...)
	}

	if memories, err := b.contextRepo.ListByDevice(ctx, target.ID, 50, 0); err == nil {
		for _, memory := range memories {
			item := operations.HistoryItem{Type: string(memory.Type), Summary: memory.Summary, Data: memory.Data, Timestamp: memory.Timestamp}
			switch memory.Type {
			case ctxdomain.MemoryTypeConnectivity:
				snapshot.ConnectivityHistory = append(snapshot.ConnectivityHistory, item)
			case ctxdomain.MemoryTypeAnomaly, ctxdomain.MemoryTypeIncident:
				snapshot.AnomalyHistory = append(snapshot.AnomalyHistory, item)
			}
		}
	}

	if events, err := b.eventRepo.ListByDevice(ctx, target.ID, 50, 0); err == nil {
		for _, ev := range events {
			if ev.CreatedAt.Before(time.Now().Add(-7 * 24 * time.Hour)) {
				continue
			}
			snapshot.AnomalyHistory = append(snapshot.AnomalyHistory, operations.HistoryItem{
				Type: ev.Type, Summary: ev.Summary, Data: ev.Metadata, Timestamp: ev.CreatedAt,
			})
		}
	}

	sort.Slice(snapshot.IncidentHistory, func(i, j int) bool {
		return snapshot.IncidentHistory[i].StartTime.After(snapshot.IncidentHistory[j].StartTime)
	})

	return snapshot, nil
}

func (b *SnapshotBuilder) loadTelemetryWindows(ctx context.Context, sessionID, deviceID string) map[string][]operations.TelemetryPoint {
	windows := make(map[string][]operations.TelemetryPoint)
	rawItems, _ := b.redisClient.Client().LRange(ctx, fmt.Sprintf("session:%s:device:%s:window", sessionID, deviceID), -250, -1).Result()
	if len(rawItems) > 0 {
		var points []operations.TelemetryPoint
		for _, raw := range rawItems {
			var t telemetry.Telemetry
			if json.Unmarshal([]byte(raw), &t) == nil {
				var metrics map[string]any
				if json.Unmarshal(t.Metrics, &metrics) == nil {
					points = append(points, operations.TelemetryPoint{RecordedAt: t.RecordedAt, Metrics: metrics})
				}
			}
		}
		windows["recent"] = points
	}
	return windows
}

func (b *SnapshotBuilder) loadTrends(ctx context.Context, sessionID, deviceID string, latest map[string]any) []operations.MetricTrend {
	var trends []operations.MetricTrend
	metricsKeys, _ := b.redisClient.Client().Keys(ctx, fmt.Sprintf("session:%s:device:%s:metric:*", sessionID, deviceID)).Result()
	for _, key := range metricsKeys {
		var agg struct {
			Count    int64   `json:"count"`
			Minimum  float64 `json:"minimum"`
			Maximum  float64 `json:"maximum"`
			Average  float64 `json:"average"`
			Variance float64 `json:"variance"`
		}
		if raw, err := b.redisClient.Client().Get(ctx, key).Result(); err == nil {
			if json.Unmarshal([]byte(raw), &agg) == nil {
				metricName := key[len(fmt.Sprintf("session:%s:device:%s:metric:", sessionID, deviceID)):]
				trends = append(trends, operations.MetricTrend{
					Metric:   metricName,
					Current:  latest[metricName],
					Count:    agg.Count,
					Minimum:  agg.Minimum,
					Maximum:  agg.Maximum,
					Average:  agg.Average,
					Variance: agg.Variance,
				})
			}
		}
	}
	return trends
}

func (b *SnapshotBuilder) loadActiveIncidents(ctx context.Context, sessionID, deviceID string) []operations.Incident {
	rawKeys, _ := b.redisClient.Client().SMembers(ctx, fmt.Sprintf("session:%s:incidents", sessionID)).Result()
	var incidents []operations.Incident
	if len(rawKeys) > 0 {
		values, _ := b.redisClient.Client().MGet(ctx, rawKeys...).Result()
		for _, val := range values {
			raw, ok := val.(string)
			if !ok {
				continue
			}
			var incident operations.Incident
			if json.Unmarshal([]byte(raw), &incident) == nil && incident.DeviceID == deviceID {
				incident.Status = "open"
				incidents = append(incidents, incident)
			}
		}
	}
	return incidents
}

func (b *SnapshotBuilder) loadClosedIncidents(ctx context.Context, sessionID, deviceID string) []operations.Incident {
	rawItems, _ := b.redisClient.Client().LRange(ctx, fmt.Sprintf("session:%s:artifact_buffer", sessionID), -250, -1).Result()
	result := make([]operations.Incident, 0)
	for _, raw := range rawItems {
		var incident operations.Incident
		if json.Unmarshal([]byte(raw), &incident) == nil && incident.DeviceID == deviceID {
			incident.Status = "resolved"
			result = append(result, incident)
		}
	}
	return result
}
