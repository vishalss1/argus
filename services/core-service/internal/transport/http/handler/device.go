package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vishalss1/argus/core/internal/domain/certificate"
	"github.com/vishalss1/argus/core/internal/domain/device"
	"github.com/vishalss1/argus/core/internal/firmware"
	"github.com/vishalss1/argus/core/internal/transport/http/dto"
	"github.com/vishalss1/argus/shared/common"
)

type DeviceHandler struct {
	service         *device.Service
	presenceService *device.PresenceService
	ca              *certificate.CertificateAuthority
	fwGen           *firmware.Generator
}

func NewDeviceHandler(
	service *device.Service,
	presenceService *device.PresenceService,
	ca *certificate.CertificateAuthority,
	fwGen *firmware.Generator,
) *DeviceHandler {
	return &DeviceHandler{
		service:         service,
		presenceService: presenceService,
		ca:              ca,
		fwGen:           fwGen,
	}
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
		ID:              req.ID,
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

	var workspaceID string
	if entity.WorkspaceID != nil {
		workspaceID = *entity.WorkspaceID
	} else if val, ok := common.GetWorkspaceID(r.Context()); ok {
		workspaceID = val
	} else {
		workspaceID = "00000000-0000-0000-0000-000000000000"
	}

	cert, err := h.ca.IssueDeviceCertificate(entity.ID, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue device certificate: "+err.Error())
		return
	}

	apiKey := ""
	if entity.RawAPIKey != nil {
		apiKey = *entity.RawAPIKey
	}

	fwVersion := entity.FirmwareVersion
	fwBytes, err := h.fwGen.Generate(entity.ID, workspaceID, apiKey, fwVersion, cert.CertPEM, cert.PrivateKeyPEM)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate firmware: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", "attachment; filename=\"firmware_"+entity.ID+".ino\"")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(fwBytes)
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

	existing, err := h.service.GetByID(r.Context(), id)
	if errors.Is(err, device.ErrDeviceNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get device")
		return
	}

	var oldHardwareID string
	if len(existing.Metadata) > 0 {
		var meta map[string]any
		if err := json.Unmarshal(existing.Metadata, &meta); err == nil {
			if hwID, ok := meta["hardware_id"].(string); ok {
				oldHardwareID = hwID
			}
		}
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

	if oldHardwareID != "" {
		h.presenceService.InvalidateResolutionCache(oldHardwareID)
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

	h.recordHeartbeat(w, r, req.DeviceID, req.Status, req.FirmwareVersion)
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
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}

	h.recordHeartbeat(w, r, id, req.Status, req.FirmwareVersion)
}

func (h *DeviceHandler) recordHeartbeat(w http.ResponseWriter, r *http.Request, id string, status string, firmwareVersion string) {
	entity, err := h.service.RecordHeartbeat(r.Context(), id, device.HeartbeatInput{
		Status:          status,
		FirmwareVersion: firmwareVersion,
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

	h.presenceService.InvalidateResolutionCache(id)

	w.WriteHeader(http.StatusNoContent)
}

// RegenerateAPIKey godoc
// @Summary Regenerate device API key
// @Tags devices
// @Produce json
// @Param deviceID path string true "Device ID"
// @Success 200 {object} device.Device
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/regenerate-api-key [post]
func (h *DeviceHandler) RegenerateAPIKey(w http.ResponseWriter, r *http.Request, id string) {
	entity, err := h.service.RegenerateAPIKey(r.Context(), id)
	if errors.Is(err, device.ErrDeviceNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, entity)
}

