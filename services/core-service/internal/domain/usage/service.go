package usage

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Plan string

const (
	PlanFree     Plan = "free"
	PlanStarter  Plan = "starter"
	PlanPro      Plan = "pro"
	PlanBusiness Plan = "business"
)

type Limits struct {
	MaxWorkspaces int
	MaxDevices    int
	MaxSessions   int // per month, -1 for unlimited
}

var PlanLimits = map[Plan]Limits{
	PlanFree:     {MaxWorkspaces: 1, MaxDevices: 5, MaxSessions: 10},
	PlanStarter:  {MaxWorkspaces: 5, MaxDevices: 50, MaxSessions: 100},
	PlanPro:      {MaxWorkspaces: 20, MaxDevices: 100, MaxSessions: 500},
	PlanBusiness: {MaxWorkspaces: 100, MaxDevices: 1000, MaxSessions: -1},
}

type Usage struct {
	TenantID          string `json:"tenant_id"`
	BillingMonth      string `json:"billing_month"`
	DevicesUsed       int    `json:"devices_used"`
	WorkspacesUsed    int    `json:"workspaces_used"`
	SessionsRun       int    `json:"sessions_run"`
	MessagesProcessed int64  `json:"messages_processed"`
}

type Repository interface {
	GetUsage(ctx context.Context, tenantID string, month string) (*Usage, error)
	IncrementSessions(ctx context.Context, tenantID string, month string) error
	UpdateUsage(ctx context.Context, tenantID string, month string, devices int, workspaces int) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CheckSessionLimit(ctx context.Context, tenantID string, plan Plan) error {
	limits, ok := PlanLimits[plan]
	if !ok {
		return fmt.Errorf("unknown plan: %s", plan)
	}

	if limits.MaxSessions == -1 {
		return nil // Unlimited
	}

	month := time.Now().UTC().Format("2006-01")
	u, err := s.repo.GetUsage(ctx, tenantID, month)
	if err != nil {
		return err
	}

	currentSessions := 0
	if u != nil {
		currentSessions = u.SessionsRun
	}

	if currentSessions >= limits.MaxSessions {
		return errors.New("monthly session limit reached for current plan")
	}

	return nil
}

func (s *Service) RecordSessionStarted(ctx context.Context, tenantID string) error {
	month := time.Now().UTC().Format("2006-01")
	return s.repo.IncrementSessions(ctx, tenantID, month)
}

