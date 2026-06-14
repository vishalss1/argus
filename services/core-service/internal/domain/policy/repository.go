package policy

import (
	"context"
)

type Repository interface {
	CreatePolicy(ctx context.Context, p Policy) (*Policy, error)
	GetPolicyByAction(ctx context.Context, action ActionType) (*Policy, error)
	ListPolicies(ctx context.Context) ([]Policy, error)
	UpdatePolicy(ctx context.Context, p Policy) (*Policy, error)
	DeletePolicy(ctx context.Context, id string) error

	CreateExecutionRecord(ctx context.Context, r ExecutionRecord) (*ExecutionRecord, error)
	GetExecutionRecord(ctx context.Context, id string) (*ExecutionRecord, error)
	ListExecutionRecords(ctx context.Context) ([]ExecutionRecord, error)
	UpdateExecutionStatus(ctx context.Context, id string, status string, approvedBy *string) error
}

