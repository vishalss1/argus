package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/vishalss1/argus/src/internal/model"
	"github.com/vishalss1/argus/src/internal/repository"
)

type DeviceService struct {
	repo repository.DeviceRepository
}

func NewDeviceService(repo repository.DeviceRepository) *DeviceService {
	return &DeviceService{repo: repo}
}

func (s *DeviceService) Create(ctx context.Context, req model.CreateDeviceRequest) (*model.Device, error) {
	id, err := newDeviceID()
	if err != nil {
		return nil, err
	}

	device := model.Device{
		ID:              id,
		Name:            strings.TrimSpace(req.Name),
		Type:            strings.TrimSpace(req.Type),
		FirmwareVersion: strings.TrimSpace(req.FirmwareVersion),
		Status:          strings.TrimSpace(req.Status),
		Metadata:        req.Metadata,
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

func (s *DeviceService) List(ctx context.Context) ([]model.Device, error) {
	return s.repo.List(ctx)
}

func (s *DeviceService) GetByID(ctx context.Context, id string) (*model.Device, error) {
	return s.repo.GetByID(ctx, strings.TrimSpace(id))
}

func (s *DeviceService) Update(ctx context.Context, id string, req model.UpdateDeviceRequest) (*model.Device, error) {
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, errors.New("name cannot be empty")
		}
		req.Name = &name
	}
	if req.Type != nil {
		deviceType := strings.TrimSpace(*req.Type)
		if deviceType == "" {
			return nil, errors.New("type cannot be empty")
		}
		req.Type = &deviceType
	}
	if req.FirmwareVersion != nil {
		firmwareVersion := strings.TrimSpace(*req.FirmwareVersion)
		req.FirmwareVersion = &firmwareVersion
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if status == "" {
			return nil, errors.New("status cannot be empty")
		}
		req.Status = &status
	}
	if req.Metadata != nil && !json.Valid(*req.Metadata) {
		return nil, errors.New("metadata must be valid JSON")
	}

	return s.repo.Update(ctx, strings.TrimSpace(id), req)
}

func (s *DeviceService) Delete(ctx context.Context, id string) error {
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
