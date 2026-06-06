package event

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, event Event) (*Event, error)
	List(ctx context.Context, limit, offset int) ([]Event, error)
	ListByDevice(ctx context.Context, deviceID string, limit, offset int) ([]Event, error)
	ListByType(ctx context.Context, eventType string, limit, offset int) ([]Event, error)
}
