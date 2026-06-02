package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vishalss1/argus/internal/domain/session"
)

type SessionHandler struct {
	service *session.Service
	manager *session.Manager
}

func NewSessionHandler(service *session.Service, manager *session.Manager) *SessionHandler {
	return &SessionHandler{
		service: service,
		manager: manager,
	}
}

func (h *SessionHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace id is required")
		return
	}

	sess, err := h.service.Create(r.Context(), workspaceID, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, sess)
}

func (h *SessionHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sessionID")
	sess, err := h.manager.StartSession(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (h *SessionHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sessionID")
	var input struct {
		Success bool `json:"success"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)

	sess, err := h.manager.StopSession(r.Context(), id, input.Success)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (h *SessionHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	sessions, err := h.service.List(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}
