package workspace

import (
	"context"
	"time"
)

type Workspace struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	DeviceCount int       `json:"device_count"`
}

type DeviceSummary struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	FirmwareVersion string     `json:"firmware_version"`
	LastSeen        *time.Time `json:"last_seen"`
}

type Repository interface {
	Create(ctx context.Context, w Workspace) (*Workspace, error)
	Get(ctx context.Context, id string) (*Workspace, error)
	List(ctx context.Context) ([]Workspace, error)
	Update(ctx context.Context, id string, name string, description string) (*Workspace, error)
	Delete(ctx context.Context, id string) error
	AssignDevice(ctx context.Context, workspaceID string, deviceID string) error
	UnassignDevice(ctx context.Context, workspaceID string, deviceID string) error
	ListDevices(ctx context.Context, workspaceID string) ([]DeviceSummary, error)
}
