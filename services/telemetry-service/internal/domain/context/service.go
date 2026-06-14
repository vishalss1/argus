package context

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vishalss1/argus/telemetry/internal/domain/device"
)

type Service struct {
	repo       Repository
	deviceRepo device.Repository
	OnRecord   func(ctx context.Context, mem OperationalMemory)
}

func NewService(repo Repository, deviceRepo device.Repository) *Service {
	return &Service{
		repo:       repo,
		deviceRepo: deviceRepo,
	}
}

func (s *Service) RecordMemory(ctx context.Context, mem OperationalMemory) (*OperationalMemory, error) {
	if mem.ID == "" {
		mem.ID = uuid.New().String()
	}
	if mem.CreatedAt.IsZero() {
		mem.CreatedAt = time.Now()
	}
	if mem.Timestamp.IsZero() {
		mem.Timestamp = mem.CreatedAt
	}
	if mem.Data == nil {
		mem.Data = json.RawMessage("{}")
	}

	if mem.WorkspaceID == "" && mem.DeviceID != nil && *mem.DeviceID != "" {
		if s.deviceRepo != nil {
			dev, err := s.deviceRepo.GetByID(ctx, *mem.DeviceID)
			if err == nil && dev != nil && dev.WorkspaceID != nil {
				mem.WorkspaceID = *dev.WorkspaceID
			}
		}
	}

	created, err := s.repo.Create(ctx, mem)
	if err == nil && s.OnRecord != nil {
		s.OnRecord(ctx, *created)
	}

	return created, err
}

func (s *Service) GetDeviceHistory(ctx context.Context, deviceID string, limit, offset int) ([]OperationalMemory, error) {
	return s.repo.ListByDevice(ctx, deviceID, limit, offset)
}

func (s *Service) GetLatestMemory(ctx context.Context, deviceID string, memoryType MemoryType) (*OperationalMemory, error) {
	return s.repo.GetLatestByDevice(ctx, deviceID, memoryType)
}
