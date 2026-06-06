package auth

import (
	"time"
)

type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	PasswordHash *string    `json:"-"` // Nullable if local password is not configured
	Name         string     `json:"name"`
	Status       string     `json:"status"` // "active", "disabled"
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type RefreshToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	TokenHash string     `json:"token_hash"`
	ExpiresAt time.Time  `json:"expires_at"`
	Revoked   bool       `json:"revoked"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type WorkspaceMembership struct {
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
}

type AuthAuditLog struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	UserID    *string   `json:"user_id"`
	EventType string    `json:"event_type"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Success   bool      `json:"success"`
}

type WorkspaceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
