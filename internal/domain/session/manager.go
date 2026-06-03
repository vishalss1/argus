package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vishalss1/argus/internal/domain/finding"
	"github.com/vishalss1/argus/internal/domain/usage"
	"github.com/vishalss1/argus/internal/domain/workspace"
	"github.com/vishalss1/argus/internal/infrastructure/redis"
)

type Manager struct {
	sessionService *Service
	usageService   *usage.Service
	redisClient    *redis.Client
	workspaceRepo  workspace.Repository
	findingRepo    finding.Repository
}

func NewManager(
	sessionService *Service,
	usageService *usage.Service,
	redisClient *redis.Client,
	workspaceRepo workspace.Repository,
	findingRepo finding.Repository,
) *Manager {
	return &Manager{
		sessionService: sessionService,
		usageService:   usageService,
		redisClient:    redisClient,
		workspaceRepo:  workspaceRepo,
		findingRepo:    findingRepo,
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

	return started, nil
}

func (m *Manager) StopSession(ctx context.Context, id string, success bool) (*Session, error) {
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

	if success {
		SessionsCompletedTotal.Inc()
	} else {
		SessionsFailedTotal.Inc()
	}

	// 2. Fetch rolling aggregates from Redis & compute metrics
	durationSec := 0
	if sess.StartedAt != nil {
		durationSec = int(now.Sub(*sess.StartedAt).Seconds())
	}
	if durationSec < 0 {
		durationSec = 0
	}

	getRedisFloat := func(key string) float64 {
		val, err := m.redisClient.Client().Get(ctx, key).Result()
		if err != nil {
			return 0.0
		}
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}

	getRedisInt := func(key string) int {
		val, err := m.redisClient.Client().Get(ctx, key).Result()
		if err != nil {
			return 0
		}
		i, _ := strconv.Atoi(val)
		return i
	}

	msgCount := getRedisInt(fmt.Sprintf("session:%s:metrics:count", id))
	batteryCount := getRedisFloat(fmt.Sprintf("session:%s:metrics:battery:count", id))
	batterySum := getRedisFloat(fmt.Sprintf("session:%s:metrics:battery:sum", id))
	avgBattery := 0.0
	if batteryCount > 0 {
		avgBattery = batterySum / batteryCount
	}
	minBattery := getRedisFloat(fmt.Sprintf("session:%s:metrics:battery:min", id))
	maxBattery := getRedisFloat(fmt.Sprintf("session:%s:metrics:battery:max", id))

	tempCount := getRedisFloat(fmt.Sprintf("session:%s:metrics:temp:count", id))
	tempSum := getRedisFloat(fmt.Sprintf("session:%s:metrics:temp:sum", id))
	avgTemp := 0.0
	if tempCount > 0 {
		avgTemp = tempSum / tempCount
	}
	minTemp := getRedisFloat(fmt.Sprintf("session:%s:metrics:temp:min", id))
	maxTemp := getRedisFloat(fmt.Sprintf("session:%s:metrics:temp:max", id))

	distance := getRedisFloat(fmt.Sprintf("session:%s:metrics:distance", id))
	devices, _ := m.redisClient.Client().SMembers(ctx, fmt.Sprintf("session:%s:devices", id)).Result()
	deviceCount := len(devices)

	// Fetch database counts for alerts, commands, anomalies
	alerts, _ := m.sessionService.repo.ListAlertsBySession(ctx, id)
	commands, _ := m.sessionService.repo.ListCommandsBySession(ctx, id)
	events, _ := m.sessionService.repo.ListEventsBySession(ctx, id)

	alertCount := len(alerts)
	commandCount := len(commands)
	anomalyCount := len(events)

	// Device summaries, timeline, alerts, commands, findings, rollups payload generation
	deviceSummaries := make(map[string]DeviceSummaryReport)
	
	// Format Alerts
	var alertArchives []AlertArchive
	for _, a := range alerts {
		resState := "Active"
		if a.Resolved {
			resState = "Resolved"
		}
		alertArchives = append(alertArchives, AlertArchive{
			Timestamp:       a.CreatedAt.Format(time.RFC3339),
			Severity:        a.Severity,
			SourceDevice:    a.DeviceID,
			AlertType:       "Rule Violation",
			Message:         a.Message,
			ResolutionState: resState,
		})
	}

	// Format Commands
	var commandArchives []CommandArchive
	for _, c := range commands {
		var ackTime *string
		if c.CompletedAt != nil {
			tStr := c.CompletedAt.Format(time.RFC3339)
			ackTime = &tStr
		}
		commandArchives = append(commandArchives, CommandArchive{
			Timestamp:           c.IssuedAt.Format(time.RFC3339),
			TargetDevice:        c.DeviceID,
			Command:             c.Command,
			Status:              c.Status,
			AcknowledgementTime: ackTime,
		})
	}

	// AI Findings
	var findingArchives []AIFindingsArchive
	for _, devID := range devices {
		findings, _ := m.findingRepo.ListByDevice(ctx, devID)
		for _, f := range findings {
			// Check if finding is within session interval
			if (sess.StartedAt != nil && (f.CreatedAt.After(*sess.StartedAt) || f.CreatedAt.Equal(*sess.StartedAt))) && f.CreatedAt.Before(now) {
				findingArchives = append(findingArchives, AIFindingsArchive{
					Timestamp:       f.CreatedAt.Format(time.RFC3339),
					DeviceID:        f.DeviceID,
					FindingType:     "AI Anomaly Insight",
					Severity:        f.Severity,
					Recommendation:  f.Summary,
					ConfidenceScore: 1.0 - f.RiskScore,
				})
			}
		}
	}

	// Timeline construction
	var timeline []TimelineEntry
	startedAtStr := ""
	if sess.StartedAt != nil {
		startedAtStr = sess.StartedAt.Format(time.RFC3339)
	}
	timeline = append(timeline, TimelineEntry{
		Timestamp: startedAtStr,
		Type:      "Session Started",
		Message:   fmt.Sprintf("Operational session %s started in workspace %s.", id, sess.WorkspaceID),
	})

	for _, devID := range devices {
		firstSeen := getRedisInt(fmt.Sprintf("session:%s:device:%s:first_seen", id, devID))
		lastSeen := getRedisInt(fmt.Sprintf("session:%s:device:%s:last_seen", id, devID))
		sampleCount := getRedisInt(fmt.Sprintf("session:%s:device:%s:sample_count", id, devID))

		// uptime
		uptimePercentage := 100.0
		if durationSec > 0 && lastSeen >= firstSeen {
			uptimePercentage = float64(lastSeen-firstSeen) / float64(durationSec) * 100.0
			if uptimePercentage > 100.0 {
				uptimePercentage = 100.0
			}
		}

		// battery
		batSum := getRedisFloat(fmt.Sprintf("session:%s:device:%s:battery:sum", id, devID))
		batCount := getRedisFloat(fmt.Sprintf("session:%s:device:%s:battery:count", id, devID))
		batMin := getRedisFloat(fmt.Sprintf("session:%s:device:%s:battery:min", id, devID))
		batMax := getRedisFloat(fmt.Sprintf("session:%s:device:%s:battery:max", id, devID))
		batAvg := 0.0
		if batCount > 0 {
			batAvg = batSum / batCount
		}

		// temp
		tempSum := getRedisFloat(fmt.Sprintf("session:%s:device:%s:temp:sum", id, devID))
		tempCount := getRedisFloat(fmt.Sprintf("session:%s:device:%s:temp:count", id, devID))
		tempMin := getRedisFloat(fmt.Sprintf("session:%s:device:%s:temp:min", id, devID))
		tempMax := getRedisFloat(fmt.Sprintf("session:%s:device:%s:temp:max", id, devID))
		tempAvg := 0.0
		if tempCount > 0 {
			tempAvg = tempSum / tempCount
		}

		// signal
		sigSum := getRedisFloat(fmt.Sprintf("session:%s:device:%s:signal:sum", id, devID))
		sigCount := getRedisFloat(fmt.Sprintf("session:%s:device:%s:signal:count", id, devID))
		sigMin := getRedisFloat(fmt.Sprintf("session:%s:device:%s:signal:min", id, devID))
		sigMax := getRedisFloat(fmt.Sprintf("session:%s:device:%s:signal:max", id, devID))
		sigAvg := 0.0
		if sigCount > 0 {
			sigAvg = sigSum / sigCount
		}

		// distance
		devDistance := getRedisFloat(fmt.Sprintf("session:%s:device:%s:distance", id, devID))

		// counts
		warningCount := 0
		criticalCount := 0
		for _, a := range alerts {
			if a.DeviceID == devID {
				if a.Severity == "WARNING" || a.Severity == "warning" {
					warningCount++
				} else if a.Severity == "CRITICAL" || a.Severity == "critical" {
					criticalCount++
				}
			}
		}

		devCommandCount := 0
		for _, c := range commands {
			if c.DeviceID == devID {
				devCommandCount++
			}
		}

		devAnomalyCount := 0
		for _, e := range events {
			if e.DeviceID == devID {
				devAnomalyCount++
			}
		}

		// First seen event to timeline
		if firstSeen > 0 {
			timeline = append(timeline, TimelineEntry{
				Timestamp: time.Unix(int64(firstSeen), 0).UTC().Format(time.RFC3339),
				Type:      "Device Joined",
				DeviceID:  &devID,
				Message:   fmt.Sprintf("Device %s joined the active session.", devID),
			})
		}

		deviceSummaries[devID] = DeviceSummaryReport{
			DeviceID:           devID,
			FirstSeen:          time.Unix(int64(firstSeen), 0).UTC().Format(time.RFC3339),
			LastSeen:           time.Unix(int64(lastSeen), 0).UTC().Format(time.RFC3339),
			UptimePercentage:   uptimePercentage,
			SampleCount:        sampleCount,
			BatteryAverage:     batAvg,
			BatteryMin:         batMin,
			BatteryMax:         batMax,
			TemperatureAverage: tempAvg,
			TemperatureMin:     tempMin,
			TemperatureMax:     tempMax,
			SignalAverage:      sigAvg,
			SignalMin:          sigMin,
			SignalMax:          sigMax,
			DistanceTravelled:  devDistance,
			WarningCount:       warningCount,
			CriticalCount:      criticalCount,
			CommandsReceived:   devCommandCount,
			AnomaliesDetected:  devAnomalyCount,
		}
	}

	// Add alerts to timeline
	for _, a := range alerts {
		timeline = append(timeline, TimelineEntry{
			Timestamp: a.CreatedAt.Format(time.RFC3339),
			Type:      "Alert Triggered",
			DeviceID:  &a.DeviceID,
			Message:   fmt.Sprintf("Alert Triggered (%s): %s", a.Severity, a.Message),
		})
		if a.Resolved && a.ResolvedAt != nil {
			timeline = append(timeline, TimelineEntry{
				Timestamp: a.ResolvedAt.Format(time.RFC3339),
				Type:      "Alert Cleared",
				DeviceID:  &a.DeviceID,
				Message:   fmt.Sprintf("Alert cleared: %s", a.Message),
			})
		}
	}

	// Add commands to timeline
	for _, c := range commands {
		issuedByVal := "-"
		if c.IssuedBy != nil {
			issuedByVal = *c.IssuedBy
		}
		timeline = append(timeline, TimelineEntry{
			Timestamp: c.IssuedAt.Format(time.RFC3339),
			Type:      "Command Sent",
			DeviceID:  &c.DeviceID,
			Message:   fmt.Sprintf("Command dispatched: %s (Issued by: %s)", c.Command, issuedByVal),
		})
		if c.CompletedAt != nil {
			timeline = append(timeline, TimelineEntry{
				Timestamp: c.CompletedAt.Format(time.RFC3339),
				Type:      "Command Acknowledged",
				DeviceID:  &c.DeviceID,
				Message:   fmt.Sprintf("Command acknowledged status: %s", c.Status),
			})
		}
	}

	// Add findings to timeline
	for _, f := range findingArchives {
		devIDCopy := f.DeviceID
		timeline = append(timeline, TimelineEntry{
			Timestamp: f.Timestamp,
			Type:      "AI Finding Generated",
			DeviceID:  &devIDCopy,
			Message:   fmt.Sprintf("AI Finding (%s): %s", f.Severity, f.Recommendation),
		})
	}

	// Add anomalies to timeline
	for _, e := range events {
		timeline = append(timeline, TimelineEntry{
			Timestamp: e.CreatedAt.Format(time.RFC3339),
			Type:      "Anomaly Detected",
			DeviceID:  &e.DeviceID,
			Message:   fmt.Sprintf("Anomaly detected (%s): %s", e.Severity, e.Type),
		})
	}

	// Add session completed to timeline
	timeline = append(timeline, TimelineEntry{
		Timestamp: now.Format(time.RFC3339),
		Type:      "Session Completed",
		Message:   fmt.Sprintf("Operational session %s stopped with status %s.", id, status),
	})

	// Sort timeline chronologically
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp < timeline[j].Timestamp
	})

	// Rollups Construction
	rollupsMap := make(map[string][]TelemetryRollup)
	for _, devID := range devices {
		minutesKey := fmt.Sprintf("session:%s:device:%s:rollup_minutes", id, devID)
		minutes, _ := m.redisClient.Client().SMembers(ctx, minutesKey).Result()

		var deviceRollups []TelemetryRollup
		for _, minute := range minutes {
			rollupKey := fmt.Sprintf("session:%s:device:%s:rollup:%s", id, devID, minute)
			rollupData, _ := m.redisClient.Client().HGetAll(ctx, rollupKey).Result()
			if len(rollupData) == 0 {
				continue
			}

			getFloat := func(field string) float64 {
				v, _ := strconv.ParseFloat(rollupData[field], 64)
				return v
			}
			getInt := func(field string) int {
				v, _ := strconv.Atoi(rollupData[field])
				return v
			}

			sampleCount := getInt("sample_count")
			if sampleCount == 0 {
				continue
			}

			// battery
			batSum := getFloat("battery:sum")
			batCount := getFloat("battery:count")
			batMin := getFloat("battery:min")
			batMax := getFloat("battery:max")
			batAvg := 0.0
			if batCount > 0 {
				batAvg = batSum / batCount
			}

			// temp
			tempSum := getFloat("temp:sum")
			tempCount := getFloat("temp:count")
			tempMin := getFloat("temp:min")
			tempMax := getFloat("temp:max")
			tempAvg := 0.0
			if tempCount > 0 {
				tempAvg = tempSum / tempCount
			}

			// signal
			sigSum := getFloat("signal:sum")
			sigCount := getFloat("signal:count")
			sigMin := getFloat("signal:min")
			sigMax := getFloat("signal:max")
			sigAvg := 0.0
			if sigCount > 0 {
				sigAvg = sigSum / sigCount
			}

			deviceRollups = append(deviceRollups, TelemetryRollup{
				Timestamp:      minute,
				BatteryAvg:     batAvg,
				BatteryMin:     batMin,
				BatteryMax:     batMax,
				TemperatureAvg: tempAvg,
				TemperatureMin: tempMin,
				TemperatureMax: tempMax,
				SignalAvg:      sigAvg,
				SignalMin:      sigMin,
				SignalMax:      sigMax,
				SampleCount:    sampleCount,
			})
		}

		// Sort device rollups
		sort.Slice(deviceRollups, func(i, j int) bool {
			return deviceRollups[i].Timestamp < deviceRollups[j].Timestamp
		})

		rollupsMap[devID] = deviceRollups
	}

	// 3. Save Statistics
	stats := Statistics{
		SessionID:                id,
		DurationSeconds:          durationSec,
		MessagesProcessed:        msgCount,
		AlertsCount:              alertCount,
		CriticalEvents:           anomalyCount,
		UptimePercentage:         100.0,
		AvgLatencyMS:             0.0,
		AvgBattery:               avgBattery,
		MinBattery:               minBattery,
		MaxBattery:               maxBattery,
		AvgTemperature:           avgTemp,
		MinTemperature:           minTemp,
		MaxTemperature:           maxTemp,
		DistanceTravelled:        distance,
		DeviceParticipationCount: deviceCount,
		CommandCount:             commandCount,
		AnomalyCount:             anomalyCount,
		UpdatedAt:                now,
	}
	_ = m.sessionService.repo.UpsertStatistics(ctx, stats)

	// 4. Save Report (extensible JSONB representation)
	reportJSONData := map[string]interface{}{
		"session_id":        id,
		"generated_at":      now.Format(time.RFC3339),
		"duration":          durationSec,
		"report_version":    "1.0",
		"summary":           fmt.Sprintf("Session completed with %d device(s) participating, processing %d total telemetry samples.", deviceCount, msgCount),
		"aggregated_metrics": map[string]interface{}{
			"messages_processed":         msgCount,
			"device_participation_count": deviceCount,
			"battery_average":            avgBattery,
			"battery_min":                minBattery,
			"battery_max":                maxBattery,
			"temperature_average":        avgTemp,
			"temperature_min":            minTemp,
			"temperature_max":            maxTemp,
			"distance_travelled_km":      distance,
			"alert_count":                alertCount,
			"command_count":              commandCount,
			"anomaly_count":              anomalyCount,
		},
	}
	reportJSONBytes, _ := json.Marshal(reportJSONData)
	report := Report{
		ID:          uuid.New().String(),
		SessionID:   id,
		ReportJSON:  json.RawMessage(reportJSONBytes),
		GeneratedAt: now,
	}
	_, _ = m.sessionService.repo.CreateReport(ctx, report)

	// 4b. Save v2 Artifact
	artifactPayload := SessionArtifactPayload{
		SessionID:        id,
		GeneratedAt:      now.Format(time.RFC3339),
		ReportVersion:    "2.0",
		WorkspaceID:      stopped.WorkspaceID,
		SessionSummary:   fmt.Sprintf("Session completed with %d device(s) participating, processing %d total telemetry samples.", deviceCount, msgCount),
		DeviceSummaries:  deviceSummaries,
		Alerts:           alertArchives,
		Commands:         commandArchives,
		AIFindings:       findingArchives,
		Timeline:         timeline,
		TelemetryRollups: rollupsMap,
	}
	artifactBytes, _ := json.Marshal(artifactPayload)
	artifact := Artifact{
		SessionID:     id,
		WorkspaceID:   stopped.WorkspaceID,
		GeneratedAt:   now,
		ReportVersion: "2.0",
		ArtifactJSON:  json.RawMessage(artifactBytes),
	}
	_, _ = m.sessionService.repo.CreateArtifact(ctx, artifact)

	// 5. Cleanup Redis State
	activeKey := fmt.Sprintf("session:%s:active", id)
	_ = m.redisClient.Client().Del(ctx, activeKey).Err()
	_ = m.redisClient.Client().SRem(ctx, "sessions:active", id).Err()

	wsActiveKey := fmt.Sprintf("workspace:%s:active_session", stopped.WorkspaceID)
	_ = m.redisClient.Client().Del(ctx, wsActiveKey).Err()

	// 6. strictly session-centric pipeline cleanup: delete cached device:*:latest hot keys
	keysPattern := "device:*:latest"
	var cursor uint64
	for {
		keys, nextCursor, err := m.redisClient.Client().Scan(ctx, cursor, keysPattern, 100).Result()
		if err != nil {
			break
		}
		for _, k := range keys {
			if strings.HasPrefix(k, "device:") && strings.HasSuffix(k, ":latest") {
				devID := strings.TrimSuffix(strings.TrimPrefix(k, "device:"), ":latest")
				wsKey := fmt.Sprintf("device:%s:workspace", devID)
				cachedWS, err := m.redisClient.Client().Get(ctx, wsKey).Result()
				if err == nil && cachedWS == stopped.WorkspaceID {
					_ = m.redisClient.Client().Del(ctx, k).Err()
				}
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	// Delete other session rolling aggregate keys from Redis
	m.redisClient.Client().Del(ctx,
		fmt.Sprintf("session:%s:metrics:count", id),
		fmt.Sprintf("session:%s:metrics:battery:count", id),
		fmt.Sprintf("session:%s:metrics:battery:sum", id),
		fmt.Sprintf("session:%s:metrics:battery:min", id),
		fmt.Sprintf("session:%s:metrics:battery:max", id),
		fmt.Sprintf("session:%s:metrics:temp:count", id),
		fmt.Sprintf("session:%s:metrics:temp:sum", id),
		fmt.Sprintf("session:%s:metrics:temp:min", id),
		fmt.Sprintf("session:%s:metrics:temp:max", id),
		fmt.Sprintf("session:%s:metrics:distance", id),
		fmt.Sprintf("session:%s:devices", id),
	)
	for _, devID := range devices {
		minutesKey := fmt.Sprintf("session:%s:device:%s:rollup_minutes", id, devID)
		minutes, _ := m.redisClient.Client().SMembers(ctx, minutesKey).Result()
		for _, minute := range minutes {
			m.redisClient.Client().Del(ctx, fmt.Sprintf("session:%s:device:%s:rollup:%s", id, devID, minute))
		}
		m.redisClient.Client().Del(ctx,
			fmt.Sprintf("session:%s:device:%s:first_seen", id, devID),
			fmt.Sprintf("session:%s:device:%s:last_seen", id, devID),
			fmt.Sprintf("session:%s:device:%s:uptime_s:min", id, devID),
			fmt.Sprintf("session:%s:device:%s:uptime_s:max", id, devID),
			fmt.Sprintf("session:%s:device:%s:last_gps", id, devID),
			fmt.Sprintf("session:%s:device:%s:sample_count", id, devID),
			fmt.Sprintf("session:%s:device:%s:battery:sum", id, devID),
			fmt.Sprintf("session:%s:device:%s:battery:count", id, devID),
			fmt.Sprintf("session:%s:device:%s:battery:min", id, devID),
			fmt.Sprintf("session:%s:device:%s:battery:max", id, devID),
			fmt.Sprintf("session:%s:device:%s:temp:sum", id, devID),
			fmt.Sprintf("session:%s:device:%s:temp:count", id, devID),
			fmt.Sprintf("session:%s:device:%s:temp:min", id, devID),
			fmt.Sprintf("session:%s:device:%s:temp:max", id, devID),
			fmt.Sprintf("session:%s:device:%s:signal:sum", id, devID),
			fmt.Sprintf("session:%s:device:%s:signal:count", id, devID),
			fmt.Sprintf("session:%s:device:%s:signal:min", id, devID),
			fmt.Sprintf("session:%s:device:%s:signal:max", id, devID),
			fmt.Sprintf("session:%s:device:%s:distance", id, devID),
			minutesKey,
		)
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
