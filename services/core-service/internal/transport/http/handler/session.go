package handler

import (
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	goredis "github.com/redis/go-redis/v9"
	"github.com/vishalss1/argus/core/internal/domain/session"
	pb "github.com/vishalss1/argus/shared/proto/telemetry"
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

func (h *SessionHandler) compilePayload(sess *session.Session, telemetryResponse *pb.SessionTelemetryResponse) session.SessionArtifactPayload {
	deviceSummaries := make(map[string]session.DeviceSummaryArtifact)
	var incidentsArchive []session.ArtifactIncident
	metricsAggregates := make(map[string]map[string]session.MetricAggregate)
	hourlySummaries := make(map[string][]session.HourlySummaryArtifact)

	if telemetryResponse != nil {
		for _, summary := range telemetryResponse.DeviceSummaries {
			deviceSummaries[summary.DeviceId] = session.DeviceSummaryArtifact{
				DeviceID:               summary.DeviceId,
				FirstSeen:              summary.FirstSeen,
				LastSeen:               summary.LastSeen,
				SampleCount:            int(summary.SampleCount),
				WarningIncidentsCount:  int(summary.WarningIncidentsCount),
				CriticalIncidentsCount: int(summary.CriticalIncidentsCount),
				ActiveAtEnd:            summary.ActiveAtEnd,
			}
		}

		for _, inc := range telemetryResponse.IncidentsArchive {
			var resolvedAt *time.Time
			if inc.ResolvedAt != nil {
				t := inc.ResolvedAt.AsTime()
				resolvedAt = &t
			}
			incidentsArchive = append(incidentsArchive, session.ArtifactIncident{
				DeviceID:     inc.DeviceId,
				Metric:       inc.Metric,
				IncidentType: inc.IncidentType,
				Severity:     inc.Severity,
				StartTime:    inc.StartTime.AsTime(),
				ResolvedAt:   resolvedAt,
				Occurrences:  int(inc.Occurrences),
				PeakScore:    inc.PeakScore,
				Summary:      inc.Summary,
			})
		}

		for devID, metricAggs := range telemetryResponse.MetricsAggregates {
			devAggs := make(map[string]session.MetricAggregate)
			for metricName, agg := range metricAggs.Aggregates {
				devAggs[metricName] = session.MetricAggregate{
					Count:    int(agg.Count),
					Min:      agg.Min,
					Max:      agg.Max,
					Average:  agg.Average,
					Variance: agg.Variance,
				}
			}
			metricsAggregates[devID] = devAggs
		}

		for devID, listMsg := range telemetryResponse.HourlySummaries {
			var devSummaries []session.HourlySummaryArtifact
			for _, sum := range listMsg.Summaries {
				devSummaries = append(devSummaries, session.HourlySummaryArtifact{
					DeviceID:       sum.DeviceId,
					Hour:           sum.Hour,
					Metric:         sum.Metric,
					SampleCount:    int(sum.SampleCount),
					Min:            sum.Min,
					Max:            sum.Max,
					Average:        sum.Average,
					Variance:       sum.Variance,
					Stddev:         sum.Stddev,
					FirstTimestamp: sum.FirstTimestamp.AsTime(),
					LastTimestamp:  sum.LastTimestamp.AsTime(),
				})
			}
			hourlySummaries[devID] = devSummaries
		}

	}

	var sessionSummary string
	if len(incidentsArchive) == 0 {
		sessionSummary = fmt.Sprintf("AI Session Summary\n\nAnalyzed fleet metrics. Observed stable behavior with 0 incidents.")
	} else {
		var summaries []string
		for _, inc := range incidentsArchive {
			statusStr := "resolved"
			if inc.ResolvedAt == nil {
				statusStr = "active"
			}
			summaries = append(summaries, fmt.Sprintf("- %s on %s (%s)", inc.Summary, inc.DeviceID, statusStr))
		}
		sessionSummary = fmt.Sprintf("AI Session Summary\n\nDetected %d incidents:\n\n%s", len(incidentsArchive), summaries)
	}

	return session.SessionArtifactPayload{
		SessionID:            sess.ID,
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
		WorkspaceID:          sess.WorkspaceID,
		ReportVersion:        "3.0",
		SessionSummary:       sessionSummary,
		DeviceSummaries:      deviceSummaries,
		IncidentsArchive:     incidentsArchive,
		MetricsAggregates: metricsAggregates,
		HourlySummaries:   hourlySummaries,
	}
}

func (h *SessionHandler) ExportJSON(w http.ResponseWriter, r *http.Request) {
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

	// Completed sessions: return stored artifact directly (no reconstruction)
	if sess.Status == session.StatusCompleted || sess.Status == session.StatusFailed || sess.Status == session.StatusCancelled {
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
		return
	}

	// Running sessions: compile live payload from gRPC
	client := h.manager.TelemetryClient()
	if client == nil {
		writeError(w, http.StatusInternalServerError, "telemetry client unavailable")
		return
	}
	resp, err := client.GetSessionTelemetry(r.Context(), &pb.GetSessionTelemetryRequest{
		SessionId:   id,
		WorkspaceId: sess.WorkspaceID,
		Cleanup:     false,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to fetch live telemetry: %w", err).Error())
		return
	}
	payload := h.compilePayload(sess, resp)
	writeJSON(w, http.StatusOK, payload)
}

func (h *SessionHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
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

	var payload *session.SessionArtifactPayload

	// Completed sessions: load from stored artifact
	if sess.Status == session.StatusCompleted || sess.Status == session.StatusFailed || sess.Status == session.StatusCancelled {
		artifact, err := h.service.Repo().GetArtifactBySession(r.Context(), id)
		if err == nil && artifact != nil {
			payload, err = session.ParseArtifactPayload(artifact.ArtifactJSON)
			if err != nil {
				payload = nil
			}
		}
	}

	// Running sessions (or fallback): compile live from gRPC
	if payload == nil {
		client := h.manager.TelemetryClient()
		if client != nil {
			resp, err := client.GetSessionTelemetry(r.Context(), &pb.GetSessionTelemetryRequest{
				SessionId:   id,
				WorkspaceId: sess.WorkspaceID,
				Cleanup:     false,
			})
			if err == nil {
				compiled := h.compilePayload(sess, resp)
				payload = &compiled
			}
		}
	}

	if payload == nil {
		writeError(w, http.StatusInternalServerError, "no data available for export")
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="session_%s_export.csv"`, id))

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"device_id", "metric", "count", "min", "max", "average", "variance"}); err != nil {
		return
	}

	// Flatten metrics_aggregates into CSV rows
	for devID, metrics := range payload.MetricsAggregates {
		for metricName, agg := range metrics {
			row := []string{
				devID,
				metricName,
				strconv.Itoa(agg.Count),
				strconv.FormatFloat(agg.Min, 'f', -1, 64),
				strconv.FormatFloat(agg.Max, 'f', -1, 64),
				strconv.FormatFloat(agg.Average, 'f', -1, 64),
				strconv.FormatFloat(agg.Variance, 'f', -1, 64),
			}
			if err := writer.Write(row); err != nil {
				return
			}
		}
	}
}

func (h *SessionHandler) GetTelemetryExport(w http.ResponseWriter, r *http.Request, format string) {
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

	// Completed sessions: stream from MinIO
	if sess.Status == session.StatusCompleted || sess.Status == session.StatusFailed || sess.Status == session.StatusCancelled {
		artifact, err := h.service.Repo().GetArtifactBySession(r.Context(), id)
		if err != nil || artifact == nil {
			writeError(w, http.StatusNotFound, "session artifact not found")
			return
		}
		payload, err := session.ParseArtifactPayload(artifact.ArtifactJSON)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to parse artifact")
			return
		}

		if payload.ExportsExpired {
			writeJSON(w, http.StatusGone, map[string]interface{}{
				"error":          "telemetry export expired",
				"retention_days": 7,
			})
			return
		}

		if payload.TelemetryExportPaths == nil {
			writeError(w, http.StatusNotFound, "telemetry export not found")
			return
		}

		objectKey := payload.TelemetryExportPaths.CSV
		contentType := "text/csv"
		filename := fmt.Sprintf("session_%s_telemetry.csv", id)
		if format == "json" {
			objectKey = payload.TelemetryExportPaths.JSON
			contentType = "application/json"
			filename = fmt.Sprintf("session_%s_telemetry.json", id)
		}
		if objectKey == "" {
			writeError(w, http.StatusNotFound, "telemetry export not found")
			return
		}

		mc := h.manager.MinioClient()
		if mc == nil {
			writeError(w, http.StatusInternalServerError, "minio client unavailable")
			return
		}

		reader, err := mc.GetObject(r.Context(), objectKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read export: %v", err))
			return
		}
		defer reader.Close()

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.WriteHeader(http.StatusOK)
		gzReader, err := gzip.NewReader(reader)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to decompress: %v", err))
			return
		}
		defer gzReader.Close()
		io.Copy(w, gzReader)
		return
	}

	// Running sessions: generate live from Redis (no export paths available)
	client := h.manager.TelemetryClient()
	if client == nil {
		writeError(w, http.StatusInternalServerError, "telemetry client unavailable")
		return
	}
	resp, err := client.GetSessionTelemetry(r.Context(), &pb.GetSessionTelemetryRequest{
		SessionId:   id,
		WorkspaceId: sess.WorkspaceID,
		Cleanup:     false,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to fetch live telemetry: %w", err).Error())
		return
	}

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="session_%s_telemetry.json"`, id))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp.CurrentHourTelemetry)
	} else {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="session_%s_telemetry.csv"`, id))
		writer := csv.NewWriter(w)
		defer writer.Flush()
		writer.Write([]string{"timestamp", "device_id", "metric", "value"})
		for devID, samples := range resp.CurrentHourTelemetry {
			for _, sample := range samples.Samples {
				ts := sample.Timestamp.AsTime().Format(time.RFC3339)
				for mKey, mVal := range sample.Metrics {
					writer.Write([]string{ts, devID, mKey, strconv.FormatFloat(mVal, 'f', -1, 64)})
				}
			}
		}
	}
}

func (h *SessionHandler) GetTelemetryJSON(w http.ResponseWriter, r *http.Request) {
	h.GetTelemetryExport(w, r, "json")
}

func (h *SessionHandler) GetTelemetryCSV(w http.ResponseWriter, r *http.Request) {
	h.GetTelemetryExport(w, r, "csv")
}

func (h *SessionHandler) GetLiveTelemetry(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	deviceID := chi.URLParam(r, "deviceID")

	if sessionID == "" || deviceID == "" {
		writeError(w, http.StatusBadRequest, "sessionID and deviceID are required")
		return
	}

	rdbClient := h.manager.RedisClient()
	if rdbClient == nil {
		writeError(w, http.StatusInternalServerError, "redis client unavailable")
		return
	}
	rdb := rdbClient.Client()

	minScore := "-inf"
	maxScore := "+inf"
	if startTimeStr := r.URL.Query().Get("start_time"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			minScore = strconv.FormatInt(t.UnixMilli(), 10)
		}
	}
	if endTimeStr := r.URL.Query().Get("end_time"); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			maxScore = strconv.FormatInt(t.UnixMilli(), 10)
		}
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	desc := true
	if orderStr := r.URL.Query().Get("order"); orderStr == "asc" {
		desc = false
	}

	activeHours, _ := rdb.SMembers(r.Context(), fmt.Sprintf("session:%s:hours", sessionID)).Result()
	if len(activeHours) == 0 {
		activeHours = []string{time.Now().UTC().Format("2006-01-02-15")}
	}

	var allZ []goredis.Z
	for _, hourStr := range activeHours {
		key := fmt.Sprintf("session:%s:hour:%s:device:%s:telemetry_history", sessionID, hourStr, deviceID)
		zRange := goredis.ZRangeBy{
			Min: minScore,
			Max: maxScore,
		}
		zs, err := rdb.ZRangeByScoreWithScores(r.Context(), key, &zRange).Result()
		if err == nil {
			allZ = append(allZ, zs...)
		}
	}

	sort.Slice(allZ, func(i, j int) bool {
		if desc {
			return allZ[i].Score > allZ[j].Score
		}
		return allZ[i].Score < allZ[j].Score
	})

	total := len(allZ)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	paginated := allZ[start:end]

	rawSamples := make([]json.RawMessage, 0, len(paginated))
	for _, z := range paginated {
		if valStr, ok := z.Member.(string); ok {
			rawSamples = append(rawSamples, json.RawMessage(valStr))
		}
	}

	writeJSON(w, http.StatusOK, rawSamples)
}


