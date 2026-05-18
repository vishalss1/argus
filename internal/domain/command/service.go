package command

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	repo Repository
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

	return record(ctx, deviceID, id, strings.TrimSpace(input.Message))
}

func newCommandID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate command id: %w", err)
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	encoded := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}
