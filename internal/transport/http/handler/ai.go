package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vishalss1/argus/internal/ai/query"
	"github.com/vishalss1/argus/internal/domain/event"
	"github.com/vishalss1/argus/internal/domain/incident"
	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
	"github.com/vishalss1/argus/internal/transport/http/dto"
)

type AIHandler struct {
	eventRepo      event.Repository
	incidentService *incident.Service
	contextService *ctxdomain.Service
	queryEngine    *query.Engine
}

func NewAIHandler(
	eventRepo event.Repository,
	incidentService *incident.Service,
	contextService *ctxdomain.Service,
	queryEngine *query.Engine,
) *AIHandler {
	return &AIHandler{
		eventRepo:      eventRepo,
		incidentService: incidentService,
		contextService: contextService,
		queryEngine:    queryEngine,
	}
}

func (h *AIHandler) Ask(w http.ResponseWriter, r *http.Request) {
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

func (h *AIHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.eventRepo.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list events")
		return
	}
	writeJSON(w, http.StatusOK, dto.ToEventResponses(events))
}

func (h *AIHandler) ListDeviceEvents(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")
	events, err := h.eventRepo.ListByDevice(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list device events")
		return
	}
	writeJSON(w, http.StatusOK, dto.ToEventResponses(events))
}

func (h *AIHandler) ListIncidents(w http.ResponseWriter, r *http.Request) {
	incidents, err := h.incidentService.ListIncidents(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list incidents")
		return
	}
	writeJSON(w, http.StatusOK, dto.ToIncidentResponses(incidents))
}

func (h *AIHandler) GetIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "incidentID")
	inc, err := h.incidentService.GetIncident(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "incident not found")
		return
	}
	writeJSON(w, http.StatusOK, dto.ToIncidentResponse(*inc))
}

func (h *AIHandler) ResolveIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "incidentID")
	if err := h.incidentService.ResolveIncident(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve incident")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AIHandler) GetDeviceHistory(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")
	memories, err := h.contextService.GetDeviceHistory(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get device history")
		return
	}
	writeJSON(w, http.StatusOK, dto.ToOperationalMemoryResponses(memories))
}
