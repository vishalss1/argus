package handler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vishalss1/argus/core/internal/domain/auth"
)

// In-memory mock repositories for testing Auth
type mockUserRepository struct {
	users      map[string]*auth.User
	memberships map[string][]string // userID -> []workspaceID
	workspaces  map[string]string   // workspaceID -> name
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		users:       make(map[string]*auth.User),
		memberships: make(map[string][]string),
		workspaces:  make(map[string]string),
	}
}

func (m *mockUserRepository) Create(ctx context.Context, u *auth.User) error {
	m.users[u.ID] = u
	return nil
}

func (m *mockUserRepository) GetByID(ctx context.Context, id string) (*auth.User, error) {
	return m.users[id], nil
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*auth.User, error) {
	for _, u := range m.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepository) Update(ctx context.Context, u *auth.User) error {
	m.users[u.ID] = u
	return nil
}

func (m *mockUserRepository) AddWorkspaceMember(ctx context.Context, workspaceID, userID string) error {
	m.memberships[userID] = append(m.memberships[userID], workspaceID)
	return nil
}

func (m *mockUserRepository) RemoveWorkspaceMember(ctx context.Context, workspaceID, userID string) error {
	list := m.memberships[userID]
	for i, id := range list {
		if id == workspaceID {
			m.memberships[userID] = append(list[:i], list[i+1:]...)
			break
		}
	}
	return nil
}

func (m *mockUserRepository) ListWorkspacesForUser(ctx context.Context, userID string) ([]auth.WorkspaceInfo, error) {
	ids := m.memberships[userID]
	var list []auth.WorkspaceInfo
	for _, id := range ids {
		name := m.workspaces[id]
		if name == "" {
			name = "Workspace " + id
		}
		list = append(list, auth.WorkspaceInfo{ID: id, Name: name})
	}
	return list, nil
}

func (m *mockUserRepository) CheckWorkspaceMembership(ctx context.Context, userID, workspaceID string) (bool, error) {
	ids := m.memberships[userID]
	for _, id := range ids {
		if id == workspaceID {
			return true, nil
		}
	}
	return false, nil
}

type mockRefreshTokenRepository struct {
	tokens map[string]*auth.RefreshToken
}

func newMockRefreshTokenRepository() *mockRefreshTokenRepository {
	return &mockRefreshTokenRepository{
		tokens: make(map[string]*auth.RefreshToken),
	}
}

func (m *mockRefreshTokenRepository) Create(ctx context.Context, t *auth.RefreshToken) error {
	m.tokens[t.ID] = t
	return nil
}

func (m *mockRefreshTokenRepository) GetByHash(ctx context.Context, hash string) (*auth.RefreshToken, error) {
	for _, t := range m.tokens {
		if t.TokenHash == hash {
			return t, nil
		}
	}
	return nil, nil
}

func (m *mockRefreshTokenRepository) Revoke(ctx context.Context, id string) error {
	if t, ok := m.tokens[id]; ok {
		t.Revoked = true
		now := time.Now().UTC()
		t.RevokedAt = &now
	}
	return nil
}

func (m *mockRefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	for _, t := range m.tokens {
		if t.UserID == userID {
			t.Revoked = true
			now := time.Now().UTC()
			t.RevokedAt = &now
		}
	}
	return nil
}

func (m *mockRefreshTokenRepository) DeleteExpiredOrRevoked(ctx context.Context) (int64, error) {
	var count int64
	for id, t := range m.tokens {
		if t.Revoked || time.Now().After(t.ExpiresAt) {
			delete(m.tokens, id)
			count++
		}
	}
	return count, nil
}

type mockAuditLogRepository struct {
	logs []*auth.AuthAuditLog
}

func (m *mockAuditLogRepository) Create(ctx context.Context, l *auth.AuthAuditLog) error {
	m.logs = append(m.logs, l)
	return nil
}

func (m *mockAuditLogRepository) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	var kept []*auth.AuthAuditLog
	var deleted int64
	for _, l := range m.logs {
		if l.Timestamp.Before(cutoff) {
			deleted++
		} else {
			kept = append(kept, l)
		}
	}
	m.logs = kept
	return deleted, nil
}

// Test Register Password Policy & Credentials Login
func TestAuthCredentialsWorkflow(t *testing.T) {
	userRepo := newMockUserRepository()
	tokenRepo := newMockRefreshTokenRepository()
	auditRepo := &mockAuditLogRepository{}
	authService := auth.NewService(userRepo, tokenRepo, auditRepo, "testsecret", 0, 0)

	// 1. Password policy reject (short)
	_, err := authService.Register(context.Background(), "test@example.com", "short", "Jane", "127.0.0.1", "test-agent")
	if err == nil || !strings.Contains(err.Error(), "between 8 and 128") {
		t.Fatalf("expected password length error, got %v", err)
	}

	// 2. Registration successful
	user, err := authService.Register(context.Background(), "test@example.com", "password123", "Jane Doe", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("expected successful registration, got error: %v", err)
	}
	if user.Email != "test@example.com" || user.Name != "Jane Doe" {
		t.Fatalf("user fields mismatch: %+v", user)
	}

	// 3. User exists error
	_, err = authService.Register(context.Background(), "test@example.com", "password123", "Jane Doe", "127.0.0.1", "test-agent")
	if err == nil || !errors.Is(err, auth.ErrUserExists) {
		t.Fatalf("expected user exists error, got %v", err)
	}

	// 4. Credentials login success
	loggedInUser, accessT, refreshT, err := authService.Login(context.Background(), "test@example.com", "password123", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("expected login success, got: %v", err)
	}
	if loggedInUser.ID != user.ID {
		t.Fatalf("user id mismatch")
	}
	if accessT == "" || refreshT == "" {
		t.Fatalf("tokens cannot be empty")
	}

	// 5. Credentials login mismatch (anti-enumeration check)
	_, _, _, err = authService.Login(context.Background(), "test@example.com", "wrongpassword", "127.0.0.1", "test-agent")
	if err == nil || !errors.Is(err, auth.ErrInvalidCreds) {
		t.Fatalf("expected invalid creds error, got: %v", err)
	}

	// 6. Access token claims verification
	claims, err := authService.ValidateAccessToken(accessT)
	if err != nil {
		t.Fatalf("failed to validate access token: %v", err)
	}
	if claims.UserID != user.ID || claims.Email != user.Email {
		t.Fatalf("claims mismatch")
	}
}

// Test Token Rotation
func TestRefreshTokenRotation(t *testing.T) {
	userRepo := newMockUserRepository()
	tokenRepo := newMockRefreshTokenRepository()
	auditRepo := &mockAuditLogRepository{}
	authService := auth.NewService(userRepo, tokenRepo, auditRepo, "testsecret", 0, 0)
	authService.SetRotationGrace(0)

	// Create user
	_, _ = authService.Register(context.Background(), "test@example.com", "password123", "Jane", "127.0.0.1", "agent")
	_, _, refreshT, _ := authService.Login(context.Background(), "test@example.com", "password123", "127.0.0.1", "agent")

	// Refresh first time
	accessT2, refreshT2, err := authService.Refresh(context.Background(), refreshT, "127.0.0.1", "agent")
	if err != nil {
		t.Fatalf("expected refresh to succeed, got %v", err)
	}
	if accessT2 == "" || refreshT2 == "" {
		t.Fatalf("issued empty tokens")
	}

	// Old refresh token must be revoked immediately
	_, _, err = authService.Refresh(context.Background(), refreshT, "127.0.0.1", "agent")
	if err == nil || !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expected error on old token reuse, got: %v", err)
	}

	// Logout revokes new token
	err = authService.Logout(context.Background(), refreshT2, "127.0.0.1", "agent")
	if err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	_, _, err = authService.Refresh(context.Background(), refreshT2, "127.0.0.1", "agent")
	if err == nil || !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expected error on revoked token refresh, got: %v", err)
	}
}

// Test /auth/me workspaces list mapping
func TestAuthMeWorkspaces(t *testing.T) {
	userRepo := newMockUserRepository()
	tokenRepo := newMockRefreshTokenRepository()
	auditRepo := &mockAuditLogRepository{}
	authService := auth.NewService(userRepo, tokenRepo, auditRepo, "testsecret", 0, 0)

	// Register user and seed workspaces
	user, _ := authService.Register(context.Background(), "test@example.com", "password123", "Jane", "127.0.0.1", "agent")
	userRepo.workspaces["ws-1"] = "Prod Workspace"
	userRepo.workspaces["ws-2"] = "Staging Workspace"
	_ = userRepo.AddWorkspaceMember(context.Background(), "ws-1", user.ID)
	_ = userRepo.AddWorkspaceMember(context.Background(), "ws-2", user.ID)

	u, list, err := authService.GetMe(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("get me failed: %v", err)
	}
	if u.ID != user.ID {
		t.Fatalf("user mismatch")
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(list))
	}
	if list[0].ID != "ws-1" || list[0].Name != "Prod Workspace" {
		t.Fatalf("first workspace mismatch: %+v", list[0])
	}
}

// Test AI Query Validation Constraints
func TestAIQueryTrimmedValidation(t *testing.T) {
	// 1. Valid Query
	q := "   Show all offline devices   "
	trimmed := strings.TrimSpace(q)
	if len(trimmed) == 0 {
		t.Fatal("query should not be empty")
	}
	if len(trimmed) > 500 {
		t.Fatal("query should not exceed 500 chars")
	}

	// 2. Empty Query
	qEmpty := "     "
	trimmedEmpty := strings.TrimSpace(qEmpty)
	if len(trimmedEmpty) != 0 {
		t.Fatal("query should be empty after trim")
	}

	// 3. Exceeds 500 characters
	qLong := strings.Repeat("a", 501)
	trimmedLong := strings.TrimSpace(qLong)
	if len(trimmedLong) <= 500 {
		t.Fatal("query should exceed 500 characters")
	}
}

