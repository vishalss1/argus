package kafka

import (
	"context"
	"fmt"

	commanddomain "github.com/vishalss1/argus/internal/domain/command"
)

type CommandRepository struct {
	next     commanddomain.Repository
	producer *Producer
}

func NewCommandRepository(next commanddomain.Repository, producer *Producer) *CommandRepository {
	return &CommandRepository{
		next:     next,
		producer: producer,
	}
}

func (r *CommandRepository) Create(ctx context.Context, entity commanddomain.Command) (*commanddomain.Command, error) {
	created, err := r.next.Create(ctx, entity)
	if err != nil {
		return nil, err
	}

	if err := r.producer.PublishCommand(ctx, *created); err != nil {
		return nil, fmt.Errorf("forward command event: %w", err)
	}

	return created, nil
}

func (r *CommandRepository) ListByDevice(ctx context.Context, deviceID string) ([]commanddomain.Command, error) {
	return r.next.ListByDevice(ctx, deviceID)
}

func (r *CommandRepository) Get(ctx context.Context, deviceID string, id string) (*commanddomain.Command, error) {
	return r.next.Get(ctx, deviceID, id)
}

func (r *CommandRepository) Ack(ctx context.Context, deviceID string, id string, message string) (*commanddomain.Command, error) {
	return r.next.Ack(ctx, deviceID, id, message)
}

func (r *CommandRepository) Nack(ctx context.Context, deviceID string, id string, reason string) (*commanddomain.Command, error) {
	return r.next.Nack(ctx, deviceID, id, reason)
}
