package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vishalss1/argus/src/internal/model"
	"github.com/vishalss1/argus/src/internal/repository"
)

type TelemetryService struct {
	repo repository.TelemetryRepository
}

func NewTelemetryService(repo repository.TelemetryRepository) *TelemetryService {
	return &TelemetryService{repo: repo}
}

func (s *TelemetryService) Ingest(ctx context.Context, deviceID string, req model.CreateTelemetryRequest) (*model.Telemetry, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	if len(req.Metrics) == 0 {
		return nil, errors.New("metrics are required")
	}
	if !json.Valid(req.Metrics) {
		return nil, errors.New("metrics must be valid JSON")
	}

	recordedAt := time.Now().UTC()
	if req.RecordedAt != nil {
		recordedAt = req.RecordedAt.UTC()
	}

	id, err := newTelemetryID()
	if err != nil {
		return nil, err
	}

	return s.repo.Create(ctx, model.Telemetry{
		ID:         id,
		DeviceID:   deviceID,
		RecordedAt: recordedAt,
		Metrics:    req.Metrics,
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
