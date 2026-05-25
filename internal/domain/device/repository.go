package device

import (
	"context"
	"time"
)

type Repository interface {
	Create(ctx context.Context, device Device) (*Device, error)
	List(ctx context.Context) ([]Device, error)
	GetByID(ctx context.Context, id string) (*Device, error)
	GetByHardwareID(ctx context.Context, hardwareID string) (*Device, error)
	Update(ctx context.Context, id string, input UpdateInput) (*Device, error)
	UpdateHeartbeat(ctx context.Context, id string, status string) (*Device, error)
	MarkStaleOffline(ctx context.Context, timeout time.Duration) ([]Device, error)
	Delete(ctx context.Context, id string) error
}
