package context

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, memory OperationalMemory) (*OperationalMemory, error)
	ListByDevice(ctx context.Context, deviceID string, limit, offset int) ([]OperationalMemory, error)
	ListByType(ctx context.Context, memoryType MemoryType, limit, offset int) ([]OperationalMemory, error)
	GetLatestByDevice(ctx context.Context, deviceID string, memoryType MemoryType) (*OperationalMemory, error)
}
