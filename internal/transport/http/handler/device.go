package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vishalss1/argus/internal/domain/device"
	"github.com/vishalss1/argus/internal/transport/http/dto"
)

type DeviceHandler struct {
	service *device.Service
}

func NewDeviceHandler(service *device.Service) *DeviceHandler {
	return &DeviceHandler{service: service}
}

// CreateDevice godoc
// @Summary Create device
// @Tags devices
// @Accept json
// @Produce json
// @Param request body dto.CreateDeviceRequest true "Device payload"
// @Success 201 {object} device.Device
// @Failure 400 {object} dto.ErrorResponse
// @Router /devices [post]
func (h *DeviceHandler) CreateDevice(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	entity, err := h.service.Create(r.Context(), device.CreateInput{
		Name:            req.Name,
		Type:            req.Type,
		FirmwareVersion: req.FirmwareVersion,
		Status:          req.Status,
		Metadata:        req.Metadata,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, entity)
}

// ListDevices godoc
// @Summary List devices
// @Tags devices
// @Produce json
// @Success 200 {array} device.Device
// @Failure 500 {object} dto.ErrorResponse
// @Router /devices [get]
func (h *DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list devices")
		return
	}

	writeJSON(w, http.StatusOK, devices)
}

// GetDevice godoc
// @Summary Get device
// @Tags devices
// @Produce json
// @Param deviceID path string true "Device ID"
// @Success 200 {object} device.Device
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /devices/{deviceID} [get]
func (h *DeviceHandler) GetDevice(w http.ResponseWriter, r *http.Request, id string) {
	entity, err := h.service.GetByID(r.Context(), id)
	if errors.Is(err, device.ErrDeviceNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get device")
		return
	}

	writeJSON(w, http.StatusOK, entity)
}

// ProvisionDevice godoc
// @Summary Provision device
// @Tags devices
// @Accept json
// @Produce json
// @Param request body dto.ProvisionDeviceRequest true "Provisioning payload"
// @Success 200 {object} device.ProvisionResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /provision [post]
func (h *DeviceHandler) ProvisionDevice(w http.ResponseWriter, r *http.Request) {
	var req dto.ProvisionDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	response, err := h.service.Provision(r.Context(), device.ProvisionInput{
		HardwareID:      req.HardwareID,
		DeviceType:      req.DeviceType,
		FirmwareVersion: req.FirmwareVersion,
		Capabilities:    req.Capabilities,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response)
}

// UpdateDevice godoc
// @Summary Update device
// @Tags devices
// @Accept json
// @Produce json
// @Param deviceID path string true "Device ID"
// @Param request body dto.UpdateDeviceRequest true "Device update payload"
// @Success 200 {object} device.Device
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /devices/{deviceID} [put]
func (h *DeviceHandler) UpdateDevice(w http.ResponseWriter, r *http.Request, id string) {
	var req dto.UpdateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	entity, err := h.service.Update(r.Context(), id, device.UpdateInput{
		Name:            req.Name,
		Type:            req.Type,
		FirmwareVersion: req.FirmwareVersion,
		Status:          req.Status,
		Metadata:        req.Metadata,
	})
	if errors.Is(err, device.ErrDeviceNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, entity)
}

// RecordGlobalHeartbeat godoc
// @Summary Record global device heartbeat
// @Tags devices
// @Accept json
// @Produce json
// @Param request body dto.HeartbeatRequest true "Heartbeat payload"
// @Success 200 {object} device.Device
// @Failure 400 {object} dto.ErrorResponse
// @Router /devices/heartbeat [post]
func (h *DeviceHandler) RecordGlobalHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req dto.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.DeviceID == "" {
		writeError(w, http.StatusBadRequest, "device_id is required in body for global heartbeat")
		return
	}

	h.recordHeartbeat(w, r, req.DeviceID, req.Status)
}

// RecordHeartbeat godoc
// @Summary Record device heartbeat
// @Tags devices
// @Accept json
// @Produce json
// @Param deviceID path string true "Device ID"
// @Param request body dto.HeartbeatRequest false "Heartbeat payload"
// @Success 200 {object} device.Device
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/heartbeat [post]
func (h *DeviceHandler) RecordHeartbeat(w http.ResponseWriter, r *http.Request, id string) {
	var req dto.HeartbeatRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	h.recordHeartbeat(w, r, id, req.Status)
}

func (h *DeviceHandler) recordHeartbeat(w http.ResponseWriter, r *http.Request, id string, status string) {
	entity, err := h.service.RecordHeartbeat(r.Context(), id, device.HeartbeatInput{Status: status})
	if errors.Is(err, device.ErrDeviceNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, entity)
}

// DeleteDevice godoc
// @Summary Delete device
// @Tags devices
// @Param deviceID path string true "Device ID"
// @Success 204
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /devices/{deviceID} [delete]
func (h *DeviceHandler) DeleteDevice(w http.ResponseWriter, r *http.Request, id string) {
	err := h.service.Delete(r.Context(), id)
	if errors.Is(err, device.ErrDeviceNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete device")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
