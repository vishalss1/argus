package device

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
	repo      Repository
	publisher EventPublisher
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

type EventPublisher interface {
	PublishDeviceUpdate(ctx context.Context, entity Device)
}

func (s *Service) SetEventPublisher(publisher EventPublisher) {
	s.publisher = publisher
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*Device, error) {
	id, err := newDeviceID()
	if err != nil {
		return nil, err
	}

	device := Device{
		ID:              id,
		Name:            strings.TrimSpace(input.Name),
		Type:            strings.TrimSpace(input.Type),
		FirmwareVersion: strings.TrimSpace(input.FirmwareVersion),
		Status:          strings.TrimSpace(input.Status),
		Metadata:        input.Metadata,
	}

	if device.Name == "" {
		return nil, errors.New("name is required")
	}
	if device.Type == "" {
		return nil, errors.New("type is required")
	}
	if device.Status == "" {
		device.Status = "offline"
	}
	if len(device.Metadata) == 0 {
		device.Metadata = json.RawMessage(`{}`)
	}
	if !json.Valid(device.Metadata) {
		return nil, errors.New("metadata must be valid JSON")
	}

	return s.repo.Create(ctx, device)
}

func (s *Service) List(ctx context.Context) ([]Device, error) {
	return s.repo.List(ctx)
}

func (s *Service) GetByID(ctx context.Context, id string) (*Device, error) {
	return s.repo.GetByID(ctx, strings.TrimSpace(id))
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*Device, error) {
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, errors.New("name cannot be empty")
		}
		input.Name = &name
	}
	if input.Type != nil {
		deviceType := strings.TrimSpace(*input.Type)
		if deviceType == "" {
			return nil, errors.New("type cannot be empty")
		}
		input.Type = &deviceType
	}
	if input.FirmwareVersion != nil {
		firmwareVersion := strings.TrimSpace(*input.FirmwareVersion)
		input.FirmwareVersion = &firmwareVersion
	}
	if input.Status != nil {
		status := strings.TrimSpace(*input.Status)
		if status == "" {
			return nil, errors.New("status cannot be empty")
		}
		input.Status = &status
	}
	if input.Metadata != nil && !json.Valid(*input.Metadata) {
		return nil, errors.New("metadata must be valid JSON")
	}

	return s.repo.Update(ctx, strings.TrimSpace(id), input)
}

func (s *Service) RecordHeartbeat(ctx context.Context, id string, input HeartbeatInput) (*Device, error) {
	deviceID := strings.TrimSpace(id)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "online"
	}

	entity, err := s.repo.UpdateHeartbeat(ctx, deviceID, status)
	if err != nil {
		return nil, err
	}
	if s.publisher != nil {
		s.publisher.PublishDeviceUpdate(ctx, *entity)
	}

	return entity, nil
}

func (s *Service) MarkStaleOffline(ctx context.Context, timeout time.Duration) ([]Device, error) {
	if timeout <= 0 {
		return nil, errors.New("heartbeat timeout must be positive")
	}

	devices, err := s.repo.MarkStaleOffline(ctx, timeout)
	if err != nil {
		return nil, err
	}
	if s.publisher != nil {
		for _, entity := range devices {
			s.publisher.PublishDeviceUpdate(ctx, entity)
		}
	}

	return devices, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, strings.TrimSpace(id))
}

func newDeviceID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate device id: %w", err)
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	encoded := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}
