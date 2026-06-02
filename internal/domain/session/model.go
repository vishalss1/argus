package session

import (
	"context"
	"time"
)

type Status string

const (
	StatusCreated   Status = "CREATED"
	StatusRunning   Status = "RUNNING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
	StatusCancelled Status = "CANCELLED"
)

type Session struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Status      Status     `json:"status"`
	StartedAt   *time.Time `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at"`
	CreatedBy   *string    `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Repository interface {
	Create(ctx context.Context, s Session) (*Session, error)
	Get(ctx context.Context, id string) (*Session, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]Session, error)
	UpdateStatus(ctx context.Context, id string, status Status, startedAt *time.Time, endedAt *time.Time) (*Session, error)
	CloseStaleSessions(ctx context.Context, timeout time.Duration) (int64, error)
	Delete(ctx context.Context, id string) error
}
