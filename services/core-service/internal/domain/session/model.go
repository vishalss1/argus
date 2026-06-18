package session

import (
	"context"
	"errors"
	"time"
)

var ErrInvalidTransition = errors.New("invalid session state transition")

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
	ListAllRunning(ctx context.Context) ([]Session, error)
	UpdateStatus(ctx context.Context, id string, status Status, startedAt *time.Time, endedAt *time.Time) (*Session, error)
	TransitionStatus(ctx context.Context, id string, fromStatus Status, toStatus Status, startedAt *time.Time, endedAt *time.Time) (*Session, error)
	CloseStaleSessions(ctx context.Context, timeout time.Duration) (int64, error)
	ListTerminalBefore(ctx context.Context, cutoff time.Time) ([]Session, error)
	Delete(ctx context.Context, id string) error

	UpsertStatistics(ctx context.Context, s Statistics) error
	GetStatistics(ctx context.Context, sessionID string) (*Statistics, error)
	ListEventsBySession(ctx context.Context, sessionID string) ([]Event, error)
	ListAlertsBySession(ctx context.Context, sessionID string) ([]Alert, error)
	ListCommandsBySession(ctx context.Context, sessionID string) ([]Command, error)
	CreateArtifact(ctx context.Context, a Artifact) (*Artifact, error)
	GetArtifactBySession(ctx context.Context, sessionID string) (*Artifact, error)
	UpdateArtifact(ctx context.Context, a Artifact) error
}

