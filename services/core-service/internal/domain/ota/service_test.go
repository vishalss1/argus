package ota

import (
	"context"
	"errors"
	"testing"
)

type mockRepository struct {
	Repository // Embed to avoid implementing all methods
	artifacts  map[string]FirmwareArtifact
	deleted    []string
}

func (m *mockRepository) GetArtifact(ctx context.Context, id string) (*FirmwareArtifact, error) {
	artifact, ok := m.artifacts[id]
	if !ok {
		return nil, ErrFirmwareNotFound
	}
	return &artifact, nil
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
}

func (m *mockObjectStore) RemoveFirmware(ctx context.Context, objectKey string) error {
	m.removed = append(m.removed, objectKey)
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
