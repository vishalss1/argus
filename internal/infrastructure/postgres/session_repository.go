package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vishalss1/argus/internal/domain/session"
)

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, s session.Session) (*session.Session, error) {
	query := `
		INSERT INTO workspace_sessions (id, workspace_id, status, started_at, ended_at, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, workspace_id, status, started_at, ended_at, created_by, created_at
	`

	var created session.Session
	err := r.db.QueryRowContext(
		ctx, query,
		s.ID, s.WorkspaceID, s.Status, s.StartedAt, s.EndedAt, s.CreatedBy, s.CreatedAt,
	).Scan(
		&created.ID, &created.WorkspaceID, &created.Status, &created.StartedAt, &created.EndedAt, &created.CreatedBy, &created.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &created, nil
}

func (r *SessionRepository) Get(ctx context.Context, id string) (*session.Session, error) {
	query := `
		SELECT id, workspace_id, status, started_at, ended_at, created_by, created_at
		FROM workspace_sessions WHERE id = $1
	`
	var s session.Session
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.WorkspaceID, &s.Status, &s.StartedAt, &s.EndedAt, &s.CreatedBy, &s.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return &s, nil
}

func (r *SessionRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]session.Session, error) {
	query := `
		SELECT id, workspace_id, status, started_at, ended_at, created_by, created_at
		FROM workspace_sessions
		WHERE workspace_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list sessions by workspace: %w", err)
	}
	defer rows.Close()

	var sessions []session.Session
	for rows.Next() {
		var s session.Session
		if err := rows.Scan(
			&s.ID, &s.WorkspaceID, &s.Status, &s.StartedAt, &s.EndedAt, &s.CreatedBy, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (r *SessionRepository) ListAllRunning(ctx context.Context) ([]session.Session, error) {
	query := `
		SELECT id, workspace_id, status, started_at, ended_at, created_by, created_at
		FROM workspace_sessions
		WHERE status = 'RUNNING'
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list running sessions: %w", err)
	}
	defer rows.Close()

	var sessions []session.Session
	for rows.Next() {
		var s session.Session
		if err := rows.Scan(
			&s.ID, &s.WorkspaceID, &s.Status, &s.StartedAt, &s.EndedAt, &s.CreatedBy, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan running session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (r *SessionRepository) UpdateStatus(ctx context.Context, id string, status session.Status, startedAt *time.Time, endedAt *time.Time) (*session.Session, error) {
	query := `
		UPDATE workspace_sessions
		SET status = $1, started_at = COALESCE($2, started_at), ended_at = COALESCE($3, ended_at)
		WHERE id = $4
		RETURNING id, workspace_id, status, started_at, ended_at, created_by, created_at
	`
	var s session.Session
	err := r.db.QueryRowContext(ctx, query, status, startedAt, endedAt, id).Scan(
		&s.ID, &s.WorkspaceID, &s.Status, &s.StartedAt, &s.EndedAt, &s.CreatedBy, &s.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update session status: %w", err)
	}
	return &s, nil
}

func (r *SessionRepository) TransitionStatus(ctx context.Context, id string, fromStatus session.Status, toStatus session.Status, startedAt *time.Time, endedAt *time.Time) (*session.Session, error) {
	query := `
		UPDATE workspace_sessions
		SET status = $1, started_at = COALESCE($2, started_at), ended_at = COALESCE($3, ended_at)
		WHERE id = $4 AND status = $5
		RETURNING id, workspace_id, status, started_at, ended_at, created_by, created_at
	`
	var s session.Session
	err := r.db.QueryRowContext(ctx, query, toStatus, startedAt, endedAt, id, fromStatus).Scan(
		&s.ID, &s.WorkspaceID, &s.Status, &s.StartedAt, &s.EndedAt, &s.CreatedBy, &s.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, session.ErrInvalidTransition
	}
	if err != nil {
		return nil, fmt.Errorf("transition session status: %w", err)
	}
	return &s, nil
}

func (r *SessionRepository) CreateEvent(ctx context.Context, e session.Event) (*session.Event, error) {
	query := `
		INSERT INTO session_events (id, session_id, device_id, event_type, severity, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, session_id, device_id, event_type, severity, payload, created_at
	`
	var created session.Event
	err := r.db.QueryRowContext(
		ctx, query,
		e.ID, e.SessionID, e.DeviceID, e.Type, e.Severity, e.Payload, e.CreatedAt,
	).Scan(&created.ID, &created.SessionID, &created.DeviceID, &created.Type, &created.Severity, &created.Payload, &created.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("create session event: %w", err)
	}
	return &created, nil
}

func (r *SessionRepository) ListEventsBySession(ctx context.Context, sessionID string) ([]session.Event, error) {
	query := `SELECT id, session_id, device_id, event_type, severity, payload, created_at FROM session_events WHERE session_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list session events: %w", err)
	}
	defer rows.Close()

	var events []session.Event
	for rows.Next() {
		var e session.Event
		if err := rows.Scan(&e.ID, &e.SessionID, &e.DeviceID, &e.Type, &e.Severity, &e.Payload, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan session event: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *SessionRepository) CreateAlert(ctx context.Context, a session.Alert) (*session.Alert, error) {
	query := `
		INSERT INTO session_alerts (id, session_id, device_id, severity, message, resolved, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, session_id, device_id, severity, message, resolved, created_at, resolved_at
	`
	var created session.Alert
	err := r.db.QueryRowContext(
		ctx, query,
		a.ID, a.SessionID, a.DeviceID, a.Severity, a.Message, a.Resolved, a.CreatedAt,
	).Scan(&created.ID, &created.SessionID, &created.DeviceID, &created.Severity, &created.Message, &created.Resolved, &created.CreatedAt, &created.ResolvedAt)

	if err != nil {
		return nil, fmt.Errorf("create session alert: %w", err)
	}
	return &created, nil
}

func (r *SessionRepository) ResolveAlert(ctx context.Context, id string) error {
	now := time.Now().UTC()
	query := `UPDATE session_alerts SET resolved = TRUE, resolved_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, now, id)
	if err != nil {
		return fmt.Errorf("resolve session alert: %w", err)
	}
	return nil
}

func (r *SessionRepository) ListAlertsBySession(ctx context.Context, sessionID string) ([]session.Alert, error) {
	query := `SELECT id, session_id, device_id, severity, message, resolved, created_at, resolved_at FROM session_alerts WHERE session_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list session alerts: %w", err)
	}
	defer rows.Close()

	var alerts []session.Alert
	for rows.Next() {
		var a session.Alert
		if err := rows.Scan(&a.ID, &a.SessionID, &a.DeviceID, &a.Severity, &a.Message, &a.Resolved, &a.CreatedAt, &a.ResolvedAt); err != nil {
			return nil, fmt.Errorf("scan session alert: %w", err)
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (r *SessionRepository) CreateCommand(ctx context.Context, c session.Command) (*session.Command, error) {
	query := `
		INSERT INTO session_commands (id, session_id, device_id, command, issued_by, status, issued_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, session_id, device_id, command, issued_by, status, issued_at, completed_at
	`
	var created session.Command
	err := r.db.QueryRowContext(
		ctx, query,
		c.ID, c.SessionID, c.DeviceID, c.Command, c.IssuedBy, c.Status, c.IssuedAt,
	).Scan(&created.ID, &created.SessionID, &created.DeviceID, &created.Command, &created.IssuedBy, &created.Status, &created.IssuedAt, &created.CompletedAt)

	if err != nil {
		return nil, fmt.Errorf("create session command: %w", err)
	}
	return &created, nil
}

func (r *SessionRepository) UpdateCommandStatus(ctx context.Context, id string, status string, completedAt *time.Time) error {
	query := `UPDATE session_commands SET status = $1, completed_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, completedAt, id)
	if err != nil {
		return fmt.Errorf("update session command status: %w", err)
	}
	return nil
}

func (r *SessionRepository) ListCommandsBySession(ctx context.Context, sessionID string) ([]session.Command, error) {
	query := `SELECT id, session_id, device_id, command, issued_by, status, issued_at, completed_at FROM session_commands WHERE session_id = $1 ORDER BY issued_at ASC`
	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list session commands: %w", err)
	}
	defer rows.Close()

	var commands []session.Command
	for rows.Next() {
		var c session.Command
		if err := rows.Scan(&c.ID, &c.SessionID, &c.DeviceID, &c.Command, &c.IssuedBy, &c.Status, &c.IssuedAt, &c.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan session command: %w", err)
		}
		commands = append(commands, c)
	}
	return commands, nil
}

func (r *SessionRepository) UpsertStatistics(ctx context.Context, s session.Statistics) error {
	query := `
		INSERT INTO session_statistics (
			session_id, duration_seconds, messages_processed, alerts_count, critical_events, uptime_percentage, average_latency_ms,
			average_battery, minimum_battery, maximum_battery, average_temperature, minimum_temperature, maximum_temperature,
			distance_travelled, device_participation_count, command_count, anomaly_count, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (session_id) DO UPDATE SET
			duration_seconds = EXCLUDED.duration_seconds,
			messages_processed = EXCLUDED.messages_processed,
			alerts_count = EXCLUDED.alerts_count,
			critical_events = EXCLUDED.critical_events,
			uptime_percentage = EXCLUDED.uptime_percentage,
			average_latency_ms = EXCLUDED.average_latency_ms,
			average_battery = EXCLUDED.average_battery,
			minimum_battery = EXCLUDED.minimum_battery,
			maximum_battery = EXCLUDED.maximum_battery,
			average_temperature = EXCLUDED.average_temperature,
			minimum_temperature = EXCLUDED.minimum_temperature,
			maximum_temperature = EXCLUDED.maximum_temperature,
			distance_travelled = EXCLUDED.distance_travelled,
			device_participation_count = EXCLUDED.device_participation_count,
			command_count = EXCLUDED.command_count,
			anomaly_count = EXCLUDED.anomaly_count,
			updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.ExecContext(
		ctx, query,
		s.SessionID, s.DurationSeconds, s.MessagesProcessed, s.AlertsCount, s.CriticalEvents, s.UptimePercentage, s.AvgLatencyMS,
		s.AvgBattery, s.MinBattery, s.MaxBattery, s.AvgTemperature, s.MinTemperature, s.MaxTemperature,
		s.DistanceTravelled, s.DeviceParticipationCount, s.CommandCount, s.AnomalyCount, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert session statistics: %w", err)
	}
	return nil
}

func (r *SessionRepository) GetStatistics(ctx context.Context, sessionID string) (*session.Statistics, error) {
	query := `
		SELECT session_id, duration_seconds, messages_processed, alerts_count, critical_events, uptime_percentage, average_latency_ms,
		       average_battery, minimum_battery, maximum_battery, average_temperature, minimum_temperature, maximum_temperature,
		       distance_travelled, device_participation_count, command_count, anomaly_count, updated_at
		FROM session_statistics WHERE session_id = $1
	`
	var s session.Statistics
	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(
		&s.SessionID, &s.DurationSeconds, &s.MessagesProcessed, &s.AlertsCount, &s.CriticalEvents, &s.UptimePercentage, &s.AvgLatencyMS,
		&s.AvgBattery, &s.MinBattery, &s.MaxBattery, &s.AvgTemperature, &s.MinTemperature, &s.MaxTemperature,
		&s.DistanceTravelled, &s.DeviceParticipationCount, &s.CommandCount, &s.AnomalyCount, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session statistics: %w", err)
	}
	return &s, nil
}

func (r *SessionRepository) CreateReport(ctx context.Context, rp session.Report) (*session.Report, error) {
	query := `
		INSERT INTO session_reports (id, session_id, report_json, generated_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, session_id, report_json, generated_at
	`
	var created session.Report
	err := r.db.QueryRowContext(
		ctx, query,
		rp.ID, rp.SessionID, rp.ReportJSON, rp.GeneratedAt,
	).Scan(&created.ID, &created.SessionID, &created.ReportJSON, &created.GeneratedAt)

	if err != nil {
		return nil, fmt.Errorf("create session report: %w", err)
	}
	return &created, nil
}

func (r *SessionRepository) GetReportBySession(ctx context.Context, sessionID string) (*session.Report, error) {
	query := `SELECT id, session_id, report_json, generated_at FROM session_reports WHERE session_id = $1`
	var rp session.Report
	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(&rp.ID, &rp.SessionID, &rp.ReportJSON, &rp.GeneratedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session report: %w", err)
	}
	return &rp, nil
}

func (r *SessionRepository) CreateAIReport(ctx context.Context, ar session.AIReport) (*session.AIReport, error) {
	query := `
		INSERT INTO session_ai_reports (id, session_id, summary_text, metadata, generated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, session_id, summary_text, metadata, generated_at
	`
	var created session.AIReport
	err := r.db.QueryRowContext(
		ctx, query,
		ar.ID, ar.SessionID, ar.SummaryText, ar.Metadata, ar.GeneratedAt,
	).Scan(&created.ID, &created.SessionID, &created.SummaryText, &created.Metadata, &created.GeneratedAt)

	if err != nil {
		return nil, fmt.Errorf("create ai report: %w", err)
	}
	return &created, nil
}

func (r *SessionRepository) GetAIReportBySession(ctx context.Context, sessionID string) (*session.AIReport, error) {
	query := `SELECT id, session_id, summary_text, metadata, generated_at FROM session_ai_reports WHERE session_id = $1`
	var ar session.AIReport
	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(&ar.ID, &ar.SessionID, &ar.SummaryText, &ar.Metadata, &ar.GeneratedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ai report: %w", err)
	}
	return &ar, nil
}

func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM workspace_sessions WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r *SessionRepository) CloseStaleSessions(ctx context.Context, timeout time.Duration) (int64, error) {
	query := `
		UPDATE workspace_sessions
		SET status = 'FAILED', ended_at = NOW()
		WHERE status = 'RUNNING' AND started_at < $1
	`
	cutoff := time.Now().UTC().Add(-timeout)
	res, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("close stale sessions: %w", err)
	}
	return res.RowsAffected()
}

func (r *SessionRepository) CreateArtifact(ctx context.Context, a session.Artifact) (*session.Artifact, error) {
	query := `
		INSERT INTO session_artifacts (session_id, workspace_id, generated_at, report_version, artifact_json)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING session_id, workspace_id, generated_at, report_version, artifact_json
	`
	var created session.Artifact
	err := r.db.QueryRowContext(
		ctx, query,
		a.SessionID, a.WorkspaceID, a.GeneratedAt, a.ReportVersion, a.ArtifactJSON,
	).Scan(&created.SessionID, &created.WorkspaceID, &created.GeneratedAt, &created.ReportVersion, &created.ArtifactJSON)

	if err != nil {
		return nil, fmt.Errorf("create session artifact: %w", err)
	}
	return &created, nil
}

func (r *SessionRepository) GetArtifactBySession(ctx context.Context, sessionID string) (*session.Artifact, error) {
	query := `SELECT session_id, workspace_id, generated_at, report_version, artifact_json FROM session_artifacts WHERE session_id = $1`
	var a session.Artifact
	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(&a.SessionID, &a.WorkspaceID, &a.GeneratedAt, &a.ReportVersion, &a.ArtifactJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session artifact: %w", err)
	}
	return &a, nil
}

