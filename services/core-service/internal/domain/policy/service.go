package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ValidateAction(ctx context.Context, action ActionType, deviceID string) (bool, bool, error) {
	p, err := s.repo.GetPolicyByAction(ctx, action)
	if err != nil {
		return false, true, fmt.Errorf("failed to get policy: %w", err)
	}

	// 1. Check if device is allowed
	if len(p.AllowedDevices) > 0 {
		allowed := false
		for _, id := range p.AllowedDevices {
			if id == deviceID {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, true, nil
		}
	}

	// 2. Return if approval is required
	return true, p.RequiresApproval, nil
}

func (s *Service) GetPolicyByAction(ctx context.Context, action ActionType) (*Policy, error) {
	return s.repo.GetPolicyByAction(ctx, action)
}

func (s *Service) CreateRecord(ctx context.Context, record ExecutionRecord) (*ExecutionRecord, error) {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	record.CreatedAt = time.Now()
	return s.repo.CreateExecutionRecord(ctx, record)
}

func (s *Service) ApproveAction(ctx context.Context, id string, approvedBy string) error {
	return s.repo.UpdateExecutionStatus(ctx, id, "approved", &approvedBy)
}

func (s *Service) MarkExecuted(ctx context.Context, id string) error {
	return s.repo.UpdateExecutionStatus(ctx, id, "executed", nil)
}

func (s *Service) MarkFailed(ctx context.Context, id string) error {
	return s.repo.UpdateExecutionStatus(ctx, id, "failed", nil)
}

func (s *Service) GetRecord(ctx context.Context, id string) (*ExecutionRecord, error) {
	return s.repo.GetExecutionRecord(ctx, id)
}

func (s *Service) ListRecords(ctx context.Context) ([]ExecutionRecord, error) {
	return s.repo.ListExecutionRecords(ctx)
}

