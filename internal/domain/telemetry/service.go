package telemetry

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

func (s *Service) Ingest(ctx context.Context, deviceID string, input CreateInput) (*Telemetry, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	if len(input.Metrics) == 0 {
		return nil, errors.New("metrics are required")
	}
	if !json.Valid(input.Metrics) {
		return nil, errors.New("metrics must be valid JSON")
	}

	recordedAt := time.Now().UTC()
	if input.RecordedAt != nil {
		recordedAt = input.RecordedAt.UTC()
	}

	id, err := newTelemetryID()
	if err != nil {
		return nil, err
	}

	return s.repo.Create(ctx, Telemetry{
		ID:         id,
		DeviceID:   deviceID,
		RecordedAt: recordedAt,
		Metrics:    input.Metrics,
	})
}

func newTelemetryID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate telemetry id: %w", err)
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	encoded := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}
