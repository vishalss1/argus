package handler

import (
	"net/http"

	"github.com/vishalss1/argus/core/internal/domain/telemetry"
	"github.com/vishalss1/argus/core/internal/infrastructure/redis"
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
// IngestTelemetry handles legacy HTTP telemetry ingestion.
// ponytail: HTTP telemetry is deprecated in favor of MQTT; return 410 Gone immediately.
func (h *TelemetryHandler) IngestTelemetry(w http.ResponseWriter, r *http.Request, deviceID string) {
	writeError(w, http.StatusGone, "HTTP telemetry ingestion is deprecated. Use MQTT.")
}

