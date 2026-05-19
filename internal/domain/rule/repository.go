package rule

import "context"

type Repository interface {
	CreateRule(ctx context.Context, entity Rule) (*Rule, error)
	ListRules(ctx context.Context) ([]Rule, error)
	ListEnabledRules(ctx context.Context) ([]Rule, error)
	GetRule(ctx context.Context, id string) (*Rule, error)
	UpdateRule(ctx context.Context, id string, input UpdateInput) (*Rule, error)
	DeleteRule(ctx context.Context, id string) error
	CreateAlert(ctx context.Context, entity Alert) (*Alert, error)
	ListAlerts(ctx context.Context) ([]Alert, error)
}
