package command

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo     Repository
	OnResult func(ctx context.Context, cmd Command)
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Send(ctx context.Context, deviceID string, input SendInput) (*Command, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	commandType := strings.TrimSpace(input.Type)
	if commandType == "" {
		return nil, errors.New("command type is required")
	}

	payload := input.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if !json.Valid(payload) {
		return nil, errors.New("payload must be valid JSON")
	}

	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, errors.New("payload must be valid JSON")
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	id, err := newCommandID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	return s.repo.Create(ctx, Command{
		ID:        id,
		DeviceID:  deviceID,
		Type:      commandType,
		Payload:   json.RawMessage(normalized),
		Status:    StatusPending,
		CreatedAt: now,
		SentAt:    &now,
		UpdatedAt: now,
	})
}

func (s *Service) ListByDevice(ctx context.Context, deviceID string) ([]Command, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	return s.repo.ListByDevice(ctx, deviceID)
}

func (s *Service) Get(ctx context.Context, deviceID string, id string) (*Command, error) {
	deviceID = strings.TrimSpace(deviceID)
	id = strings.TrimSpace(id)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}
	if id == "" {
		return nil, errors.New("command id is required")
	}

	return s.repo.Get(ctx, deviceID, id)
}

func (s *Service) Ack(ctx context.Context, deviceID string, id string, input ResultInput) (*Command, error) {
	return s.recordResult(ctx, deviceID, id, input, s.repo.Ack)
}

func (s *Service) Nack(ctx context.Context, deviceID string, id string, input ResultInput) (*Command, error) {
	return s.recordResult(ctx, deviceID, id, input, s.repo.Nack)
}

func (s *Service) recordResult(
	ctx context.Context,
	deviceID string,
	id string,
	input ResultInput,
	record func(context.Context, string, string, string) (*Command, error),
) (*Command, error) {
	deviceID = strings.TrimSpace(deviceID)
	id = strings.TrimSpace(id)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}
	if id == "" {
		return nil, errors.New("command id is required")
	}

	cmd, err := record(ctx, deviceID, id, strings.TrimSpace(input.Message))
	if err == nil && s.OnResult != nil {
		s.OnResult(ctx, *cmd)
	}
	return cmd, err
}

func newCommandID() (string, error) {
	return uuid.New().String(), nil
}
