package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vishalss1/argus/internal/domain/session"
)

type SessionHandler struct {
	service   *session.Service
	manager   *session.Manager
	exportDir string
}

func NewSessionHandler(service *session.Service, manager *session.Manager, exportDir string) *SessionHandler {
	return &SessionHandler{
		service:   service,
		manager:   manager,
		exportDir: exportDir,
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

func (h *SessionHandler) Export(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session id is required")
		return
	}

	var input struct {
		Format string `json:"format"` // json or csv
	}
	_ = json.NewDecoder(r.Body).Decode(&input)

	format := strings.ToLower(input.Format)
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" {
		writeError(w, http.StatusBadRequest, "supported formats are 'json' and 'csv'")
		return
	}

	ctx := r.Context()
	sess, err := h.service.Get(ctx, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sess == nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	artifact, err := h.service.Repo().GetArtifactBySession(ctx, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if artifact == nil {
		writeError(w, http.StatusNotFound, "session artifact not found")
		return
	}

	// Ensure directory exists
	if err := os.MkdirAll(h.exportDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create export directory")
		return
	}

	fileName := fmt.Sprintf("export_session_%s.%s", sessionID, format)
	filePath := filepath.Join(h.exportDir, fileName)

	if format == "json" {
		file, err := os.Create(filePath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer file.Close()

		_, _ = file.Write(artifact.ArtifactJSON)
	} else if format == "csv" {
		var payload session.SessionArtifactPayload
		if err := json.Unmarshal(artifact.ArtifactJSON, &payload); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		file, err := os.Create(filePath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer file.Close()

		writer := csv.NewWriter(file)
		defer writer.Flush()

		// 1. Device Summaries
		_ = writer.Write([]string{"=== DEVICE SUMMARIES ==="})
		_ = writer.Write([]string{
			"Device ID", "First Seen", "Last Seen", "Uptime %", "Sample Count",
			"Battery Avg", "Battery Min", "Battery Max",
			"Temp Avg", "Temp Min", "Temp Max",
			"Signal Avg", "Signal Min", "Signal Max",
			"Distance Travelled (km)", "Warnings", "Criticals", "Commands", "Anomalies",
		})
		for _, summary := range payload.DeviceSummaries {
			_ = writer.Write([]string{
				summary.DeviceID, summary.FirstSeen, summary.LastSeen, fmt.Sprintf("%.2f", summary.UptimePercentage), fmt.Sprintf("%d", summary.SampleCount),
				fmt.Sprintf("%.2f", summary.BatteryAverage), fmt.Sprintf("%.2f", summary.BatteryMin), fmt.Sprintf("%.2f", summary.BatteryMax),
				fmt.Sprintf("%.2f", summary.TemperatureAverage), fmt.Sprintf("%.2f", summary.TemperatureMin), fmt.Sprintf("%.2f", summary.TemperatureMax),
				fmt.Sprintf("%.2f", summary.SignalAverage), fmt.Sprintf("%.2f", summary.SignalMin), fmt.Sprintf("%.2f", summary.SignalMax),
				fmt.Sprintf("%.4f", summary.DistanceTravelled), fmt.Sprintf("%d", summary.WarningCount), fmt.Sprintf("%d", summary.CriticalCount),
				fmt.Sprintf("%d", summary.CommandsReceived), fmt.Sprintf("%d", summary.AnomaliesDetected),
			})
		}
		_ = writer.Write([]string{""}) // spacing row

		// 2. Telemetry Rollups
		_ = writer.Write([]string{"=== TELEMETRY ROLLUPS ==="})
		_ = writer.Write([]string{
			"Device ID", "Timestamp", "Battery Avg", "Battery Min", "Battery Max",
			"Temp Avg", "Temp Min", "Temp Max",
			"Signal Avg", "Signal Min", "Signal Max", "Sample Count",
		})
		for devID, rollups := range payload.TelemetryRollups {
			for _, r := range rollups {
				_ = writer.Write([]string{
					devID, r.Timestamp,
					fmt.Sprintf("%.2f", r.BatteryAvg), fmt.Sprintf("%.2f", r.BatteryMin), fmt.Sprintf("%.2f", r.BatteryMax),
					fmt.Sprintf("%.2f", r.TemperatureAvg), fmt.Sprintf("%.2f", r.TemperatureMin), fmt.Sprintf("%.2f", r.TemperatureMax),
					fmt.Sprintf("%.2f", r.SignalAvg), fmt.Sprintf("%.2f", r.SignalMin), fmt.Sprintf("%.2f", r.SignalMax),
					fmt.Sprintf("%d", r.SampleCount),
				})
			}
		}
		_ = writer.Write([]string{""}) // spacing row

		// 3. Alerts
		_ = writer.Write([]string{"=== ALERTS ==="})
		_ = writer.Write([]string{"Timestamp", "Severity", "Source Device", "Type", "Message", "Resolution State"})
		for _, a := range payload.Alerts {
			_ = writer.Write([]string{a.Timestamp, a.Severity, a.SourceDevice, a.AlertType, a.Message, a.ResolutionState})
		}
		_ = writer.Write([]string{""}) // spacing row

		// 4. Commands
		_ = writer.Write([]string{"=== COMMANDS ==="})
		_ = writer.Write([]string{"Timestamp", "Target Device", "Command", "Status", "Ack Time"})
		for _, c := range payload.Commands {
			ackTimeVal := "-"
			if c.AcknowledgementTime != nil {
				ackTimeVal = *c.AcknowledgementTime
			}
			_ = writer.Write([]string{c.Timestamp, c.TargetDevice, c.Command, c.Status, ackTimeVal})
		}
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	go func() {
		time.Sleep(24 * time.Hour)
		_ = os.Remove(filePath)
	}()

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	response := map[string]interface{}{
		"download_url": fmt.Sprintf("%s://%s/api/telemetry/exports/%s", scheme, r.Host, fileName),
		"expires_at":   expiresAt,
	}

	writeJSON(w, http.StatusAccepted, response)
}

func (h *SessionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sessionID")
	sess, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sess == nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (h *SessionHandler) GetStatistics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sessionID")
	stats, err := h.service.Repo().GetStatistics(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stats == nil {
		writeError(w, http.StatusNotFound, "session statistics not found")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *SessionHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sessionID")
	report, err := h.service.Repo().GetReportBySession(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if report == nil {
		writeError(w, http.StatusNotFound, "session report not found")
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *SessionHandler) GetArtifact(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sessionID")
	artifact, err := h.service.Repo().GetArtifactBySession(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if artifact == nil {
		writeError(w, http.StatusNotFound, "session artifact not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(artifact.ArtifactJSON)
}

