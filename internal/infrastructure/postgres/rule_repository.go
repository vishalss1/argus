package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vishalss1/argus/internal/domain/rule"
)

type RuleRepository struct {
	db *sql.DB
}

func NewRuleRepository(db *sql.DB) *RuleRepository {
	return &RuleRepository{db: db}
}

func (r *RuleRepository) CreateRule(ctx context.Context, entity rule.Rule) (*rule.Rule, error) {
	const query = `
		INSERT INTO rules (id, name, metric, operator, threshold, enabled)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)
		RETURNING id, name, metric, operator, threshold, enabled, created_at, updated_at`

	created, err := scanRule(r.db.QueryRowContext(ctx, query, entity.ID, entity.Name, entity.Metric, entity.Operator, entity.Threshold, entity.Enabled))
	if err != nil {
		return nil, fmt.Errorf("create rule: %w", err)
	}

	return created, nil
}

func (r *RuleRepository) ListRules(ctx context.Context) ([]rule.Rule, error) {
	const query = `
		SELECT id, name, metric, operator, threshold, enabled, created_at, updated_at
		FROM rules
		ORDER BY created_at DESC`

	return r.listRules(ctx, query)
}

func (r *RuleRepository) ListEnabledRules(ctx context.Context) ([]rule.Rule, error) {
	const query = `
		SELECT id, name, metric, operator, threshold, enabled, created_at, updated_at
		FROM rules
		WHERE enabled = TRUE
		ORDER BY created_at DESC`

	return r.listRules(ctx, query)
}

func (r *RuleRepository) GetRule(ctx context.Context, id string) (*rule.Rule, error) {
	const query = `
		SELECT id, name, metric, operator, threshold, enabled, created_at, updated_at
		FROM rules
		WHERE id = $1::uuid`

	entity, err := scanRule(r.db.QueryRowContext(ctx, query, id))
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

	const query = `
		UPDATE rules
		SET name = $2,
			metric = $3,
			operator = $4,
			threshold = $5,
			enabled = $6,
			updated_at = NOW()
		WHERE id = $1::uuid
		RETURNING id, name, metric, operator, threshold, enabled, created_at, updated_at`

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
	result, err := r.db.ExecContext(ctx, "DELETE FROM rules WHERE id = $1::uuid", id)
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

func (r *RuleRepository) CreateAlert(ctx context.Context, entity rule.Alert) (*rule.Alert, error) {
	const query = `
		INSERT INTO alerts (id, rule_id, device_id, telemetry_id, metric, operator, threshold, observed_value, severity, message)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7, $8, $9, $10)
		RETURNING id, rule_id, device_id, telemetry_id, metric, operator, threshold, observed_value, severity, message, created_at`

	alert, err := scanAlert(r.db.QueryRowContext(
		ctx,
		query,
		entity.ID,
		entity.RuleID,
		entity.DeviceID,
		entity.TelemetryID,
		entity.Metric,
		entity.Operator,
		entity.Threshold,
		entity.ObservedValue,
		entity.Severity,
		entity.Message,
	))
	if err != nil {
		return nil, fmt.Errorf("create alert: %w", err)
	}

	return alert, nil
}

func (r *RuleRepository) ListAlerts(ctx context.Context) ([]rule.Alert, error) {
	const query = `
		SELECT id, rule_id, device_id, telemetry_id, metric, operator, threshold, observed_value, message, created_at
		FROM alerts
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
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

func (r *RuleRepository) listRules(ctx context.Context, query string) ([]rule.Rule, error) {
	rows, err := r.db.QueryContext(ctx, query)
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
	err := scanner.Scan(
		&entity.ID,
		&entity.RuleID,
		&entity.DeviceID,
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

	return &entity, nil
}
