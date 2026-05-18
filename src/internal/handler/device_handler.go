package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vishalss1/argus/src/internal/model"
	"github.com/vishalss1/argus/src/internal/repository"
	"github.com/vishalss1/argus/src/internal/service"
)

type DeviceHandler struct {
	service *service.DeviceService
}

func NewDeviceHandler(service *service.DeviceService) *DeviceHandler {
	return &DeviceHandler{service: service}
}

func (h *DeviceHandler) CreateDevice(w http.ResponseWriter, r *http.Request) {
	var req model.CreateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	device, err := h.service.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, device)
}

func (h *DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list devices")
		return
	}

	writeJSON(w, http.StatusOK, devices)
}

func (h *DeviceHandler) GetDevice(w http.ResponseWriter, r *http.Request, id string) {
	device, err := h.service.GetByID(r.Context(), id)
	if errors.Is(err, repository.ErrDeviceNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get device")
		return
	}

	writeJSON(w, http.StatusOK, device)
}

func (h *DeviceHandler) UpdateDevice(w http.ResponseWriter, r *http.Request, id string) {
	var req model.UpdateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	device, err := h.service.Update(r.Context(), id, req)
	if errors.Is(err, repository.ErrDeviceNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, device)
}

func (h *DeviceHandler) DeleteDevice(w http.ResponseWriter, r *http.Request, id string) {
	err := h.service.Delete(r.Context(), id)
	if errors.Is(err, repository.ErrDeviceNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete device")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
