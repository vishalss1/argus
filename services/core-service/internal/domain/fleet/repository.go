package fleet

import "context"

type Repository interface {
	Create(ctx context.Context, fleet Fleet) (*Fleet, error)
	List(ctx context.Context) ([]FleetWithStats, error)
	GetByID(ctx context.Context, id string) (*Fleet, error)
	GetWithDevices(ctx context.Context, id string) (*FleetWithStats, error)
	Delete(ctx context.Context, id string) error
}
