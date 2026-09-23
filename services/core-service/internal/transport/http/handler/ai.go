package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vishalss1/argus/core/internal/domain/auth"
	"github.com/vishalss1/argus/core/internal/transport/http/dto"
	telemetrygrpc "github.com/vishalss1/argus/core/internal/infrastructure/grpc"
	pb "github.com/vishalss1/argus/shared/proto/telemetry"
	"github.com/vishalss1/argus/core/internal/infrastructure/redis"
)

type AIHandler struct {
	telemetryClient *telemetrygrpc.TelemetryClient
	redisClient    *redis.Client
	apiKey         string
	rateLimit      int
}

func NewAIHandler(
	telemetryClient *telemetrygrpc.TelemetryClient,
	redisClient *redis.Client,
	apiKey string,
	rateLimit int,
) *AIHandler {
	return &AIHandler{
		telemetryClient: telemetryClient,
		redisClient:    redisClient,
		apiKey:         apiKey,
		rateLimit:      rateLimit,
	}
}

func (h *AIHandler) client() pb.TelemetryIntelligenceServiceClient {
	if h.telemetryClient == nil {
		return nil
	}
	return h.telemetryClient.Client()
}

func (h *AIHandler) Ask(w http.ResponseWriter, r *http.Request) {
	// 1. API Key Authentication Check
	if h.apiKey != "" {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "Authorization header is required")
			return
		}
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" || parts[1] != h.apiKey {
			writeError(w, http.StatusUnauthorized, "Invalid API key")
			return
		}
	}

	// 2. Redis-based Rate Limiting
	if h.redisClient != nil && h.rateLimit > 0 {
		ctx := r.Context()
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
			if idx := strings.LastIndex(ip, ":"); idx != -1 {
				ip = ip[:idx]
			}
		}
		key := fmt.Sprintf("rate_limit:ai_query:%s", ip)
		count, err := h.redisClient.Client().Incr(ctx, key).Result()
		if err == nil {
			if count == 1 {
				h.redisClient.Client().Expire(ctx, key, 1*time.Minute)
			}
			if count > int64(h.rateLimit) {
				writeError(w, http.StatusTooManyRequests, "Rate limit exceeded. Try again in a minute.")
				return
			}
		}
	}

	var body struct {
		Query    string `json:"query"`
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	body.Query = strings.TrimSpace(body.Query)
	if body.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	if len(body.Query) > 500 {
		writeError(w, http.StatusBadRequest, "query cannot exceed 500 characters")
		return
	}

	// Fetch workspace ID from context
	workspaceID := "00000000-0000-0000-0000-000000000000" // Fallback
	if wsID, ok := auth.GetWorkspaceID(r.Context()); ok {
		workspaceID = wsID
	}

	client := h.client()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: running in disconnected mode")
		return
	}

	resp, err := client.QueryAI(r.Context(), &pb.QueryAIRequest{
		Query:       body.Query,
		WorkspaceId: workspaceID,
		DeviceId:    body.DeviceID,
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to reason: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func parsePagination(r *http.Request) (int, int) {
	limit := 20
	offset := 0

	q := r.URL.Query()
	if limitStr := q.Get("limit"); limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 {
			limit = val
			if limit > 100 {
				limit = 100
			}
		}
	}

	if offsetStr := q.Get("offset"); offsetStr != "" {
		if val, err := strconv.Atoi(offsetStr); err == nil && val >= 0 {
			offset = val
		}
	}

	return limit, offset
}

func (h *AIHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	client := h.client()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: running in disconnected mode")
		return
	}

	limit, offset := parsePagination(r)
	resp, err := client.ListEvents(r.Context(), &pb.ListEventsRequest{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list events: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto.ToEventResponses(resp.Events))
}

func (h *AIHandler) ListDeviceEvents(w http.ResponseWriter, r *http.Request) {
	client := h.client()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: running in disconnected mode")
		return
	}

	deviceID := chi.URLParam(r, "deviceID")
	limit, offset := parsePagination(r)
	resp, err := client.ListEvents(r.Context(), &pb.ListEventsRequest{
		DeviceId: deviceID,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list device events: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto.ToEventResponses(resp.Events))
}

func (h *AIHandler) GetDeviceStatus(w http.ResponseWriter, r *http.Request) {
	client := h.client()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: running in disconnected mode")
		return
	}

	deviceID := chi.URLParam(r, "deviceID")
	
	resp, err := client.GetSnapshot(r.Context(), &pb.GetSnapshotRequest{
		DeviceId: deviceID,
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		// ponytail: do not fabricate 200 on backend failure; return 503
		writeError(w, http.StatusServiceUnavailable, "failed to get device status: "+err.Error())
		return
	}

	severity := "healthy"
	var openIncidents []map[string]interface{}
	for _, incType := range resp.ActiveIncidentTypes {
		severity = "warning" // Simple severity mapping from snapshot
		openIncidents = append(openIncidents, map[string]interface{}{
			"metric":        "telemetry",
			"incident_type": incType,
			"severity":      "warning",
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"device_id":        resp.DeviceId,
		"status":           resp.Status,
		"severity":         severity,
		"active_incidents": len(openIncidents),
		"open_incidents":   openIncidents,
	})
}

func (h *AIHandler) ListSessionActiveIncidents(w http.ResponseWriter, r *http.Request) {
	client := h.client()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: running in disconnected mode")
		return
	}

	sessionID := chi.URLParam(r, "sessionID")
	
	resp, err := client.ListIncidents(r.Context(), &pb.ListIncidentsRequest{
		SessionId:  sessionID,
		ActiveOnly: true,
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		// ponytail: do not fabricate empty 200 on backend failure; return 503
		writeError(w, http.StatusServiceUnavailable, "failed to list session active incidents: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp.Incidents)
}

func (h *AIHandler) ListFleetIncidents(w http.ResponseWriter, r *http.Request) {
	client := h.client()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: running in disconnected mode")
		return
	}

	resp, err := client.ListIncidents(r.Context(), &pb.ListIncidentsRequest{
		ActiveOnly: true,
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		// ponytail: do not fabricate empty 200 on backend failure; return 503
		writeError(w, http.StatusServiceUnavailable, "failed to list fleet incidents: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp.Incidents)
}

func (h *AIHandler) GetDeviceHistory(w http.ResponseWriter, r *http.Request) {
	client := h.client()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: running in disconnected mode")
		return
	}

	deviceID := chi.URLParam(r, "deviceID")
	limit, offset := parsePagination(r)
	
	resp, err := client.GetDeviceHistory(r.Context(), &pb.GetDeviceHistoryRequest{
		DeviceId: deviceID,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get device history: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto.ToOperationalMemoryResponses(resp.Entries))
}

func (h *AIHandler) ListActions(w http.ResponseWriter, r *http.Request) {
	// ponytail: return 501 Not Implemented instead of fake empty list
	writeError(w, http.StatusNotImplemented, "AI action remediation is not implemented")
}

func (h *AIHandler) ApproveAction(w http.ResponseWriter, r *http.Request) {
	// ponytail: return 501 Not Implemented instead of fake 204 success
	writeError(w, http.StatusNotImplemented, "AI action remediation is not implemented")
}
