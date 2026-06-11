package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/vishalss1/argus/core/internal/domain/device"
	"github.com/vishalss1/argus/core/internal/domain/ota"
	"github.com/vishalss1/argus/core/internal/transport/http/dto"
)

const maxFirmwareUploadBytes = 256 << 20

type OTAHandler struct {
	service *ota.Service
}

func NewOTAHandler(service *ota.Service) *OTAHandler {
	return &OTAHandler{service: service}
}

// UploadFirmware godoc
// @Summary Upload firmware artifact
// @Tags ota
// @Accept multipart/form-data
// @Produce json
// @Param version formData string true "Firmware version"
// @Param firmware formData file true "Firmware binary"
// @Success 201 {object} ota.FirmwareArtifact
// @Failure 400 {object} dto.ErrorResponse
// @Router /ota/firmware [post]
func (h *OTAHandler) UploadFirmware(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFirmwareUploadBytes)
	if err := r.ParseMultipartForm(maxFirmwareUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("firmware")
	if err != nil {
		writeError(w, http.StatusBadRequest, "firmware file is required")
		return
	}
	defer file.Close()

	entity, err := h.service.UploadFirmware(r.Context(), ota.UploadInput{
		Version:     r.FormValue("version"),
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		SizeBytes:   header.Size,
	}, file)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, entity)
}

// ListFirmware godoc
// @Summary List firmware artifacts
// @Tags ota
// @Produce json
// @Success 200 {array} ota.FirmwareArtifact
// @Failure 500 {object} dto.ErrorResponse
// @Router /ota/firmware [get]
func (h *OTAHandler) ListFirmware(w http.ResponseWriter, r *http.Request) {
	artifacts, err := h.service.ListFirmware(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list firmware artifacts")
		return
	}

	writeJSON(w, http.StatusOK, artifacts)
}

func (h *OTAHandler) ListAllDeployments(w http.ResponseWriter, r *http.Request) {
	deployments, err := h.service.ListDeployments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list ota deployments")
		return
	}

	writeJSON(w, http.StatusOK, deployments)
}

func (h *OTAHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load ota stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// GetFirmware godoc
// @Summary Get firmware artifact
// @Tags ota
// @Produce json
// @Param firmwareID path string true "Firmware artifact ID"
// @Success 200 {object} ota.FirmwareArtifact
// @Failure 404 {object} dto.ErrorResponse
// @Router /ota/firmware/{firmwareID} [get]
func (h *OTAHandler) GetFirmware(w http.ResponseWriter, r *http.Request, firmwareID string) {
	artifact, err := h.service.GetFirmware(r.Context(), firmwareID)
	if errors.Is(err, ota.ErrFirmwareNotFound) {
		writeError(w, http.StatusNotFound, "firmware artifact not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, artifact)
}

// DeployFirmware godoc
// @Summary Create OTA deployment manifest for a device
// @Tags ota
// @Accept json
// @Produce json
// @Param deviceID path string true "Device ID"
// @Param request body dto.DeployOTARequest true "OTA deployment payload"
// @Success 201 {object} ota.Manifest
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/ota [post]
func (h *OTAHandler) DeployFirmware(w http.ResponseWriter, r *http.Request, deviceID string) {
	var req dto.DeployOTARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	log.Printf("[OTA] deployment create handler request device=%s artifact=%s", deviceID, req.ArtifactID)
	manifest, err := h.service.Deploy(r.Context(), deviceID, ota.DeployInput{ArtifactID: req.ArtifactID})
	if errors.Is(err, device.ErrDeviceNotFound) {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if errors.Is(err, ota.ErrFirmwareNotFound) {
		writeError(w, http.StatusNotFound, "firmware artifact not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("[OTA] deployment create handler manifest device=%s deployment=%s version=%s", manifest.DeviceID, manifest.DeploymentID, manifest.Version)
	writeJSON(w, http.StatusCreated, manifest)
}

// ListDeployments godoc
// @Summary List device OTA deployments
// @Tags ota
// @Produce json
// @Param deviceID path string true "Device ID"
// @Success 200 {array} ota.Deployment
// @Failure 400 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/ota [get]
func (h *OTAHandler) ListDeployments(w http.ResponseWriter, r *http.Request, deviceID string) {
	deployments, err := h.service.ListDeploymentsByDevice(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, deployments)
}

// GetManifest godoc
// @Summary Get OTA deployment manifest
// @Tags ota
// @Produce json
// @Param deviceID path string true "Device ID"
// @Param deploymentID path string true "OTA deployment ID"
// @Success 200 {object} ota.Manifest
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/ota/{deploymentID}/manifest [get]
func (h *OTAHandler) GetManifest(w http.ResponseWriter, r *http.Request, deviceID string, deploymentID string) {
	manifest, err := h.service.GetManifest(r.Context(), deviceID, deploymentID)
	if errors.Is(err, ota.ErrDeploymentNotFound) {
		writeError(w, http.StatusNotFound, "ota deployment not found")
		return
	}
	if errors.Is(err, ota.ErrFirmwareNotFound) {
		writeError(w, http.StatusNotFound, "firmware artifact not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, manifest)
}

func (h *OTAHandler) ListDeploymentEvents(w http.ResponseWriter, r *http.Request, deploymentID string) {
	events, err := h.service.ListDeploymentEvents(r.Context(), deploymentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, events)
}

// GetPendingDeployment godoc
// @Summary Get pending OTA deployment manifest
// @Tags ota
// @Produce json
// @Param deviceID path string true "Device ID"
// @Success 200 {object} ota.Manifest
// @Success 204
// @Failure 400 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/ota/pending [get]
func (h *OTAHandler) GetPendingDeployment(w http.ResponseWriter, r *http.Request, deviceID string) {
	log.Printf("[OTA] pending handler request device=%s", deviceID)
	manifest, err := h.service.GetPendingManifest(r.Context(), deviceID)
	if errors.Is(err, ota.ErrDeploymentNotFound) {
		log.Printf("[OTA] pending handler no deployment device=%s", deviceID)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		log.Printf("[OTA] pending handler failed device=%s error=%v", deviceID, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("[OTA] pending handler manifest delivery device=%s deployment=%s version=%s", deviceID, manifest.DeploymentID, manifest.Version)
	writeJSON(w, http.StatusOK, manifest)
}

// AckDeployment godoc
// @Summary ACK OTA deployment
// @Tags ota
// @Accept json
// @Produce json
// @Param deviceID path string true "Device ID"
// @Param deploymentID path string true "OTA deployment ID"
// @Param request body dto.OTAResultRequest false "ACK payload"
// @Success 200 {object} ota.Deployment
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/ota/{deploymentID}/ack [post]
func (h *OTAHandler) AckDeployment(w http.ResponseWriter, r *http.Request, deviceID string, deploymentID string) {
	h.recordResult(w, r, deviceID, deploymentID, h.service.Ack)
}

// NackDeployment godoc
// @Summary NACK OTA deployment
// @Tags ota
// @Accept json
// @Produce json
// @Param deviceID path string true "Device ID"
// @Param deploymentID path string true "OTA deployment ID"
// @Param request body dto.OTAResultRequest false "NACK payload"
// @Success 200 {object} ota.Deployment
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /devices/{deviceID}/ota/{deploymentID}/nack [post]
func (h *OTAHandler) NackDeployment(w http.ResponseWriter, r *http.Request, deviceID string, deploymentID string) {
	h.recordResult(w, r, deviceID, deploymentID, h.service.Nack)
}

func (h *OTAHandler) recordResult(
	w http.ResponseWriter,
	r *http.Request,
	deviceID string,
	deploymentID string,
	record func(context.Context, string, string, ota.ResultInput) (*ota.Deployment, error),
) {
	var req dto.OTAResultRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if !errors.Is(err, io.EOF) {
				writeError(w, http.StatusBadRequest, "invalid JSON body")
				return
			}
		}
	}

	deployment, err := record(r.Context(), deviceID, deploymentID, ota.ResultInput{Message: req.Message})
	if errors.Is(err, ota.ErrDeploymentNotFound) {
		writeError(w, http.StatusNotFound, "ota deployment not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, deployment)
}

