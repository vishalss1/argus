package query

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/vishalss1/argus/internal/ai/operations"
	"github.com/vishalss1/argus/internal/domain/auth"
	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
	"github.com/vishalss1/argus/internal/domain/device"
	"github.com/vishalss1/argus/internal/domain/event"
	"github.com/vishalss1/argus/internal/domain/telemetry"
	"github.com/vishalss1/argus/internal/infrastructure/ai"
	"github.com/vishalss1/argus/internal/infrastructure/embedding"
	"github.com/vishalss1/argus/internal/infrastructure/postgres"
	redisinfra "github.com/vishalss1/argus/internal/infrastructure/redis"
)

type Response struct {
	Summary            string                        `json:"summary"`
	Confidence         float64                       `json:"confidence"`
	Evidence           []string                      `json:"evidence"`
	SuggestedActions   []string                      `json:"suggested_actions"`
	Intent             operations.Intent             `json:"intent"`
	DeviceSummary      *operations.DeviceSummary     `json:"device_summary,omitempty"`
	RootCauseAnalysis  *operations.RootCauseAnalysis `json:"root_cause_analysis,omitempty"`
	Remediations       []operations.Remediation      `json:"remediations,omitempty"`
	RelatedDevices     []operations.RelatedDevice    `json:"related_devices,omitempty"`
	OperationalContext *operations.Snapshot          `json:"operational_context,omitempty"`
}

type Engine struct {
	embeddingProvider   embedding.Provider
	aiProvider          ai.Provider
	vectorStore         *postgres.VectorStore
	eventRepo           *postgres.EventRepository
	contextRepo         *postgres.ContextRepository
	deviceRepo          device.Repository
	redisClient         *redisinfra.Client
	similarityThreshold float32
	summaryAnalyzer     *operations.DeviceSummaryAnalyzer
	rootCauseAnalyzer   *operations.RootCauseAnalyzer
	remediationEngine   *operations.RemediationEngine
}

func NewEngine(
	embeddingProvider embedding.Provider,
	aiProvider ai.Provider,
	vectorStore *postgres.VectorStore,
	eventRepo *postgres.EventRepository,
	contextRepo *postgres.ContextRepository,
	deviceRepo device.Repository,
	redisClient *redisinfra.Client,
	similarityThreshold float32,
) *Engine {
	return &Engine{
		embeddingProvider:   embeddingProvider,
		aiProvider:          aiProvider,
		vectorStore:         vectorStore,
		eventRepo:           eventRepo,
		contextRepo:         contextRepo,
		deviceRepo:          deviceRepo,
		redisClient:         redisClient,
		similarityThreshold: similarityThreshold,
		summaryAnalyzer:     operations.NewDeviceSummaryAnalyzer(),
		rootCauseAnalyzer:   operations.NewRootCauseAnalyzer(),
		remediationEngine:   operations.NewRemediationEngine(),
	}
}

func (e *Engine) Ask(ctx context.Context, queryString, preferredDeviceID string) (*Response, error) {
	intent := operations.ClassifyIntent(queryString)

	if preferredDeviceID != "" {
		target, err := e.deviceRepo.GetByID(ctx, preferredDeviceID)
		if err != nil {
			return nil, fmt.Errorf("get selected device: %w", err)
		}
		snapshot, err := e.buildSnapshot(ctx, *target)
		if err != nil {
			return nil, fmt.Errorf("build operational context: %w", err)
		}
		return e.analyzeDevice(ctx, queryString, intent, snapshot), nil
	}

	if intent == operations.IntentFleetSummary || intent == operations.IntentDeviceComparison {
		return e.analyzeFleet(ctx, intent), nil
	}

	devices, err := e.deviceRepo.Search(ctx, deviceSearchTerms(queryString), 10)
	if err != nil {
		return nil, fmt.Errorf("search devices for query resolution: %w", err)
	}
	target := resolveDevice(queryString, preferredDeviceID, devices)

	if target != nil {
		snapshot, err := e.buildSnapshot(ctx, *target)
		if err != nil {
			return nil, fmt.Errorf("build operational context: %w", err)
		}
		return e.analyzeDevice(ctx, queryString, intent, snapshot), nil
	}

	return e.semanticFallback(ctx, queryString, intent)
}

func (e *Engine) analyzeDevice(ctx context.Context, queryString string, intent operations.Intent, snapshot *operations.Snapshot) *Response {
	summary := e.summaryAnalyzer.Analyze(*snapshot)
	rca := e.rootCauseAnalyzer.Analyze(*snapshot)
	remediations := e.remediationEngine.Analyze(snapshot.IncidentHistory)
	snapshot.GeneratedAnalysis = map[string]any{
		"deviceSummary": summary,
		"rootCause":     rca,
		"remediations":  remediations,
	}

	response := &Response{
		Intent:             intent,
		DeviceSummary:      &summary,
		OperationalContext: snapshot,
		RelatedDevices:     e.relatedDevices(ctx, *snapshot),
	}
	switch intent {
	case operations.IntentRootCauseAnalysis:
		response.Summary = rca.PrimaryCause
		response.Confidence = float64(rca.Confidence) / 100
		response.Evidence = rca.SupportingEvidence
		response.SuggestedActions = rca.RecommendedActions
		response.RootCauseAnalysis = &rca
	case operations.IntentRemediation:
		response.Summary = remediationSummary(snapshot.Device.Name, remediations)
		response.Confidence = float64(rca.Confidence) / 100
		response.Evidence = rca.SupportingEvidence
		response.SuggestedActions = flattenActions(remediations, summary.Recommendations)
		response.RootCauseAnalysis = &rca
		response.Remediations = remediations
	case operations.IntentIncidentLookup:
		response.Summary = fmt.Sprintf("%s has %d open and %d recent incidents.", snapshot.Device.Name, summary.OpenIncidents, summary.RecentIncidents)
		response.Confidence = 0.85
		response.Evidence = incidentEvidence(snapshot.IncidentHistory)
		response.SuggestedActions = summary.Recommendations
		response.Remediations = remediations
	default:
		response.Summary = fmt.Sprintf("%s is %s with a health score of %d/100.", snapshot.Device.Name, summary.Severity, summary.HealthScore)
		response.Confidence = 0.9
		response.Evidence = summary.KeyFindings
		response.SuggestedActions = summary.Recommendations
	}

	// The deterministic analysis remains authoritative; LLM reasoning enriches the
	// narrative when available and receives the full structured operational context.
	if enriched, err := e.reasonOverSnapshot(ctx, queryString, intent, snapshot); err == nil {
		if enriched.Summary != "" {
			response.Summary = enriched.Summary
		}
		if len(enriched.Evidence) > 0 {
			response.Evidence = uniqueStrings(append(response.Evidence, enriched.Evidence...), 8)
		}
		if len(enriched.SuggestedActions) > 0 {
			response.SuggestedActions = uniqueStrings(append(response.SuggestedActions, enriched.SuggestedActions...), 8)
		}
		if enriched.Confidence > response.Confidence {
			response.Confidence = enriched.Confidence
		}
	} else {
		log.Printf("[QUERY ENGINE] structured LLM reasoning unavailable, returning deterministic analysis: %v", err)
	}
	return response
}

func (e *Engine) reasonOverSnapshot(ctx context.Context, queryString string, intent operations.Intent, snapshot *operations.Snapshot) (*ai.ReasoningResponse, error) {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	systemPrompt := `You are ARGUS AI, an operational analytics and root-cause reasoning system.
Reason only from the structured device context. Explain causal order, distinguish evidence from hypotheses, and provide concrete operator actions.
Never say "No matching incidents found". If incidents are absent, summarize current health, telemetry observations, and recommendations.
Output valid JSON with keys: summary, confidence (0-1), evidence (array of strings), suggested_actions (array of strings).`
	userPrompt := fmt.Sprintf("Intent: %s\nQuery: %s\nStructured operational context:\n%s", intent, queryString, data)
	return e.aiProvider.Reason(ctx, systemPrompt, userPrompt)
}

func (e *Engine) buildSnapshot(ctx context.Context, target device.Device) (*operations.Snapshot, error) {
	snapshot := &operations.Snapshot{
		Device:           target,
		LatestTelemetry:  make(map[string]any),
		TelemetryWindows: make(map[string][]operations.TelemetryPoint),
		FirmwareInfo: map[string]any{
			"version": target.FirmwareVersion,
		},
	}
	if raw, err := e.redisClient.Client().Get(ctx, fmt.Sprintf("device:%s:latest", target.ID)).Bytes(); err == nil {
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
	sessionID, _ := e.redisClient.Client().Get(ctx, fmt.Sprintf("workspace:%s:active_session", workspaceID)).Result()
	if sessionID != "" {
		snapshot.TelemetryWindows = e.loadTelemetryWindows(ctx, sessionID, target.ID)
		snapshot.TelemetryTrends = e.loadTrends(ctx, sessionID, target.ID, snapshot.LatestTelemetry)
		snapshot.IncidentHistory = append(snapshot.IncidentHistory, e.loadActiveIncidents(ctx, sessionID, target.ID)...)
		snapshot.IncidentHistory = append(snapshot.IncidentHistory, e.loadClosedIncidents(ctx, sessionID, target.ID)...)
	}

	if memories, err := e.contextRepo.ListByDevice(ctx, target.ID, 50, 0); err == nil {
		for _, memory := range memories {
			item := operations.HistoryItem{Type: string(memory.Type), Summary: memory.Summary, Data: memory.Data, Timestamp: memory.Timestamp}
			switch memory.Type {
			case ctxdomain.MemoryTypeConnectivity:
				snapshot.ConnectivityHistory = append(snapshot.ConnectivityHistory, item)
			case ctxdomain.MemoryTypeAnomaly:
				snapshot.AnomalyHistory = append(snapshot.AnomalyHistory, item)
			case ctxdomain.MemoryTypeIncident:
				snapshot.AnomalyHistory = append(snapshot.AnomalyHistory, item)
			}
		}
	}
	if events, err := e.eventRepo.ListByDevice(ctx, target.ID, 50, 0); err == nil {
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

func (e *Engine) loadTelemetryWindows(ctx context.Context, sessionID, deviceID string) map[string][]operations.TelemetryPoint {
	result := make(map[string][]operations.TelemetryPoint)
	key := fmt.Sprintf("session:%s:device:%s:telemetry_history", sessionID, deviceID)
	now := time.Now().UTC()
	for _, window := range []struct {
		name     string
		duration time.Duration
	}{
		{name: "previous1h", duration: time.Hour},
		{name: "previous6h", duration: 6 * time.Hour},
		{name: "previous24h", duration: 24 * time.Hour},
	} {
		items, _ := e.redisClient.Client().ZRevRangeByScore(ctx, key, &goredis.ZRangeBy{
			Min: strconv.FormatInt(now.Add(-window.duration).UnixMilli(), 10),
			Max: "+inf", Offset: 0, Count: 120,
		}).Result()
		points := make([]operations.TelemetryPoint, 0, len(items))
		for _, item := range items {
			var sample telemetry.Telemetry
			if json.Unmarshal([]byte(item), &sample) != nil {
				continue
			}
			metrics := make(map[string]any)
			if json.Unmarshal(sample.Metrics, &metrics) == nil {
				points = append(points, operations.TelemetryPoint{RecordedAt: sample.RecordedAt, Metrics: metrics})
			}
		}
		result[window.name] = points
	}
	return result
}

func (e *Engine) loadTrends(ctx context.Context, sessionID, deviceID string, latest map[string]any) []operations.MetricTrend {
	metrics, _ := e.redisClient.Client().SMembers(ctx, fmt.Sprintf("session:%s:metrics", sessionID)).Result()
	if len(metrics) > 200 {
		metrics = metrics[:200]
	}
	trends := make([]operations.MetricTrend, 0, len(metrics))
	for _, metric := range metrics {
		values, err := e.redisClient.Client().HGetAll(ctx, fmt.Sprintf("session:%s:device:%s:metric:%s", sessionID, deviceID, metric)).Result()
		if err != nil || len(values) == 0 {
			continue
		}
		count, _ := strconv.ParseInt(values["count"], 10, 64)
		sum, _ := strconv.ParseFloat(values["sum"], 64)
		minimum, _ := strconv.ParseFloat(values["min"], 64)
		maximum, _ := strconv.ParseFloat(values["max"], 64)
		m2, _ := strconv.ParseFloat(values["m2"], 64)
		average := 0.0
		variance := 0.0
		if count > 0 {
			average = sum / float64(count)
		}
		if count > 1 {
			variance = m2 / float64(count-1)
		}
		current := nestedValue(latest, metric)
		currentNumber, numeric := current.(float64)
		direction := "stable"
		anomalous := false
		if numeric && count > 1 {
			stddev := math.Sqrt(variance)
			if currentNumber > average {
				direction = "up"
			} else if currentNumber < average {
				direction = "down"
			}
			anomalous = stddev > 0 && math.Abs(currentNumber-average) > 3*stddev
		}
		trends = append(trends, operations.MetricTrend{
			Metric: metric, Current: current, Count: count, Minimum: minimum, Maximum: maximum,
			Average: average, Variance: variance, Direction: direction, Anomalous: anomalous,
		})
	}
	return trends
}

func (e *Engine) loadActiveIncidents(ctx context.Context, sessionID, deviceID string) []operations.Incident {
	keys, _ := e.redisClient.Client().SMembers(ctx, fmt.Sprintf("session:%s:device:%s:incidents", sessionID, deviceID)).Result()
	if len(keys) > 100 {
		keys = keys[:100]
	}
	result := make([]operations.Incident, 0)
	for _, key := range keys {
		if raw, err := e.redisClient.Client().Get(ctx, key).Bytes(); err == nil {
			var incident operations.Incident
			if json.Unmarshal(raw, &incident) == nil {
				incident.Status = "open"
				result = append(result, incident)
			}
		}
	}
	return result
}

func (e *Engine) loadSessionActiveIncidents(ctx context.Context, sessionID string) []operations.Incident {
	if sessionID == "" {
		return nil
	}
	keys, _ := e.redisClient.Client().SMembers(ctx, fmt.Sprintf("session:%s:incidents", sessionID)).Result()
	if len(keys) == 0 {
		return nil
	}
	if len(keys) > 1000 {
		keys = keys[:1000]
	}
	values, err := e.redisClient.Client().MGet(ctx, keys...).Result()
	if err != nil {
		return nil
	}
	result := make([]operations.Incident, 0, len(values))
	for _, value := range values {
		raw, ok := value.(string)
		if !ok || raw == "" {
			continue
		}
		var incident operations.Incident
		if json.Unmarshal([]byte(raw), &incident) == nil {
			incident.Status = "open"
			result = append(result, incident)
		}
	}
	return result
}

func (e *Engine) loadClosedIncidents(ctx context.Context, sessionID, deviceID string) []operations.Incident {
	rawItems, _ := e.redisClient.Client().LRange(ctx, fmt.Sprintf("session:%s:artifact_buffer", sessionID), -250, -1).Result()
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

func (e *Engine) relatedDevices(ctx context.Context, snapshot operations.Snapshot) []operations.RelatedDevice {
	targetPatterns := make(map[string]bool)
	for _, incident := range snapshot.IncidentHistory {
		targetPatterns[incident.Metric+"."+incident.IncidentType] = true
	}
	if len(targetPatterns) == 0 {
		return nil
	}

	workspaceID := ""
	if snapshot.Device.WorkspaceID != nil {
		workspaceID = *snapshot.Device.WorkspaceID
	}
	sessionID, _ := e.redisClient.Client().Get(ctx, fmt.Sprintf("workspace:%s:active_session", workspaceID)).Result()
	if sessionID == "" {
		return nil
	}

	incidents := e.loadSessionActiveIncidents(ctx, sessionID)
	sharedByDevice := make(map[string][]string)
	for _, incident := range incidents {
		if incident.DeviceID == snapshot.Device.ID {
			continue
		}
		pattern := incident.Metric + "." + incident.IncidentType
		if targetPatterns[pattern] {
			sharedByDevice[incident.DeviceID] = append(sharedByDevice[incident.DeviceID], pattern)
		}
	}

	candidates := rankedDevicePatterns(sharedByDevice, 10)
	result := make([]operations.RelatedDevice, 0, len(candidates))
	for _, candidate := range candidates {
		name := candidate.deviceID
		if dev, err := e.deviceRepo.GetByID(ctx, candidate.deviceID); err == nil && dev.Name != "" {
			name = dev.Name
		}
		result = append(result, operations.RelatedDevice{
			DeviceID: candidate.deviceID, DeviceName: name,
			Similarity: min(100, 50+len(candidate.patterns)*20), SharedPatterns: uniqueStrings(candidate.patterns, 5),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Similarity > result[j].Similarity })
	return result
}

func (e *Engine) analyzeFleet(ctx context.Context, intent operations.Intent) *Response {
	sessionID := e.activeSessionForContext(ctx)
	if sessionID == "" {
		return &Response{
			Intent:           intent,
			Summary:          "No active session is available for indexed fleet analysis. Select a device for single-device health analysis.",
			Confidence:       0.45,
			Evidence:         []string{"Fleet-wide analysis uses the active session incident index to avoid scanning every device"},
			SuggestedActions: []string{"Start or select an active session", "Ask about a specific device for direct analysis"},
		}
	}

	incidents := e.loadSessionActiveIncidents(ctx, sessionID)
	devicePatterns := make(map[string][]string)
	severityCounts := map[string]int{"critical": 0, "warning": 0}
	for _, incident := range incidents {
		pattern := incident.Metric + "." + incident.IncidentType
		devicePatterns[incident.DeviceID] = append(devicePatterns[incident.DeviceID], pattern)
		severityCounts[incident.Severity]++
	}

	candidates := rankedDevicePatterns(devicePatterns, 10)
	related := make([]operations.RelatedDevice, 0, len(candidates))
	for _, candidate := range candidates {
		name := candidate.deviceID
		if dev, err := e.deviceRepo.GetByID(ctx, candidate.deviceID); err == nil && dev.Name != "" {
			name = dev.Name
		}
		related = append(related, operations.RelatedDevice{
			DeviceID: candidate.deviceID, DeviceName: name,
			Similarity: min(100, 40+len(candidate.patterns)*15), SharedPatterns: uniqueStrings(candidate.patterns, 5),
		})
	}
	sort.Slice(related, func(i, j int) bool { return related[i].Similarity > related[j].Similarity })

	evidence := []string{
		fmt.Sprintf("%d active incident streams are indexed in the current session", len(incidents)),
		fmt.Sprintf("%d devices currently have active anomaly patterns", len(devicePatterns)),
		fmt.Sprintf("%d critical and %d warning incident streams are open", severityCounts["critical"], severityCounts["warning"]),
	}
	if len(incidents) == 0 {
		evidence = []string{"No active incident streams are currently indexed for this session"}
	}

	return &Response{
		Intent:           intent,
		Summary:          fmt.Sprintf("Fleet analysis found %d open incident streams across %d affected devices.", len(incidents), len(devicePatterns)),
		Confidence:       0.9,
		Evidence:         evidence,
		SuggestedActions: []string{"Prioritize critical and offline devices", "Review devices sharing the same anomaly pattern", "Continue monitoring healthy devices"},
		RelatedDevices:   related,
	}
}

type devicePatternCandidate struct {
	deviceID string
	patterns []string
}

func rankedDevicePatterns(patternsByDevice map[string][]string, limit int) []devicePatternCandidate {
	candidates := make([]devicePatternCandidate, 0, len(patternsByDevice))
	for deviceID, patterns := range patternsByDevice {
		candidates = append(candidates, devicePatternCandidate{deviceID: deviceID, patterns: patterns})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if len(candidates[i].patterns) == len(candidates[j].patterns) {
			return candidates[i].deviceID < candidates[j].deviceID
		}
		return len(candidates[i].patterns) > len(candidates[j].patterns)
	})
	if len(candidates) > limit {
		return candidates[:limit]
	}
	return candidates
}

func (e *Engine) activeSessionForContext(ctx context.Context) string {
	if workspaceID, ok := auth.GetWorkspaceID(ctx); ok && workspaceID != "" {
		sessionID, _ := e.redisClient.Client().Get(ctx, fmt.Sprintf("workspace:%s:active_session", workspaceID)).Result()
		return sessionID
	}
	return ""
}

func (e *Engine) semanticFallback(ctx context.Context, queryString string, intent operations.Intent) (*Response, error) {
	retrieved, err := e.RetrieveContext(ctx, queryString)
	if err != nil {
		log.Printf("[QUERY ENGINE] semantic retrieval unavailable: %v", err)
		retrieved = &Context{}
	}
	data, _ := json.Marshal(retrieved)
	systemPrompt := `You are ARGUS AI. Answer from the supplied operational events and memory. If exact incidents are absent, provide useful fleet observations and recommended next checks. Never answer "No matching incidents found". Output JSON with summary, confidence, evidence, suggested_actions.`
	reasoning, reasonErr := e.aiProvider.Reason(ctx, systemPrompt, fmt.Sprintf("Intent: %s\nQuery: %s\nContext: %s", intent, queryString, data))
	if reasonErr != nil {
		return &Response{
			Intent: intent, Summary: "No device was identified in the query. Specify a device name or select a device to run operational analysis.",
			Confidence: 0.2, Evidence: []string{"Operational analysis requires a resolvable device or fleet-level query"},
			SuggestedActions: []string{"Select a device and ask for its health summary", "Ask what happened in the fleet recently"},
		}, nil
	}
	return &Response{
		Intent: intent, Summary: reasoning.Summary, Confidence: reasoning.Confidence,
		Evidence: reasoning.Evidence, SuggestedActions: reasoning.SuggestedActions,
	}, nil
}

type Context struct {
	Events   []event.Event                 `json:"events"`
	Memories []ctxdomain.OperationalMemory `json:"memories"`
}

func (e *Engine) RetrieveContext(ctx context.Context, queryString string) (*Context, error) {
	retrieved := &Context{}
	seen := make(map[string]bool)
	uuidRegex := regexp.MustCompile(`[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}`)
	for _, id := range uuidRegex.FindAllString(queryString, -1) {
		if ev, err := e.eventRepo.GetByID(ctx, id); err == nil {
			retrieved.Events = append(retrieved.Events, *ev)
			seen[ev.ID] = true
		}
	}
	vector, err := e.embeddingProvider.Embed(ctx, queryString)
	if err != nil {
		return retrieved, err
	}
	if results, err := e.vectorStore.Search(ctx, "events", vector, 5); err == nil {
		for _, result := range results {
			if result.Score >= e.similarityThreshold && !seen[result.ID] {
				if ev, getErr := e.eventRepo.GetByID(ctx, result.ID); getErr == nil {
					retrieved.Events = append(retrieved.Events, *ev)
					seen[result.ID] = true
				}
			}
		}
	}
	if results, err := e.vectorStore.Search(ctx, "operational_memory", vector, 5); err == nil {
		for _, result := range results {
			if result.Score >= e.similarityThreshold && !seen[result.ID] {
				if memory, getErr := e.contextRepo.GetByID(ctx, result.ID); getErr == nil {
					retrieved.Memories = append(retrieved.Memories, *memory)
					seen[result.ID] = true
				}
			}
		}
	}
	return retrieved, nil
}

func resolveDevice(query, preferredID string, devices []device.Device) *device.Device {
	for i := range devices {
		if preferredID != "" && devices[i].ID == preferredID {
			return &devices[i]
		}
	}
	lower := strings.ToLower(query)
	for i := range devices {
		if strings.Contains(lower, strings.ToLower(devices[i].ID)) || (devices[i].Name != "" && strings.Contains(lower, strings.ToLower(devices[i].Name))) {
			return &devices[i]
		}
	}
	return nil
}

func deviceSearchTerms(query string) []string {
	stopWords := map[string]bool{
		"why": true, "did": true, "fail": true, "failed": true, "device": true, "summarize": true,
		"summary": true, "health": true, "how": true, "fix": true, "what": true, "wrong": true,
		"with": true, "before": true, "incident": true, "happened": true, "cause": true,
	}
	cleaned := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return ' '
	}, query)
	seen := make(map[string]bool)
	terms := make([]string, 0)
	for _, term := range strings.Fields(cleaned) {
		term = strings.ToLower(term)
		if stopWords[term] || len(term) < 2 || seen[term] {
			continue
		}
		seen[term] = true
		terms = append(terms, term)
		if len(terms) == 8 {
			break
		}
	}
	return terms
}

func nestedValue(values map[string]any, path string) any {
	var current any = values
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}

func incidentEvidence(incidents []operations.Incident) []string {
	result := make([]string, 0)
	for _, incident := range incidents {
		result = append(result, fmt.Sprintf("%s %s (%s, %d occurrences)", incident.Metric, incident.IncidentType, incident.Status, incident.Occurrences))
	}
	if len(result) == 0 {
		result = append(result, "No active incidents; health summary generated from device state and telemetry")
	}
	return uniqueStrings(result, 8)
}

func remediationSummary(deviceName string, remediations []operations.Remediation) string {
	if len(remediations) == 0 {
		return fmt.Sprintf("%s has no active incident requiring immediate remediation; continue monitoring and inspect logs if symptoms persist.", deviceName)
	}
	return fmt.Sprintf("%s has %d active remediation pattern(s). Start with the highest-severity incident and verify telemetry recovery after each action.", deviceName, len(remediations))
}

func flattenActions(remediations []operations.Remediation, fallback []string) []string {
	result := make([]string, 0)
	for _, remediation := range remediations {
		result = append(result, remediation.Actions...)
	}
	if len(result) == 0 {
		result = append(result, fallback...)
	}
	return uniqueStrings(result, 8)
}

func uniqueStrings(values []string, limit int) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
			if len(result) == limit {
				break
			}
		}
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
