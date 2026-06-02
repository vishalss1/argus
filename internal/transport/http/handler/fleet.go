package handler

import (
	"net/http"

	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type FleetHandler struct {
	repo telemetry.FleetRepository
}

func NewFleetHandler(repo telemetry.FleetRepository) *FleetHandler {
	return &FleetHandler{repo: repo}
}

func (h *FleetHandler) GetLatestSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.repo.GetLatest(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if summary == nil {
		writeError(w, http.StatusNotFound, "no fleet summary found")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (h *FleetHandler) ListSummaries(w http.ResponseWriter, r *http.Request) {
	summaries, err := h.repo.List(r.Context(), 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, summaries)
}
