package handler

import (
	"net/http"

	"github.com/vishalss1/argus/internal/domain/finding"
)

type FindingHandler struct {
	repo finding.Repository
}

func NewFindingHandler(repo finding.Repository) *FindingHandler {
	return &FindingHandler{repo: repo}
}

func (h *FindingHandler) ListByDevice(w http.ResponseWriter, r *http.Request, deviceID string) {
	findings, err := h.repo.ListByDevice(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, findings)
}
