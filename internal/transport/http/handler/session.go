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

	stats, err := h.service.Repo().GetStatistics(ctx, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	report, err := h.service.Repo().GetReportBySession(ctx, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
		exportData := map[string]interface{}{
			"session":    sess,
			"statistics": stats,
			"report":     report,
		}
		file, err := os.Create(filePath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer file.Close()

		if err := json.NewEncoder(file).Encode(exportData); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else if format == "csv" {
		file, err := os.Create(filePath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer file.Close()

		writer := csv.NewWriter(file)
		defer writer.Flush()

		writer.Write([]string{"Metric", "Value"})
		writer.Write([]string{"Session ID", sess.ID})
		writer.Write([]string{"Workspace ID", sess.WorkspaceID})
		writer.Write([]string{"Status", string(sess.Status)})
		if sess.StartedAt != nil {
			writer.Write([]string{"Started At", sess.StartedAt.Format(time.RFC3339)})
		} else {
			writer.Write([]string{"Started At", "-"})
		}
		if sess.EndedAt != nil {
			writer.Write([]string{"Ended At", sess.EndedAt.Format(time.RFC3339)})
		} else {
			writer.Write([]string{"Ended At", "-"})
		}

		if stats != nil {
			writer.Write([]string{"Duration (Seconds)", fmt.Sprintf("%d", stats.DurationSeconds)})
			writer.Write([]string{"Messages Processed", fmt.Sprintf("%d", stats.MessagesProcessed)})
			writer.Write([]string{"Alerts Count", fmt.Sprintf("%d", stats.AlertsCount)})
			writer.Write([]string{"Critical Events", fmt.Sprintf("%d", stats.CriticalEvents)})
			writer.Write([]string{"Uptime Percentage", fmt.Sprintf("%.2f", stats.UptimePercentage)})
			writer.Write([]string{"Average Latency (ms)", fmt.Sprintf("%.2f", stats.AvgLatencyMS)})
			writer.Write([]string{"Average Battery", fmt.Sprintf("%.2f", stats.AvgBattery)})
			writer.Write([]string{"Minimum Battery", fmt.Sprintf("%.2f", stats.MinBattery)})
			writer.Write([]string{"Maximum Battery", fmt.Sprintf("%.2f", stats.MaxBattery)})
			writer.Write([]string{"Average Temperature", fmt.Sprintf("%.2f", stats.AvgTemperature)})
			writer.Write([]string{"Minimum Temperature", fmt.Sprintf("%.2f", stats.MinTemperature)})
			writer.Write([]string{"Maximum Temperature", fmt.Sprintf("%.2f", stats.MaxTemperature)})
			writer.Write([]string{"Distance Travelled (km)", fmt.Sprintf("%.4f", stats.DistanceTravelled)})
			writer.Write([]string{"Device Participation Count", fmt.Sprintf("%d", stats.DeviceParticipationCount)})
			writer.Write([]string{"Command Count", fmt.Sprintf("%d", stats.CommandCount)})
			writer.Write([]string{"Anomaly Count", fmt.Sprintf("%d", stats.AnomalyCount)})
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

