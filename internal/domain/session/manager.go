package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vishalss1/argus/internal/domain/usage"
	"github.com/vishalss1/argus/internal/infrastructure/redis"
)

type Manager struct {
	sessionService *Service
	usageService   *usage.Service
	redisClient    *redis.Client
}

func NewManager(sessionService *Service, usageService *usage.Service, redisClient *redis.Client) *Manager {
	return &Manager{
		sessionService: sessionService,
		usageService:   usageService,
		redisClient:    redisClient,
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
	deviceCount := int(m.redisClient.Client().SCard(ctx, fmt.Sprintf("session:%s:devices", id)).Val())

	// Fetch database counts for alerts, commands, anomalies
	alerts, _ := m.sessionService.repo.ListAlertsBySession(ctx, id)
	commands, _ := m.sessionService.repo.ListCommandsBySession(ctx, id)
	events, _ := m.sessionService.repo.ListEventsBySession(ctx, id)

	alertCount := len(alerts)
	commandCount := len(commands)
	anomalyCount := len(events)

	// Device participation & device uptime detailed list (for extensible metrics in report)
	devices, _ := m.redisClient.Client().SMembers(ctx, fmt.Sprintf("session:%s:devices", id)).Result()
	devicesUptime := make(map[string]interface{})
	for _, devID := range devices {
		firstSeen := getRedisInt(fmt.Sprintf("session:%s:device:%s:first_seen", id, devID))
		lastSeen := getRedisInt(fmt.Sprintf("session:%s:device:%s:last_seen", id, devID))
		calculatedUptime := 0
		if lastSeen >= firstSeen {
			calculatedUptime = lastSeen - firstSeen
		}
		minUptime := getRedisFloat(fmt.Sprintf("session:%s:device:%s:uptime_s:min", id, devID))
		maxUptime := getRedisFloat(fmt.Sprintf("session:%s:device:%s:uptime_s:max", id, devID))

		devicesUptime[devID] = map[string]interface{}{
			"first_seen_ts":       firstSeen,
			"last_seen_ts":        lastSeen,
			"calculated_uptime_s": calculatedUptime,
			"min_reported_uptime": minUptime,
			"max_reported_uptime": maxUptime,
		}
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
			"devices_detail":             devicesUptime,
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
		m.redisClient.Client().Del(ctx,
			fmt.Sprintf("session:%s:device:%s:first_seen", id, devID),
			fmt.Sprintf("session:%s:device:%s:last_seen", id, devID),
			fmt.Sprintf("session:%s:device:%s:uptime_s:min", id, devID),
			fmt.Sprintf("session:%s:device:%s:uptime_s:max", id, devID),
			fmt.Sprintf("session:%s:device:%s:last_gps", id, devID),
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
