package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vishalss1/argus/core/internal/domain/device"
	"github.com/vishalss1/argus/core/internal/domain/ota"
)

const (
	testDeviceID     = "372fa1f0-2d4f-44ff-8e6c-ced30602e7d5"
	otherDeviceID    = "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"
	testDeploymentID = "0635b91f-1111-4111-8111-111111111111"
	testArtifactID   = "bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb"
)

func TestGetPendingDeploymentReturnsManifest(t *testing.T) {
	repo := newFakeOTARepository()
	repo.deployments = append(repo.deployments, ota.Deployment{
		ID:         testDeploymentID,
		DeviceID:   testDeviceID,
		ArtifactID: testArtifactID,
		Status:     ota.StatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	handler := NewOTAHandler(ota.NewService(repo, fakeObjectStore{}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/devices/"+testDeviceID+"/ota/pending", nil)
	handler.GetPendingDeployment(rr, req, testDeviceID)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"deployment_id":"`+testDeploymentID+`"`)) {
		t.Fatalf("expected manifest deployment id, got %s", rr.Body.String())
	}
	if repo.deployments[0].Status != ota.StatusAvailable {
		t.Fatalf("expected deployment to transition to available, got %s", repo.deployments[0].Status)
	}
}

func TestGetPendingDeploymentWrongDeviceReturnsNoContent(t *testing.T) {
	repo := newFakeOTARepository()
	repo.deployments = append(repo.deployments, ota.Deployment{
		ID:         testDeploymentID,
		DeviceID:   otherDeviceID,
		ArtifactID: testArtifactID,
		Status:     ota.StatusAvailable,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	handler := NewOTAHandler(ota.NewService(repo, fakeObjectStore{}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/devices/"+testDeviceID+"/ota/pending", nil)
	handler.GetPendingDeployment(rr, req, testDeviceID)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGetPendingDeploymentActiveStatusStillReturnsManifest(t *testing.T) {
	repo := newFakeOTARepository()
	repo.deployments = append(repo.deployments, ota.Deployment{
		ID:         testDeploymentID,
		DeviceID:   testDeviceID,
		ArtifactID: testArtifactID,
		Status:     ota.StatusDownloading,
		Progress:   35,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	handler := NewOTAHandler(ota.NewService(repo, fakeObjectStore{}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/devices/"+testDeviceID+"/ota/pending", nil)
	handler.GetPendingDeployment(rr, req, testDeviceID)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for active deployment, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGetPendingDeploymentTerminalStatusReturnsNoContent(t *testing.T) {
	repo := newFakeOTARepository()
	repo.deployments = append(repo.deployments, ota.Deployment{
		ID:         testDeploymentID,
		DeviceID:   testDeviceID,
		ArtifactID: testArtifactID,
		Status:     ota.StatusNacked,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	handler := NewOTAHandler(ota.NewService(repo, fakeObjectStore{}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/devices/"+testDeviceID+"/ota/pending", nil)
	handler.GetPendingDeployment(rr, req, testDeviceID)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for terminal deployment, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDeployFirmwarePersistsResolvedDeviceID(t *testing.T) {
	repo := newFakeOTARepository()
	hardwareID := "esp32-board-01"
	repo.deviceAliases[hardwareID] = testDeviceID
	service := ota.NewService(repo, fakeObjectStore{})

	manifest, err := service.Deploy(context.Background(), hardwareID, ota.DeployInput{ArtifactID: testArtifactID})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	if manifest.DeviceID != testDeviceID {
		t.Fatalf("expected manifest device %s, got %s", testDeviceID, manifest.DeviceID)
	}
	if len(repo.deployments) != 1 {
		t.Fatalf("expected one deployment, got %d", len(repo.deployments))
	}
	if repo.deployments[0].DeviceID != testDeviceID {
		t.Fatalf("expected deployment device %s, got %s", testDeviceID, repo.deployments[0].DeviceID)
	}
}

func TestDeployFirmwareRejectsUnknownDevice(t *testing.T) {
	repo := newFakeOTARepository()
	service := ota.NewService(repo, fakeObjectStore{})

	_, err := service.Deploy(context.Background(), "missing-device", ota.DeployInput{ArtifactID: testArtifactID})
	if !errors.Is(err, device.ErrDeviceNotFound) {
		t.Fatalf("expected device not found, got %v", err)
	}
	if len(repo.deployments) != 0 {
		t.Fatalf("expected no deployment to be persisted, got %d", len(repo.deployments))
	}
}

type fakeObjectStore struct{}

func (fakeObjectStore) PutFirmware(ctx context.Context, objectKey string, reader io.Reader, sizeBytes int64, contentType string) error {
	return nil
}

func (fakeObjectStore) FirmwareURL(ctx context.Context, objectKey string, filename string, expires time.Duration) (string, error) {
	return "http://minio.local/" + objectKey, nil
}

func (fakeObjectStore) GetFirmware(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader([]byte("fake firmware binary content"))), nil
}

func (fakeObjectStore) RemoveFirmware(ctx context.Context, objectKey string) error {
	return nil
}

type fakeOTARepository struct {
	artifacts     map[string]ota.FirmwareArtifact
	deviceAliases map[string]string
	deployments   []ota.Deployment
}

func newFakeOTARepository() *fakeOTARepository {
	return &fakeOTARepository{
		deviceAliases: map[string]string{
			testDeviceID:  testDeviceID,
			otherDeviceID: otherDeviceID,
		},
		artifacts: map[string]ota.FirmwareArtifact{
			testArtifactID: {
				ID:             testArtifactID,
				Version:        "v1.4.0",
				Filename:       "firmware.bin",
				ObjectKey:      "firmware/test/firmware.bin",
				ContentType:    "application/octet-stream",
				SizeBytes:      1024,
				ChecksumSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				CreatedAt:      time.Now(),
			},
		},
	}
}

func (r *fakeOTARepository) ResolveDeviceID(ctx context.Context, idOrHardwareID string) (string, string, error) {
	id, ok := r.deviceAliases[idOrHardwareID]
	if !ok {
		return "", "", device.ErrDeviceNotFound
	}
	return id, "online", nil
}

func (r *fakeOTARepository) CreateArtifact(ctx context.Context, artifact ota.FirmwareArtifact) (*ota.FirmwareArtifact, error) {
	r.artifacts[artifact.ID] = artifact
	return &artifact, nil
}

func (r *fakeOTARepository) ListArtifacts(ctx context.Context) ([]ota.FirmwareArtifact, error) {
	artifacts := make([]ota.FirmwareArtifact, 0, len(r.artifacts))
	for _, artifact := range r.artifacts {
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func (r *fakeOTARepository) GetArtifact(ctx context.Context, id string) (*ota.FirmwareArtifact, error) {
	artifact, ok := r.artifacts[id]
	if !ok {
		return nil, ota.ErrFirmwareNotFound
	}
	return &artifact, nil
}

func (r *fakeOTARepository) DeleteArtifact(ctx context.Context, id string) error {
	if _, ok := r.artifacts[id]; !ok {
		return ota.ErrFirmwareNotFound
	}
	delete(r.artifacts, id)
	return nil
}

func (r *fakeOTARepository) CreateDeployment(ctx context.Context, deployment ota.Deployment) (*ota.Deployment, error) {
	r.deployments = append(r.deployments, deployment)
	return &r.deployments[len(r.deployments)-1], nil
}

func (r *fakeOTARepository) ListDeployments(ctx context.Context) ([]ota.Deployment, error) {
	return append([]ota.Deployment(nil), r.deployments...), nil
}

func (r *fakeOTARepository) ListDeploymentsByDevice(ctx context.Context, deviceID string) ([]ota.Deployment, error) {
	var out []ota.Deployment
	for _, deployment := range r.deployments {
		if deployment.DeviceID == deviceID {
			out = append(out, deployment)
		}
	}
	return out, nil
}

func (r *fakeOTARepository) GetDeployment(ctx context.Context, deviceID string, id string) (*ota.Deployment, error) {
	for i := range r.deployments {
		if r.deployments[i].DeviceID == deviceID && r.deployments[i].ID == id {
			return &r.deployments[i], nil
		}
	}
	return nil, ota.ErrDeploymentNotFound
}

func (r *fakeOTARepository) GetOldestPendingDeployment(ctx context.Context, deviceID string) (*ota.Deployment, error) {
	for i := range r.deployments {
		deployment := &r.deployments[i]
		if deployment.DeviceID == deviceID && isFakeActiveOTAStatus(deployment.Status) {
			return deployment, nil
		}
	}
	return nil, ota.ErrDeploymentNotFound
}

func (r *fakeOTARepository) MarkDeploymentAvailable(ctx context.Context, deviceID string, id string) (*ota.Deployment, error) {
	return r.setStatus(deviceID, id, ota.StatusAvailable, nil, "")
}

func (r *fakeOTARepository) UpdateDeploymentProgress(ctx context.Context, deviceID string, input ota.ProgressInput) (*ota.Deployment, error) {
	return r.setStatus(deviceID, input.DeploymentID, input.Status, input.Progress, input.Message)
}

func (r *fakeOTARepository) AckDeployment(ctx context.Context, deviceID string, id string, message string) (*ota.Deployment, error) {
	progress := 100
	return r.setStatus(deviceID, id, ota.StatusAcked, &progress, message)
}

func (r *fakeOTARepository) NackDeployment(ctx context.Context, deviceID string, id string, reason string) (*ota.Deployment, error) {
	return r.setStatus(deviceID, id, ota.StatusNacked, nil, reason)
}

func (r *fakeOTARepository) MarkTimedOut(ctx context.Context, policy ota.TimeoutPolicy) ([]ota.Deployment, error) {
	return nil, nil
}

func (r *fakeOTARepository) ListDeploymentEvents(ctx context.Context, deploymentID string) ([]ota.DeploymentEvent, error) {
	return nil, nil
}

func (r *fakeOTARepository) Stats(ctx context.Context) (*ota.FleetStats, error) {
	return &ota.FleetStats{}, nil
}

func (r *fakeOTARepository) setStatus(deviceID string, id string, status string, progress *int, message string) (*ota.Deployment, error) {
	for i := range r.deployments {
		if r.deployments[i].DeviceID == deviceID && r.deployments[i].ID == id {
			r.deployments[i].Status = status
			if progress != nil {
				r.deployments[i].Progress = *progress
			}
			if message != "" {
				r.deployments[i].ResultMessage = &message
			}
			return &r.deployments[i], nil
		}
	}
	return nil, ota.ErrDeploymentNotFound
}

func isFakeActiveOTAStatus(status string) bool {
	switch status {
	case ota.StatusPending, ota.StatusAvailable, ota.StatusDownloading, ota.StatusFlashing, ota.StatusRebooting:
		return true
	default:
		return false
	}
}

var _ ota.Repository = (*fakeOTARepository)(nil)
var _ ota.ObjectStore = fakeObjectStore{}

