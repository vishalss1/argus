package auth

import (
	"context"
	"time"
)

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	
	// Workspace membership methods
	AddWorkspaceMember(ctx context.Context, workspaceID, userID string) error
	RemoveWorkspaceMember(ctx context.Context, workspaceID, userID string) error
	ListWorkspacesForUser(ctx context.Context, userID string) ([]WorkspaceInfo, error)
	CheckWorkspaceMembership(ctx context.Context, userID, workspaceID string) (bool, error)
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*RefreshToken, error)
	Revoke(ctx context.Context, id string) error
	RevokeAllForUser(ctx context.Context, userID string) error
	DeleteExpiredOrRevoked(ctx context.Context) (int64, error)
}

type AuditLogRepository interface {
	Create(ctx context.Context, log *AuthAuditLog) error
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
}
