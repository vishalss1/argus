package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type DeviceWorkspaceCache interface {
	SetDeviceWorkspace(ctx context.Context, deviceID string, workspaceID string) error
	DeleteDeviceWorkspace(ctx context.Context, deviceID string) error
}

type Service struct {
	repo  Repository
	cache DeviceWorkspaceCache
}

func NewService(repo Repository, cache DeviceWorkspaceCache) *Service {
	return &Service{
		repo:  repo,
		cache: cache,
	}
}

func (s *Service) Create(ctx context.Context, name string, description string) (*Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("workspace name is required")
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}

	w := Workspace{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().UTC(),
	}

	return s.repo.Create(ctx, w)
}

func (s *Service) Get(ctx context.Context, id string) (*Workspace, error) {
	if id == "" {
		return nil, errors.New("workspace id is required")
	}
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]Workspace, error) {
	return s.repo.List(ctx)
}

func (s *Service) Update(ctx context.Context, id string, name string, description string) (*Workspace, error) {
	if id == "" {
		return nil, errors.New("workspace id is required")
	}
	return s.repo.Update(ctx, id, name, description)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("workspace id is required")
	}
	return s.repo.Delete(ctx, id)
}

func (s *Service) AssignDevice(ctx context.Context, workspaceID string, deviceID string) error {
	if workspaceID == "" || deviceID == "" {
		return errors.New("workspace id and device id are required")
	}
	err := s.repo.AssignDevice(ctx, workspaceID, deviceID)
	if err != nil {
		return err
	}
	if s.cache != nil {
		_ = s.cache.SetDeviceWorkspace(ctx, deviceID, workspaceID)
	}
	return nil
}

func (s *Service) UnassignDevice(ctx context.Context, workspaceID string, deviceID string) error {
	if workspaceID == "" || deviceID == "" {
		return errors.New("workspace id and device id are required")
	}
	err := s.repo.UnassignDevice(ctx, workspaceID, deviceID)
	if err != nil {
		return err
	}
	if s.cache != nil {
		_ = s.cache.DeleteDeviceWorkspace(ctx, deviceID)
	}
	return nil
}

func (s *Service) ListDevices(ctx context.Context, workspaceID string) ([]DeviceSummary, error) {
	if workspaceID == "" {
		return nil, errors.New("workspace id is required")
	}
	return s.repo.ListDevices(ctx, workspaceID)
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
