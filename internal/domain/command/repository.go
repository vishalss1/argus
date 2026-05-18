package command

import "context"

type Repository interface {
	Create(ctx context.Context, entity Command) (*Command, error)
	ListByDevice(ctx context.Context, deviceID string) ([]Command, error)
	Get(ctx context.Context, deviceID string, id string) (*Command, error)
	Ack(ctx context.Context, deviceID string, id string, message string) (*Command, error)
	Nack(ctx context.Context, deviceID string, id string, reason string) (*Command, error)
}
