package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	commanddomain "github.com/vishalss1/argus/internal/domain/command"
	"github.com/vishalss1/argus/internal/domain/device"
)

type CommandRepository struct {
	db *sql.DB
}

func NewCommandRepository(db *sql.DB) *CommandRepository {
	return &CommandRepository{db: db}
}

func (r *CommandRepository) Create(ctx context.Context, entity commanddomain.Command) (*commanddomain.Command, error) {
	const query = `
		INSERT INTO commands (id, device_id, command_type, payload, status, sent_at)
		VALUES ($1::uuid, $2::uuid, $3, $4::jsonb, $5, $6)
		RETURNING id, device_id, command_type, payload, status, result_message, created_at, sent_at, acknowledged_at, updated_at`

	created, err := scanCommand(r.db.QueryRowContext(
		ctx,
		query,
		entity.ID,
		entity.DeviceID,
		entity.Type,
		entity.Payload,
		entity.Status,
		entity.SentAt,
	))
	if isForeignKeyViolation(err) {
		return nil, device.ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("create command: %w", err)
	}

	return created, nil
}

func (r *CommandRepository) ListByDevice(ctx context.Context, deviceID string) ([]commanddomain.Command, error) {
	const query = `
		SELECT id, device_id, command_type, payload, status, result_message, created_at, sent_at, acknowledged_at, updated_at
		FROM commands
		WHERE device_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT 200`

	rows, err := r.db.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("list commands: %w", err)
	}
	defer rows.Close()

	commands := make([]commanddomain.Command, 0)
	for rows.Next() {
		entity, err := scanCommand(rows)
		if err != nil {
			return nil, err
		}
		commands = append(commands, *entity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list commands rows: %w", err)
	}

	return commands, nil
}

func (r *CommandRepository) Get(ctx context.Context, deviceID string, id string) (*commanddomain.Command, error) {
	const query = `
		SELECT id, device_id, command_type, payload, status, result_message, created_at, sent_at, acknowledged_at, updated_at
		FROM commands
		WHERE device_id = $1::uuid AND id = $2::uuid`

	entity, err := scanCommand(r.db.QueryRowContext(ctx, query, deviceID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, commanddomain.ErrCommandNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get command: %w", err)
	}

	return entity, nil
}

func (r *CommandRepository) Ack(ctx context.Context, deviceID string, id string, message string) (*commanddomain.Command, error) {
	return r.updateResult(ctx, deviceID, id, commanddomain.StatusAcked, message)
}

func (r *CommandRepository) Nack(ctx context.Context, deviceID string, id string, reason string) (*commanddomain.Command, error) {
	return r.updateResult(ctx, deviceID, id, commanddomain.StatusNacked, reason)
}

func (r *CommandRepository) updateResult(ctx context.Context, deviceID string, id string, status string, message string) (*commanddomain.Command, error) {
	const query = `
		UPDATE commands
		SET status = $3,
			result_message = NULLIF($4, ''),
			acknowledged_at = NOW(),
			updated_at = NOW()
		WHERE device_id = $1::uuid AND id = $2::uuid
		RETURNING id, device_id, command_type, payload, status, result_message, created_at, sent_at, acknowledged_at, updated_at`

	entity, err := scanCommand(r.db.QueryRowContext(ctx, query, deviceID, id, status, message))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, commanddomain.ErrCommandNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update command result: %w", err)
	}

	return entity, nil
}

type commandScanner interface {
	Scan(dest ...any) error
}

func scanCommand(scanner commandScanner) (*commanddomain.Command, error) {
	var entity commanddomain.Command
	var resultMessage sql.NullString
	var sentAt sql.NullTime
	var acknowledgedAt sql.NullTime

	err := scanner.Scan(
		&entity.ID,
		&entity.DeviceID,
		&entity.Type,
		&entity.Payload,
		&entity.Status,
		&resultMessage,
		&entity.CreatedAt,
		&sentAt,
		&acknowledgedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if resultMessage.Valid {
		entity.ResultMessage = &resultMessage.String
	}
	if sentAt.Valid {
		entity.SentAt = timePtr(sentAt.Time)
	}
	if acknowledgedAt.Valid {
		entity.AcknowledgedAt = timePtr(acknowledgedAt.Time)
	}

	return &entity, nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}
