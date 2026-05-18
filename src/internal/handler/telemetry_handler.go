package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vishalss1/argus/src/internal/model"
	"github.com/vishalss1/argus/src/internal/repository"
	"github.com/vishalss1/argus/src/internal/service"
)

type TelemetryHandler struct {
	service *service.TelemetryService
}

func NewTelemetryHandler(service *service.TelemetryService) *TelemetryHandler {
	return &TelemetryHandler{service: service}
}

func (h *TelemetryHandler) IngestTelemetry(w http.ResponseWriter, r *http.Request, deviceID string) {
	var req model.CreateTelemetryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	telemetry, err := h.service.Ingest(r.Context(), deviceID, req)
	if errors.Is(err, repository.ErrDeviceNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, telemetry)
}
