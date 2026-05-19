package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vishalss1/argus/internal/domain/device"
	"github.com/vishalss1/argus/internal/domain/ota"
)

type OTARepository struct {
	db *sql.DB
}

func NewOTARepository(db *sql.DB) *OTARepository {
	return &OTARepository{db: db}
}

func (r *OTARepository) CreateArtifact(ctx context.Context, artifact ota.FirmwareArtifact) (*ota.FirmwareArtifact, error) {
	const query = `
		INSERT INTO firmware_artifacts (id, version, filename, object_key, content_type, size_bytes, checksum_sha256)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
		RETURNING id, version, filename, object_key, content_type, size_bytes, checksum_sha256, created_at`

	created, err := scanFirmwareArtifact(r.db.QueryRowContext(
		ctx,
		query,
		artifact.ID,
		artifact.Version,
		artifact.Filename,
		artifact.ObjectKey,
		artifact.ContentType,
		artifact.SizeBytes,
		artifact.ChecksumSHA256,
	))
	if err != nil {
		return nil, fmt.Errorf("create firmware artifact: %w", err)
	}

	return created, nil
}

func (r *OTARepository) ListArtifacts(ctx context.Context) ([]ota.FirmwareArtifact, error) {
	const query = `
		SELECT id, version, filename, object_key, content_type, size_bytes, checksum_sha256, created_at
		FROM firmware_artifacts
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list firmware artifacts: %w", err)
	}
	defer rows.Close()

	artifacts := make([]ota.FirmwareArtifact, 0)
	for rows.Next() {
		artifact, err := scanFirmwareArtifact(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, *artifact)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list firmware artifacts rows: %w", err)
	}

	return artifacts, nil
}

func (r *OTARepository) GetArtifact(ctx context.Context, id string) (*ota.FirmwareArtifact, error) {
	const query = `
		SELECT id, version, filename, object_key, content_type, size_bytes, checksum_sha256, created_at
		FROM firmware_artifacts
		WHERE id = $1::uuid`

	artifact, err := scanFirmwareArtifact(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ota.ErrFirmwareNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get firmware artifact: %w", err)
	}

	return artifact, nil
}

func (r *OTARepository) CreateDeployment(ctx context.Context, deployment ota.Deployment) (*ota.Deployment, error) {
	const query = `
		INSERT INTO ota_deployments (id, device_id, artifact_id, status)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4)
		RETURNING id, device_id, artifact_id, status, result_message, created_at, acknowledged_at, updated_at`

	created, err := scanDeployment(r.db.QueryRowContext(
		ctx,
		query,
		deployment.ID,
		deployment.DeviceID,
		deployment.ArtifactID,
		deployment.Status,
	))
	if isForeignKeyViolation(err) {
		return nil, device.ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("create ota deployment: %w", err)
	}

	return created, nil
}

func (r *OTARepository) ListDeploymentsByDevice(ctx context.Context, deviceID string) ([]ota.Deployment, error) {
	const query = `
		SELECT id, device_id, artifact_id, status, result_message, created_at, acknowledged_at, updated_at
		FROM ota_deployments
		WHERE device_id = $1::uuid
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("list ota deployments: %w", err)
	}
	defer rows.Close()

	deployments := make([]ota.Deployment, 0)
	for rows.Next() {
		deployment, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, *deployment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list ota deployments rows: %w", err)
	}

	return deployments, nil
}

func (r *OTARepository) GetDeployment(ctx context.Context, deviceID string, id string) (*ota.Deployment, error) {
	const query = `
		SELECT id, device_id, artifact_id, status, result_message, created_at, acknowledged_at, updated_at
		FROM ota_deployments
		WHERE device_id = $1::uuid AND id = $2::uuid`

	deployment, err := scanDeployment(r.db.QueryRowContext(ctx, query, deviceID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ota.ErrDeploymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get ota deployment: %w", err)
	}

	return deployment, nil
}

func (r *OTARepository) AckDeployment(ctx context.Context, deviceID string, id string, message string) (*ota.Deployment, error) {
	return r.updateDeploymentResult(ctx, deviceID, id, ota.StatusAcked, message)
}

func (r *OTARepository) NackDeployment(ctx context.Context, deviceID string, id string, reason string) (*ota.Deployment, error) {
	return r.updateDeploymentResult(ctx, deviceID, id, ota.StatusNacked, reason)
}

func (r *OTARepository) updateDeploymentResult(ctx context.Context, deviceID string, id string, status string, message string) (*ota.Deployment, error) {
	const query = `
		UPDATE ota_deployments
		SET status = $3,
			result_message = NULLIF($4, ''),
			acknowledged_at = NOW(),
			updated_at = NOW()
		WHERE device_id = $1::uuid AND id = $2::uuid
		RETURNING id, device_id, artifact_id, status, result_message, created_at, acknowledged_at, updated_at`

	deployment, err := scanDeployment(r.db.QueryRowContext(ctx, query, deviceID, id, status, message))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ota.ErrDeploymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update ota deployment result: %w", err)
	}

	return deployment, nil
}

type firmwareArtifactScanner interface {
	Scan(dest ...any) error
}

func scanFirmwareArtifact(scanner firmwareArtifactScanner) (*ota.FirmwareArtifact, error) {
	var artifact ota.FirmwareArtifact
	err := scanner.Scan(
		&artifact.ID,
		&artifact.Version,
		&artifact.Filename,
		&artifact.ObjectKey,
		&artifact.ContentType,
		&artifact.SizeBytes,
		&artifact.ChecksumSHA256,
		&artifact.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &artifact, nil
}

type deploymentScanner interface {
	Scan(dest ...any) error
}

func scanDeployment(scanner deploymentScanner) (*ota.Deployment, error) {
	var deployment ota.Deployment
	var resultMessage sql.NullString
	var acknowledgedAt sql.NullTime

	err := scanner.Scan(
		&deployment.ID,
		&deployment.DeviceID,
		&deployment.ArtifactID,
		&deployment.Status,
		&resultMessage,
		&deployment.CreatedAt,
		&acknowledgedAt,
		&deployment.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if resultMessage.Valid {
		deployment.ResultMessage = &resultMessage.String
	}
	if acknowledgedAt.Valid {
		deployment.AcknowledgedAt = otaTimePtr(acknowledgedAt.Time)
	}

	return &deployment, nil
}

func otaTimePtr(t time.Time) *time.Time {
	return &t
}
