package shadow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Get(ctx context.Context, deviceID string) (*Shadow, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	return s.repo.Get(ctx, deviceID)
}

func (s *Service) UpdateDesired(ctx context.Context, deviceID string, input UpdateInput) (*Shadow, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	state, err := normalizeState(input.State)
	if err != nil {
		return nil, err
	}

	return s.repo.UpdateDesired(ctx, deviceID, state)
}

func (s *Service) UpdateReported(ctx context.Context, deviceID string, input UpdateInput) (*Shadow, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	state, err := normalizeState(input.State)
	if err != nil {
		return nil, err
	}

	return s.repo.UpdateReported(ctx, deviceID, state)
}

func normalizeState(state json.RawMessage) (json.RawMessage, error) {
	if len(state) == 0 {
		return nil, errors.New("state is required")
	}
	if !json.Valid(state) {
		return nil, errors.New("state must be valid JSON")
	}

	var value any
	if err := json.Unmarshal(state, &value); err != nil {
		return nil, errors.New("state must be valid JSON")
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, errors.New("state must be a JSON object")
	}

	normalized, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(normalized), nil
}
