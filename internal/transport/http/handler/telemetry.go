package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/vishalss1/argus/internal/domain/device"
	"github.com/vishalss1/argus/internal/domain/telemetry"
	"github.com/vishalss1/argus/internal/infrastructure/redis"
	"github.com/vishalss1/argus/internal/transport/http/dto"
)

type TelemetryHandler struct {
	service       *telemetry.Service
	exportService telemetry.ExportService
	redisRepo     *redis.TelemetryRepository
	exportDir     string
}

func NewTelemetryHandler(service *telemetry.Service, exportService telemetry.ExportService, redisRepo *redis.TelemetryRepository, exportDir string) *TelemetryHandler {
	return &TelemetryHandler{
		service:       service,
		exportService: exportService,
		redisRepo:     redisRepo,
		exportDir:     exportDir,
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

func (h *TelemetryHandler) ExportTelemetry(w http.ResponseWriter, r *http.Request, deviceID string) {
	var req telemetry.ExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.DeviceID = deviceID

	if req.From.IsZero() {
		req.From = time.Now().Add(-24 * time.Hour)
	}
	if req.To.IsZero() {
		req.To = time.Now()
	}
	if req.Format == "" {
		req.Format = telemetry.ExportFormatCSV
	}

	resp, err := h.exportService.Export(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, resp)
}

func (h *TelemetryHandler) DownloadExport(w http.ResponseWriter, r *http.Request, fileName string) {
	filePath := filepath.Join(h.exportDir, fileName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "export file not found or expired")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filePath)
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
