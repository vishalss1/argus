package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vishalss1/argus/core/internal/domain/device"
	"github.com/vishalss1/argus/core/internal/domain/telemetry"
	"github.com/vishalss1/argus/core/internal/infrastructure/redis"
	"github.com/vishalss1/argus/core/internal/transport/http/dto"
)

type TelemetryHandler struct {
	service   *telemetry.Service
	redisRepo *redis.TelemetryRepository
}

func NewTelemetryHandler(service *telemetry.Service, redisRepo *redis.TelemetryRepository) *TelemetryHandler {
	return &TelemetryHandler{
		service:   service,
		redisRepo: redisRepo,
	}
}

func (h *TelemetryHandler) GetLatestTelemetry(w http.ResponseWriter, r *http.Request, deviceID string) {
	if h.redisRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "live telemetry not available")
		return
	}

	entity, err := h.redisRepo.GetLatest(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "no live telemetry found for device")
		return
	}

	writeJSON(w, http.StatusOK, entity)
}



// IngestTelemetry godoc
// @Summary Ingest telemetry
// @Tags telemetry
// @Accept json
// @Produce json
// @Param deviceID path string true "Device ID"
// @Param request body dto.CreateTelemetryRequest true "Telemetry payload"
// @Success 201 {object} telemetry.Telemetry
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/telemetry [post]
func (h *TelemetryHandler) IngestTelemetry(w http.ResponseWriter, r *http.Request, deviceID string) {
	var req dto.CreateTelemetryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	entity, err := h.service.Ingest(r.Context(), deviceID, telemetry.CreateInput{
		RecordedAt: req.RecordedAt,
		Metrics:    req.Metrics,
	})
	if errors.Is(err, device.ErrDeviceNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, entity)
}

