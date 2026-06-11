package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vishalss1/argus/core/internal/domain/command"
	"github.com/vishalss1/argus/core/internal/domain/device"
	"github.com/vishalss1/argus/core/internal/transport/http/dto"
)

type CommandHandler struct {
	service *command.Service
}

func NewCommandHandler(service *command.Service) *CommandHandler {
	return &CommandHandler{service: service}
}

// SendCommand godoc
// @Summary Send command to device
// @Tags commands
// @Accept json
// @Produce json
// @Param deviceID path string true "Device ID"
// @Param request body dto.SendCommandRequest true "Command payload"
// @Success 201 {object} command.Command
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/commands [post]
func (h *CommandHandler) SendCommand(w http.ResponseWriter, r *http.Request, deviceID string) {
	var req dto.SendCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	entity, err := h.service.Send(r.Context(), deviceID, command.SendInput{
		Type:    req.Type,
		Payload: req.Payload,
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

// ListCommands godoc
// @Summary List device commands
// @Tags commands
// @Produce json
// @Param deviceID path string true "Device ID"
// @Success 200 {array} command.Command
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/commands [get]
func (h *CommandHandler) ListCommands(w http.ResponseWriter, r *http.Request, deviceID string) {
	commands, err := h.service.ListByDevice(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, commands)
}

// GetCommand godoc
// @Summary Get device command
// @Tags commands
// @Produce json
// @Param deviceID path string true "Device ID"
// @Param commandID path string true "Command ID"
// @Success 200 {object} command.Command
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/commands/{commandID} [get]
func (h *CommandHandler) GetCommand(w http.ResponseWriter, r *http.Request, deviceID string, commandID string) {
	entity, err := h.service.Get(r.Context(), deviceID, commandID)
	if errors.Is(err, command.ErrCommandNotFound) {
		writeError(w, http.StatusNotFound, "command not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, entity)
}

