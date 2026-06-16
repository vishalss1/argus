package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/vishalss1/argus/shared/common"
	"github.com/vishalss1/argus/core/internal/domain/device"
	"github.com/vishalss1/argus/core/internal/domain/ota"
)

type OTARepository struct {
	db *sql.DB
}

func NewOTARepository(db *sql.DB) *OTARepository {
	return &OTARepository{db: db}
}

func (r *OTARepository) CreateArtifact(ctx context.Context, artifact ota.FirmwareArtifact) (*ota.FirmwareArtifact, error) {
	const query = `
		INSERT INTO firmware_artifacts (id, version, filename, object_key, content_type, size_bytes, checksum_sha256, signature_alg, signature, signing_key_id)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''))
		RETURNING id, version, filename, object_key, content_type, size_bytes, checksum_sha256, COALESCE(signature_alg, ''), COALESCE(signature, ''), COALESCE(signing_key_id, ''), created_at`

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
		artifact.SignatureAlg,
		artifact.Signature,
		artifact.SigningKeyID,
	))
	if err != nil {
		return nil, fmt.Errorf("create firmware artifact: %w", err)
	}

	return created, nil
}

func (r *OTARepository) ListArtifacts(ctx context.Context) ([]ota.FirmwareArtifact, error) {
	const query = `
		SELECT id, version, filename, object_key, content_type, size_bytes, checksum_sha256, COALESCE(signature_alg, ''), COALESCE(signature, ''), COALESCE(signing_key_id, ''), created_at
		FROM firmware_artifacts
		ORDER BY created_at DESC
		LIMIT 100`

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
		SELECT id, version, filename, object_key, content_type, size_bytes, checksum_sha256, COALESCE(signature_alg, ''), COALESCE(signature, ''), COALESCE(signing_key_id, ''), created_at
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

func (r *OTARepository) DeleteArtifact(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Delete associated deployments (associated events will cascade delete automatically)
	const deleteDeploymentsQuery = `DELETE FROM ota_deployments WHERE artifact_id = $1::uuid`
	if _, err := tx.ExecContext(ctx, deleteDeploymentsQuery, id); err != nil {
		return fmt.Errorf("delete associated ota deployments: %w", err)
	}

	// 2. Delete the firmware artifact itself
	const deleteArtifactQuery = `DELETE FROM firmware_artifacts WHERE id = $1::uuid`
	res, err := tx.ExecContext(ctx, deleteArtifactQuery, id)
	if err != nil {
		return fmt.Errorf("delete firmware artifact: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ota.ErrFirmwareNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete transaction: %w", err)
	}
	return nil
}

func (r *OTARepository) ResolveDeviceID(ctx context.Context, idOrHardwareID string) (string, error) {
	const query = `
		SELECT id
		FROM devices
		WHERE id::text = $1 OR metadata->>'hardware_id' = $1
		ORDER BY CASE WHEN id::text = $1 THEN 0 ELSE 1 END, created_at ASC
		LIMIT 1`

	var id string
	err := r.db.QueryRowContext(ctx, query, idOrHardwareID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", device.ErrDeviceNotFound
	}
	if err != nil {
		return "", fmt.Errorf("resolve ota device id: %w", err)
	}
	return id, nil
}

func (r *OTARepository) CreateDeployment(ctx context.Context, deployment ota.Deployment) (*ota.Deployment, error) {
	query := `
		WITH created AS (
			INSERT INTO ota_deployments (id, device_id, artifact_id, status, progress)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, 0)
			RETURNING *
		), event AS (
			INSERT INTO ota_deployment_events (deployment_id, device_id, status, progress, message)
			SELECT id, device_id, status, progress, 'Deployment created'
			FROM created
		)
		SELECT ` + deploymentColumns("created") + `
		FROM created
		JOIN firmware_artifacts fa ON fa.id = created.artifact_id
		JOIN devices d ON d.id = created.device_id`

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

func (r *OTARepository) ListDeployments(ctx context.Context) ([]ota.Deployment, error) {
	var query string
	var args []any

	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query = `
			SELECT ` + deploymentColumns("od") + `
			FROM ota_deployments od
			JOIN firmware_artifacts fa ON fa.id = od.artifact_id
			JOIN devices d ON d.id = od.device_id
			WHERE d.workspace_id = $1::uuid
			ORDER BY od.created_at DESC
			LIMIT 500`
		args = append(args, wID)
	} else {
		query = `
			SELECT ` + deploymentColumns("od") + `
			FROM ota_deployments od
			JOIN firmware_artifacts fa ON fa.id = od.artifact_id
			JOIN devices d ON d.id = od.device_id
			ORDER BY od.created_at DESC
			LIMIT 500`
	}
	return r.listDeployments(ctx, query, args...)
}

func (r *OTARepository) ListDeploymentsByDevice(ctx context.Context, deviceID string) ([]ota.Deployment, error) {
	var query string
	var args []any

	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query = `
			SELECT ` + deploymentColumns("od") + `
			FROM ota_deployments od
			JOIN firmware_artifacts fa ON fa.id = od.artifact_id
			JOIN devices d ON d.id = od.device_id
			WHERE od.device_id = $1::uuid AND d.workspace_id = $2::uuid
			ORDER BY od.created_at DESC
			LIMIT 100`
		args = append(args, deviceID, wID)
	} else {
		query = `
			SELECT ` + deploymentColumns("od") + `
			FROM ota_deployments od
			JOIN firmware_artifacts fa ON fa.id = od.artifact_id
			JOIN devices d ON d.id = od.device_id
			WHERE od.device_id = $1::uuid
			ORDER BY od.created_at DESC
			LIMIT 100`
		args = append(args, deviceID)
	}
	return r.listDeployments(ctx, query, args...)
}

func (r *OTARepository) listDeployments(ctx context.Context, query string, args ...any) ([]ota.Deployment, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
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
	var query string
	var row *sql.Row

	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query = `
			SELECT ` + deploymentColumns("od") + `
			FROM ota_deployments od
			JOIN firmware_artifacts fa ON fa.id = od.artifact_id
			JOIN devices d ON d.id = od.device_id
			WHERE od.device_id = $1::uuid AND od.id = $2::uuid AND d.workspace_id = $3::uuid`
		row = r.db.QueryRowContext(ctx, query, deviceID, id, wID)
	} else {
		query = `
			SELECT ` + deploymentColumns("od") + `
			FROM ota_deployments od
			JOIN firmware_artifacts fa ON fa.id = od.artifact_id
			JOIN devices d ON d.id = od.device_id
			WHERE od.device_id = $1::uuid AND od.id = $2::uuid`
		row = r.db.QueryRowContext(ctx, query, deviceID, id)
	}

	deployment, err := scanDeployment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ota.ErrDeploymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get ota deployment: %w", err)
	}

	return deployment, nil
}

func (r *OTARepository) GetOldestPendingDeployment(ctx context.Context, deviceID string) (*ota.Deployment, error) {
	log.Printf("[OTA] repository pending lookup device=%s statuses=%s,%s,%s,%s,%s", deviceID, ota.StatusPending, ota.StatusAvailable, ota.StatusDownloading, ota.StatusFlashing, ota.StatusRebooting)
	query := `
		SELECT ` + deploymentColumns("od") + `
		FROM ota_deployments od
		JOIN firmware_artifacts fa ON fa.id = od.artifact_id
		JOIN devices d ON d.id = od.device_id
		WHERE od.device_id = $1::uuid AND od.status IN ($2, $3, $4, $5, $6)
		ORDER BY od.created_at ASC
		LIMIT 1`

	deployment, err := scanDeployment(r.db.QueryRowContext(ctx, query, deviceID, ota.StatusPending, ota.StatusAvailable, ota.StatusDownloading, ota.StatusFlashing, ota.StatusRebooting))
	if errors.Is(err, sql.ErrNoRows) {
		r.logPendingLookupDiagnostics(ctx, deviceID)
		return nil, ota.ErrDeploymentNotFound
	}
	if err != nil {
		log.Printf("[OTA] repository pending lookup failed device=%s error=%v", deviceID, err)
		return nil, fmt.Errorf("get oldest pending ota deployment: %w", err)
	}

	log.Printf("[OTA] repository pending lookup selected deployment=%s status=%s target_device=%s", deployment.ID, deployment.Status, deployment.DeviceID)
	return deployment, nil
}

func (r *OTARepository) logPendingLookupDiagnostics(ctx context.Context, requestedDeviceID string) {
	const query = `
		SELECT od.id, od.device_id, od.status, od.artifact_id, d.id IS NOT NULL, fa.id IS NOT NULL
		FROM ota_deployments od
		LEFT JOIN devices d ON d.id = od.device_id
		LEFT JOIN firmware_artifacts fa ON fa.id = od.artifact_id
		WHERE od.status IN ($1, $2, $3, $4, $5)
		ORDER BY od.created_at DESC
		LIMIT 20`

	rows, err := r.db.QueryContext(ctx, query, ota.StatusPending, ota.StatusAvailable, ota.StatusDownloading, ota.StatusFlashing, ota.StatusRebooting)
	if err != nil {
		log.Printf("[OTA] pending lookup diagnostics failed requested_device=%s error=%v", requestedDeviceID, err)
		return
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		found = true
		var deploymentID, targetDeviceID, status, artifactID string
		var deviceExists, artifactExists bool
		if err := rows.Scan(&deploymentID, &targetDeviceID, &status, &artifactID, &deviceExists, &artifactExists); err != nil {
			log.Printf("[OTA] pending lookup diagnostics scan failed requested_device=%s error=%v", requestedDeviceID, err)
			return
		}
		reason := "eligible for another device"
		if targetDeviceID == requestedDeviceID {
			if !deviceExists {
				reason = "target device row missing"
			} else if !artifactExists {
				reason = "firmware artifact row missing"
			} else {
				reason = "unexpectedly not selected"
			}
		}
		log.Printf("[OTA] deployment rejected: requested_device=%s deployment=%s status=%s target_device=%s artifact=%s reason=%s", requestedDeviceID, deploymentID, status, targetDeviceID, artifactID, reason)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[OTA] pending lookup diagnostics rows failed requested_device=%s error=%v", requestedDeviceID, err)
		return
	}
	if !found {
		log.Printf("[OTA] no non-terminal ota deployments exist while looking up device=%s", requestedDeviceID)
	}
}

func (r *OTARepository) MarkDeploymentAvailable(ctx context.Context, deviceID string, id string) (*ota.Deployment, error) {
	return r.updateStatus(ctx, deviceID, id, ota.StatusAvailable, nil, "Manifest available")
}

func (r *OTARepository) UpdateDeploymentProgress(ctx context.Context, deviceID string, input ota.ProgressInput) (*ota.Deployment, error) {
	return r.updateStatus(ctx, deviceID, input.DeploymentID, input.Status, input.Progress, input.Message)
}

func (r *OTARepository) AckDeployment(ctx context.Context, deviceID string, id string, message string) (*ota.Deployment, error) {
	progress := 100
	return r.updateStatus(ctx, deviceID, id, ota.StatusAcked, &progress, message)
}

func (r *OTARepository) NackDeployment(ctx context.Context, deviceID string, id string, reason string) (*ota.Deployment, error) {
	return r.updateStatus(ctx, deviceID, id, ota.StatusNacked, nil, reason)
}

func (r *OTARepository) updateStatus(ctx context.Context, deviceID string, id string, status string, progress *int, message string) (*ota.Deployment, error) {
	query := `
		WITH updated AS (
			UPDATE ota_deployments
			SET status = $3,
				progress = COALESCE($4, progress),
				result_message = CASE WHEN $3 IN ('acked', 'nacked', 'timeout') THEN NULLIF($5, '') ELSE result_message END,
				failure_reason = CASE WHEN $3 IN ('nacked', 'timeout') THEN NULLIF($5, '') ELSE failure_reason END,
				available_at = CASE WHEN $3 = 'available' THEN COALESCE(available_at, NOW()) ELSE available_at END,
				downloading_at = CASE WHEN $3 = 'downloading' THEN COALESCE(downloading_at, NOW()) ELSE downloading_at END,
				flashing_at = CASE WHEN $3 = 'flashing' THEN COALESCE(flashing_at, NOW()) ELSE flashing_at END,
				rebooting_at = CASE WHEN $3 = 'rebooting' THEN COALESCE(rebooting_at, NOW()) ELSE rebooting_at END,
				acknowledged_at = CASE WHEN $3 IN ('acked', 'nacked') THEN NOW() ELSE acknowledged_at END,
				completed_at = CASE WHEN $3 = 'acked' THEN NOW() ELSE completed_at END,
				failed_at = CASE WHEN $3 = 'nacked' THEN NOW() ELSE failed_at END,
				timed_out_at = CASE WHEN $3 = 'timeout' THEN NOW() ELSE timed_out_at END,
				updated_at = NOW()
			WHERE device_id = $1::uuid AND id = $2::uuid
			RETURNING *
		), event AS (
			INSERT INTO ota_deployment_events (deployment_id, device_id, status, progress, message)
			SELECT id, device_id, status, progress, NULLIF($5, '')
			FROM updated
		)
		SELECT ` + deploymentColumns("updated") + `
		FROM updated
		JOIN firmware_artifacts fa ON fa.id = updated.artifact_id
		JOIN devices d ON d.id = updated.device_id`

	deployment, err := scanDeployment(r.db.QueryRowContext(ctx, query, deviceID, id, status, progress, message))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ota.ErrDeploymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update ota deployment status: %w", err)
	}

	return deployment, nil
}

func (r *OTARepository) MarkTimedOut(ctx context.Context, policy ota.TimeoutPolicy) ([]ota.Deployment, error) {
	if policy.PendingTimeout <= 0 {
		policy.PendingTimeout = 30 * time.Minute
	}
	if policy.ActiveTimeout <= 0 {
		policy.ActiveTimeout = 30 * time.Minute
	}

	query := `
		WITH updated AS (
			UPDATE ota_deployments
			SET status = 'timeout',
				failure_reason = CASE
					WHEN status IN ('pending', 'available') THEN 'Deployment was not picked up before timeout'
					ELSE 'Deployment progress stopped before ACK/NACK'
				END,
				result_message = CASE
					WHEN status IN ('pending', 'available') THEN 'Deployment was not picked up before timeout'
					ELSE 'Deployment progress stopped before ACK/NACK'
				END,
				timed_out_at = NOW(),
				updated_at = NOW()
			WHERE status IN ('pending', 'available', 'downloading', 'flashing', 'rebooting')
			  AND (
				(status IN ('pending', 'available') AND created_at < NOW() - $1::interval)
				OR
				(status IN ('downloading', 'flashing', 'rebooting') AND updated_at < NOW() - $2::interval)
			  )
			RETURNING *
		), event AS (
			INSERT INTO ota_deployment_events (deployment_id, device_id, status, progress, message)
			SELECT id, device_id, status, progress, failure_reason
			FROM updated
		)
		SELECT ` + deploymentColumns("updated") + `
		FROM updated
		JOIN firmware_artifacts fa ON fa.id = updated.artifact_id
		JOIN devices d ON d.id = updated.device_id
		ORDER BY updated.updated_at DESC`

	return r.listDeployments(ctx, query, fmt.Sprintf("%f seconds", policy.PendingTimeout.Seconds()), fmt.Sprintf("%f seconds", policy.ActiveTimeout.Seconds()))
}

func (r *OTARepository) ListDeploymentEvents(ctx context.Context, deploymentID string) ([]ota.DeploymentEvent, error) {
	const query = `
		SELECT id, deployment_id, device_id, status, progress, message, created_at
		FROM ota_deployment_events
		WHERE deployment_id = $1::uuid
		ORDER BY created_at ASC, id ASC
		LIMIT 500`

	rows, err := r.db.QueryContext(ctx, query, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("list ota deployment events: %w", err)
	}
	defer rows.Close()

	events := make([]ota.DeploymentEvent, 0)
	for rows.Next() {
		var event ota.DeploymentEvent
		var progress sql.NullInt64
		var message sql.NullString
		if err := rows.Scan(&event.ID, &event.DeploymentID, &event.DeviceID, &event.Status, &progress, &message, &event.CreatedAt); err != nil {
			return nil, err
		}
		if progress.Valid {
			value := int(progress.Int64)
			event.Progress = &value
		}
		if message.Valid {
			event.Message = &message.String
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list ota deployment events rows: %w", err)
	}

	return events, nil
}

func (r *OTARepository) Stats(ctx context.Context) (*ota.FleetStats, error) {
	var query string
	var row *sql.Row

	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query = `
			SELECT
				COUNT(*)::int,
				COUNT(*) FILTER (WHERE od.status = 'acked')::int,
				COUNT(*) FILTER (WHERE od.status IN ('nacked', 'timeout'))::int,
				COUNT(DISTINCT od.device_id) FILTER (WHERE od.status IN ('pending', 'available', 'downloading', 'flashing', 'rebooting'))::int
			FROM ota_deployments od
			JOIN devices d ON od.device_id = d.id
			WHERE d.workspace_id = $1::uuid`
		row = r.db.QueryRowContext(ctx, query, wID)
	} else {
		query = `
			SELECT
				COUNT(*)::int,
				COUNT(*) FILTER (WHERE status = 'acked')::int,
				COUNT(*) FILTER (WHERE status IN ('nacked', 'timeout'))::int,
				COUNT(DISTINCT device_id) FILTER (WHERE status IN ('pending', 'available', 'downloading', 'flashing', 'rebooting'))::int
			FROM ota_deployments`
		row = r.db.QueryRowContext(ctx, query)
	}

	var stats ota.FleetStats
	if err := row.Scan(
		&stats.TotalDeployments,
		&stats.SuccessfulDeployments,
		&stats.FailedDeployments,
		&stats.DevicesPendingUpdate,
	); err != nil {
		return nil, fmt.Errorf("ota stats: %w", err)
	}
	if stats.TotalDeployments > 0 {
		stats.SuccessRate = float64(stats.SuccessfulDeployments) / float64(stats.TotalDeployments) * 100
	}
	return &stats, nil
}

func deploymentColumns(alias string) string {
	return fmt.Sprintf(`%[1]s.id, %[1]s.device_id, %[1]s.artifact_id, %[1]s.status, %[1]s.progress, %[1]s.result_message, %[1]s.failure_reason,
		d.name, fa.version, fa.filename,
		%[1]s.created_at, %[1]s.available_at, %[1]s.downloading_at, %[1]s.flashing_at, %[1]s.rebooting_at,
		%[1]s.acknowledged_at, %[1]s.completed_at, %[1]s.failed_at, %[1]s.timed_out_at, %[1]s.updated_at`, alias)
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
		&artifact.SignatureAlg,
		&artifact.Signature,
		&artifact.SigningKeyID,
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
	var failureReason sql.NullString
	var availableAt sql.NullTime
	var downloadingAt sql.NullTime
	var flashingAt sql.NullTime
	var rebootingAt sql.NullTime
	var acknowledgedAt sql.NullTime
	var completedAt sql.NullTime
	var failedAt sql.NullTime
	var timedOutAt sql.NullTime

	err := scanner.Scan(
		&deployment.ID,
		&deployment.DeviceID,
		&deployment.ArtifactID,
		&deployment.Status,
		&deployment.Progress,
		&resultMessage,
		&failureReason,
		&deployment.DeviceName,
		&deployment.Version,
		&deployment.Filename,
		&deployment.CreatedAt,
		&availableAt,
		&downloadingAt,
		&flashingAt,
		&rebootingAt,
		&acknowledgedAt,
		&completedAt,
		&failedAt,
		&timedOutAt,
		&deployment.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if resultMessage.Valid {
		deployment.ResultMessage = &resultMessage.String
	}
	if failureReason.Valid {
		deployment.FailureReason = &failureReason.String
	}
	if availableAt.Valid {
		deployment.AvailableAt = otaTimePtr(availableAt.Time)
	}
	if downloadingAt.Valid {
		deployment.DownloadingAt = otaTimePtr(downloadingAt.Time)
	}
	if flashingAt.Valid {
		deployment.FlashingAt = otaTimePtr(flashingAt.Time)
	}
	if rebootingAt.Valid {
		deployment.RebootingAt = otaTimePtr(rebootingAt.Time)
	}
	if acknowledgedAt.Valid {
		deployment.AcknowledgedAt = otaTimePtr(acknowledgedAt.Time)
	}
	if completedAt.Valid {
		deployment.CompletedAt = otaTimePtr(completedAt.Time)
	}
	if failedAt.Valid {
		deployment.FailedAt = otaTimePtr(failedAt.Time)
	}
	if timedOutAt.Valid {
		deployment.TimedOutAt = otaTimePtr(timedOutAt.Time)
	}

	return &deployment, nil
}

func otaTimePtr(t time.Time) *time.Time {
	return &t
}


