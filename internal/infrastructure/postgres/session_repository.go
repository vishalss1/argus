package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vishalss1/argus/internal/domain/auth"
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
	var query string
	var row *sql.Row

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `
			SELECT id, workspace_id, status, started_at, ended_at, created_by, created_at
			FROM workspace_sessions WHERE id = $1 AND workspace_id = $2
		`
		row = r.db.QueryRowContext(ctx, query, id, wID)
	} else {
		query = `
			SELECT id, workspace_id, status, started_at, ended_at, created_by, created_at
			FROM workspace_sessions WHERE id = $1
		`
		row = r.db.QueryRowContext(ctx, query, id)
	}

	var s session.Session
	err := row.Scan(
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
	var query string
	var row *sql.Row

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `
			UPDATE workspace_sessions
			SET status = $1, started_at = COALESCE($2, started_at), ended_at = COALESCE($3, ended_at)
			WHERE id = $4 AND workspace_id = $5
			RETURNING id, workspace_id, status, started_at, ended_at, created_by, created_at
		`
		row = r.db.QueryRowContext(ctx, query, status, startedAt, endedAt, id, wID)
	} else {
		query = `
			UPDATE workspace_sessions
			SET status = $1, started_at = COALESCE($2, started_at), ended_at = COALESCE($3, ended_at)
			WHERE id = $4
			RETURNING id, workspace_id, status, started_at, ended_at, created_by, created_at
		`
		row = r.db.QueryRowContext(ctx, query, status, startedAt, endedAt, id)
	}

	var s session.Session
	err := row.Scan(
		&s.ID, &s.WorkspaceID, &s.Status, &s.StartedAt, &s.EndedAt, &s.CreatedBy, &s.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update session status: %w", err)
	}
	return &s, nil
}

func (r *SessionRepository) TransitionStatus(ctx context.Context, id string, fromStatus session.Status, toStatus session.Status, startedAt *time.Time, endedAt *time.Time) (*session.Session, error) {
	var query string
	var row *sql.Row

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `
			UPDATE workspace_sessions
			SET status = $1, started_at = COALESCE($2, started_at), ended_at = COALESCE($3, ended_at)
			WHERE id = $4 AND status = $5 AND workspace_id = $6
			RETURNING id, workspace_id, status, started_at, ended_at, created_by, created_at
		`
		row = r.db.QueryRowContext(ctx, query, toStatus, startedAt, endedAt, id, fromStatus, wID)
	} else {
		query = `
			UPDATE workspace_sessions
			SET status = $1, started_at = COALESCE($2, started_at), ended_at = COALESCE($3, ended_at)
			WHERE id = $4 AND status = $5
			RETURNING id, workspace_id, status, started_at, ended_at, created_by, created_at
		`
		row = r.db.QueryRowContext(ctx, query, toStatus, startedAt, endedAt, id, fromStatus)
	}

	var s session.Session
	err := row.Scan(
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
	return &e, nil
}

func (r *SessionRepository) ListEventsBySession(ctx context.Context, sessionID string) ([]session.Event, error) {
	return []session.Event{}, nil
}

func (r *SessionRepository) CreateAlert(ctx context.Context, a session.Alert) (*session.Alert, error) {
	return &a, nil
}

func (r *SessionRepository) ResolveAlert(ctx context.Context, id string) error {
	return nil
}

func (r *SessionRepository) ListAlertsBySession(ctx context.Context, sessionID string) ([]session.Alert, error) {
	return []session.Alert{}, nil
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
	var query string
	var row *sql.Row

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `
			SELECT s.session_id, s.duration_seconds, s.messages_processed, s.alerts_count, s.critical_events, s.uptime_percentage, s.average_latency_ms,
			       s.average_battery, s.minimum_battery, s.maximum_battery, s.average_temperature, s.minimum_temperature, s.maximum_temperature,
			       s.distance_travelled, s.device_participation_count, s.command_count, s.anomaly_count, s.updated_at
			FROM session_statistics s
			JOIN workspace_sessions ws ON s.session_id = ws.id
			WHERE s.session_id = $1 AND ws.workspace_id = $2
		`
		row = r.db.QueryRowContext(ctx, query, sessionID, wID)
	} else {
		query = `
			SELECT session_id, duration_seconds, messages_processed, alerts_count, critical_events, uptime_percentage, average_latency_ms,
			       average_battery, minimum_battery, maximum_battery, average_temperature, minimum_temperature, maximum_temperature,
			       distance_travelled, device_participation_count, command_count, anomaly_count, updated_at
			FROM session_statistics WHERE session_id = $1
		`
		row = r.db.QueryRowContext(ctx, query, sessionID)
	}

	var s session.Statistics
	err := row.Scan(
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


func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	var query string
	var err error

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `DELETE FROM workspace_sessions WHERE id = $1 AND workspace_id = $2`
		_, err = r.db.ExecContext(ctx, query, id, wID)
	} else {
		query = `DELETE FROM workspace_sessions WHERE id = $1`
		_, err = r.db.ExecContext(ctx, query, id)
	}

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
	`
	_, err := r.db.ExecContext(
		ctx, query,
		a.SessionID, a.WorkspaceID, a.GeneratedAt, a.ReportVersion, a.ArtifactJSON,
	)

	if err != nil {
		return nil, fmt.Errorf("create session artifact: %w", err)
	}
	return &a, nil
}

func (r *SessionRepository) GetArtifactBySession(ctx context.Context, sessionID string) (*session.Artifact, error) {
	var query string
	var row *sql.Row

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `SELECT session_id, workspace_id, generated_at, report_version, artifact_json FROM session_artifacts WHERE session_id = $1 AND workspace_id = $2`
		row = r.db.QueryRowContext(ctx, query, sessionID, wID)
	} else {
		query = `SELECT session_id, workspace_id, generated_at, report_version, artifact_json FROM session_artifacts WHERE session_id = $1`
		row = r.db.QueryRowContext(ctx, query, sessionID)
	}

	var a session.Artifact
	err := row.Scan(&a.SessionID, &a.WorkspaceID, &a.GeneratedAt, &a.ReportVersion, &a.ArtifactJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session artifact: %w", err)
	}
	return &a, nil
}

