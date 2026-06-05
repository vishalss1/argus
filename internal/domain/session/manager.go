package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/vishalss1/argus/internal/domain/usage"
	"github.com/vishalss1/argus/internal/domain/workspace"
	"github.com/vishalss1/argus/internal/infrastructure/redis"
	goredis "github.com/redis/go-redis/v9"
)

type Manager struct {
	sessionService *Service
	usageService   *usage.Service
	redisClient    *redis.Client
	workspaceRepo  workspace.Repository
}

func NewManager(
	sessionService *Service,
	usageService *usage.Service,
	redisClient *redis.Client,
	workspaceRepo workspace.Repository,
) *Manager {
	return &Manager{
		sessionService: sessionService,
		usageService:   usageService,
		redisClient:    redisClient,
		workspaceRepo:  workspaceRepo,
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
			// Another thread might have started it. Check again for idempotency.
			currentSess, getErr := m.sessionService.Get(ctx, id)
			if getErr == nil && currentSess != nil && currentSess.Status == StatusRunning {
				return currentSess, nil
			}
			return nil, fmt.Errorf("concurrent start conflict or invalid status")
		}
		return nil, err
	}

	SessionsStartedTotal.Inc()

	// 3. Initialize Redis state
	activeKey := fmt.Sprintf("session:%s:active", id)
	if err := m.redisClient.Client().Set(ctx, activeKey, "1", 0).Err(); err != nil {
		// Log the error but don't fail the overall start, reaper will catch it if telemetry isn't processed
		// or we can add a robust retry.
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

	// 2. Read runtime state from Redis
	rdb := m.redisClient.Client()

	// Fetch all participating devices
	devices, _ := rdb.SMembers(ctx, fmt.Sprintf("session:%s:devices", id)).Result()
	deviceCount := len(devices)

	// Fetch all metric keys recorded for this session from Redis
	metricKeys, _ := rdb.SMembers(ctx, fmt.Sprintf("session:%s:metrics", id)).Result()

	// Gather Device Summaries using Pipelined HGetAll
	deviceSummaries := make(map[string]DeviceSummaryArtifact)
	var sampleCountTotal int

	if len(devices) > 0 {
		statePipe := rdb.Pipeline()
		stateCmds := make(map[string]*goredis.MapStringStringCmd)
		for _, devID := range devices {
			devStateKey := fmt.Sprintf("session:%s:device:%s:state", id, devID)
			stateCmds[devID] = statePipe.HGetAll(ctx, devStateKey)
		}
		_, _ = statePipe.Exec(ctx)

		for _, devID := range devices {
			state, err := stateCmds[devID].Result()
			if err == nil && len(state) > 0 {
				firstSeenVal := state["first_seen"]
				lastSeenVal := state["last_seen"]
				samples, _ := strconv.Atoi(state["sample_count"])
				warnCount, _ := strconv.Atoi(state["warning_incidents_count"])
				critCount, _ := strconv.Atoi(state["critical_incidents_count"])
				worstSev := state["worst_severity"]

				sampleCountTotal += samples

				firstSeenStr := ""
				if fUnix, err := strconv.ParseInt(firstSeenVal, 10, 64); err == nil {
					firstSeenStr = time.Unix(fUnix, 0).UTC().Format(time.RFC3339)
				}
				lastSeenStr := ""
				if lUnix, err := strconv.ParseInt(lastSeenVal, 10, 64); err == nil {
					lastSeenStr = time.Unix(lUnix, 0).UTC().Format(time.RFC3339)
				}

				activeAtEnd := worstSev != "healthy" && (warnCount+critCount > 0)

				deviceSummaries[devID] = DeviceSummaryArtifact{
					DeviceID:              devID,
					FirstSeen:             firstSeenStr,
					LastSeen:              lastSeenStr,
					SampleCount:           samples,
					WarningIncidentsCount:  warnCount,
					CriticalIncidentsCount: critCount,
					ActiveAtEnd:           activeAtEnd,
				}
			}
		}
	}

	// Fetch Closed Incidents from Redis artifact buffer
	var incidentsArchive []ArtifactIncident
	bufferKey := fmt.Sprintf("session:%s:artifact_buffer", id)
	closedIncidentsStr, _ := rdb.LRange(ctx, bufferKey, 0, -1).Result()
	for _, incStr := range closedIncidentsStr {
		var closed struct {
			DeviceID     string    `json:"device_id"`
			Metric       string    `json:"metric"`
			IncidentType string    `json:"incident_type"`
			Severity     string    `json:"severity"`
			StartTime    time.Time `json:"start_time"`
			ResolvedAt   time.Time `json:"resolved_at"`
			Occurrences  int       `json:"occurrences"`
			PeakScore    float64   `json:"peak_score"`
			Summary      string    `json:"summary"`
		}
		if err := json.Unmarshal([]byte(incStr), &closed); err == nil {
			incidentsArchive = append(incidentsArchive, ArtifactIncident{
				DeviceID:     closed.DeviceID,
				Metric:       closed.Metric,
				IncidentType: closed.IncidentType,
				Severity:     closed.Severity,
				StartTime:    closed.StartTime,
				ResolvedAt:   &closed.ResolvedAt,
				Occurrences:  closed.Occurrences,
				PeakScore:    closed.PeakScore,
				Summary:      closed.Summary,
			})
		}
	}

	// Fetch Active Incidents (still open) from Redis
	incidentsSetKey := fmt.Sprintf("session:%s:incidents", id)
	activeIncidentKeys, _ := rdb.SMembers(ctx, incidentsSetKey).Result()
	if len(activeIncidentKeys) > 0 {
		vals, err := rdb.MGet(ctx, activeIncidentKeys...).Result()
		if err == nil {
			for _, v := range vals {
				if vStr, ok := v.(string); ok && vStr != "" {
					var open struct {
						DeviceID     string    `json:"device_id"`
						Metric       string    `json:"metric"`
						IncidentType string    `json:"incident_type"`
						Severity     string    `json:"severity"`
						StartTime    time.Time `json:"start_time"`
						LastSeen     time.Time `json:"last_seen"`
						Occurrences  int       `json:"occurrences"`
						PeakScore    float64   `json:"peak_score"`
						Summary      string    `json:"summary"`
					}
					if err := json.Unmarshal([]byte(vStr), &open); err == nil {
						incidentsArchive = append(incidentsArchive, ArtifactIncident{
							DeviceID:     open.DeviceID,
							Metric:       open.Metric,
							IncidentType: open.IncidentType,
							Severity:     open.Severity,
							StartTime:    open.StartTime,
							ResolvedAt:   nil, // still open
							Occurrences:  open.Occurrences,
							PeakScore:    open.PeakScore,
							Summary:      open.Summary,
						})
					}
				}
			}
		}
	}

	// Append Capacity Suppression synthetic entry if suppressed > 0
	suppressedCountStr, _ := rdb.Get(ctx, fmt.Sprintf("session:%s:incidents:suppressed", id)).Result()
	if suppressedCountStr != "" {
		if count, err := strconv.Atoi(suppressedCountStr); err == nil && count > 0 {
			incidentsArchive = append(incidentsArchive, ArtifactIncident{
				DeviceID:     "system",
				Metric:       "multiple",
				IncidentType: "capacity_exceeded",
				Severity:     "warning",
				StartTime:    now,
				ResolvedAt:   &now,
				Occurrences:  count,
				PeakScore:    0.0,
				Summary:      fmt.Sprintf("%d additional closed incidents were suppressed to protect artifact capacity.", count),
			})
		}
	}

	// Fetch Running Aggregates (Welford) from Redis using pipelined batches of 1000
	metricsAggregates := make(map[string]map[string]MetricAggregate)
	if len(devices) > 0 && len(metricKeys) > 0 {
		type aggCmdKey struct {
			devID string
			mKey  string
		}
		aggCmds := make(map[aggCmdKey]*goredis.MapStringStringCmd)
		aggPipe := rdb.Pipeline()

		count := 0
		for _, devID := range devices {
			for _, mKey := range metricKeys {
				welfordKey := fmt.Sprintf("session:%s:device:%s:metric:%s", id, devID, mKey)
				aggCmds[aggCmdKey{devID, mKey}] = aggPipe.HGetAll(ctx, welfordKey)
				count++
				if count%1000 == 0 {
					_, _ = aggPipe.Exec(ctx)
					aggPipe = rdb.Pipeline()
				}
			}
		}
		if count%1000 != 0 {
			_, _ = aggPipe.Exec(ctx)
		}

		for _, devID := range devices {
			devAggs := make(map[string]MetricAggregate)
			for _, mKey := range metricKeys {
				data, err := aggCmds[aggCmdKey{devID, mKey}].Result()
				if err == nil && len(data) > 0 {
					cnt, _ := strconv.Atoi(data["count"])
					if cnt > 0 {
						sumVal, _ := strconv.ParseFloat(data["sum"], 64)
						minVal, _ := strconv.ParseFloat(data["min"], 64)
						maxVal, _ := strconv.ParseFloat(data["max"], 64)
						m2Val, _ := strconv.ParseFloat(data["m2"], 64)

						avg := sumVal / float64(cnt)
						variance := m2Val / float64(cnt)
						if math.IsNaN(variance) || math.IsInf(variance, 0) {
							variance = 0.0
						}

						devAggs[mKey] = MetricAggregate{
							Count:    cnt,
							Min:      minVal,
							Max:      maxVal,
							Average:  avg,
							Variance: variance,
						}
					}
				}
			}
			if len(devAggs) > 0 {
				metricsAggregates[devID] = devAggs
			}
		}
	}

	// 3. Save Statistics
	durationSec := 0
	if sess.StartedAt != nil {
		durationSec = int(now.Sub(*sess.StartedAt).Seconds())
	}
	if durationSec < 0 {
		durationSec = 0
	}

	// Calculate alert and command counts from DB
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
		DeviceParticipationCount: deviceCount,
		CommandCount:             commandCount,
		AnomalyCount:             len(incidentsArchive),
		UpdatedAt:                now,
	}
	_ = m.sessionService.repo.UpsertStatistics(ctx, stats)

	// Create AI Session Summary
	var sessionSummary string
	if len(incidentsArchive) == 0 {
		sessionSummary = fmt.Sprintf("AI Session Summary\n\nAnalyzed %d metrics across %d devices. Observed stable behavior with 0 incidents.", len(metricKeys), deviceCount)
	} else {
		var summaries []string
		for _, inc := range incidentsArchive {
			statusStr := "resolved"
			if inc.ResolvedAt == nil {
				statusStr = "active"
			}
			summaries = append(summaries, fmt.Sprintf("- %s on %s (%s)", inc.Summary, inc.DeviceID, statusStr))
		}
		sessionSummary = fmt.Sprintf("AI Session Summary\n\nAnalyzed %d metrics across %d devices. Detected %d incidents:\n\n%s",
			len(metricKeys), deviceCount, len(incidentsArchive), strings.Join(summaries, "\n"))
	}

	// Save v3 Artifact (Postgres sole persistence)
	artifactPayload := SessionArtifactPayload{
		SessionID:         id,
		GeneratedAt:       now.Format(time.RFC3339),
		ReportVersion:     "3.0",
		WorkspaceID:       stopped.WorkspaceID,
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

	// Record observability metrics
	SessionArtifactGenerationDurationSeconds.Observe(time.Since(stopStart).Seconds())
	SessionArtifactSizeBytes.Observe(float64(len(artifactBytes)))

	// 5. Cleanup Redis State
	var keysToDelete []string
	keysToDelete = append(keysToDelete,
		fmt.Sprintf("session:%s:active", id),
		fmt.Sprintf("workspace:%s:active_session", stopped.WorkspaceID),
		fmt.Sprintf("session:%s:devices", id),
		fmt.Sprintf("session:%s:metrics", id),
		fmt.Sprintf("session:%s:incidents", id),
		fmt.Sprintf("session:%s:artifact_buffer", id),
		fmt.Sprintf("session:%s:incidents:suppressed", id),
	)

	for _, devID := range devices {
		keysToDelete = append(keysToDelete,
			fmt.Sprintf("session:%s:device:%s:state", id, devID),
		)
		for _, mKey := range metricKeys {
			keysToDelete = append(keysToDelete,
				fmt.Sprintf("session:%s:device:%s:metric:%s", id, devID, mKey),
				fmt.Sprintf("session:%s:device:%s:metric:%s:last", id, devID, mKey),
				fmt.Sprintf("session:%s:device:%s:incident:%s:numeric_spike", id, devID, mKey),
				fmt.Sprintf("session:%s:device:%s:incident:%s:numeric_drop", id, devID, mKey),
				fmt.Sprintf("session:%s:device:%s:incident:%s:numeric_stuck", id, devID, mKey),
				fmt.Sprintf("session:%s:device:%s:incident:%s:binary_toggle", id, devID, mKey),
				fmt.Sprintf("session:%s:device:%s:incident:%s:categorical_change", id, devID, mKey),
			)
		}
	}

	var latestKeys []string
	keysPattern := "device:*:latest"
	var cursor uint64
	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, keysPattern, 100).Result()
		if err != nil {
			break
		}
		latestKeys = append(latestKeys, keys...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if len(latestKeys) > 0 {
		wsPipe := rdb.Pipeline()
		wsCmds := make(map[string]*goredis.StringCmd)
		for _, k := range latestKeys {
			if strings.HasPrefix(k, "device:") && strings.HasSuffix(k, ":latest") {
				devID := strings.TrimSuffix(strings.TrimPrefix(k, "device:"), ":latest")
				wsKey := fmt.Sprintf("device:%s:workspace", devID)
				wsCmds[k] = wsPipe.Get(ctx, wsKey)
			}
		}
		_, _ = wsPipe.Exec(ctx)

		for k, cmd := range wsCmds {
			cachedWS, err := cmd.Result()
			if err == nil && cachedWS == stopped.WorkspaceID {
				keysToDelete = append(keysToDelete, k)
			}
		}
	}

	// Scan for any other keys matching session:{id}:* to ensure a complete cleanup (no leaks)
	var sessionKeys []string
	var sessionCursor uint64
	sessionPattern := fmt.Sprintf("session:%s:*", id)
	for {
		keys, nextCursor, err := rdb.Scan(ctx, sessionCursor, sessionPattern, 250).Result()
		if err == nil && len(keys) > 0 {
			sessionKeys = append(sessionKeys, keys...)
		}
		sessionCursor = nextCursor
		if sessionCursor == 0 {
			break
		}
	}
	keysToDelete = append(keysToDelete, sessionKeys...)

	// Deduplicate keysToDelete
	uniqueKeys := make(map[string]struct{})
	var dedupedKeys []string
	for _, k := range keysToDelete {
		if _, exists := uniqueKeys[k]; !exists {
			uniqueKeys[k] = struct{}{}
			dedupedKeys = append(dedupedKeys, k)
		}
	}
	keysToDelete = dedupedKeys

	// Remove session from active sessions set
	rdb.SRem(ctx, "sessions:active", id)

	// Delete all session keys in batches of 1000 using a single pipeline
	if len(keysToDelete) > 0 {
		delPipe := rdb.Pipeline()
		const batchSize = 1000
		for i := 0; i < len(keysToDelete); i += batchSize {
			end := i + batchSize
			if end > len(keysToDelete) {
				end = len(keysToDelete)
			}
			delPipe.Del(ctx, keysToDelete[i:end]...)
		}
		_, _ = delPipe.Exec(ctx)
		log.Printf("[SESSION STOP] cleaned up %d Redis keys for session %s", len(keysToDelete), id)
	}

	return stopped, nil
}

func (m *Manager) CleanupStaleSessions(ctx context.Context, timeout time.Duration) (int64, error) {
	// Not ideal coupling, but good enough for this prototype.
	// Ideally the repository provides this directly via the service.
	// Since the requirement says "add session cleanup", we'll just expose it via repo and call it.
	// We'll define a quick method on service to do this.
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
	}

	fmt.Printf("[SESSION MANAGER] recovered %d active sessions to Redis\n", len(sessions))
	return nil
}
