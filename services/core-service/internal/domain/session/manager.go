package session

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/vishalss1/argus/core/internal/domain/workspace"
	"github.com/vishalss1/argus/core/internal/infrastructure/minio"
	"github.com/vishalss1/argus/core/internal/infrastructure/redis"
	"github.com/vishalss1/argus/shared/common"
	pb "github.com/vishalss1/argus/shared/proto/telemetry"
)

type Manager struct {
	sessionService  *Service
	redisClient     *redis.Client
	workspaceRepo   workspace.Repository
	telemetryClient pb.TelemetryIntelligenceServiceClient
	minioClient     *minio.Client
}

func NewManager(
	sessionService *Service,
	redisClient *redis.Client,
	workspaceRepo workspace.Repository,
	telemetryClient pb.TelemetryIntelligenceServiceClient,
	minioClient *minio.Client,
) *Manager {
	return &Manager{
		sessionService:  sessionService,
		redisClient:     redisClient,
		workspaceRepo:   workspaceRepo,
		telemetryClient: telemetryClient,
		minioClient:     minioClient,
	}
}

func (m *Manager) TelemetryClient() pb.TelemetryIntelligenceServiceClient {
	return m.telemetryClient
}

func (m *Manager) RedisClient() *redis.Client {
	return m.redisClient
}

func (m *Manager) MinioClient() *minio.Client {
	return m.minioClient
}

func (m *Manager) StartSession(ctx context.Context, id string) (*Session, error) {
	// 0. Fetch session to ensure it exists
	sess, err := m.sessionService.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, errors.New("session not found")
	}

	if sess.Status == StatusRunning {
		return sess, nil // Idempotent start
	}

	if sess.Status != StatusCreated {
		return nil, fmt.Errorf("session cannot be started from status: %s", sess.Status)
	}

	// 1. Transition DB status safely
	now := time.Now().UTC()
	started, err := m.sessionService.repo.TransitionStatus(ctx, id, StatusCreated, StatusRunning, &now, nil)
	if err != nil {
		if errors.Is(err, ErrInvalidTransition) {
			SessionTransitionErrorsTotal.Inc()
			currentSess, getErr := m.sessionService.Get(ctx, id)
			if getErr == nil && currentSess != nil && currentSess.Status == StatusRunning {
				return currentSess, nil
			}
			return nil, fmt.Errorf("concurrent start conflict or invalid status")
		}
		return nil, err
	}

	SessionsStartedTotal.Inc()
	common.ActiveSessions.Inc()

	// 2. Initialize Redis state
	activeKey := fmt.Sprintf("session:%s:active", id)
	if err := m.redisClient.Client().Set(ctx, activeKey, "1", 0).Err(); err != nil {
		fmt.Printf("[SESSION MANAGER] Warning: failed to set Redis active key for session %s: %v\n", id, err)
	}

	if err := m.redisClient.Client().SAdd(ctx, "sessions:active", id).Err(); err != nil {
		fmt.Printf("[SESSION MANAGER] Warning: failed to add session %s to Redis active set: %v\n", id, err)
	}

	wsActiveKey := fmt.Sprintf("workspace:%s:active_session", started.WorkspaceID)
	_ = m.redisClient.Client().Set(ctx, wsActiveKey, started.ID, 0).Err()

	// Seed Redis device-to-workspace cache for all devices in the workspace
	devices, err := m.workspaceRepo.ListDevices(ctx, started.WorkspaceID)
	if err == nil {
		pipe := m.redisClient.Client().Pipeline()
		for _, dev := range devices {
			wsKey := fmt.Sprintf("device:%s:workspace", dev.ID)
			pipe.Set(ctx, wsKey, started.WorkspaceID, 24*time.Hour)
		}
		_, _ = pipe.Exec(ctx)
	} else {
		fmt.Printf("[SESSION MANAGER] Warning: failed to list workspace devices to seed Redis: %v\n", err)
	}

	return started, nil
}

func (m *Manager) StopSession(ctx context.Context, id string, success bool) (*Session, error) {
	stopStart := time.Now()
	defer func() {
		SessionStopDurationSeconds.Observe(time.Since(stopStart).Seconds())
	}()

	// 0. Fetch session to check idempotency
	sess, err := m.sessionService.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, errors.New("session not found")
	}

	if sess.Status == StatusCompleted || sess.Status == StatusFailed || sess.Status == StatusCancelled {
		return sess, nil // Idempotent stop
	}

	if sess.Status != StatusRunning {
		return nil, fmt.Errorf("session cannot be stopped from status: %s", sess.Status)
	}

	status := StatusCompleted
	if !success {
		status = StatusFailed
	}
	now := time.Now().UTC()

	// 1. Transition DB status safely
	stopped, err := m.sessionService.repo.TransitionStatus(ctx, id, StatusRunning, status, nil, &now)
	if err != nil {
		if errors.Is(err, ErrInvalidTransition) {
			SessionTransitionErrorsTotal.Inc()
			currentSess, getErr := m.sessionService.Get(ctx, id)
			if getErr == nil && currentSess != nil && (currentSess.Status == StatusCompleted || currentSess.Status == StatusFailed) {
				return currentSess, nil
			}
			return nil, fmt.Errorf("concurrent stop conflict or invalid status")
		}
		return nil, err
	}

	common.ActiveSessions.Dec()

	// 2. Signal stopped in Redis (blocks telemetry live consumer)
	rdb := m.redisClient.Client()
	stoppedKey := fmt.Sprintf("session:%s:stopped", id)
	rdb.Set(ctx, stoppedKey, "1", 60*time.Second)

	// Wait 1 second for live consumer pipeline drain
	select {
	case <-ctx.Done():
	case <-time.After(1 * time.Second):
	}

	// 3. Export full telemetry to MinIO before Redis cleanup
	exportPaths, exportErr := m.exportTelemetryToMinIO(ctx, id)
	if exportErr != nil {
		log.Printf("[SESSION STOP] Warning: telemetry export failed: %v", exportErr)
	}

	// 4. Query Telemetry Service via gRPC to compile aggregates & closed incidents, and delete Redis keys
	var telemetryResponse *pb.SessionTelemetryResponse
	if m.telemetryClient != nil {
		resp, err := m.telemetryClient.GetSessionTelemetry(ctx, &pb.GetSessionTelemetryRequest{
			SessionId:   id,
			WorkspaceId: stopped.WorkspaceID,
			Cleanup:     true,
		})
		if err == nil {
			telemetryResponse = resp
		} else {
			log.Printf("[SESSION STOP] Warning: Telemetry Service GetSessionTelemetry failed: %v", err)
		}
	}

	// Map gRPC response to DTOs
	deviceSummaries := make(map[string]DeviceSummaryArtifact)
	var incidentsArchive []ArtifactIncident
	metricsAggregates := make(map[string]map[string]MetricAggregate)
	hourlySummaries := make(map[string][]HourlySummaryArtifact)
	var sampleCountTotal int = 0
	var anomalyCount int = 0

	if telemetryResponse != nil {
		sampleCountTotal = int(telemetryResponse.MessagesProcessed)
		anomalyCount = int(telemetryResponse.AnomalyCount)

		for _, summary := range telemetryResponse.DeviceSummaries {
			deviceSummaries[summary.DeviceId] = DeviceSummaryArtifact{
				DeviceID:               summary.DeviceId,
				FirstSeen:              summary.FirstSeen,
				LastSeen:               summary.LastSeen,
				SampleCount:            int(summary.SampleCount),
				WarningIncidentsCount:  int(summary.WarningIncidentsCount),
				CriticalIncidentsCount: int(summary.CriticalIncidentsCount),
				ActiveAtEnd:            summary.ActiveAtEnd,
			}
		}

		for _, inc := range telemetryResponse.IncidentsArchive {
			var resolvedAt *time.Time
			if inc.ResolvedAt != nil {
				t := inc.ResolvedAt.AsTime()
				resolvedAt = &t
			}
			incidentsArchive = append(incidentsArchive, ArtifactIncident{
				DeviceID:     inc.DeviceId,
				Metric:       inc.Metric,
				IncidentType: inc.IncidentType,
				Severity:     inc.Severity,
				StartTime:    inc.StartTime.AsTime(),
				ResolvedAt:   resolvedAt,
				Occurrences:  int(inc.Occurrences),
				PeakScore:    inc.PeakScore,
				Summary:      inc.Summary,
			})
		}

		for devID, metricAggs := range telemetryResponse.MetricsAggregates {
			devAggs := make(map[string]MetricAggregate)
			for metricName, agg := range metricAggs.Aggregates {
				devAggs[metricName] = MetricAggregate{
					Count:    int(agg.Count),
					Min:      agg.Min,
					Max:      agg.Max,
					Average:  agg.Average,
					Variance: agg.Variance,
				}
			}
			metricsAggregates[devID] = devAggs
		}

		for devID, listMsg := range telemetryResponse.HourlySummaries {
			var devSummaries []HourlySummaryArtifact
			for _, sum := range listMsg.Summaries {
				devSummaries = append(devSummaries, HourlySummaryArtifact{
					DeviceID:       sum.DeviceId,
					Hour:           sum.Hour,
					Metric:         sum.Metric,
					SampleCount:    int(sum.SampleCount),
					Min:            sum.Min,
					Max:            sum.Max,
					Average:        sum.Average,
					Variance:       sum.Variance,
					Stddev:         sum.Stddev,
					FirstTimestamp: sum.FirstTimestamp.AsTime(),
					LastTimestamp:  sum.LastTimestamp.AsTime(),
				})
			}
			hourlySummaries[devID] = devSummaries
		}

	}

	// Calculate session statistics
	durationSec := 0
	if sess.StartedAt != nil {
		durationSec = int(now.Sub(*sess.StartedAt).Seconds())
	}
	if durationSec < 0 {
		durationSec = 0
	}

	alerts, _ := m.sessionService.repo.ListAlertsBySession(ctx, id)
	commands, _ := m.sessionService.repo.ListCommandsBySession(ctx, id)
	alertCount := len(alerts)
	commandCount := len(commands)

	stats := Statistics{
		SessionID:                id,
		DurationSeconds:          durationSec,
		MessagesProcessed:        sampleCountTotal,
		AlertsCount:              alertCount,
		CriticalEvents:           0,
		UptimePercentage:         100.0,
		AvgLatencyMS:             0.0,
		DeviceParticipationCount: len(deviceSummaries),
		CommandCount:             commandCount,
		AnomalyCount:             anomalyCount,
		UpdatedAt:                now,
	}
	_ = m.sessionService.repo.UpsertStatistics(ctx, stats)

	// Create AI Session Summary
	var sessionSummary string
	if len(incidentsArchive) == 0 {
		sessionSummary = fmt.Sprintf("AI Session Summary\n\nAnalyzed fleet metrics. Observed stable behavior with 0 incidents.")
	} else {
		var summaries []string
		for _, inc := range incidentsArchive {
			statusStr := "resolved"
			if inc.ResolvedAt == nil {
				statusStr = "active"
			}
			summaries = append(summaries, fmt.Sprintf("- %s on %s (%s)", inc.Summary, inc.DeviceID, statusStr))
		}
		sessionSummary = fmt.Sprintf("AI Session Summary\n\nDetected %d incidents:\n\n%s", len(incidentsArchive), summaries)
	}

	// Save final compiled report in postgres
	artifactPayload := SessionArtifactPayload{
		SessionID:            id,
		GeneratedAt:          now.Format(time.RFC3339),
		WorkspaceID:          stopped.WorkspaceID,
		ReportVersion:        "3.0",
		SessionSummary:       sessionSummary,
		DeviceSummaries:      deviceSummaries,
		IncidentsArchive:     incidentsArchive,
		MetricsAggregates:    metricsAggregates,
		HourlySummaries:      hourlySummaries,
		TelemetryExportPaths: exportPaths,
	}
	artifactBytes, _ := json.Marshal(artifactPayload)
	artifact := Artifact{
		SessionID:     id,
		WorkspaceID:   stopped.WorkspaceID,
		GeneratedAt:   now,
		ReportVersion: "3.0",
		ArtifactJSON:  json.RawMessage(artifactBytes),
	}
	_, _ = m.sessionService.repo.CreateArtifact(ctx, artifact)

	// Cleanup Core's own active session Redis keys
	rdb.SRem(ctx, "sessions:active", id)
	rdb.Del(ctx,
		fmt.Sprintf("session:%s:active", id),
		fmt.Sprintf("workspace:%s:active_session", stopped.WorkspaceID),
	)

	// Re-verify stopped key survived cleanup
	rdb.Set(ctx, stoppedKey, "1", 60*time.Second)

	return stopped, nil
}

// rawTelemetryEntry holds a single telemetry sample from Redis
type rawTelemetryEntry struct {
	RecordedAt time.Time              `json:"recorded_at"`
	CreatedAt  time.Time              `json:"created_at"`
	DeviceID   string                 `json:"device_id"`
	Metrics    map[string]interface{} `json:"metrics"`
}

func flattenTelemetryMetrics(m map[string]interface{}, prefix string, out map[string]float64) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case float64:
			out[key] = val
		case int:
			out[key] = float64(val)
		case int64:
			out[key] = float64(val)
		case map[string]interface{}:
			flattenTelemetryMetrics(val, key, out)
		case []interface{}:
			out[key+".len"] = float64(len(val))
		}
	}
}

// readAllRawTelemetry reads all hourly telemetry partitions for a session from Redis.
func (m *Manager) readAllRawTelemetry(ctx context.Context, sessionID string) ([]rawTelemetryEntry, error) {
	rdb := m.redisClient.Client()
	hours, err := rdb.SMembers(ctx, fmt.Sprintf("session:%s:hours", sessionID)).Result()
	if err != nil {
		hours = nil
	}
	if len(hours) == 0 {
		hours = []string{time.Now().UTC().Format("2006-01-02-15")}
	}

	devices, _ := rdb.SMembers(ctx, fmt.Sprintf("session:%s:devices", sessionID)).Result()

	var entries []rawTelemetryEntry
	for _, hourStr := range hours {
		for _, devID := range devices {
			key := fmt.Sprintf("session:%s:hour:%s:device:%s:telemetry_history", sessionID, hourStr, devID)
			items, err := rdb.ZRange(ctx, key, 0, -1).Result()
			if err != nil || len(items) == 0 {
				continue
			}
			for _, item := range items {
				var raw struct {
					RecordedAt time.Time              `json:"recorded_at"`
					CreatedAt  time.Time              `json:"created_at"`
					DeviceID   string                 `json:"device_id"`
					Metrics    map[string]interface{} `json:"metrics"`
				}
				if err := json.Unmarshal([]byte(item), &raw); err != nil {
					continue
				}
				entries = append(entries, rawTelemetryEntry{
					RecordedAt: raw.RecordedAt,
					CreatedAt:  raw.CreatedAt,
					DeviceID:   raw.DeviceID,
					Metrics:    raw.Metrics,
				})
			}
		}
	}
	return entries, nil
}

// exportTelemetryToMinIO reads raw telemetry from Redis and archived hourly partitions from
// MinIO, generates a merged JSON.gz and CSV.gz, uploads them to MinIO, and returns the object keys.
func (m *Manager) exportTelemetryToMinIO(ctx context.Context, sessionID string) (*TelemetryExportPaths, error) {
	if m.minioClient == nil {
		log.Printf("[SESSION MANAGER] MinIO client unavailable, skipping telemetry export")
		return nil, nil
	}

	rdb := m.redisClient.Client()

	// 1. Read archived hourly partitions from MinIO (past hours compacted by compactor)
	type archiveSample struct {
		Timestamp string             `json:"timestamp"`
		DeviceID  string             `json:"device_id"`
		Metrics   map[string]float64 `json:"metrics"`
	}
	var allSamples []archiveSample

	manifestEntries, _ := rdb.LRange(ctx, fmt.Sprintf("session:%s:export_paths", sessionID), 0, -1).Result()
	for _, entryStr := range manifestEntries {
		var entry struct {
			Hour    string `json:"hour"`
			JSONKey string `json:"json_key"`
		}
		if err := json.Unmarshal([]byte(entryStr), &entry); err != nil || entry.JSONKey == "" {
			continue
		}
		obj, err := m.minioClient.GetObject(ctx, entry.JSONKey)
		if err != nil {
			log.Printf("[SESSION MANAGER] failed to get archived export %s: %v", entry.JSONKey, err)
			continue
		}
		gzReader, err := gzip.NewReader(obj)
		if err != nil {
			obj.Close()
			continue
		}
		data, err := io.ReadAll(gzReader)
		gzReader.Close()
		obj.Close()
		if err != nil {
			continue
		}
		var hourSamples []archiveSample
		if err := json.Unmarshal(data, &hourSamples); err != nil {
			continue
		}
		allSamples = append(allSamples, hourSamples...)
	}

	// 2. Read remaining (current) hour from Redis
	redisEntries, err := m.readAllRawTelemetry(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("read raw telemetry: %w", err)
	}
	for _, e := range redisEntries {
		ts := e.RecordedAt
		if ts.IsZero() {
			ts = e.CreatedAt
		}
		flat := make(map[string]float64)
		flattenTelemetryMetrics(e.Metrics, "", flat)
		allSamples = append(allSamples, archiveSample{
			Timestamp: ts.UTC().Format(time.RFC3339),
			DeviceID:  e.DeviceID,
			Metrics:   flat,
		})
	}

	if len(allSamples) == 0 {
		log.Printf("[SESSION MANAGER] no raw telemetry to export for session %s", sessionID)
		return nil, nil
	}

	// 3. Sort by timestamp
	sort.Slice(allSamples, func(i, j int) bool {
		return allSamples[i].Timestamp < allSamples[j].Timestamp
	})

	// 4. Discover all metric keys
	allMetricKeys := make(map[string]bool)
	for _, s := range allSamples {
		for k := range s.Metrics {
			allMetricKeys[k] = true
		}
	}
	var metricKeys []string
	for k := range allMetricKeys {
		metricKeys = append(metricKeys, k)
	}
	sort.Strings(metricKeys)

	prefix := fmt.Sprintf("session-exports/%s", sessionID)

	// --- Build JSON export ---
	jsonData, err := json.Marshal(allSamples)
	if err != nil {
		return nil, fmt.Errorf("marshal json export: %w", err)
	}

	// --- Build CSV export ---
	var csvBuf bytes.Buffer
	csvWriter := csv.NewWriter(&csvBuf)
	header := append([]string{"timestamp", "device_id"}, metricKeys...)
	if err := csvWriter.Write(header); err != nil {
		return nil, fmt.Errorf("csv header: %w", err)
	}
	for _, s := range allSamples {
		row := []string{s.Timestamp, s.DeviceID}
		for _, mk := range metricKeys {
			row = append(row, strconv.FormatFloat(s.Metrics[mk], 'f', -1, 64))
		}
		if err := csvWriter.Write(row); err != nil {
			return nil, fmt.Errorf("csv row: %w", err)
		}
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return nil, fmt.Errorf("csv flush: %w", err)
	}

	// --- Upload JSON.gz ---
	jsonKey := prefix + "/telemetry.json.gz"
	{
		var gzBuf bytes.Buffer
		gzWriter := gzip.NewWriter(&gzBuf)
		if _, err := gzWriter.Write(jsonData); err != nil {
			return nil, fmt.Errorf("gzip json: %w", err)
		}
		if err := gzWriter.Close(); err != nil {
			return nil, fmt.Errorf("gzip json close: %w", err)
		}
		if err := m.minioClient.PutObject(ctx, jsonKey, &gzBuf, int64(gzBuf.Len()), "application/gzip"); err != nil {
			return nil, fmt.Errorf("upload json export: %w", err)
		}
	}

	// --- Upload CSV.gz ---
	csvKey := prefix + "/telemetry.csv.gz"
	{
		var gzBuf bytes.Buffer
		gzWriter := gzip.NewWriter(&gzBuf)
		if _, err := gzWriter.Write(csvBuf.Bytes()); err != nil {
			return nil, fmt.Errorf("gzip csv: %w", err)
		}
		if err := gzWriter.Close(); err != nil {
			return nil, fmt.Errorf("gzip csv close: %w", err)
		}
		if err := m.minioClient.PutObject(ctx, csvKey, &gzBuf, int64(gzBuf.Len()), "application/gzip"); err != nil {
			return nil, fmt.Errorf("upload csv export: %w", err)
		}
	}

	return &TelemetryExportPaths{JSON: jsonKey, CSV: csvKey}, nil
}

// StartTelemetryExportCleaner runs a periodic background job that removes expired
// telemetry exports from MinIO and marks them as expired in the session artifact.
// Run every 12 hours. Idempotent. Missing objects are warnings, not errors.
func (m *Manager) StartTelemetryExportCleaner(ctx context.Context, retentionDays int, interval time.Duration) {
	if m.minioClient == nil {
		log.Printf("[EXPORT CLEANER] MinIO client unavailable, telemetry export cleanup disabled")
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.cleanExpiredExports(ctx, retentionDays)
			}
		}
	}()
}

func (m *Manager) cleanExpiredExports(ctx context.Context, retentionDays int) {
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	sessions, err := m.sessionService.repo.ListTerminalBefore(ctx, cutoff)
	if err != nil {
		log.Printf("[EXPORT CLEANER] failed to list terminal sessions: %v", err)
		return
	}

	for _, sess := range sessions {
		select {
		case <-ctx.Done():
			return
		default:
		}

		artifact, err := m.sessionService.repo.GetArtifactBySession(ctx, sess.ID)
		if err != nil || artifact == nil {
			continue
		}

		payload, err := ParseArtifactPayload(artifact.ArtifactJSON)
		if err != nil {
			continue
		}

		if payload.ExportsExpired || payload.TelemetryExportPaths == nil {
			continue // Already cleaned or no exports
		}

		// Delete JSON export
		if payload.TelemetryExportPaths.JSON != "" {
			if err := m.minioClient.DeleteObject(ctx, payload.TelemetryExportPaths.JSON); err != nil {
				log.Printf("[EXPORT CLEANER] Warning: failed to delete %s: %v", payload.TelemetryExportPaths.JSON, err)
			} else {
				log.Printf("Deleted telemetry exports for session %s (retention period exceeded %d days)", sess.ID, retentionDays)
			}
		}

		// Delete CSV export
		if payload.TelemetryExportPaths.CSV != "" {
			if err := m.minioClient.DeleteObject(ctx, payload.TelemetryExportPaths.CSV); err != nil {
				log.Printf("[EXPORT CLEANER] Warning: failed to delete %s: %v", payload.TelemetryExportPaths.CSV, err)
			} else {
				log.Printf("Deleted telemetry exports for session %s (retention period exceeded %d days)", sess.ID, retentionDays)
			}
		}

		// Mark artifact as expired
		payload.ExportsExpired = true
		payload.TelemetryExportPaths = nil

		updatedJSON, _ := json.Marshal(payload)
		artifact.ArtifactJSON = json.RawMessage(updatedJSON)
		artifact.GeneratedAt = time.Now().UTC()

		if err := m.sessionService.repo.UpdateArtifact(ctx, *artifact); err != nil {
			log.Printf("[EXPORT CLEANER] failed to update artifact for session %s: %v", sess.ID, err)
		}
	}
}

func (m *Manager) CleanupStaleSessions(ctx context.Context, timeout time.Duration) (int64, error) {
	return m.sessionService.CleanupStale(ctx, timeout)
}

func (m *Manager) RecoverActiveSessions(ctx context.Context) error {
	sessions, err := m.sessionService.repo.ListAllRunning(ctx)
	if err != nil {
		return fmt.Errorf("list running sessions: %w", err)
	}

	for _, s := range sessions {
		activeKey := fmt.Sprintf("session:%s:active", s.ID)
		_ = m.redisClient.Client().Set(ctx, activeKey, "1", 0).Err()
		_ = m.redisClient.Client().SAdd(ctx, "sessions:active", s.ID).Err()
		SessionRecoveryTotal.Inc()
		common.ActiveSessions.Inc()
	}

	fmt.Printf("[SESSION MANAGER] recovered %d active sessions to Redis\n", len(sessions))
	return nil
}
