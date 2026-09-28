package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vishalss1/argus/core/internal/domain/telemetry"
	"github.com/vishalss1/argus/core/internal/domain/workspace"
	"github.com/vishalss1/argus/core/internal/infrastructure/redis"
)

type TelemetryHandler struct {
	service   *telemetry.Service
	redisRepo *redis.TelemetryRepository
	workspace *workspace.Service
}

func NewTelemetryHandler(service *telemetry.Service, redisRepo *redis.TelemetryRepository, workspaceService *workspace.Service) *TelemetryHandler {
	return &TelemetryHandler{
		service:   service,
		redisRepo: redisRepo,
		workspace: workspaceService,
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



// ListLatestTelemetry returns the latest reading for every device in a
// workspace, keyed by device ID. Devices with no live reading are omitted.
// This exists so a fleet view can render link metrics in one request instead
// of one request per device.
func (h *TelemetryHandler) ListLatestTelemetry(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace id is required")
		return
	}
	if h.redisRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "live telemetry not available")
		return
	}
	if h.workspace == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace lookup not available")
		return
	}

	devices, err := h.workspace.ListDevices(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workspace devices")
		return
	}

	deviceIDs := make([]string, 0, len(devices))
	for _, d := range devices {
		deviceIDs = append(deviceIDs, d.ID)
	}

	latest, err := h.redisRepo.GetLatestForDevices(r.Context(), deviceIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read live telemetry")
		return
	}

	writeJSON(w, http.StatusOK, latest)
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

