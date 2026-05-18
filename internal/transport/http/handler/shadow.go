package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vishalss1/argus/internal/domain/shadow"
	"github.com/vishalss1/argus/internal/transport/http/dto"
)

type ShadowHandler struct {
	service *shadow.Service
}

func NewShadowHandler(service *shadow.Service) *ShadowHandler {
	return &ShadowHandler{service: service}
}

// GetShadow godoc
// @Summary Get device shadow
// @Tags shadows
// @Produce json
// @Param deviceID path string true "Device ID"
// @Success 200 {object} shadow.Shadow
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/shadow [get]
func (h *ShadowHandler) GetShadow(w http.ResponseWriter, r *http.Request, deviceID string) {
	entity, err := h.service.Get(r.Context(), deviceID)
	if errors.Is(err, shadow.ErrShadowNotFound) {
		writeError(w, http.StatusNotFound, "shadow not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get shadow")
		return
	}

	writeJSON(w, http.StatusOK, entity)
}

// UpdateDesiredShadow godoc
// @Summary Update desired device shadow state
// @Tags shadows
// @Accept json
// @Produce json
// @Param deviceID path string true "Device ID"
// @Param request body dto.UpdateShadowStateRequest true "Desired state payload"
// @Success 200 {object} shadow.Shadow
// @Failure 400 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/shadow/desired [put]
func (h *ShadowHandler) UpdateDesiredShadow(w http.ResponseWriter, r *http.Request, deviceID string) {
	h.update(w, r, deviceID, h.service.UpdateDesired)
}

// UpdateReportedShadow godoc
// @Summary Update reported device shadow state
// @Tags shadows
// @Accept json
// @Produce json
// @Param deviceID path string true "Device ID"
// @Param request body dto.UpdateShadowStateRequest true "Reported state payload"
// @Success 200 {object} shadow.Shadow
// @Failure 400 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/shadow/reported [put]
func (h *ShadowHandler) UpdateReportedShadow(w http.ResponseWriter, r *http.Request, deviceID string) {
	h.update(w, r, deviceID, h.service.UpdateReported)
}

func (h *ShadowHandler) update(
	w http.ResponseWriter,
	r *http.Request,
	deviceID string,
	update func(context.Context, string, shadow.UpdateInput) (*shadow.Shadow, error),
) {
	var req dto.UpdateShadowStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	entity, err := update(r.Context(), deviceID, shadow.UpdateInput{State: req.State})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, entity)
}
