package rule

import "context"

type Repository interface {
	CreateRule(ctx context.Context, entity Rule) (*Rule, error)
	ListRules(ctx context.Context) ([]Rule, error)
	ListEnabledRulesByWorkspace(ctx context.Context, workspaceID string) ([]Rule, error)
	GetRule(ctx context.Context, id string) (*Rule, error)
	UpdateRule(ctx context.Context, id string, input UpdateInput) (*Rule, error)
	DeleteRule(ctx context.Context, id string) error
	ListAlerts(ctx context.Context) ([]Alert, error)
}
