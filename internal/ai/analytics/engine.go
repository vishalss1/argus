package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/vishalss1/argus/internal/ai/anomaly"
	"github.com/vishalss1/argus/internal/domain/telemetry"
	"github.com/vishalss1/argus/internal/infrastructure/kafka"
)

var welfordScript = goredis.NewScript(`
	local hashKey = KEYS[1]
	local val = tonumber(ARGV[1])

	local data = redis.call('hmget', hashKey, 'count', 'sum', 'min', 'max', 'm2')
	local count = tonumber(data[1]) or 0
	local sum   = tonumber(data[2]) or 0
	local mn    = tonumber(data[3])
	local mx    = tonumber(data[4])
	local m2    = tonumber(data[5]) or 0

	local oldMean = 0
	if count > 0 then oldMean = sum / count end

	count = count + 1
	sum   = sum + val

	if not mn or val < mn then mn = val end
	if not mx or val > mx then mx = val end

	local newMean = sum / count
	m2 = m2 + (val - oldMean) * (val - newMean)

	redis.call('hmset', hashKey,
	    'count', count, 'sum', sum,
	    'min', mn, 'max', mx, 'm2', m2)
	return 1
`)

type cacheEntry struct {
	value     string
	createdAt time.Time
}

type Engine struct {
	mu            sync.Mutex
	redisClient   *goredis.Client
	kafkaProducer *kafka.Producer
	devices       map[string]map[string]*anomaly.RollingStats

	openIncidents         map[string]bool
	deviceWorkspaceCache  map[string]cacheEntry
	workspaceSessionCache map[string]cacheEntry
	sessionMetrics        map[string]map[string]bool
	lastValues            map[string]string
}

type ActiveIncident struct {
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

type ClosedIncident struct {
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

func NewEngine(redisClient *goredis.Client, kafkaProducer *kafka.Producer) *Engine {
	// Pre-load Lua script to enable pipelined EVALSHA
	_ = welfordScript.Load(context.Background(), redisClient).Err()

	return &Engine{
		redisClient:           redisClient,
		kafkaProducer:         kafkaProducer,
		devices:               make(map[string]map[string]*anomaly.RollingStats),
		openIncidents:         make(map[string]bool),
		deviceWorkspaceCache:  make(map[string]cacheEntry),
		workspaceSessionCache: make(map[string]cacheEntry),
		sessionMetrics:        make(map[string]map[string]bool),
		lastValues:            make(map[string]string),
	}
}

func flattenMetrics(m map[string]interface{}, prefix string, numerics map[string]float64, binaries map[string]bool, categoricals map[string]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case float64:
			numerics[key] = val
		case int:
			numerics[key] = float64(val)
		case int64:
			numerics[key] = float64(val)
		case bool:
			binaries[key] = val
		case string:
			categoricals[key] = val
		case map[string]interface{}:
			flattenMetrics(val, key, numerics, binaries, categoricals)
		case []interface{}:
			// Sequence: Count elements, track length changes
			numerics[key+".len"] = float64(len(val))
		}
	}
}

func (e *Engine) getActiveSessionID(ctx context.Context, deviceID string) (string, error) {
	e.mu.Lock()
	wsEntry, wsCached := e.deviceWorkspaceCache[deviceID]
	e.mu.Unlock()

	if !wsCached || time.Since(wsEntry.createdAt) > 5*time.Second {
		wsKey := fmt.Sprintf("device:%s:workspace", deviceID)
		val, err := e.redisClient.Get(ctx, wsKey).Result()
		if err != nil {
			return "", err
		}
		wsEntry = cacheEntry{value: val, createdAt: time.Now()}
		e.mu.Lock()
		e.deviceWorkspaceCache[deviceID] = wsEntry
		e.mu.Unlock()
	}

	workspaceID := wsEntry.value

	e.mu.Lock()
	sessEntry, sessCached := e.workspaceSessionCache[workspaceID]
	e.mu.Unlock()

	if !sessCached || time.Since(sessEntry.createdAt) > 5*time.Second {
		sessionKey := fmt.Sprintf("workspace:%s:active_session", workspaceID)
		val, err := e.redisClient.Get(ctx, sessionKey).Result()
		if err != nil {
			return "", err
		}
		sessEntry = cacheEntry{value: val, createdAt: time.Now()}
		e.mu.Lock()
		e.workspaceSessionCache[workspaceID] = sessEntry
		e.mu.Unlock()
	}

	return sessEntry.value, nil
}

func (e *Engine) Analyze(ctx context.Context, t telemetry.Telemetry) error {
	sessionID, err := e.getActiveSessionID(ctx, t.DeviceID)
	if err != nil {
		return nil
	}

	// Gate: if the session has been stopped, skip all writes.
	// The live consumer's SessionTrackPipeline LUA script also checks this key,
	// but the analytics engine runs in a separate process (ai-worker) and must
	// independently refuse late writes to keep artifact data consistent.
	stoppedKey := fmt.Sprintf("session:%s:stopped", sessionID)
	exists, err := e.redisClient.Exists(ctx, stoppedKey).Result()
	if err == nil && exists > 0 {
		return nil
	}

	var metrics map[string]interface{}
	if err := json.Unmarshal(t.Metrics, &metrics); err != nil {
		return err
	}

	numerics := make(map[string]float64)
	binaries := make(map[string]bool)
	categoricals := make(map[string]string)
	flattenMetrics(metrics, "", numerics, binaries, categoricals)

	e.mu.Lock()
	if e.devices == nil {
		e.devices = make(map[string]map[string]*anomaly.RollingStats)
	}
	devStats, ok := e.devices[t.DeviceID]
	if !ok {
		devStats = make(map[string]*anomaly.RollingStats)
		e.devices[t.DeviceID] = devStats
	}
	e.mu.Unlock()

	pipe := e.redisClient.Pipeline()

	historyKey := fmt.Sprintf("session:%s:device:%s:telemetry_history", sessionID, t.DeviceID)
	if telemetryJSON, marshalErr := json.Marshal(t); marshalErr == nil {
		scoreTime := t.RecordedAt
		if scoreTime.IsZero() {
			scoreTime = time.Now().UTC()
		}
		pipe.ZAdd(ctx, historyKey, goredis.Z{Score: float64(scoreTime.UnixMilli()), Member: string(telemetryJSON)})
		pipe.ZRemRangeByScore(ctx, historyKey, "-inf", fmt.Sprintf("%d", time.Now().Add(-24*time.Hour).UnixMilli()))
		pipe.Expire(ctx, historyKey, 25*time.Hour)
	}

	metricsIndexKey := fmt.Sprintf("session:%s:metrics", sessionID)

	// Process Numeric Metrics
	for metricKey, val := range numerics {
		// Cardinality protection check using in-memory cache
		e.mu.Lock()
		if e.sessionMetrics == nil {
			e.sessionMetrics = make(map[string]map[string]bool)
		}
		if e.sessionMetrics[sessionID] == nil {
			e.sessionMetrics[sessionID] = make(map[string]bool)
		}
		seen := e.sessionMetrics[sessionID][metricKey]
		if !seen {
			e.sessionMetrics[sessionID][metricKey] = true
		}
		e.mu.Unlock()

		if !seen {
			pipe.SAdd(ctx, metricsIndexKey, metricKey)
			pipe.Expire(ctx, metricsIndexKey, 24*time.Hour)
		}

		// Update running aggregates (Welford's Algorithm) via Lua script on pipeline
		welfordKey := fmt.Sprintf("session:%s:device:%s:metric:%s", sessionID, t.DeviceID, metricKey)
		welfordScript.Run(ctx, pipe, []string{welfordKey}, val)
		pipe.Expire(ctx, welfordKey, 24*time.Hour)

		// Push to in-memory RollingStats window for anomaly detection
		e.mu.Lock()
		stats, ok := devStats[metricKey]
		if !ok {
			stats = anomaly.NewRollingStats(30)
			devStats[metricKey] = stats
		}
		_, _, _, zScore, outlier, stuck := stats.Push(val)
		e.mu.Unlock()

		var isAnomaly bool
		var severity string
		var incidentType string
		var peakScore float64

		if stuck {
			isAnomaly = true
			severity = "warning"
			incidentType = "numeric_stuck"
			peakScore = 1.0
		} else if outlier {
			isAnomaly = true
			absZ := math.Abs(zScore)
			if absZ > 5.0 {
				severity = "critical"
			} else {
				severity = "warning"
			}
			if zScore > 0 {
				incidentType = "numeric_spike"
			} else {
				incidentType = "numeric_drop"
			}
			peakScore = absZ
		}

		incidentKey := fmt.Sprintf("session:%s:device:%s:incident:%s:%s", sessionID, t.DeviceID, metricKey, incidentType)

		if isAnomaly {
			// Flush current pipeline first before handling anomaly
			_, _ = pipe.Exec(ctx)
			pipe = e.redisClient.Pipeline()

			// Manage Open Incident
			err := e.handleOpenIncident(ctx, sessionID, t.DeviceID, metricKey, incidentType, severity, peakScore, incidentKey)
			if err != nil {
				fmt.Printf("[AI ANALYTICS] error opening incident: %v\n", err)
			}
		} else {
			// Check if any incident is currently open for this metric and close it
			for _, possibleType := range []string{"numeric_spike", "numeric_drop", "numeric_stuck"} {
				pKey := fmt.Sprintf("session:%s:device:%s:incident:%s:%s", sessionID, t.DeviceID, metricKey, possibleType)

				e.mu.Lock()
				isOpen := e.openIncidents[pKey]
				e.mu.Unlock()

				if isOpen {
					_, _ = pipe.Exec(ctx)
					pipe = e.redisClient.Pipeline()
					err := e.handleCloseIncident(ctx, sessionID, t.DeviceID, metricKey, possibleType, pKey)
					if err != nil {
						fmt.Printf("[AI ANALYTICS] error closing incident: %v\n", err)
					}
				}
			}
		}
	}

	// Process Binary Metrics
	for metricKey, val := range binaries {
		lastValueKey := fmt.Sprintf("session:%s:device:%s:metric:%s:last", sessionID, t.DeviceID, metricKey)
		valStr := fmt.Sprintf("%t", val)

		e.mu.Lock()
		lastValStr, cached := e.lastValues[lastValueKey]
		if !cached {
			var err error
			lastValStr, err = e.redisClient.Get(ctx, lastValueKey).Result()
			if err == nil {
				e.lastValues[lastValueKey] = lastValStr
				cached = true
			}
		}
		e.lastValues[lastValueKey] = valStr
		e.mu.Unlock()

		pipe.Set(ctx, lastValueKey, valStr, 24*time.Hour)

		incidentType := "binary_toggle"
		incidentKey := fmt.Sprintf("session:%s:device:%s:incident:%s:%s", sessionID, t.DeviceID, metricKey, incidentType)

		if cached {
			lastVal := lastValStr == "true"
			if lastVal != val {
				_, _ = pipe.Exec(ctx)
				pipe = e.redisClient.Pipeline()
				_ = e.handleOpenIncident(ctx, sessionID, t.DeviceID, metricKey, incidentType, "warning", 1.0, incidentKey)
			} else {
				e.mu.Lock()
				isOpen := e.openIncidents[incidentKey]
				e.mu.Unlock()
				if isOpen {
					_, _ = pipe.Exec(ctx)
					pipe = e.redisClient.Pipeline()
					_ = e.handleCloseIncident(ctx, sessionID, t.DeviceID, metricKey, incidentType, incidentKey)
				}
			}
		}
	}

	// Process Categorical Metrics
	for metricKey, val := range categoricals {
		lastValueKey := fmt.Sprintf("session:%s:device:%s:metric:%s:last", sessionID, t.DeviceID, metricKey)

		e.mu.Lock()
		lastValStr, cached := e.lastValues[lastValueKey]
		if !cached {
			var err error
			lastValStr, err = e.redisClient.Get(ctx, lastValueKey).Result()
			if err == nil {
				e.lastValues[lastValueKey] = lastValStr
				cached = true
			}
		}
		e.lastValues[lastValueKey] = val
		e.mu.Unlock()

		pipe.Set(ctx, lastValueKey, val, 24*time.Hour)

		incidentType := "categorical_change"
		incidentKey := fmt.Sprintf("session:%s:device:%s:incident:%s:%s", sessionID, t.DeviceID, metricKey, incidentType)

		if cached {
			if lastValStr != val {
				_, _ = pipe.Exec(ctx)
				pipe = e.redisClient.Pipeline()
				_ = e.handleOpenIncident(ctx, sessionID, t.DeviceID, metricKey, incidentType, "warning", 1.0, incidentKey)
			} else {
				e.mu.Lock()
				isOpen := e.openIncidents[incidentKey]
				e.mu.Unlock()
				if isOpen {
					_, _ = pipe.Exec(ctx)
					pipe = e.redisClient.Pipeline()
					_ = e.handleCloseIncident(ctx, sessionID, t.DeviceID, metricKey, incidentType, incidentKey)
				}
			}
		}
	}

	_, err = pipe.Exec(ctx)
	return err
}

func (e *Engine) handleOpenIncident(ctx context.Context, sessionID, deviceID, metric, incidentType, severity string, score float64, incidentKey string) error {
	e.mu.Lock()
	isAlreadyOpen := e.openIncidents[incidentKey]
	e.mu.Unlock()

	now := time.Now().UTC()

	if isAlreadyOpen {
		// Update incident entry
		data, err := e.redisClient.Get(ctx, incidentKey).Result()
		if err != nil {
			return err
		}

		var inc ActiveIncident
		if err := json.Unmarshal([]byte(data), &inc); err != nil {
			return err
		}

		inc.Occurrences++
		inc.LastSeen = now
		if score > inc.PeakScore {
			inc.PeakScore = score
		}
		inc.Summary = fmt.Sprintf("Metric '%s' %s detected. Peak value: %.2f", metric, incidentType, inc.PeakScore)

		payloadBytes, err := json.Marshal(inc)
		if err != nil {
			return err
		}

		return e.redisClient.Set(ctx, incidentKey, string(payloadBytes), 24*time.Hour).Err()
	}

	// Create new active incident entry
	summary := fmt.Sprintf("Metric '%s' %s detected. Score: %.2f", metric, incidentType, score)
	inc := ActiveIncident{
		DeviceID:     deviceID,
		Metric:       metric,
		IncidentType: incidentType,
		Severity:     severity,
		StartTime:    now,
		LastSeen:     now,
		Occurrences:  1,
		PeakScore:    score,
		Summary:      summary,
	}

	payloadBytes, err := json.Marshal(inc)
	if err != nil {
		return err
	}

	incidentsSetKey := fmt.Sprintf("session:%s:incidents", sessionID)
	deviceIncidentsSetKey := fmt.Sprintf("session:%s:device:%s:incidents", sessionID, deviceID)
	pipe := e.redisClient.Pipeline()
	pipe.Set(ctx, incidentKey, string(payloadBytes), 24*time.Hour)
	pipe.SAdd(ctx, incidentsSetKey, incidentKey)
	pipe.Expire(ctx, incidentsSetKey, 24*time.Hour)
	pipe.SAdd(ctx, deviceIncidentsSetKey, incidentKey)
	pipe.Expire(ctx, deviceIncidentsSetKey, 24*time.Hour)

	// Update device state count and worst severity
	devStateKey := fmt.Sprintf("session:%s:device:%s:state", sessionID, deviceID)
	pipe.HIncrBy(ctx, devStateKey, severity+"_incidents_count", 1)
	if severity == "critical" {
		pipe.HSet(ctx, devStateKey, "worst_severity", "critical")
	}
	_, _ = pipe.Exec(ctx)

	// Adjust worst_severity in Go to be safe
	currSev, _ := e.redisClient.HGet(ctx, devStateKey, "worst_severity").Result()
	if currSev != "critical" {
		e.redisClient.HSet(ctx, devStateKey, "worst_severity", severity)
	}

	e.mu.Lock()
	e.openIncidents[incidentKey] = true
	e.mu.Unlock()

	// Publish OPEN to Kafka
	if e.kafkaProducer != nil {
		return e.kafkaProducer.PublishIncident(ctx, kafka.IncidentEvent{
			DeviceID:     deviceID,
			SessionID:    sessionID,
			IncidentType: incidentType,
			Metric:       metric,
			Severity:     severity,
			Score:        score,
			Status:       "OPEN",
			Timestamp:    now,
		})
	}
	return nil
}

func (e *Engine) handleCloseIncident(ctx context.Context, sessionID, deviceID, metric, incidentType, incidentKey string) error {
	e.mu.Lock()
	isOpen := e.openIncidents[incidentKey]
	e.mu.Unlock()
	if !isOpen {
		return nil // Not open, nothing to do
	}

	data, err := e.redisClient.Get(ctx, incidentKey).Result()
	if err != nil {
		return err
	}

	var inc ActiveIncident
	if err := json.Unmarshal([]byte(data), &inc); err != nil {
		return err
	}

	now := time.Now().UTC()

	closedInc := ClosedIncident{
		DeviceID:     inc.DeviceID,
		Metric:       inc.Metric,
		IncidentType: inc.IncidentType,
		Severity:     inc.Severity,
		StartTime:    inc.StartTime,
		ResolvedAt:   now,
		Occurrences:  inc.Occurrences,
		PeakScore:    inc.PeakScore,
		Summary:      fmt.Sprintf("Metric '%s' %s resolved.", metric, incidentType),
	}

	closedJSON, err := json.Marshal(closedInc)
	if err != nil {
		return err
	}

	bufferKey := fmt.Sprintf("session:%s:artifact_buffer", sessionID)
	length, err := e.redisClient.LLen(ctx, bufferKey).Result()
	if err == nil && length >= 1000 {
		// Suppress! Increment counter
		e.redisClient.Incr(ctx, fmt.Sprintf("session:%s:incidents:suppressed", sessionID))
	} else {
		e.redisClient.RPush(ctx, bufferKey, string(closedJSON))
		e.redisClient.Expire(ctx, bufferKey, 24*time.Hour)
	}

	incidentsSetKey := fmt.Sprintf("session:%s:incidents", sessionID)
	deviceIncidentsSetKey := fmt.Sprintf("session:%s:device:%s:incidents", sessionID, deviceID)
	pipe := e.redisClient.Pipeline()
	pipe.SRem(ctx, incidentsSetKey, incidentKey)
	pipe.SRem(ctx, deviceIncidentsSetKey, incidentKey)
	pipe.Del(ctx, incidentKey)

	// Decrement active incident counts
	devStateKey := fmt.Sprintf("session:%s:device:%s:state", sessionID, deviceID)
	pipe.HIncrBy(ctx, devStateKey, inc.Severity+"_incidents_count", -1)
	_, _ = pipe.Exec(ctx)

	// Recalculate worst severity
	stateVals, err := e.redisClient.HMGet(ctx, devStateKey, "warning_incidents_count", "critical_incidents_count").Result()
	if err == nil && len(stateVals) == 2 {
		warnCount, _ := parseInterfaceToInt(stateVals[0])
		critCount, _ := parseInterfaceToInt(stateVals[1])

		worst := "healthy"
		if critCount > 0 {
			worst = "critical"
		} else if warnCount > 0 {
			worst = "warning"
		}
		e.redisClient.HSet(ctx, devStateKey, "worst_severity", worst)
	}

	e.mu.Lock()
	delete(e.openIncidents, incidentKey)
	e.mu.Unlock()

	// Publish CLOSE to Kafka
	if e.kafkaProducer != nil {
		return e.kafkaProducer.PublishIncident(ctx, kafka.IncidentEvent{
			DeviceID:     deviceID,
			SessionID:    sessionID,
			IncidentType: incidentType,
			Metric:       metric,
			Severity:     inc.Severity,
			Score:        inc.PeakScore,
			Status:       "CLOSE",
			Timestamp:    now,
		})
	}
	return nil
}

func parseInterfaceToInt(val interface{}) (int, bool) {
	if val == nil {
		return 0, false
	}
	switch v := val.(type) {
	case string:
		var i int
		_, err := fmt.Sscanf(v, "%d", &i)
		return i, err == nil
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	}
	return 0, false
}
