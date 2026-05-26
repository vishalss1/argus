package incident

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, incident Incident) (*Incident, error)
	GetByID(ctx context.Context, id string) (*Incident, error)
	List(ctx context.Context) ([]Incident, error)
	Update(ctx context.Context, incident Incident) (*Incident, error)
	Resolve(ctx context.Context, id string) error
}
