package context

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
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

	return s.repo.Create(ctx, mem)
}

func (s *Service) GetDeviceHistory(ctx context.Context, deviceID string) ([]OperationalMemory, error) {
	return s.repo.ListByDevice(ctx, deviceID)
}

func (s *Service) GetLatestMemory(ctx context.Context, deviceID string, memoryType MemoryType) (*OperationalMemory, error) {
	return s.repo.GetLatestByDevice(ctx, deviceID, memoryType)
}
