package device

import "context"

type Repository interface {
	GetByID(ctx context.Context, id string) (*Device, error)
	Search(ctx context.Context, terms []string, limit int) ([]Device, error)
}
