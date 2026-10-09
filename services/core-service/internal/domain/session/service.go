package session

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Repo() Repository {
	return s.repo
}

func (s *Service) Create(ctx context.Context, workspaceID string, createdBy *string) (*Session, error) {
	if workspaceID == "" {
		return nil, errors.New("workspace id is required")
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}

	sess := Session{
		ID:          id,
		WorkspaceID: workspaceID,
		Status:      StatusCreated,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now().UTC(),
	}

	created, err := s.repo.Create(ctx, sess)
	if err == nil {
		SessionsCreatedTotal.Inc()
	}
	return created, err
}

func (s *Service) Start(ctx context.Context, id string) (*Session, error) {
	now := time.Now().UTC()
	return s.repo.UpdateStatus(ctx, id, StatusRunning, &now, nil)
}

func (s *Service) Stop(ctx context.Context, id string, success bool) (*Session, error) {
	status := StatusCompleted
	if !success {
		status = StatusFailed
	}
	now := time.Now().UTC()
	return s.repo.UpdateStatus(ctx, id, status, nil, &now)
}

func (s *Service) Get(ctx context.Context, id string) (*Session, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, workspaceID string) ([]Session, error) {
	return s.repo.ListByWorkspace(ctx, workspaceID)
}

func (s *Service) CleanupStale(ctx context.Context, timeout time.Duration) (int64, error) {
	return s.repo.CloseStaleSessions(ctx, timeout)
}

func newID() (string, error) {
	return uuid.New().String(), nil
}
