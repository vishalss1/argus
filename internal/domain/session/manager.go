package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vishalss1/argus/internal/domain/usage"
	"github.com/vishalss1/argus/internal/infrastructure/redis"
)

type Manager struct {
	sessionService *Service
	usageService   *usage.Service
	redisClient    *redis.Client
}

func NewManager(sessionService *Service, usageService *usage.Service, redisClient *redis.Client) *Manager {
	return &Manager{
		sessionService: sessionService,
		usageService:   usageService,
		redisClient:    redisClient,
	}
}

func (m *Manager) StartSession(ctx context.Context, id string) (*Session, error) {
	sess, err := m.sessionService.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, errors.New("session not found")
	}

	if sess.Status != StatusCreated {
		return nil, fmt.Errorf("session cannot be started from status: %s", sess.Status)
	}

	// 0. Enforce Subscription Limits
	// Using a placeholder tenant ID for now until full auth is implemented
	tenantID := "default-tenant"
	currentPlan := usage.PlanFree // Hardcoded for prototype

	if err := m.usageService.CheckSessionLimit(ctx, tenantID, currentPlan); err != nil {
		return nil, fmt.Errorf("subscription limit reached: %w", err)
	}

	// 1. Update DB status
	started, err := m.sessionService.Start(ctx, id)
	if err != nil {
		return nil, err
	}

	// 2. Initialize Redis state
	// session:{sessionId}:active = 1
	activeKey := fmt.Sprintf("session:%s:active", id)
	if err := m.redisClient.Client().Set(ctx, activeKey, "1", 0).Err(); err != nil {
		return nil, fmt.Errorf("redis set active: %w", err)
	}

	// Register in global active sessions list
	if err := m.redisClient.Client().SAdd(ctx, "sessions:active", id).Err(); err != nil {
		return nil, fmt.Errorf("redis sadd active: %w", err)
	}

	// 3. Record Usage
	_ = m.usageService.RecordSessionStarted(ctx, tenantID)

	return started, nil
}

func (m *Manager) StopSession(ctx context.Context, id string, success bool) (*Session, error) {
	// 1. Update DB status
	stopped, err := m.sessionService.Stop(ctx, id, success)
	if err != nil {
		return nil, err
	}

	// 2. Cleanup Redis (as per Phase 2 Stop Session actions)
	activeKey := fmt.Sprintf("session:%s:active", id)
	_ = m.redisClient.Client().Del(ctx, activeKey).Err()
	_ = m.redisClient.Client().SRem(ctx, "sessions:active", id).Err()

	// 3. TODO: Trigger report/AI analysis (Phase 10 & 11)

	return stopped, nil
}

func (m *Manager) CleanupStaleSessions(ctx context.Context, timeout time.Duration) (int64, error) {
	// Not ideal coupling, but good enough for this prototype.
	// Ideally the repository provides this directly via the service.
	// Since the requirement says "add session cleanup", we'll just expose it via repo and call it.
	// We'll define a quick method on service to do this.
	return m.sessionService.CleanupStale(ctx, timeout)
}
