package event

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, event Event) (*Event, error)
	List(ctx context.Context) ([]Event, error)
	ListByDevice(ctx context.Context, deviceID string) ([]Event, error)
	ListByType(ctx context.Context, eventType string) ([]Event, error)
}
