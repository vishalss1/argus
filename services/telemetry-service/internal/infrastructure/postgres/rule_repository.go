package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	common "github.com/vishalss1/argus/shared/common"
	"github.com/vishalss1/argus/telemetry/internal/domain/rule"
)

type RuleRepository struct {
	db *sql.DB
}

func NewRuleRepository(db *sql.DB) *RuleRepository {
	return &RuleRepository{db: db}
}

func (r *RuleRepository) CreateRule(ctx context.Context, entity rule.Rule) (*rule.Rule, error) {
	const query = `
		INSERT INTO rules (id, workspace_id, name, metric, operator, threshold, enabled)
		VALUES ($1::uuid, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7)
		RETURNING id, COALESCE(workspace_id::text, ''), name, metric, operator, threshold, enabled, created_at, updated_at`

	workspaceID := entity.WorkspaceID
	if wID, ok := common.GetWorkspaceID(ctx); ok && workspaceID == "" {
		workspaceID = wID
	}

	created, err := scanRule(r.db.QueryRowContext(ctx, query, entity.ID, workspaceID, entity.Name, entity.Metric, entity.Operator, entity.Threshold, entity.Enabled))
	if err != nil {
		return nil, fmt.Errorf("create rule: %w", err)
	}

	return created, nil
}

func (r *RuleRepository) ListRules(ctx context.Context) ([]rule.Rule, error) {
	if wID, ok := common.GetWorkspaceID(ctx); ok {
		const query = `
			SELECT id, COALESCE(workspace_id::text, ''), name, metric, operator, threshold, enabled, created_at, updated_at
			FROM rules
			WHERE workspace_id = $1::uuid
			ORDER BY created_at DESC
			LIMIT 200`

		return r.listRules(ctx, query, wID)
	}

	const query = `
		SELECT id, COALESCE(workspace_id::text, ''), name, metric, operator, threshold, enabled, created_at, updated_at
		FROM rules
		ORDER BY created_at DESC
		LIMIT 200`

	return r.listRules(ctx, query)
}

func (r *RuleRepository) ListEnabledRulesByWorkspace(ctx context.Context, workspaceID string) ([]rule.Rule, error) {
	const query = `
		SELECT id, COALESCE(workspace_id::text, ''), name, metric, operator, threshold, enabled, created_at, updated_at
		FROM rules
		WHERE workspace_id = $1::uuid AND enabled = TRUE
		ORDER BY created_at DESC`

	return r.listRules(ctx, query, workspaceID)
}

func (r *RuleRepository) GetRule(ctx context.Context, id string) (*rule.Rule, error) {
	query := `
		SELECT id, COALESCE(workspace_id::text, ''), name, metric, operator, threshold, enabled, created_at, updated_at
		FROM rules
		WHERE id = $1::uuid`
	args := []any{id}
	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query += " AND workspace_id = $2::uuid"
		args = append(args, wID)
	}

	entity, err := scanRule(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, rule.ErrRuleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get rule: %w", err)
	}

	return entity, nil
}

func (r *RuleRepository) UpdateRule(ctx context.Context, id string, input rule.UpdateInput) (*rule.Rule, error) {
	current, err := r.GetRule(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		current.Name = *input.Name
	}
	if input.Metric != nil {
		current.Metric = *input.Metric
	}
	if input.Operator != nil {
		current.Operator = *input.Operator
	}
	if input.Threshold != nil {
		current.Threshold = *input.Threshold
	}
	if input.Enabled != nil {
		current.Enabled = *input.Enabled
	}

	// GetRule above already enforced workspace ownership for this id.
	const query = `
		UPDATE rules
		SET name = $2,
			metric = $3,
			operator = $4,
			threshold = $5,
			enabled = $6,
			updated_at = NOW()
		WHERE id = $1::uuid
		RETURNING id, COALESCE(workspace_id::text, ''), name, metric, operator, threshold, enabled, created_at, updated_at`

	updated, err := scanRule(r.db.QueryRowContext(ctx, query, id, current.Name, current.Metric, current.Operator, current.Threshold, current.Enabled))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, rule.ErrRuleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update rule: %w", err)
	}

	return updated, nil
}

func (r *RuleRepository) DeleteRule(ctx context.Context, id string) error {
	query := "DELETE FROM rules WHERE id = $1::uuid"
	args := []any{id}
	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query += " AND workspace_id = $2::uuid"
		args = append(args, wID)
	}
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete rule rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return rule.ErrRuleNotFound
	}

	return nil
}

func (r *RuleRepository) ListAlerts(ctx context.Context) ([]rule.Alert, error) {
	var rows *sql.Rows
	var err error

	if wID, ok := common.GetWorkspaceID(ctx); ok {
		const query = `
			SELECT a.id, a.rule_id, a.device_id, COALESCE(a.workspace_id::text, ''), a.telemetry_id, a.metric, a.operator, a.threshold, a.observed_value, a.severity, a.message, a.created_at
			FROM alerts a
			WHERE a.workspace_id = $1::uuid
			ORDER BY a.created_at DESC
			LIMIT 500`
		rows, err = r.db.QueryContext(ctx, query, wID)
	} else {
		const query = `
			SELECT id, rule_id, device_id, COALESCE(workspace_id::text, ''), telemetry_id, metric, operator, threshold, observed_value, severity, message, created_at
			FROM alerts
			ORDER BY created_at DESC
			LIMIT 500`
		rows, err = r.db.QueryContext(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	alerts := make([]rule.Alert, 0)
	for rows.Next() {
		alert, err := scanAlert(rows)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, *alert)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list alerts rows: %w", err)
	}

	return alerts, nil
}

func (r *RuleRepository) listRules(ctx context.Context, query string, args ...any) ([]rule.Rule, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	defer rows.Close()

	rules := make([]rule.Rule, 0)
	for rows.Next() {
		entity, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *entity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list rules rows: %w", err)
	}

	return rules, nil
}

type ruleScanner interface {
	Scan(dest ...any) error
}

func scanRule(scanner ruleScanner) (*rule.Rule, error) {
	var entity rule.Rule
	err := scanner.Scan(
		&entity.ID,
		&entity.WorkspaceID,
		&entity.Name,
		&entity.Metric,
		&entity.Operator,
		&entity.Threshold,
		&entity.Enabled,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &entity, nil
}

func scanAlert(scanner ruleScanner) (*rule.Alert, error) {
	var entity rule.Alert
	var wIDStr string
	err := scanner.Scan(
		&entity.ID,
		&entity.RuleID,
		&entity.DeviceID,
		&wIDStr,
		&entity.TelemetryID,
		&entity.Metric,
		&entity.Operator,
		&entity.Threshold,
		&entity.ObservedValue,
		&entity.Severity,
		&entity.Message,
		&entity.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	entity.WorkspaceID = wIDStr

	return &entity, nil
}

func (r *RuleRepository) CreateAlertsBatch(ctx context.Context, entities []rule.Alert) error {
	if len(entities) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for alerts batch: %w", err)
	}
	defer tx.Rollback()

	const placeholdersPerAlert = 12
	chunkSize := 100

	for i := 0; i < len(entities); i += chunkSize {
		end := i + chunkSize
		if end > len(entities) {
			end = len(entities)
		}
		chunk := entities[i:end]

		query := "INSERT INTO alerts (id, rule_id, device_id, workspace_id, telemetry_id, metric, operator, threshold, observed_value, severity, message, created_at) VALUES "
		vals := make([]any, 0, len(chunk)*placeholdersPerAlert)

		for idx, entity := range chunk {
			pStart := idx * placeholdersPerAlert
			query += fmt.Sprintf(
				"($%d::uuid, $%d::uuid, $%d::uuid, NULLIF($%d, '')::uuid, $%d::uuid, $%d, $%d, $%d, $%d, $%d, $%d, $%d),",
				pStart+1, pStart+2, pStart+3, pStart+4, pStart+5, pStart+6, pStart+7, pStart+8, pStart+9, pStart+10, pStart+11, pStart+12,
			)

			var telemetryIDVal *string
			if entity.TelemetryID != nil && *entity.TelemetryID != "" {
				telemetryIDVal = entity.TelemetryID
			}

			wID, _ := common.GetWorkspaceID(ctx)
			if wID == "" && entity.WorkspaceID != "" {
				wID = entity.WorkspaceID
			}

			vals = append(vals,
				entity.ID,
				entity.RuleID,
				entity.DeviceID,
				wID,
				telemetryIDVal,
				entity.Metric,
				entity.Operator,
				entity.Threshold,
				entity.ObservedValue,
				entity.Severity,
				entity.Message,
				entity.CreatedAt,
			)
		}

		query = query[:len(query)-1] // trim trailing comma

		_, err = tx.ExecContext(ctx, query, vals...)
		if err != nil {
			return fmt.Errorf("execute alerts batch chunk: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit alerts batch: %w", err)
	}

	return nil
}
