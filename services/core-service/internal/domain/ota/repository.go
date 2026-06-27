package ota

import (
	"context"
	"io"
	"time"
)

type Repository interface {
	CreateArtifact(ctx context.Context, artifact FirmwareArtifact) (*FirmwareArtifact, error)
	ListArtifacts(ctx context.Context) ([]FirmwareArtifact, error)
	GetArtifact(ctx context.Context, id string) (*FirmwareArtifact, error)
	DeleteArtifact(ctx context.Context, id string) error
	ResolveDeviceID(ctx context.Context, idOrHardwareID string) (string, string, error)
	CreateDeployment(ctx context.Context, deployment Deployment) (*Deployment, error)
	ListDeployments(ctx context.Context) ([]Deployment, error)
	ListDeploymentsByDevice(ctx context.Context, deviceID string) ([]Deployment, error)
	GetDeployment(ctx context.Context, deviceID string, id string) (*Deployment, error)
	GetOldestPendingDeployment(ctx context.Context, deviceID string) (*Deployment, error)
	MarkDeploymentAvailable(ctx context.Context, deviceID string, id string) (*Deployment, error)
	UpdateDeploymentProgress(ctx context.Context, deviceID string, input ProgressInput) (*Deployment, error)
	AckDeployment(ctx context.Context, deviceID string, id string, message string) (*Deployment, error)
	NackDeployment(ctx context.Context, deviceID string, id string, reason string) (*Deployment, error)
	MarkTimedOut(ctx context.Context, policy TimeoutPolicy) ([]Deployment, error)
	ListDeploymentEvents(ctx context.Context, deploymentID string) ([]DeploymentEvent, error)
	Stats(ctx context.Context) (*FleetStats, error)
}

type ObjectStore interface {
	PutFirmware(ctx context.Context, objectKey string, reader io.Reader, sizeBytes int64, contentType string) error
	FirmwareURL(ctx context.Context, objectKey string, filename string, expires time.Duration) (string, error)
	GetFirmware(ctx context.Context, objectKey string) (io.ReadCloser, error)
	RemoveFirmware(ctx context.Context, objectKey string) error
}

