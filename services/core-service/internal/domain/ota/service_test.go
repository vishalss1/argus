package ota

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

type mockRepository struct {
	Repository // Embed to avoid implementing all methods
	artifacts  map[string]FirmwareArtifact
	deleted    []string
	created    []FirmwareArtifact
}

func (m *mockRepository) GetArtifact(ctx context.Context, id string) (*FirmwareArtifact, error) {
	artifact, ok := m.artifacts[id]
	if !ok {
		return nil, ErrFirmwareNotFound
	}
	return &artifact, nil
}

func (m *mockRepository) CreateArtifact(ctx context.Context, artifact FirmwareArtifact) (*FirmwareArtifact, error) {
	created := artifact
	m.created = append(m.created, artifact)
	if m.artifacts == nil {
		m.artifacts = make(map[string]FirmwareArtifact)
	}
	m.artifacts[artifact.ID] = artifact
	return &created, nil
}

func (m *mockRepository) DeleteArtifact(ctx context.Context, id string) error {
	if _, ok := m.artifacts[id]; !ok {
		return ErrFirmwareNotFound
	}
	delete(m.artifacts, id)
	m.deleted = append(m.deleted, id)
	return nil
}

type mockObjectStore struct {
	ObjectStore
	removed []string
	stored  map[string][]byte
	puts    int
	// putSizeBytes, when non-zero, limits how many bytes PutFirmware reads,
	// simulating a store that stops early.
	putSizeBytes int64
}

func (m *mockObjectStore) RemoveFirmware(ctx context.Context, objectKey string) error {
	m.removed = append(m.removed, objectKey)
	delete(m.stored, objectKey)
	return nil
}

// PutFirmware consumes the reader the service hands it, recording the bytes so
// tests can assert the artifact body and the version marker both saw the same
// stream. It deliberately ignores sizeBytes so the scanner sees real bytes.
func (m *mockObjectStore) PutFirmware(ctx context.Context, objectKey string, reader io.Reader, sizeBytes int64, contentType string) error {
	limit := sizeBytes
	if m.putSizeBytes > 0 {
		limit = m.putSizeBytes
	}
	data, err := io.ReadAll(io.LimitReader(reader, limit))
	if err != nil {
		return err
	}
	if m.stored == nil {
		m.stored = make(map[string][]byte)
	}
	m.stored[objectKey] = data
	m.puts++
	return nil
}

func TestDeleteFirmware(t *testing.T) {
	repo := &mockRepository{
		artifacts: map[string]FirmwareArtifact{
			"art-123": {
				ID:        "art-123",
				ObjectKey: "firmware/art-123/firmware.bin",
			},
		},
	}
	store := &mockObjectStore{}
	service := NewService(repo, store)

	// Test case 1: Successful deletion
	err := service.DeleteFirmware(context.Background(), "art-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != "art-123" {
		t.Errorf("expected repo to delete art-123, got: %v", repo.deleted)
	}

	if len(store.removed) != 1 || store.removed[0] != "firmware/art-123/firmware.bin" {
		t.Errorf("expected object store to remove firmware/art-123/firmware.bin, got: %v", store.removed)
	}

	// Test case 2: Deleting non-existent firmware
	err = service.DeleteFirmware(context.Background(), "non-existent")
	if !errors.Is(err, ErrFirmwareNotFound) {
		t.Errorf("expected ErrFirmwareNotFound, got: %v", err)
	}

	// Test case 3: Empty ID validation
	err = service.DeleteFirmware(context.Background(), "  ")
	if err == nil || err.Error() != "firmware id is required" {
		t.Errorf("expected 'firmware id is required' error, got: %v", err)
	}
}

// A successful upload derives its version from the binary rather than from any
// caller input, and the stored artifact records that derived value.
func TestUploadFirmwareDerivesVersionFromBinary(t *testing.T) {
	image := loadFirmwareFixture(t)
	repo := &mockRepository{}
	store := &mockObjectStore{}
	service := NewService(repo, store)

	artifact, err := service.UploadFirmware(context.Background(), UploadInput{
		Filename:  "fleet_firmware.bin",
		SizeBytes: int64(len(image)),
	}, bytes.NewReader(image))
	if err != nil {
		t.Fatalf("UploadFirmware returned error: %v", err)
	}

	if artifact.Version != "1.2.0" {
		t.Errorf("artifact version = %q, want %q", artifact.Version, "1.2.0")
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected exactly one artifact row, got %d", len(repo.created))
	}
	if repo.created[0].Version != "1.2.0" {
		t.Errorf("persisted version = %q, want %q", repo.created[0].Version, "1.2.0")
	}
	if artifact.ChecksumSHA256 == "" {
		t.Error("expected a checksum over the uploaded bytes")
	}

	// The whole image must reach the store, not just the bytes around the marker.
	for key, data := range store.stored {
		if !bytes.Equal(data, image) {
			t.Errorf("stored object %s has %d bytes, want the full %d-byte image", key, len(data), len(image))
		}
	}
}

// An image with no usable marker must be rejected and must not leave a stored
// object behind. Failing open here would let an artifact be published under a
// version that is not in the binary at all.
func TestUploadFirmwareRejectsBinaryWithoutVersionMarker(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{name: "no marker", body: []byte("\x7fELF opaque firmware image")},
		{name: "malformed version", body: []byte("ARGUSVER:\x00not-a-version\x00")},
		{name: "not semver", body: []byte("ARGUSVER:\x001.2.x\x00")},
		{name: "missing patch", body: []byte("ARGUSVER:\x001.2\x00")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{}
			store := &mockObjectStore{}
			service := NewService(repo, store)

			_, err := service.UploadFirmware(context.Background(), UploadInput{
				Filename:  "fleet_firmware.bin",
				SizeBytes: int64(len(tt.body)),
			}, bytes.NewReader(tt.body))
			if err == nil {
				t.Fatal("expected the upload to be rejected")
			}

			if len(repo.created) != 0 {
				t.Errorf("no artifact row should exist, got %d", len(repo.created))
			}
			if len(store.stored) != 0 {
				t.Errorf("no firmware object should remain, got %d", len(store.stored))
			}
			if len(store.removed) == 0 {
				t.Error("the rejected object should have been removed from the store")
			}
		})
	}
}

// A marker present but truncated by a short read must still fail closed rather
// than producing an empty version.
func TestUploadFirmwareRejectsTruncatedBinary(t *testing.T) {
	image := loadFirmwareFixture(t)
	repo := &mockRepository{}
	store := &mockObjectStore{putSizeBytes: 16} // stops well before the marker
	service := NewService(repo, store)

	_, err := service.UploadFirmware(context.Background(), UploadInput{
		Filename:  "fleet_firmware.bin",
		SizeBytes: int64(len(image)),
	}, bytes.NewReader(image))
	if err == nil {
		t.Fatal("expected the upload to be rejected when the marker is not read")
	}
	if len(repo.created) != 0 {
		t.Errorf("no artifact row should exist, got %d", len(repo.created))
	}
}

// Upload input validation is unchanged apart from the version field, which no
// longer exists.
func TestUploadFirmwareValidatesInput(t *testing.T) {
	tests := []struct {
		name    string
		input   UploadInput
		reader  io.Reader
		wantErr string
	}{
		{
			name:    "missing filename",
			input:   UploadInput{SizeBytes: 10},
			reader:  bytes.NewReader(make([]byte, 10)),
			wantErr: "filename is required",
		},
		{
			name:    "empty file",
			input:   UploadInput{Filename: "fw.bin", SizeBytes: 0},
			reader:  bytes.NewReader(nil),
			wantErr: "firmware file is required",
		},
		{
			name:    "nil reader",
			input:   UploadInput{Filename: "fw.bin", SizeBytes: 10},
			reader:  nil,
			wantErr: "firmware file is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{}
			store := &mockObjectStore{}
			service := NewService(repo, store)

			_, err := service.UploadFirmware(context.Background(), tt.input, tt.reader)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("expected %q, got %v", tt.wantErr, err)
			}
			if store.puts != 0 {
				t.Error("invalid input must not reach the object store")
			}
		})
	}
}
