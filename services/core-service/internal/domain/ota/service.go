package ota

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vishalss1/argus/core/internal/domain/device"
)

const manifestURLTTL = 15 * time.Minute
const defaultPendingTimeout = 30 * time.Minute
const defaultActiveTimeout = 30 * time.Minute

type Service struct {
	repo      Repository
	store     ObjectStore
	signer    *FirmwareSigner
	publisher EventPublisher
	OnResult  func(ctx context.Context, deployment Deployment)
	MinioPublicURL string
}

type EventPublisher interface {
	PublishOTAEvent(ctx context.Context, eventType string, deployment Deployment)
}

func NewService(repo Repository, store ObjectStore) *Service {
	return &Service{
		repo:  repo,
		store: store,
	}
}

func (s *Service) SetFirmwareSigner(signer *FirmwareSigner) {
	s.signer = signer
}

func (s *Service) SetEventPublisher(publisher EventPublisher) {
	s.publisher = publisher
}

func (s *Service) UploadFirmware(ctx context.Context, input UploadInput, reader io.Reader) (*FirmwareArtifact, error) {
	version := strings.TrimSpace(input.Version)
	if version == "" {
		return nil, errors.New("firmware version is required")
	}
	filename := cleanFilename(input.Filename)
	if filename == "" {
		return nil, errors.New("filename is required")
	}
	if input.SizeBytes <= 0 {
		return nil, errors.New("firmware file is required")
	}
	if reader == nil {
		return nil, errors.New("firmware file is required")
	}

	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}

	objectKey := fmt.Sprintf("firmware/%s/%s", id, filename)
	hasher := sha256.New()
	if err := s.store.PutFirmware(ctx, objectKey, io.TeeReader(reader, hasher), input.SizeBytes, contentType); err != nil {
		return nil, err
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))
	signatureAlg, signature, signingKeyID, err := s.signFirmwareChecksum(checksum)
	if err != nil {
		return nil, err
	}

	return s.repo.CreateArtifact(ctx, FirmwareArtifact{
		ID:             id,
		Version:        version,
		Filename:       filename,
		ObjectKey:      objectKey,
		ContentType:    contentType,
		SizeBytes:      input.SizeBytes,
		ChecksumSHA256: checksum,
		SignatureAlg:   signatureAlg,
		Signature:      signature,
		SigningKeyID:   signingKeyID,
	})
}

func (s *Service) ListFirmware(ctx context.Context) ([]FirmwareArtifact, error) {
	return s.repo.ListArtifacts(ctx)
}

func (s *Service) GetFirmware(ctx context.Context, id string) (*FirmwareArtifact, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("firmware id is required")
	}

	return s.repo.GetArtifact(ctx, id)
}

func (s *Service) DeleteFirmware(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("firmware id is required")
	}

	artifact, err := s.repo.GetArtifact(ctx, id)
	if err != nil {
		return err
	}

	if err := s.store.RemoveFirmware(ctx, artifact.ObjectKey); err != nil {
		log.Printf("[OTA] Warning: failed to remove firmware from object store: %v", err)
	}

	return s.repo.DeleteArtifact(ctx, id)
}

func (s *Service) Deploy(ctx context.Context, deviceID string, input DeployInput) (*Manifest, error) {
	requestedDeviceID := strings.TrimSpace(deviceID)
	log.Printf("[OTA] deployment create request device=%s artifact=%s", requestedDeviceID, strings.TrimSpace(input.ArtifactID))
	if requestedDeviceID == "" {
		return nil, errors.New("device id is required")
	}
	artifactID := strings.TrimSpace(input.ArtifactID)
	if artifactID == "" {
		return nil, errors.New("firmware artifact id is required")
	}

	resolvedDeviceID, deviceStatus, err := s.repo.ResolveDeviceID(ctx, requestedDeviceID)
	if err != nil {
		if errors.Is(err, device.ErrDeviceNotFound) {
			log.Printf("[OTA] deployment create rejected device=%s reason=device not registered", requestedDeviceID)
		}
		return nil, err
	}
	if resolvedDeviceID != requestedDeviceID {
		log.Printf("[OTA] deployment resolved requested_device=%s target_device=%s", requestedDeviceID, resolvedDeviceID)
	}

	artifact, err := s.repo.GetArtifact(ctx, artifactID)
	if err != nil {
		return nil, err
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}

	deployStatus := StatusPending
	var failureReason *string
	if deviceStatus == "offline" {
		deployStatus = StatusNacked
		reason := "Device is offline"
		failureReason = &reason
	}

	deployment, err := s.repo.CreateDeployment(ctx, Deployment{
		ID:            id,
		DeviceID:      resolvedDeviceID,
		ArtifactID:    artifact.ID,
		Status:        deployStatus,
		FailureReason: failureReason,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("[OTA] deployment persisted deployment=%s device=%s artifact=%s status=%s", deployment.ID, deployment.DeviceID, deployment.ArtifactID, deployment.Status)
	s.publish(ctx, "ota_created", *deployment)

	return s.manifest(ctx, deployment, artifact)
}

func (s *Service) ListDeployments(ctx context.Context) ([]Deployment, error) {
	return s.repo.ListDeployments(ctx)
}

func (s *Service) ListDeploymentsByDevice(ctx context.Context, deviceID string) ([]Deployment, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	return s.repo.ListDeploymentsByDevice(ctx, deviceID)
}

func (s *Service) GetManifest(ctx context.Context, deviceID string, id string) (*Manifest, error) {
	deployment, err := s.getDeployment(ctx, deviceID, id)
	if err != nil {
		return nil, err
	}

	artifact, err := s.repo.GetArtifact(ctx, deployment.ArtifactID)
	if err != nil {
		return nil, err
	}

	manifest, err := s.manifest(ctx, deployment, artifact)
	if err != nil {
		return nil, err
	}
	if deployment.Status == StatusPending {
		if updated, updateErr := s.repo.MarkDeploymentAvailable(ctx, deviceID, id); updateErr == nil {
			s.publish(ctx, "ota_status_changed", *updated)
		}
	}
	return manifest, nil
}

func (s *Service) GetPendingManifest(ctx context.Context, deviceID string) (*Manifest, error) {
	deviceID = strings.TrimSpace(deviceID)
	log.Printf("[OTA] lookup device=%s", deviceID)
	deployment, err := s.repo.GetOldestPendingDeployment(ctx, deviceID)
	if err != nil {
		if errors.Is(err, ErrDeploymentNotFound) {
			s.logDeviceDeployments(ctx, deviceID)
		} else {
			log.Printf("[OTA] lookup failed device=%s error=%v", deviceID, err)
		}
		return nil, err
	}
	log.Printf("[OTA] found deployment=%s", deployment.ID)
	log.Printf("[OTA] deployment status=%s", deployment.Status)
	log.Printf("[OTA] target device=%s", deployment.DeviceID)

	artifact, err := s.repo.GetArtifact(ctx, deployment.ArtifactID)
	if err != nil {
		log.Printf("[OTA] manifest rejected deployment=%s artifact=%s error=%v", deployment.ID, deployment.ArtifactID, err)
		return nil, err
	}

	manifest, err := s.manifest(ctx, deployment, artifact)
	if err != nil {
		log.Printf("[OTA] manifest generation failed deployment=%s artifact=%s error=%v", deployment.ID, artifact.ID, err)
		return nil, err
	}
	if deployment.Status == StatusPending {
		if updated, updateErr := s.repo.MarkDeploymentAvailable(ctx, deviceID, deployment.ID); updateErr == nil {
			s.publish(ctx, "ota_status_changed", *updated)
			log.Printf("[OTA] deployment status changed deployment=%s pending->available", deployment.ID)
		} else {
			log.Printf("[OTA] failed to mark available deployment=%s error=%v", deployment.ID, updateErr)
		}
	}
	log.Printf("[OTA] manifest generated deployment=%s firmware=%s version=%s expires_at=%s", manifest.DeploymentID, manifest.FirmwareID, manifest.Version, manifest.ExpiresAt.Format(time.RFC3339))
	return manifest, nil
}

func (s *Service) RecordProgress(ctx context.Context, deviceID string, input ProgressInput) (*Deployment, error) {
	deviceID = strings.TrimSpace(deviceID)
	input.DeploymentID = strings.TrimSpace(input.DeploymentID)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.Message = strings.TrimSpace(input.Message)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}
	if input.DeploymentID == "" {
		return nil, errors.New("deployment id is required")
	}
	if !isProgressStatus(input.Status) {
		return nil, errors.New("invalid ota progress status")
	}
	if input.Progress != nil {
		if *input.Progress < 0 || *input.Progress > 100 {
			return nil, errors.New("progress must be between 0 and 100")
		}
	}

	deployment, err := s.repo.UpdateDeploymentProgress(ctx, deviceID, input)
	if err == nil {
		s.publish(ctx, "ota_progress", *deployment)
	}
	return deployment, err
}

func (s *Service) Ack(ctx context.Context, deviceID string, id string, input ResultInput) (*Deployment, error) {
	return s.recordResult(ctx, deviceID, id, input, s.repo.AckDeployment)
}

func (s *Service) Nack(ctx context.Context, deviceID string, id string, input ResultInput) (*Deployment, error) {
	return s.recordResult(ctx, deviceID, id, input, s.repo.NackDeployment)
}

func (s *Service) getDeployment(ctx context.Context, deviceID string, id string) (*Deployment, error) {
	deviceID = strings.TrimSpace(deviceID)
	id = strings.TrimSpace(id)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}
	if id == "" {
		return nil, errors.New("deployment id is required")
	}

	return s.repo.GetDeployment(ctx, deviceID, id)
}

func (s *Service) recordResult(
	ctx context.Context,
	deviceID string,
	id string,
	input ResultInput,
	record func(context.Context, string, string, string) (*Deployment, error),
) (*Deployment, error) {
	deviceID = strings.TrimSpace(deviceID)
	id = strings.TrimSpace(id)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}
	if id == "" {
		return nil, errors.New("deployment id is required")
	}

	deployment, err := record(ctx, deviceID, id, strings.TrimSpace(input.Message))
	if err == nil && s.OnResult != nil {
		s.OnResult(ctx, *deployment)
	}
	if err == nil {
		if deployment.Status == StatusAcked {
			s.publish(ctx, "ota_completed", *deployment)
		} else {
			s.publish(ctx, "ota_failed", *deployment)
		}
	}
	return deployment, err
}

func (s *Service) MarkTimedOut(ctx context.Context) ([]Deployment, error) {
	deployments, err := s.repo.MarkTimedOut(ctx, TimeoutPolicy{
		PendingTimeout: defaultPendingTimeout,
		ActiveTimeout:  defaultActiveTimeout,
	})
	if err != nil {
		return nil, err
	}
	for _, deployment := range deployments {
		s.publish(ctx, "ota_failed", deployment)
	}
	return deployments, nil
}

func (s *Service) ListDeploymentEvents(ctx context.Context, deploymentID string) ([]DeploymentEvent, error) {
	deploymentID = strings.TrimSpace(deploymentID)
	if deploymentID == "" {
		return nil, errors.New("deployment id is required")
	}
	return s.repo.ListDeploymentEvents(ctx, deploymentID)
}

func (s *Service) Stats(ctx context.Context) (*FleetStats, error) {
	return s.repo.Stats(ctx)
}

func (s *Service) publish(ctx context.Context, eventType string, deployment Deployment) {
	if s.publisher != nil {
		s.publisher.PublishOTAEvent(ctx, eventType, deployment)
	}
}

func isProgressStatus(status string) bool {
	switch status {
	case StatusDownloading, StatusFlashing, StatusRebooting, StatusAvailable:
		return true
	default:
		return false
	}
}

func (s *Service) logDeviceDeployments(ctx context.Context, deviceID string) {
	deployments, err := s.repo.ListDeploymentsByDevice(ctx, deviceID)
	if err != nil {
		log.Printf("[OTA] no pending deployment diagnostics failed device=%s error=%v", deviceID, err)
		return
	}
	if len(deployments) == 0 {
		log.Printf("[OTA] no deployments found for requested device=%s", deviceID)
		return
	}
	for _, deployment := range deployments {
		log.Printf("[OTA] deployment rejected: device=%s deployment=%s status=%s target_device=%s", deviceID, deployment.ID, deployment.Status, deployment.DeviceID)
	}
}

func (s *Service) manifest(ctx context.Context, deployment *Deployment, artifact *FirmwareArtifact) (*Manifest, error) {
	signatureAlg := artifact.SignatureAlg
	signature := artifact.Signature
	signingKeyID := artifact.SigningKeyID
	checksumSHA256 := artifact.ChecksumSHA256

	if signature == "" && s.signer != nil {
		log.Printf("[OTA] firmware artifact %s is unsigned, signing it dynamically", artifact.ID)
		reader, err := s.store.GetFirmware(ctx, artifact.ObjectKey)
		if err != nil {
			return nil, fmt.Errorf("failed to download firmware binary for hashing: %w", err)
		}
		defer reader.Close()

		hasher := sha256.New()
		if _, err := io.Copy(hasher, reader); err != nil {
			return nil, fmt.Errorf("failed to hash firmware binary: %w", err)
		}
		computedChecksum := hex.EncodeToString(hasher.Sum(nil))

		if checksumSHA256 == "" {
			checksumSHA256 = computedChecksum
		} else if !strings.EqualFold(checksumSHA256, computedChecksum) {
			log.Printf("[OTA] warning: stored checksum %s does not match computed checksum %s", checksumSHA256, computedChecksum)
			checksumSHA256 = computedChecksum
		}

		alg, sig, keyID, err := s.signer.SignChecksum(checksumSHA256)
		if err != nil {
			return nil, fmt.Errorf("failed to sign checksum: %w", err)
		}
		signatureAlg = alg
		signature = sig
		signingKeyID = keyID
	}

	if s.signer != nil && s.signer.RequireSignatures() {
		if signatureAlg != SignatureAlgEd25519 || signature == "" || signingKeyID == "" {
			return nil, errors.New("firmware artifact is unsigned but OTA_REQUIRE_SIGNATURES=true")
		}
	}

	expiresAt := time.Now().UTC().Add(manifestURLTTL)
	fwURL, err := s.store.FirmwareURL(ctx, artifact.ObjectKey, artifact.Filename, manifestURLTTL)
	if err != nil {
		return nil, err
	}

	// Replace internal MinIO service hostname with LAN-accessible host and port
	minioPublicHost := os.Getenv("ARGUS_MINIO_PUBLIC_HOST")
	if minioPublicHost == "" && s.MinioPublicURL != "" {
		if u, err := url.Parse(s.MinioPublicURL); err == nil {
			minioPublicHost = u.Host
		}
	}
	if minioPublicHost == "" {
		minioPublicHost = "minio:9000"
	}

	if os.Getenv("ARGUS_MINIO_PUBLIC_HOST") != "" {
		fwURL = strings.Replace(fwURL, "http://minio:9000", "https://"+minioPublicHost, -1)
		fwURL = strings.Replace(fwURL, "http://localhost:9000", "https://"+minioPublicHost, -1)
	} else {
		fwURL = strings.Replace(fwURL, "minio:9000", minioPublicHost, -1)
		fwURL = strings.Replace(fwURL, "localhost:9000", minioPublicHost, -1)
	}

	return &Manifest{
		DeploymentID:   deployment.ID,
		DeviceID:       deployment.DeviceID,
		FirmwareID:     artifact.ID,
		Version:        artifact.Version,
		Filename:       artifact.Filename,
		ContentType:    artifact.ContentType,
		SizeBytes:      artifact.SizeBytes,
		ChecksumSHA256: checksumSHA256,
		SignatureAlg:   signatureAlg,
		Signature:      signature,
		SigningKeyID:   signingKeyID,
		DownloadURL:    fwURL,
		ExpiresAt:      expiresAt,
	}, nil
}

func (s *Service) signFirmwareChecksum(checksum string) (string, string, string, error) {
	if s.signer == nil {
		return "", "", "", nil
	}
	return s.signer.SignChecksum(checksum)
}

func cleanFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return ""
	}

	return filepath.Base(filename)
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	encoded := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}

