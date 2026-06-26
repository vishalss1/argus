package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/vishalss1/argus/core/internal/domain/fleet"
	"github.com/vishalss1/argus/core/internal/transport/http/dto"
)

type FleetHandler struct {
	service *fleet.Service
}

func NewFleetHandler(service *fleet.Service) *FleetHandler {
	return &FleetHandler{service: service}
}

// CreateFleet godoc
// @Summary Create fleet
// @Tags fleets
// @Accept json
// @Produce application/zip
// @Param request body dto.CreateFleetRequest true "Fleet payload"
// @Success 201
// @Failure 400 {object} dto.ErrorResponse
// @Router /fleets [post]
func (h *FleetHandler) CreateFleet(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateFleetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	result, err := h.service.CreateFleet(r.Context(), fleet.CreateFleetInput{
		Name:             req.Name,
		NodeRole:         req.NodeRole,
		HardwareType:     req.HardwareType,
		NodePrefix:       req.NodePrefix,
		NodeCount:        req.NodeCount,
		FirmwareVersion:  req.FirmwareVersion,
		FirmwareTemplate: req.FirmwareTemplate,
		WiFiSSID:         req.WiFiSSID,
		WiFiPassword:     req.WiFiPassword,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	filename := fmt.Sprintf("fleet_%s.zip", strings.ReplaceAll(result.Fleet.Name, " ", "_"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(result.ZipData)
}

// ListFleets godoc
// @Summary List fleets
// @Tags fleets
// @Produce json
// @Success 200 {array} dto.FleetResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /fleets [get]
func (h *FleetHandler) ListFleets(w http.ResponseWriter, r *http.Request) {
	fleets, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list fleets")
		return
	}

	responses := make([]dto.FleetResponse, 0, len(fleets))
	for _, f := range fleets {
		responses = append(responses, mapFleetToDTO(f))
	}

	writeJSON(w, http.StatusOK, responses)
}

// GetFleet godoc
// @Summary Get fleet
// @Tags fleets
// @Produce json
// @Param fleetID path string true "Fleet ID"
// @Success 200 {object} dto.FleetResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /fleets/{fleetID} [get]
func (h *FleetHandler) GetFleet(w http.ResponseWriter, r *http.Request, id string) {
	f, err := h.service.GetWithDevices(r.Context(), id)
	if errors.Is(err, fleet.ErrFleetNotFound) {
		writeError(w, http.StatusNotFound, "fleet not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get fleet")
		return
	}

	writeJSON(w, http.StatusOK, mapFleetToDTO(*f))
}

// DeleteFleet godoc
// @Summary Delete fleet
// @Tags fleets
// @Param fleetID path string true "Fleet ID"
// @Success 204
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /fleets/{fleetID} [delete]
func (h *FleetHandler) DeleteFleet(w http.ResponseWriter, r *http.Request, id string) {
	err := h.service.Delete(r.Context(), id)
	if errors.Is(err, fleet.ErrFleetNotFound) {
		writeError(w, http.StatusNotFound, "fleet not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete fleet")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func mapFleetToDTO(f fleet.FleetWithStats) dto.FleetResponse {
	return dto.FleetResponse{
		ID:               f.ID,
		WorkspaceID:      f.WorkspaceID,
		Name:             f.Name,
		NodeRole:         f.NodeRole,
		HardwareType:     f.HardwareType,
		NodePrefix:       f.NodePrefix,
		FirmwareVersion:  f.FirmwareVersion,
		FirmwareTemplate: f.FirmwareTemplate,
		NodeCount:        f.NodeCount,
		TotalNodes:       f.TotalNodes,
		OnlineNodes:      f.OnlineNodes,
		OfflineNodes:     f.OfflineNodes,
		Devices:          f.Devices,
		CreatedAt:        f.CreatedAt,
		UpdatedAt:        f.UpdatedAt,
	}
}
