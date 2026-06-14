package session

import (
	"context"
	"time"
)

type Command struct {
	ID          string     `json:"id"`
	SessionID   string     `json:"session_id"`
	DeviceID    string     `json:"device_id"`
	Command     string     `json:"command"`
	IssuedBy    *string    `json:"issued_by"`
	Status      string     `json:"status"`
	IssuedAt    time.Time  `json:"issued_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

type CommandRepository interface {
	CreateCommand(ctx context.Context, c Command) (*Command, error)
	UpdateCommandStatus(ctx context.Context, id string, status string, completedAt *time.Time) error
	ListCommandsBySession(ctx context.Context, sessionID string) ([]Command, error)
}

