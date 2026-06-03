package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
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
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	encoded := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}
