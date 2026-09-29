package session_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/vishalss1/argus/core/internal/domain/session"
	"github.com/vishalss1/argus/core/internal/domain/workspace"
	"github.com/vishalss1/argus/core/internal/infrastructure/redis"
)

type mockSessionRepo struct {
	mu       sync.Mutex
	sessions map[string]*session.Session
	pgFail   bool
}

func (m *mockSessionRepo) Create(ctx context.Context, s session.Session) (*session.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = &s
	return &s, nil
}

func (m *mockSessionRepo) Get(ctx context.Context, id string) (*session.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, nil
	}
	// return copy
	sc := *s
	return &sc, nil
}

func (m *mockSessionRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]session.Session, error) { return nil, nil }

func (m *mockSessionRepo) ListAllRunning(ctx context.Context) ([]session.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []session.Session
	for _, s := range m.sessions {
		if s.Status == session.StatusRunning {
			res = append(res, *s)
		}
	}
	return res, nil
}

func (m *mockSessionRepo) UpdateStatus(ctx context.Context, id string, status session.Status, startedAt *time.Time, endedAt *time.Time) (*session.Session, error) {
	return nil, nil
}

func (m *mockSessionRepo) TransitionStatus(ctx context.Context, id string, fromStatus session.Status, toStatus session.Status, startedAt *time.Time, endedAt *time.Time) (*session.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pgFail {
		return nil, errors.New("simulated pg failure")
	}

	s, ok := m.sessions[id]
	if !ok {
		return nil, errors.New("not found")
	}

	if s.Status != fromStatus {
		return nil, session.ErrInvalidTransition
	}

	s.Status = toStatus
	if startedAt != nil {
		s.StartedAt = startedAt
	}
	if endedAt != nil {
		s.EndedAt = endedAt
	}
	sc := *s
	return &sc, nil
}

func (m *mockSessionRepo) CloseStaleSessions(ctx context.Context, timeout time.Duration) (int64, error) { return 0, nil }
func (m *mockSessionRepo) Delete(ctx context.Context, id string) error { return nil }

func (m *mockSessionRepo) UpsertStatistics(ctx context.Context, s session.Statistics) error { return nil }
func (m *mockSessionRepo) GetStatistics(ctx context.Context, sessionID string) (*session.Statistics, error) { return nil, nil }
func (m *mockSessionRepo) ListEventsBySession(ctx context.Context, sessionID string) ([]session.Event, error) { return nil, nil }
func (m *mockSessionRepo) ListAlertsBySession(ctx context.Context, sessionID string) ([]session.Alert, error) { return nil, nil }
func (m *mockSessionRepo) ListCommandsBySession(ctx context.Context, sessionID string) ([]session.Command, error) { return nil, nil }
func (m *mockSessionRepo) CreateArtifact(ctx context.Context, a session.Artifact) (*session.Artifact, error) { return &a, nil }
func (m *mockSessionRepo) GetArtifactBySession(ctx context.Context, sessionID string) (*session.Artifact, error) { return nil, nil }
func (m *mockSessionRepo) UpdateArtifact(ctx context.Context, a session.Artifact) error { return nil }
func (m *mockSessionRepo) ListTerminalBefore(ctx context.Context, cutoff time.Time) ([]session.Session, error) { return nil, nil }

type mockWorkspaceRepo struct{}
func (m *mockWorkspaceRepo) Create(ctx context.Context, w workspace.Workspace) (*workspace.Workspace, error) { return nil, nil }
func (m *mockWorkspaceRepo) Get(ctx context.Context, id string) (*workspace.Workspace, error) { return nil, nil }
func (m *mockWorkspaceRepo) List(ctx context.Context) ([]workspace.Workspace, error) { return nil, nil }
func (m *mockWorkspaceRepo) Update(ctx context.Context, id string, name string, description string) (*workspace.Workspace, error) { return nil, nil }
func (m *mockWorkspaceRepo) Delete(ctx context.Context, id string) error { return nil }
func (m *mockWorkspaceRepo) AssignDevice(ctx context.Context, workspaceID string, deviceID string) error { return nil }
func (m *mockWorkspaceRepo) UnassignDevice(ctx context.Context, workspaceID string, deviceID string) error { return nil }
func (m *mockWorkspaceRepo) ListDevices(ctx context.Context, workspaceID string) ([]workspace.DeviceSummary, error) { return nil, nil }


func setupTestEnv(t *testing.T) (*session.Manager, *mockSessionRepo, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	redisClient, err := redis.New(context.Background(), redis.Config{
		Addr: mr.Addr(),
	})
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}

	mockRepo := &mockSessionRepo{sessions: make(map[string]*session.Session)}
	sessSvc := session.NewService(mockRepo)

	wRepo := &mockWorkspaceRepo{}

	manager := session.NewManager(sessSvc, redisClient, wRepo, nil, nil)
	return manager, mockRepo, mr
}

func TestValidTransitions(t *testing.T) {
	manager, repo, mr := setupTestEnv(t)
	defer mr.Close()

	// CREATE
	repo.sessions["s1"] = &session.Session{ID: "s1", WorkspaceID: "w1", Status: session.StatusCreated}

	// START
	started, err := manager.StartSession(context.Background(), "s1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if started.Status != session.StatusRunning {
		t.Fatalf("expected RUNNING, got %v", started.Status)
	}
	if !mr.Exists("session:s1:active") {
		t.Fatal("expected Redis active key to be set")
	}

	// STOP
	stopped, err := manager.StopSession(context.Background(), "s1", true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stopped.Status != session.StatusCompleted {
		t.Fatalf("expected COMPLETED, got %v", stopped.Status)
	}
	if mr.Exists("session:s1:active") {
		t.Fatal("expected Redis active key to be removed")
	}
}

func TestInvalidTransitions(t *testing.T) {
	manager, repo, mr := setupTestEnv(t)
	defer mr.Close()

	// Try stopping a CREATED session
	repo.sessions["s2"] = &session.Session{ID: "s2", WorkspaceID: "w1", Status: session.StatusCreated}
	_, err := manager.StopSession(context.Background(), "s2", true)
	if err == nil {
		t.Fatal("expected error stopping CREATED session")
	}

	// Try starting a COMPLETED session
	repo.sessions["s3"] = &session.Session{ID: "s3", WorkspaceID: "w1", Status: session.StatusCompleted}
	_, err = manager.StartSession(context.Background(), "s3")
	if err == nil {
		t.Fatal("expected error starting COMPLETED session")
	}
}

func TestIdempotency(t *testing.T) {
	manager, repo, mr := setupTestEnv(t)
	defer mr.Close()

	repo.sessions["s4"] = &session.Session{ID: "s4", WorkspaceID: "w1", Status: session.StatusCreated}

	// Duplicate Start
	manager.StartSession(context.Background(), "s4")
	started2, err := manager.StartSession(context.Background(), "s4") // second time
	if err != nil {
		t.Fatalf("expected duplicate start to be idempotent, got err: %v", err)
	}
	if started2.Status != session.StatusRunning {
		t.Fatal("expected returned session to be RUNNING")
	}

	// Duplicate Stop
	manager.StopSession(context.Background(), "s4", true)
	stopped2, err := manager.StopSession(context.Background(), "s4", true) // second time
	if err != nil {
		t.Fatalf("expected duplicate stop to be idempotent, got err: %v", err)
	}
	if stopped2.Status != session.StatusCompleted {
		t.Fatal("expected returned session to be COMPLETED")
	}
}

func TestConcurrentStarts(t *testing.T) {
	manager, repo, mr := setupTestEnv(t)
	defer mr.Close()

	repo.sessions["s5"] = &session.Session{ID: "s5", WorkspaceID: "w1", Status: session.StatusCreated}

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	// Simulate 10 simultaneous start requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := manager.StartSession(context.Background(), "s5")
			if err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent start failed idempotency, got error: %v", err)
		}
	}
}

func TestConcurrentStops(t *testing.T) {
	manager, repo, mr := setupTestEnv(t)
	defer mr.Close()

	now := time.Now().UTC()
	repo.sessions["s6"] = &session.Session{ID: "s6", WorkspaceID: "w1", Status: session.StatusRunning, StartedAt: &now}

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	// Simulate 10 simultaneous stop requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := manager.StopSession(context.Background(), "s6", true)
			if err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent stop failed idempotency, got error: %v", err)
		}
	}
}

func TestRedisFailure(t *testing.T) {
	manager, repo, mr := setupTestEnv(t)
	
	repo.sessions["s7"] = &session.Session{ID: "s7", WorkspaceID: "w1", Status: session.StatusCreated}
	
	// Kill redis to simulate failure
	mr.Close()

	// Should not fail the transition, just log warning
	started, err := manager.StartSession(context.Background(), "s7")
	if err != nil {
		t.Fatalf("expected DB transition to succeed even if redis is down, got: %v", err)
	}
	if started.Status != session.StatusRunning {
		t.Fatalf("expected RUNNING status")
	}
}

func TestPostgresFailure(t *testing.T) {
	manager, repo, mr := setupTestEnv(t)
	defer mr.Close()

	repo.sessions["s8"] = &session.Session{ID: "s8", WorkspaceID: "w1", Status: session.StatusCreated}
	repo.pgFail = true

	_, err := manager.StartSession(context.Background(), "s8")
	if err == nil {
		t.Fatal("expected error due to postgres failure")
	}

	// Verify Redis was not touched
	if mr.Exists("session:s8:active") {
		t.Fatal("expected Redis to not be updated on PG failure")
	}
}

func TestRedisRecovery(t *testing.T) {
	manager, repo, mr := setupTestEnv(t)
	defer mr.Close()

	repo.sessions["s9"] = &session.Session{ID: "s9", WorkspaceID: "w1", Status: session.StatusRunning}

	// Run recovery
	err := manager.RecoverActiveSessions(context.Background())
	if err != nil {
		t.Fatalf("expected successful recovery, got: %v", err)
	}

	if !mr.Exists("session:s9:active") {
		t.Fatal("expected session s9 to be recovered to Redis")
	}
}

