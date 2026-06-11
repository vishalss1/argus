package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/vishalss1/argus/core/internal/domain/usage"
	"github.com/vishalss1/argus/core/internal/domain/workspace"
	"github.com/vishalss1/argus/core/internal/infrastructure/redis"
	"github.com/vishalss1/argus/shared/common"
	pb "github.com/vishalss1/argus/shared/proto/telemetry"
)

type Manager struct {
	sessionService  *Service
	usageService    *usage.Service
	redisClient     *redis.Client
	workspaceRepo   workspace.Repository
	telemetryClient pb.TelemetryIntelligenceServiceClient
}

func NewManager(
	sessionService *Service,
	usageService *usage.Service,
	redisClient *redis.Client,
	workspaceRepo workspace.Repository,
	telemetryClient pb.TelemetryIntelligenceServiceClient,
) *Manager {
	return &Manager{
		sessionService:  sessionService,
		usageService:    usageService,
		redisClient:     redisClient,
		workspaceRepo:   workspaceRepo,
		telemetryClient: telemetryClient,
	}
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

	// 1. Enforce Subscription Limits
	tenantID := "00000000-0000-0000-0000-000000000000"
	currentPlan := usage.PlanFree // Hardcoded for prototype

	if err := m.usageService.CheckSessionLimit(ctx, tenantID, currentPlan); err != nil {
		return nil, fmt.Errorf("subscription limit reached: %w", err)
	}

	// 2. Transition DB status safely
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

	// 3. Initialize Redis state
	activeKey := fmt.Sprintf("session:%s:active", id)
	if err := m.redisClient.Client().Set(ctx, activeKey, "1", 0).Err(); err != nil {
		fmt.Printf("[SESSION MANAGER] Warning: failed to set Redis active key for session %s: %v\n", id, err)
	}

	if err := m.redisClient.Client().SAdd(ctx, "sessions:active", id).Err(); err != nil {
		fmt.Printf("[SESSION MANAGER] Warning: failed to add session %s to Redis active set: %v\n", id, err)
	}

	// 4. Record Usage
	_ = m.usageService.RecordSessionStarted(ctx, tenantID)

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

	// 3. Query Telemetry Service via gRPC to compile aggregates & closed incidents, and delete Redis keys
	var telemetryResponse *pb.SessionTelemetryResponse
	if m.telemetryClient != nil {
		resp, err := m.telemetryClient.GetSessionTelemetry(ctx, &pb.GetSessionTelemetryRequest{
			SessionId:   id,
			WorkspaceId: stopped.WorkspaceID,
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
		SessionID:         id,
		GeneratedAt:       now.Format(time.RFC3339),
		WorkspaceID:       stopped.WorkspaceID,
		ReportVersion:     "3.0",
		SessionSummary:    sessionSummary,
		DeviceSummaries:   deviceSummaries,
		IncidentsArchive:  incidentsArchive,
		MetricsAggregates: metricsAggregates,
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
