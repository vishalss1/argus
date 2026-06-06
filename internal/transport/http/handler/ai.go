package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vishalss1/argus/internal/ai/actions"
	"github.com/vishalss1/argus/internal/ai/query"
	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
	"github.com/vishalss1/argus/internal/domain/event"
	"github.com/vishalss1/argus/internal/domain/policy"
	"github.com/vishalss1/argus/internal/infrastructure/redis"
	"github.com/vishalss1/argus/internal/transport/http/dto"
)

type AIHandler struct {
	eventRepo       event.Repository
	contextService  *ctxdomain.Service
	queryEngine     *query.Engine
	actionEngine    *actions.Engine
	policyService   *policy.Service
	redisClient     *redis.Client
	apiKey          string
	rateLimit       int
}

func NewAIHandler(
	eventRepo event.Repository,
	contextService *ctxdomain.Service,
	queryEngine *query.Engine,
	actionEngine *actions.Engine,
	policyService *policy.Service,
	redisClient *redis.Client,
	apiKey string,
	rateLimit int,
) *AIHandler {
	return &AIHandler{
		eventRepo:       eventRepo,
		contextService:  contextService,
		queryEngine:     queryEngine,
		actionEngine:    actionEngine,
		policyService:   policyService,
		redisClient:     redisClient,
		apiKey:          apiKey,
		rateLimit:       rateLimit,
	}
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
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	response, err := h.queryEngine.Ask(r.Context(), body.Query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reason: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response)
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
	limit, offset := parsePagination(r)
	events, err := h.eventRepo.List(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list events")
		return
	}
	writeJSON(w, http.StatusOK, dto.ToEventResponses(events))
}

func (h *AIHandler) ListDeviceEvents(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")
	limit, offset := parsePagination(r)
	events, err := h.eventRepo.ListByDevice(r.Context(), deviceID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list device events")
		return
	}
	writeJSON(w, http.StatusOK, dto.ToEventResponses(events))
}

func (h *AIHandler) GetDeviceStatus(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")
	ctx := r.Context()

	wsKey := fmt.Sprintf("device:%s:workspace", deviceID)
	workspaceID, err := h.redisClient.Client().Get(ctx, wsKey).Result()
	if err != nil || workspaceID == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"device_id":        deviceID,
			"status":           "offline",
			"severity":         "healthy",
			"active_incidents": 0,
			"open_incidents":   []interface{}{},
		})
		return
	}

	sessionKey := fmt.Sprintf("workspace:%s:active_session", workspaceID)
	sessionID, err := h.redisClient.Client().Get(ctx, sessionKey).Result()
	if err != nil || sessionID == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"device_id":        deviceID,
			"status":           "offline",
			"severity":         "healthy",
			"active_incidents": 0,
			"open_incidents":   []interface{}{},
		})
		return
	}

	devStateKey := fmt.Sprintf("session:%s:device:%s:state", sessionID, deviceID)
	state, err := h.redisClient.Client().HGetAll(ctx, devStateKey).Result()
	if err != nil || len(state) == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"device_id":        deviceID,
			"status":           "online",
			"severity":         "healthy",
			"active_incidents": 0,
			"open_incidents":   []interface{}{},
		})
		return
	}

	severity := state["worst_severity"]
	if severity == "" {
		severity = "healthy"
	}

	incidentsSetKey := fmt.Sprintf("session:%s:incidents", sessionID)
	incidentKeys, _ := h.redisClient.Client().SMembers(ctx, incidentsSetKey).Result()

	var openIncidents []map[string]interface{}
	var targetKeys []string
	prefix := fmt.Sprintf("session:%s:device:%s:incident:", sessionID, deviceID)
	for _, k := range incidentKeys {
		if strings.HasPrefix(k, prefix) {
			targetKeys = append(targetKeys, k)
		}
	}

	if len(targetKeys) > 0 {
		vals, err := h.redisClient.Client().MGet(ctx, targetKeys...).Result()
		if err == nil {
			for _, v := range vals {
				if vStr, ok := v.(string); ok && vStr != "" {
					var inc struct {
						Metric       string `json:"metric"`
						IncidentType string `json:"incident_type"`
						Severity     string `json:"severity"`
					}
					if err := json.Unmarshal([]byte(vStr), &inc); err == nil {
						openIncidents = append(openIncidents, map[string]interface{}{
							"metric":        inc.Metric,
							"incident_type": inc.IncidentType,
							"severity":      inc.Severity,
						})
					}
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"device_id":        deviceID,
		"status":           "online",
		"severity":         severity,
		"active_incidents": len(openIncidents),
		"open_incidents":   openIncidents,
	})
}

func (h *AIHandler) ListSessionActiveIncidents(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	ctx := r.Context()

	incidentsSetKey := fmt.Sprintf("session:%s:incidents", sessionID)
	incidentKeys, err := h.redisClient.Client().SMembers(ctx, incidentsSetKey).Result()
	if err != nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	var openIncidents []interface{}
	if len(incidentKeys) > 0 {
		vals, err := h.redisClient.Client().MGet(ctx, incidentKeys...).Result()
		if err == nil {
			for _, v := range vals {
				if vStr, ok := v.(string); ok && vStr != "" {
					var inc map[string]interface{}
					if err := json.Unmarshal([]byte(vStr), &inc); err == nil {
						openIncidents = append(openIncidents, inc)
					}
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, openIncidents)
}

func (h *AIHandler) ListFleetIncidents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	activeSessions, err := h.redisClient.Client().SMembers(ctx, "sessions:active").Result()
	if err != nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	var allIncidentKeys []string
	for _, sessionID := range activeSessions {
		incidentsSetKey := fmt.Sprintf("session:%s:incidents", sessionID)
		keys, err := h.redisClient.Client().SMembers(ctx, incidentsSetKey).Result()
		if err == nil && len(keys) > 0 {
			allIncidentKeys = append(allIncidentKeys, keys...)
		}
	}

	var openIncidents []interface{}
	if len(allIncidentKeys) > 0 {
		vals, err := h.redisClient.Client().MGet(ctx, allIncidentKeys...).Result()
		if err == nil {
			for _, v := range vals {
				if vStr, ok := v.(string); ok && vStr != "" {
					var inc map[string]interface{}
					if err := json.Unmarshal([]byte(vStr), &inc); err == nil {
						openIncidents = append(openIncidents, inc)
					}
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, openIncidents)
}



func (h *AIHandler) GetDeviceHistory(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")
	limit, offset := parsePagination(r)
	memories, err := h.contextService.GetDeviceHistory(r.Context(), deviceID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get device history")
		return
	}
	writeJSON(w, http.StatusOK, dto.ToOperationalMemoryResponses(memories))
}

func (h *AIHandler) ListActions(w http.ResponseWriter, r *http.Request) {
	records, err := h.policyService.ListRecords(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list actions")
		return
	}
	writeJSON(w, http.StatusOK, dto.ToExecutionRecordResponses(records))
}

func (h *AIHandler) ApproveAction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "actionID")
	var req dto.ApproveActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.policyService.ApproveAction(r.Context(), id, req.ApprovedBy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to approve action")
		return
	}

	if err := h.actionEngine.Execute(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to execute action: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
