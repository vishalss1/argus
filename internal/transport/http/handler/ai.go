package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vishalss1/argus/internal/domain/event"
	"github.com/vishalss1/argus/internal/domain/incident"
	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
	"github.com/vishalss1/argus/internal/transport/http/dto"
)

type AIHandler struct {
	eventRepo      event.Repository
	incidentService *incident.Service
	contextService *ctxdomain.Service
}

func NewAIHandler(
	eventRepo event.Repository,
	incidentService *incident.Service,
	contextService *ctxdomain.Service,
) *AIHandler {
	return &AIHandler{
		eventRepo:      eventRepo,
		incidentService: incidentService,
		contextService: contextService,
	}
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
