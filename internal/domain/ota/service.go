package ota

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

const manifestURLTTL = 15 * time.Minute

type Service struct {
	repo  Repository
	store ObjectStore
}

func NewService(repo Repository, store ObjectStore) *Service {
	return &Service{
		repo:  repo,
		store: store,
	}
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

	return s.repo.CreateArtifact(ctx, FirmwareArtifact{
		ID:             id,
		Version:        version,
		Filename:       filename,
		ObjectKey:      objectKey,
		ContentType:    contentType,
		SizeBytes:      input.SizeBytes,
		ChecksumSHA256: hex.EncodeToString(hasher.Sum(nil)),
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

func (s *Service) Deploy(ctx context.Context, deviceID string, input DeployInput) (*Manifest, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}
	artifactID := strings.TrimSpace(input.ArtifactID)
	if artifactID == "" {
		return nil, errors.New("firmware artifact id is required")
	}

	artifact, err := s.repo.GetArtifact(ctx, artifactID)
	if err != nil {
		return nil, err
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}

	deployment, err := s.repo.CreateDeployment(ctx, Deployment{
		ID:         id,
		DeviceID:   deviceID,
		ArtifactID: artifact.ID,
		Status:     StatusPending,
	})
	if err != nil {
		return nil, err
	}

	return s.manifest(ctx, deployment, artifact)
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

	return s.manifest(ctx, deployment, artifact)
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

	return record(ctx, deviceID, id, strings.TrimSpace(input.Message))
}

func (s *Service) manifest(ctx context.Context, deployment *Deployment, artifact *FirmwareArtifact) (*Manifest, error) {
	expiresAt := time.Now().UTC().Add(manifestURLTTL)
	url, err := s.store.FirmwareURL(ctx, artifact.ObjectKey, artifact.Filename, manifestURLTTL)
	if err != nil {
		return nil, err
	}

	return &Manifest{
		DeploymentID:   deployment.ID,
		DeviceID:       deployment.DeviceID,
		FirmwareID:     artifact.ID,
		Version:        artifact.Version,
		Filename:       artifact.Filename,
		ContentType:    artifact.ContentType,
		SizeBytes:      artifact.SizeBytes,
		ChecksumSHA256: artifact.ChecksumSHA256,
		DownloadURL:    url,
		ExpiresAt:      expiresAt,
	}, nil
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
