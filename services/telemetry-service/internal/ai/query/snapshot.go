package query

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
	rdb := b.redisClient.Client()

	type hourlySummary struct {
		Metric      string
		SampleCount int
		Min         float64
		Max         float64
		Average     float64
		Variance    float64
	}
	var summaries []hourlySummary

	hashData, err := rdb.HGetAll(ctx, fmt.Sprintf("session:%s:hourly_summaries", sessionID)).Result()
	if err == nil && len(hashData) > 0 {
		for field, val := range hashData {
			if strings.HasPrefix(field, fmt.Sprintf("device:%s:hour:", deviceID)) {
				var listMsg struct {
					Summaries []struct {
						Metric      string  `json:"metric"`
						SampleCount int     `json:"sample_count"`
						Min         float64 `json:"min"`
						Max         float64 `json:"max"`
						Average     float64 `json:"average"`
						Variance    float64 `json:"variance"`
					} `json:"summaries"`
				}
				if json.Unmarshal([]byte(val), &listMsg) == nil {
					for _, s := range listMsg.Summaries {
						summaries = append(summaries, hourlySummary{
							Metric:      s.Metric,
							SampleCount: s.SampleCount,
							Min:         s.Min,
							Max:         s.Max,
							Average:     s.Average,
							Variance:    s.Variance,
						})
					}
				}
			}
		}
	}

	activeHours, _ := rdb.SMembers(ctx, fmt.Sprintf("session:%s:hours", sessionID)).Result()
	if len(activeHours) == 0 {
		activeHours = []string{time.Now().UTC().Format("2006-01-02-15")}
	}

	type statsTracker struct {
		count int
		min   float64
		max   float64
		mean  float64
		m2    float64
	}
	trackers := make(map[string]*statsTracker)

	mergePart := func(mKey string, n2 int, min2, max2, mean2, var2 float64) {
		tr, ok := trackers[mKey]
		if !ok {
			tr = &statsTracker{
				count: n2,
				min:   min2,
				max:   max2,
				mean:  mean2,
				m2:    float64(n2) * var2,
			}
			trackers[mKey] = tr
			return
		}
		if n2 == 0 {
			return
		}
		n1 := tr.count
		n := n1 + n2
		tr.count = n
		if min2 < tr.min {
			tr.min = min2
		}
		if max2 > tr.max {
			tr.max = max2
		}
		mean1 := tr.mean
		m2_1 := tr.m2
		m2_2 := float64(n2) * var2
		delta := mean2 - mean1
		tr.mean = mean1 + delta*float64(n2)/float64(n)
		tr.m2 = m2_1 + m2_2 + delta*delta*float64(n1)*float64(n2)/float64(n)
	}

	for _, sum := range summaries {
		mergePart(sum.Metric, sum.SampleCount, sum.Min, sum.Max, sum.Average, sum.Variance)
	}

	for _, hourStr := range activeHours {
		key := fmt.Sprintf("session:%s:hour:%s:device:%s:telemetry_history", sessionID, hourStr, deviceID)
		items, err := rdb.ZRange(ctx, key, 0, -1).Result()
		if err == nil && len(items) > 0 {
			rawMetrics := make(map[string][]float64)
			for _, item := range items {
				var sample struct {
					Metrics json.RawMessage `json:"metrics"`
				}
				if json.Unmarshal([]byte(item), &sample) == nil {
					var rawMetricsLocal map[string]interface{}
					if json.Unmarshal(sample.Metrics, &rawMetricsLocal) == nil {
						numerics := make(map[string]float64)
						flattenLocalQuery(rawMetricsLocal, "", numerics)
						for mKey, mVal := range numerics {
							rawMetrics[mKey] = append(rawMetrics[mKey], mVal)
						}
					}
				}
			}

			for mKey, vals := range rawMetrics {
				n2 := len(vals)
				if n2 > 0 {
					min2 := vals[0]
					max2 := vals[0]
					sum2 := 0.0
					for _, v := range vals {
						if v < min2 { min2 = v }
						if v > max2 { max2 = v }
						sum2 += v
					}
					mean2 := sum2 / float64(n2)
					var2 := 0.0
					for _, v := range vals {
						var2 += (v - mean2) * (v - mean2)
					}
					var2 = var2 / float64(n2)

					mergePart(mKey, n2, min2, max2, mean2, var2)
				}
			}
		}
	}

	var trends []operations.MetricTrend
	for mKey, tr := range trackers {
		if tr.count > 0 {
			variance := tr.m2 / float64(tr.count)
			if variance != variance { // NaN check
				variance = 0.0
			}

			trends = append(trends, operations.MetricTrend{
				Metric:   mKey,
				Current:  latest[mKey],
				Count:    int64(tr.count),
				Minimum:  tr.min,
				Maximum:  tr.max,
				Average:  tr.mean,
				Variance: variance,
			})
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

