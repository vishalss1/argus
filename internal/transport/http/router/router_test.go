package router

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/vishalss1/argus/internal/domain/auth"
	transportws "github.com/vishalss1/argus/internal/transport/websocket"
)

// In-memory mock repositories for router testing
type mockUserRepository struct {
	users      map[string]*auth.User
	memberships map[string][]string // userID -> []workspaceID
	workspaces  map[string]string   // workspaceID -> name
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
	return nil
}

func (m *mockUserRepository) ListWorkspacesForUser(ctx context.Context, userID string) ([]auth.WorkspaceInfo, error) {
	var list []auth.WorkspaceInfo
	for _, id := range m.memberships[userID] {
		list = append(list, auth.WorkspaceInfo{ID: id, Name: "Workspace " + id})
	}
	return list, nil
}

func (m *mockUserRepository) CheckWorkspaceMembership(ctx context.Context, userID, workspaceID string) (bool, error) {
	for _, id := range m.memberships[userID] {
		if id == workspaceID {
			return true, nil
		}
	}
	return false, nil
}

type mockRefreshTokenRepository struct {
	tokens map[string]*auth.RefreshToken
}

func (m *mockRefreshTokenRepository) Create(ctx context.Context, t *auth.RefreshToken) error {
	m.tokens[t.ID] = t
	return nil
}

func (m *mockRefreshTokenRepository) GetByHash(ctx context.Context, hash string) (*auth.RefreshToken, error) {
	return nil, nil
}

func (m *mockRefreshTokenRepository) Revoke(ctx context.Context, id string) error {
	return nil
}

func (m *mockRefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	return nil
}

func (m *mockRefreshTokenRepository) DeleteExpiredOrRevoked(ctx context.Context) (int64, error) {
	return 0, nil
}

type mockAuditLogRepository struct{}

func (m *mockAuditLogRepository) Create(ctx context.Context, l *auth.AuthAuditLog) error {
	return nil
}

func (m *mockAuditLogRepository) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

func TestWebSocketRouteUpgradesThroughMiddleware(t *testing.T) {
	// Initialize Mock Repositories
	userRepo := &mockUserRepository{
		users:       make(map[string]*auth.User),
		memberships: make(map[string][]string),
		workspaces:  make(map[string]string),
	}
	tokenRepo := &mockRefreshTokenRepository{
		tokens: make(map[string]*auth.RefreshToken),
	}
	auditRepo := &mockAuditLogRepository{}
	
	authService := auth.NewService(userRepo, tokenRepo, auditRepo, "testsecret-testsecret-testsecret", 0, 0)

	// Register test user and add workspace membership
	user, err := authService.Register(context.Background(), "test@example.com", "password123", "Jane Doe", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}
	_ = userRepo.AddWorkspaceMember(context.Background(), "ws-12345", user.ID)

	// Login to get valid JWT token
	_, accessT, _, err := authService.Login(context.Background(), "test@example.com", "password123", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	hub := transportws.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Instantiate Router with real authService
	server := httptest.NewServer(New(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, authService, transportws.NewHandler(hub, authService)))
	defer server.Close()

	url := "ws" + server.URL[len("http"):] + "/ws"

	// WebSocket now uses post-handshake auth, so no headers needed
	conn, _, err := gorilla.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	// Send auth message
	authMsg := fmt.Sprintf(`{"type":"auth","payload":{"token":"%s","workspace_id":"ws-12345"}}`, accessT)
	if err := conn.WriteMessage(gorilla.TextMessage, []byte(authMsg)); err != nil {
		t.Fatalf("send auth message: %v", err)
	}

	// Read authenticated response
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read auth response: %v", err)
	}
	if !strings.Contains(string(msg), `"type":"authenticated"`) {
		t.Fatalf("expected authenticated response, got: %s", msg)
	}
}
