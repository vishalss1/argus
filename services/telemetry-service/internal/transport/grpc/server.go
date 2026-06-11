package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/vishalss1/argus/shared/proto/telemetry"
	"github.com/vishalss1/argus/telemetry/internal/ai/query"
	ctxdomain "github.com/vishalss1/argus/telemetry/internal/domain/context"
	event "github.com/vishalss1/argus/telemetry/internal/domain/event"
	rule "github.com/vishalss1/argus/telemetry/internal/domain/rule"
	redisinfra "github.com/vishalss1/argus/telemetry/internal/infrastructure/redis"
)

type Server struct {
	pb.UnimplementedTelemetryIntelligenceServiceServer
	queryEngine    *query.Engine
	eventRepo      event.Repository
	contextService *ctxdomain.Service
	ruleService    *rule.Service
	redisClient    *redisinfra.Client
}

func NewServer(
	queryEngine *query.Engine,
	eventRepo event.Repository,
	contextService *ctxdomain.Service,
	ruleService *rule.Service,
	redisClient *redisinfra.Client,
) *Server {
	return &Server{
		queryEngine:    queryEngine,
		eventRepo:      eventRepo,
		contextService: contextService,
		ruleService:    ruleService,
		redisClient:    redisClient,
	}
}

func (s *Server) QueryAI(ctx context.Context, req *pb.QueryAIRequest) (*pb.QueryAIResponse, error) {
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	res, err := s.queryEngine.Ask(ctx, req.Query, req.DeviceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ai query failed: %v", err)
	}

	var actions []*pb.ActionSuggestion
	for _, rem := range res.Remediations {
		for _, act := range rem.Actions {
			actions = append(actions, &pb.ActionSuggestion{
				SuggestionId: uuid.New().String(),
				Action:       rem.Metric,
				DeviceId:     rem.DeviceID,
				Description:  act,
				Severity:     rem.Severity,
			})
		}
	}
	if len(actions) == 0 {
		for _, act := range res.SuggestedActions {
			actions = append(actions, &pb.ActionSuggestion{
				SuggestionId: uuid.New().String(),
				Action:       "operator_action",
				DeviceId:     req.DeviceId,
				Description:  act,
				Severity:     "warning",
			})
		}
	}

	return &pb.QueryAIResponse{
		Response:   res.Summary,
		Intent:     string(res.Intent),
		Confidence: res.Confidence,
		Sources:    res.Evidence,
		Actions:    actions,
	}, nil
}

func (s *Server) GetSnapshot(ctx context.Context, req *pb.GetSnapshotRequest) (*pb.DeviceSnapshotResponse, error) {
	if req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	rdb := s.redisClient.Client()
	wsKey := fmt.Sprintf("device:%s:workspace", req.DeviceId)
	workspaceID, _ := rdb.Get(ctx, wsKey).Result()
	if workspaceID == "" {
		workspaceID = req.WorkspaceId
	}

	var sessionID string
	if workspaceID != "" {
		sessionKey := fmt.Sprintf("workspace:%s:active_session", workspaceID)
		sessionID, _ = rdb.Get(ctx, sessionKey).Result()
	}

	devStateKey := fmt.Sprintf("session:%s:device:%s:state", sessionID, req.DeviceId)
	state, _ := rdb.HGetAll(ctx, devStateKey).Result()

	deviceStatus := "offline"
	var lastSeenProto *timestamppb.Timestamp
	if len(state) > 0 {
		deviceStatus = "online"
		if lUnix, err := strconv.ParseInt(state["last_seen"], 10, 64); err == nil {
			lastSeenProto = timestamppb.New(time.Unix(lUnix, 0).UTC())
		}
	}

	latestKey := fmt.Sprintf("device:%s:latest", req.DeviceId)
	latestJSON, _ := rdb.Get(ctx, latestKey).Result()
	latestMetricsMap := make(map[string]float64)

	if latestJSON != "" {
		var t struct {
			Metrics json.RawMessage `json:"metrics"`
		}
		if json.Unmarshal([]byte(latestJSON), &t) == nil {
			var vals map[string]interface{}
			if json.Unmarshal(t.Metrics, &vals) == nil {
				for k, v := range vals {
					if f, ok := v.(float64); ok {
						latestMetricsMap[k] = f
					}
				}
			}
		}
	}

	var activeIncidentTypes []string
	var totalIncidents int32 = 0
	if sessionID != "" {
		deviceIncidentsSetKey := fmt.Sprintf("session:%s:device:%s:incidents", sessionID, req.DeviceId)
		targetKeys, _ := rdb.SMembers(ctx, deviceIncidentsSetKey).Result()
		totalIncidents = int32(len(targetKeys))
		if len(targetKeys) > 0 {
			vals, err := rdb.MGet(ctx, targetKeys...).Result()
			if err == nil {
				for _, v := range vals {
					if vStr, ok := v.(string); ok && vStr != "" {
						var inc struct {
							IncidentType string `json:"incident_type"`
						}
						if json.Unmarshal([]byte(vStr), &inc) == nil {
							activeIncidentTypes = append(activeIncidentTypes, inc.IncidentType)
						}
					}
				}
			}
		}
	}

	return &pb.DeviceSnapshotResponse{
		DeviceId:               req.DeviceId,
		Status:                 deviceStatus,
		LastSeen:               lastSeenProto,
		LatestMetrics:          latestMetricsMap,
		ActiveIncidentTypes:    activeIncidentTypes,
		TotalIncidentsRecorded: totalIncidents,
	}, nil
}

func (s *Server) ListIncidents(ctx context.Context, req *pb.ListIncidentsRequest) (*pb.ListIncidentsResponse, error) {
	var incidentKeys []string
	rdb := s.redisClient.Client()
	if req.SessionId != "" {
		incidentsSetKey := fmt.Sprintf("session:%s:incidents", req.SessionId)
		keys, err := rdb.SMembers(ctx, incidentsSetKey).Result()
		if err == nil && len(keys) > 0 {
			incidentKeys = append(incidentKeys, keys...)
		}
	} else {
		activeSessions, err := rdb.SMembers(ctx, "sessions:active").Result()
		if err == nil {
			for _, sessionID := range activeSessions {
				incidentsSetKey := fmt.Sprintf("session:%s:incidents", sessionID)
				keys, err := rdb.SMembers(ctx, incidentsSetKey).Result()
				if err == nil && len(keys) > 0 {
					incidentKeys = append(incidentKeys, keys...)
				}
			}
		}
	}

	var pbIncidents []*pb.ActiveIncident
	if len(incidentKeys) > 0 {
		vals, err := rdb.MGet(ctx, incidentKeys...).Result()
		if err == nil {
			for _, v := range vals {
				if vStr, ok := v.(string); ok && vStr != "" {
					var inc struct {
						DeviceID     string    `json:"device_id"`
						Metric       string    `json:"metric"`
						IncidentType string    `json:"incident_type"`
						Severity     string    `json:"severity"`
						StartTime    time.Time `json:"start_time"`
						PeakScore    float64   `json:"peak_score"`
						Summary      string    `json:"summary"`
					}
					if json.Unmarshal([]byte(vStr), &inc) == nil {
						pbIncidents = append(pbIncidents, &pb.ActiveIncident{
							DeviceId:  inc.DeviceID,
							Metric:    inc.Metric,
							Type:      inc.IncidentType,
							Severity:  inc.Severity,
							StartedAt: timestamppb.New(inc.StartTime),
							PeakScore: inc.PeakScore,
							Summary:   inc.Summary,
						})
					}
				}
			}
		}
	}
	return &pb.ListIncidentsResponse{Incidents: pbIncidents}, nil
}

func (s *Server) FleetSummary(ctx context.Context, req *pb.FleetSummaryRequest) (*pb.FleetSummaryResponse, error) {
	res, err := s.queryEngine.Ask(ctx, "summarize fleet health", "")
	var summaryText string
	if err == nil {
		summaryText = res.Summary
	} else {
		summaryText = "Fleet diagnostics are currently stable. No widespread anomalies detected."
	}

	rdb := s.redisClient.Client()
	activeSessions, _ := rdb.SMembers(ctx, "sessions:active").Result()
	var totalDevices, onlineDevices, activeIncidents int32

	for _, sessionID := range activeSessions {
		devsKey := fmt.Sprintf("session:%s:devices", sessionID)
		devs, _ := rdb.SMembers(ctx, devsKey).Result()
		totalDevices += int32(len(devs))
		onlineDevices += int32(len(devs))

		incidentsKey := fmt.Sprintf("session:%s:incidents", sessionID)
		incidents, _ := rdb.SMembers(ctx, incidentsKey).Result()
		activeIncidents += int32(len(incidents))
	}

	return &pb.FleetSummaryResponse{
		TotalDevices:    totalDevices,
		OnlineDevices:   onlineDevices,
		ActiveIncidents: activeIncidents,
		SummaryText:     summaryText,
	}, nil
}

func (s *Server) GetSessionTelemetry(ctx context.Context, req *pb.GetSessionTelemetryRequest) (*pb.SessionTelemetryResponse, error) {
	if req.SessionId == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	id := req.SessionId
	rdb := s.redisClient.Client()

	devices, _ := rdb.SMembers(ctx, fmt.Sprintf("session:%s:devices", id)).Result()
	metricKeys, _ := rdb.SMembers(ctx, fmt.Sprintf("session:%s:metrics", id)).Result()

	var pbDeviceSummaries []*pb.DeviceSummary
	var sampleCountTotal int32
	var anomalyCount int32

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

				sampleCountTotal += int32(samples)

				firstSeenStr := ""
				if fUnix, err := strconv.ParseInt(firstSeenVal, 10, 64); err == nil {
					firstSeenStr = time.Unix(fUnix, 0).UTC().Format(time.RFC3339)
				}
				lastSeenStr := ""
				if lUnix, err := strconv.ParseInt(lastSeenVal, 10, 64); err == nil {
					lastSeenStr = time.Unix(lUnix, 0).UTC().Format(time.RFC3339)
				}

				activeAtEnd := worstSev != "healthy" && (warnCount+critCount > 0)

				pbDeviceSummaries = append(pbDeviceSummaries, &pb.DeviceSummary{
					DeviceId:               devID,
					FirstSeen:              firstSeenStr,
					LastSeen:               lastSeenStr,
					SampleCount:            int32(samples),
					WarningIncidentsCount:  int32(warnCount),
					CriticalIncidentsCount: int32(critCount),
					ActiveAtEnd:            activeAtEnd,
				})
			}
		}
	}

	var pbIncidents []*pb.SessionIncident
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
		if json.Unmarshal([]byte(incStr), &closed) == nil {
			pbIncidents = append(pbIncidents, &pb.SessionIncident{
				DeviceId:     closed.DeviceID,
				Metric:       closed.Metric,
				IncidentType: closed.IncidentType,
				Severity:     closed.Severity,
				StartTime:    timestamppb.New(closed.StartTime),
				ResolvedAt:   timestamppb.New(closed.ResolvedAt),
				Occurrences:  int32(closed.Occurrences),
				PeakScore:    closed.PeakScore,
				Summary:      closed.Summary,
			})
			anomalyCount++
		}
	}

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
						Occurrences  int       `json:"occurrences"`
						PeakScore    float64   `json:"peak_score"`
						Summary      string    `json:"summary"`
					}
					if json.Unmarshal([]byte(vStr), &open) == nil {
						pbIncidents = append(pbIncidents, &pb.SessionIncident{
							DeviceId:     open.DeviceID,
							Metric:       open.Metric,
							IncidentType: open.IncidentType,
							Severity:     open.Severity,
							StartTime:    timestamppb.New(open.StartTime),
							ResolvedAt:   nil,
							Occurrences:  int32(open.Occurrences),
							PeakScore:    open.PeakScore,
							Summary:      open.Summary,
						})
						anomalyCount++
					}
				}
			}
		}
	}

	suppressedCountStr, _ := rdb.Get(ctx, fmt.Sprintf("session:%s:incidents:suppressed", id)).Result()
	if suppressedCountStr != "" {
		if count, err := strconv.Atoi(suppressedCountStr); err == nil && count > 0 {
			nowTime := time.Now()
			pbIncidents = append(pbIncidents, &pb.SessionIncident{
				DeviceId:     "system",
				Metric:       "multiple",
				IncidentType: "capacity_exceeded",
				Severity:     "warning",
				StartTime:    timestamppb.New(nowTime),
				ResolvedAt:   timestamppb.New(nowTime),
				Occurrences:  int32(count),
				PeakScore:    0.0,
				Summary:      fmt.Sprintf("%d additional closed incidents were suppressed to protect artifact capacity.", count),
			})
		}
	}

	metricsAggregates := make(map[string]*pb.DeviceMetricAggregates)
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
			devAggs := make(map[string]*pb.MetricAggregate)
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

						devAggs[mKey] = &pb.MetricAggregate{
							Count:    int32(cnt),
							Min:      minVal,
							Max:      maxVal,
							Average:  avg,
							Variance: variance,
						}
					}
				}
			}
			if len(devAggs) > 0 {
				metricsAggregates[devID] = &pb.DeviceMetricAggregates{
					Aggregates: devAggs,
				}
			}
		}
	}

	var keysToDelete []string
	keysToDelete = append(keysToDelete,
		fmt.Sprintf("session:%s:devices", id),
		fmt.Sprintf("session:%s:metrics", id),
		fmt.Sprintf("session:%s:incidents", id),
		fmt.Sprintf("session:%s:artifact_buffer", id),
		fmt.Sprintf("session:%s:incidents:suppressed", id),
		fmt.Sprintf("session:%s:stopped", id),
	)

	for _, devID := range devices {
		keysToDelete = append(keysToDelete,
			fmt.Sprintf("session:%s:device:%s:state", id, devID),
			fmt.Sprintf("session:%s:device:%s:incidents", id, devID),
			fmt.Sprintf("session:%s:device:%s:telemetry_history", id, devID),
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

	if len(keysToDelete) > 0 {
		delPipe := rdb.Pipeline()
		for _, k := range keysToDelete {
			delPipe.Del(ctx, k)
		}
		_, _ = delPipe.Exec(ctx)
	}

	return &pb.SessionTelemetryResponse{
		SessionId:         id,
		MessagesProcessed: sampleCountTotal,
		AnomalyCount:      anomalyCount,
		DeviceSummaries:   pbDeviceSummaries,
		IncidentsArchive:  pbIncidents,
		MetricsAggregates: metricsAggregates,
	}, nil
}

func (s *Server) ConfigureRule(ctx context.Context, req *pb.ConfigureRuleRequest) (*pb.RuleResponse, error) {
	var r *rule.Rule
	var err error

	if req.RuleId == "" {
		r, err = s.ruleService.Create(ctx, rule.CreateInput{
			Name:      req.Name,
			Metric:    req.Metric,
			Operator:  req.Operator,
			Threshold: req.Threshold,
			Enabled:   &req.Enabled,
		})
	} else {
		r, err = s.ruleService.Update(ctx, req.RuleId, rule.UpdateInput{
			Name:      &req.Name,
			Metric:    &req.Metric,
			Operator:  &req.Operator,
			Threshold: &req.Threshold,
			Enabled:   &req.Enabled,
		})
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to configure rule: %v", err)
	}

	return &pb.RuleResponse{
		Id:        r.ID,
		Name:      r.Name,
		Metric:    r.Metric,
		Operator:  r.Operator,
		Threshold: r.Threshold,
		Enabled:   r.Enabled,
		CreatedAt: timestamppb.New(r.CreatedAt),
		UpdatedAt: timestamppb.New(r.UpdatedAt),
	}, nil
}

func (s *Server) ListRules(ctx context.Context, req *pb.ListRulesRequest) (*pb.ListRulesResponse, error) {
	rules, err := s.ruleService.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list rules: %v", err)
	}

	var pbRules []*pb.RuleResponse
	for _, r := range rules {
		if req.EnabledOnly && !r.Enabled {
			continue
		}
		pbRules = append(pbRules, &pb.RuleResponse{
			Id:        r.ID,
			Name:      r.Name,
			Metric:    r.Metric,
			Operator:  r.Operator,
			Threshold: r.Threshold,
			Enabled:   r.Enabled,
			CreatedAt: timestamppb.New(r.CreatedAt),
			UpdatedAt: timestamppb.New(r.UpdatedAt),
		})
	}

	return &pb.ListRulesResponse{Rules: pbRules}, nil
}

func (s *Server) GetRulesWithAlerts(ctx context.Context, req *pb.GetRulesWithAlertsRequest) (*pb.GetRulesWithAlertsResponse, error) {
	alerts, err := s.ruleService.ListAlerts(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list alerts: %v", err)
	}

	var alertResponses []*pb.AlertResponse
	for _, a := range alerts {
		alertResponses = append(alertResponses, &pb.AlertResponse{
			Id:            a.ID,
			RuleId:        a.RuleID,
			DeviceId:      a.DeviceID,
			Metric:        a.Metric,
			Operator:      a.Operator,
			Threshold:     a.Threshold,
			ObservedValue: a.ObservedValue,
			Severity:      a.Severity,
			Message:       a.Message,
			CreatedAt:     timestamppb.New(a.CreatedAt),
		})
	}

	start := int(req.Offset)
	if start > len(alertResponses) {
		start = len(alertResponses)
	}
	end := start + int(req.Limit)
	if req.Limit <= 0 || end > len(alertResponses) {
		end = len(alertResponses)
	}

	return &pb.GetRulesWithAlertsResponse{
		Alerts: alertResponses[start:end],
	}, nil
}

func (s *Server) ListEvents(ctx context.Context, req *pb.ListEventsRequest) (*pb.ListEventsResponse, error) {
	var events []event.Event
	var err error

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}
	offset := int(req.Offset)

	if req.DeviceId != "" {
		events, err = s.eventRepo.ListByDevice(ctx, req.DeviceId, limit, offset)
	} else {
		events, err = s.eventRepo.List(ctx, limit, offset)
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list events: %v", err)
	}

	var pbEvents []*pb.EventResponse
	for _, e := range events {
		pbEvents = append(pbEvents, &pb.EventResponse{
			Id:              e.ID,
			DeviceId:        e.DeviceID,
			Type:            e.Type,
			Severity:        string(e.Severity),
			Title:           e.Title,
			Summary:         e.Summary,
			Source:          e.Source,
			ConfidenceScore: e.ConfidenceScore,
			MetadataJson:    string(e.Metadata),
			CreatedAt:       timestamppb.New(e.CreatedAt),
		})
	}

	return &pb.ListEventsResponse{Events: pbEvents}, nil
}

func (s *Server) GetDeviceHistory(ctx context.Context, req *pb.GetDeviceHistoryRequest) (*pb.GetDeviceHistoryResponse, error) {
	if req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}
	offset := int(req.Offset)

	memories, err := s.contextService.GetDeviceHistory(ctx, req.DeviceId, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list device history: %v", err)
	}

	var pbEntries []*pb.DeviceHistoryEntry
	for _, entry := range memories {
		var devID string
		if entry.DeviceID != nil {
			devID = *entry.DeviceID
		}
		pbEntries = append(pbEntries, &pb.DeviceHistoryEntry{
			Id:        entry.ID,
			DeviceId:  devID,
			Type:      string(entry.Type),
			Summary:   entry.Summary,
			DataJson:  string(entry.Data),
			Timestamp: timestamppb.New(entry.Timestamp),
			CreatedAt: timestamppb.New(entry.CreatedAt),
		})
	}

	return &pb.GetDeviceHistoryResponse{Entries: pbEntries}, nil
}

func (s *Server) DeleteRule(ctx context.Context, req *pb.DeleteRuleRequest) (*pb.DeleteRuleResponse, error) {
	if req.RuleId == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id is required")
	}

	err := s.ruleService.Delete(ctx, req.RuleId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete rule: %v", err)
	}

	return &pb.DeleteRuleResponse{Success: true}, nil
}
