package context

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, memory OperationalMemory) (*OperationalMemory, error)
	ListByDevice(ctx context.Context, deviceID string) ([]OperationalMemory, error)
	ListByType(ctx context.Context, memoryType MemoryType) ([]OperationalMemory, error)
	GetLatestByDevice(ctx context.Context, deviceID string, memoryType MemoryType) (*OperationalMemory, error)
}
