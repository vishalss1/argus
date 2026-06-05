package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vishalss1/argus/internal/domain/workspace"
)

type WorkspaceHandler struct {
	service *workspace.Service
}

func NewWorkspaceHandler(service *workspace.Service) *WorkspaceHandler {
	return &WorkspaceHandler{service: service}
}

func (h *WorkspaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid input")
		return
	}

	ws, err := h.service.Create(r.Context(), input.Name, input.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, ws)
}

func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaces, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workspaces)
}

func (h *WorkspaceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "workspaceID")
	ws, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ws == nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

func (h *WorkspaceHandler) AssignDevice(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	var input struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid input")
		return
	}
	if err := h.service.AssignDevice(r.Context(), workspaceID, input.DeviceID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *WorkspaceHandler) UnassignDevice(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	deviceID := chi.URLParam(r, "deviceID")
	if err := h.service.UnassignDevice(r.Context(), workspaceID, deviceID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *WorkspaceHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	devices, err := h.service.ListDevices(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

