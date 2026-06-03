package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vishalss1/argus/internal/domain/finding"
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

	// 2. Fetch rolling aggregates from Redis & compute metrics
	durationSec := 0
	if sess.StartedAt != nil {
		durationSec = int(now.Sub(*sess.StartedAt).Seconds())
	}
	if durationSec < 0 {
		durationSec = 0
	}

	artifactGenStart := time.Now()

	// Fetch all participating devices first
	devices, _ := m.redisClient.Client().SMembers(ctx, fmt.Sprintf("session:%s:devices", id)).Result()
	deviceCount := len(devices)

	// Build Pipeline 1: Fetch all device and session top-level metrics in 1 RTT
	pipe := m.redisClient.Client().Pipeline()

	// Session-wide metrics
	sessCountCmd := pipe.Get(ctx, fmt.Sprintf("session:%s:metrics:count", id))
	sessBatCountCmd := pipe.Get(ctx, fmt.Sprintf("session:%s:metrics:battery:count", id))
	sessBatSumCmd := pipe.Get(ctx, fmt.Sprintf("session:%s:metrics:battery:sum", id))
	sessBatMinCmd := pipe.Get(ctx, fmt.Sprintf("session:%s:metrics:battery:min", id))
	sessBatMaxCmd := pipe.Get(ctx, fmt.Sprintf("session:%s:metrics:battery:max", id))
	sessTempCountCmd := pipe.Get(ctx, fmt.Sprintf("session:%s:metrics:temp:count", id))
	sessTempSumCmd := pipe.Get(ctx, fmt.Sprintf("session:%s:metrics:temp:sum", id))
	sessTempMinCmd := pipe.Get(ctx, fmt.Sprintf("session:%s:metrics:temp:min", id))
	sessTempMaxCmd := pipe.Get(ctx, fmt.Sprintf("session:%s:metrics:temp:max", id))
	sessDistCmd := pipe.Get(ctx, fmt.Sprintf("session:%s:metrics:distance", id))

	// Device-specific metrics mapping
	type devCmds struct {
		firstSeen   *goredis.StringCmd
		lastSeen    *goredis.StringCmd
		sampleCount *goredis.StringCmd
		batSum      *goredis.StringCmd
		batCount    *goredis.StringCmd
		batMin      *goredis.StringCmd
		batMax      *goredis.StringCmd
		tempSum     *goredis.StringCmd
		tempCount   *goredis.StringCmd
		tempMin     *goredis.StringCmd
		tempMax     *goredis.StringCmd
		sigSum      *goredis.StringCmd
		sigCount    *goredis.StringCmd
		sigMin      *goredis.StringCmd
		sigMax      *goredis.StringCmd
		dist        *goredis.StringCmd
		rollupMins  *goredis.StringSliceCmd
	}

	devCmdMap := make(map[string]devCmds)
	for _, devID := range devices {
		devCmdMap[devID] = devCmds{
			firstSeen:   pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:first_seen", id, devID)),
			lastSeen:    pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:last_seen", id, devID)),
			sampleCount: pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:sample_count", id, devID)),
			batSum:      pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:battery:sum", id, devID)),
			batCount:    pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:battery:count", id, devID)),
			batMin:      pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:battery:min", id, devID)),
			batMax:      pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:battery:max", id, devID)),
			tempSum:     pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:temp:sum", id, devID)),
			tempCount:   pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:temp:count", id, devID)),
			tempMin:     pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:temp:min", id, devID)),
			tempMax:     pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:temp:max", id, devID)),
			sigSum:      pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:signal:sum", id, devID)),
			sigCount:    pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:signal:count", id, devID)),
			sigMin:      pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:signal:min", id, devID)),
			sigMax:      pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:signal:max", id, devID)),
			dist:        pipe.Get(ctx, fmt.Sprintf("session:%s:device:%s:distance", id, devID)),
			rollupMins:  pipe.SMembers(ctx, fmt.Sprintf("session:%s:device:%s:rollup_minutes", id, devID)),
		}
	}

	_, _ = pipe.Exec(ctx)

	cmdFloatVal := func(c *goredis.StringCmd) float64 {
		val, err := c.Result()
		if err != nil {
			return 0.0
		}
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}

	cmdIntVal := func(c *goredis.StringCmd) int {
		val, err := c.Result()
		if err != nil {
			return 0
		}
		i, _ := strconv.Atoi(val)
		return i
	}

	msgCount := cmdIntVal(sessCountCmd)
	batteryCount := cmdFloatVal(sessBatCountCmd)
	batterySum := cmdFloatVal(sessBatSumCmd)
	avgBattery := 0.0
	if batteryCount > 0 {
		avgBattery = batterySum / batteryCount
	}
	minBattery := cmdFloatVal(sessBatMinCmd)
	maxBattery := cmdFloatVal(sessBatMaxCmd)

	tempCount := cmdFloatVal(sessTempCountCmd)
	tempSum := cmdFloatVal(sessTempSumCmd)
	avgTemp := 0.0
	if tempCount > 0 {
		avgTemp = tempSum / tempCount
	}
	minTemp := cmdFloatVal(sessTempMinCmd)
	maxTemp := cmdFloatVal(sessTempMaxCmd)
	distance := cmdFloatVal(sessDistCmd)

	// Fetch database counts for alerts, commands, anomalies
	alerts, _ := m.sessionService.repo.ListAlertsBySession(ctx, id)
	commands, _ := m.sessionService.repo.ListCommandsBySession(ctx, id)
	events, _ := m.sessionService.repo.ListEventsBySession(ctx, id)

	alertCount := len(alerts)
	commandCount := len(commands)
	anomalyCount := len(events)

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
		cmds := devCmdMap[devID]
		firstSeen := cmdIntVal(cmds.firstSeen)
		lastSeen := cmdIntVal(cmds.lastSeen)
		sampleCount := cmdIntVal(cmds.sampleCount)

		uptimePercentage := 100.0
		if durationSec > 0 && lastSeen >= firstSeen {
			uptimePercentage = float64(lastSeen-firstSeen) / float64(durationSec) * 100.0
			if uptimePercentage > 100.0 {
				uptimePercentage = 100.0
			}
		}

		batSum := cmdFloatVal(cmds.batSum)
		batCount := cmdFloatVal(cmds.batCount)
		batMin := cmdFloatVal(cmds.batMin)
		batMax := cmdFloatVal(cmds.batMax)
		batAvg := 0.0
		if batCount > 0 {
			batAvg = batSum / batCount
		}

		tempSum := cmdFloatVal(cmds.tempSum)
		tempCount := cmdFloatVal(cmds.tempCount)
		tempMin := cmdFloatVal(cmds.tempMin)
		tempMax := cmdFloatVal(cmds.tempMax)
		tempAvg := 0.0
		if tempCount > 0 {
			tempAvg = tempSum / tempCount
		}

		sigSum := cmdFloatVal(cmds.sigSum)
		sigCount := cmdFloatVal(cmds.sigCount)
		sigMin := cmdFloatVal(cmds.sigMin)
		sigMax := cmdFloatVal(cmds.sigMax)
		sigAvg := 0.0
		if sigCount > 0 {
			sigAvg = sigSum / sigCount
		}

		devDistance := cmdFloatVal(cmds.dist)

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

	// Add alerts, commands, findings, anomalies to timeline
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

	for _, f := range findingArchives {
		devIDCopy := f.DeviceID
		timeline = append(timeline, TimelineEntry{
			Timestamp: f.Timestamp,
			Type:      "AI Finding Generated",
			DeviceID:  &devIDCopy,
			Message:   fmt.Sprintf("AI Finding (%s): %s", f.Severity, f.Recommendation),
		})
	}

	for _, e := range events {
		timeline = append(timeline, TimelineEntry{
			Timestamp: e.CreatedAt.Format(time.RFC3339),
			Type:      "Anomaly Detected",
			DeviceID:  &e.DeviceID,
			Message:   fmt.Sprintf("Anomaly detected (%s): %s", e.Severity, e.Type),
		})
	}

	timeline = append(timeline, TimelineEntry{
		Timestamp: now.Format(time.RFC3339),
		Type:      "Session Completed",
		Message:   fmt.Sprintf("Operational session %s stopped with status %s.", id, status),
	})

	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp < timeline[j].Timestamp
	})

	// Build Pipeline 2: Fetch all Minute Rollups in 1 RTT
	rollupPipe := m.redisClient.Client().Pipeline()
	type rollupKeyCmd struct {
		devID     string
		minuteStr string
		cmd       *goredis.MapStringStringCmd
	}
	var rollupCmds []rollupKeyCmd

	for _, devID := range devices {
		minutes, _ := devCmdMap[devID].rollupMins.Result()
		for _, minute := range minutes {
			rKey := fmt.Sprintf("session:%s:device:%s:rollup:%s", id, devID, minute)
			rollupCmds = append(rollupCmds, rollupKeyCmd{
				devID:     devID,
				minuteStr: minute,
				cmd:       rollupPipe.HGetAll(ctx, rKey),
			})
		}
	}

	_, _ = rollupPipe.Exec(ctx)

	rollupsMap := make(map[string][]TelemetryRollup)
	for _, rc := range rollupCmds {
		rollupData, err := rc.cmd.Result()
		if err != nil || len(rollupData) == 0 {
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

		batSum := getFloat("battery:sum")
		batCount := getFloat("battery:count")
		batMin := getFloat("battery:min")
		batMax := getFloat("battery:max")
		batAvg := 0.0
		if batCount > 0 {
			batAvg = batSum / batCount
		}

		tempSum := getFloat("temp:sum")
		tempCount := getFloat("temp:count")
		tempMin := getFloat("temp:min")
		tempMax := getFloat("temp:max")
		tempAvg := 0.0
		if tempCount > 0 {
			tempAvg = tempSum / tempCount
		}

		sigSum := getFloat("signal:sum")
		sigCount := getFloat("signal:count")
		sigMin := getFloat("signal:min")
		sigMax := getFloat("signal:max")
		sigAvg := 0.0
		if sigCount > 0 {
			sigAvg = sigSum / sigCount
		}

		rollupsMap[rc.devID] = append(rollupsMap[rc.devID], TelemetryRollup{
			Timestamp:      rc.minuteStr,
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

	for devID := range rollupsMap {
		sort.Slice(rollupsMap[devID], func(i, j int) bool {
			return rollupsMap[devID][i].Timestamp < rollupsMap[devID][j].Timestamp
		})
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

	// Record observability metrics
	SessionArtifactGenerationDurationSeconds.Observe(time.Since(artifactGenStart).Seconds())
	SessionArtifactSizeBytes.Observe(float64(len(artifactBytes)))

	// 5. Cleanup Redis State (Gather all keys to delete in a single batch)
	var keysToDelete []string
	keysToDelete = append(keysToDelete,
		fmt.Sprintf("session:%s:active", id),
		fmt.Sprintf("workspace:%s:active_session", stopped.WorkspaceID),
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
		keysToDelete = append(keysToDelete,
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
			fmt.Sprintf("session:%s:device:%s:rollup_minutes", id, devID),
		)
		minutes, _ := devCmdMap[devID].rollupMins.Result()
		for _, minute := range minutes {
			keysToDelete = append(keysToDelete, fmt.Sprintf("session:%s:device:%s:rollup:%s", id, devID, minute))
		}
	}

	// Bulk delete cached device:*:latest hot keys
	var latestKeys []string
	keysPattern := "device:*:latest"
	var cursor uint64
	for {
		keys, nextCursor, err := m.redisClient.Client().Scan(ctx, cursor, keysPattern, 100).Result()
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
		wsPipe := m.redisClient.Client().Pipeline()
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

	// Delete everything in batches of 1000 using a single pipeline
	if len(keysToDelete) > 0 {
		delPipe := m.redisClient.Client().Pipeline()
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
