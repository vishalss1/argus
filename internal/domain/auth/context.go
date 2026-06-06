package auth

import "context"

type contextKey string

const (
	WorkspaceIDKey contextKey = "workspace_id"
	UserIDKey      contextKey = "user_id"
)

// WithWorkspaceID injects the workspace ID into context
func WithWorkspaceID(ctx context.Context, workspaceID string) context.Context {
	return context.WithValue(ctx, WorkspaceIDKey, workspaceID)
}

// GetWorkspaceID retrieves the workspace ID from context
func GetWorkspaceID(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(WorkspaceIDKey).(string)
	return val, ok
}

// WithUserID injects the user ID into context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// GetUserID retrieves the user ID from context
func GetUserID(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(UserIDKey).(string)
	return val, ok
}
