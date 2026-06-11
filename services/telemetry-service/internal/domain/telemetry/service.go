package telemetry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Service struct {
	repo      Repository
	publisher EventPublisher
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

type EventPublisher interface {
	PublishTelemetry(ctx context.Context, entity Telemetry)
}

func (s *Service) SetEventPublisher(publisher EventPublisher) {
	s.publisher = publisher
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

	id := newTelemetryID(deviceID, recordedAt, input.Metrics)

	entity, err := s.repo.Create(ctx, Telemetry{
		ID:         id,
		DeviceID:   deviceID,
		RecordedAt: recordedAt,
		Metrics:    input.Metrics,
	})
	if err != nil {
		return nil, err
	}
	if s.publisher != nil {
		s.publisher.PublishTelemetry(ctx, *entity)
	}

	return entity, nil
}

func newTelemetryID(deviceID string, recordedAt time.Time, metrics []byte) string {
	// Canonicalize metrics JSON to ensure sorted keys and no whitespaces
	canonical := metrics
	var m map[string]any
	if err := json.Unmarshal(metrics, &m); err == nil {
		if sorted, err := json.Marshal(m); err == nil {
			canonical = sorted
		}
	}

	h := sha256.New()
	h.Write([]byte(deviceID))
	h.Write([]byte(recordedAt.UTC().Format(time.RFC3339Nano)))
	h.Write(canonical)

	return hex.EncodeToString(h.Sum(nil))
}
