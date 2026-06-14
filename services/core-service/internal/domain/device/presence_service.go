package device

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/vishalss1/argus/shared/common"
)

type PresenceService struct {
	deviceService *Service
	mu            sync.RWMutex
	cache         map[string]PresenceState
}

func NewPresenceService(deviceService *Service) *PresenceService {
	return &PresenceService{
		deviceService: deviceService,
		cache:         make(map[string]PresenceState),
	}
}

func (s *PresenceService) RecordState(ctx context.Context, deviceID string, input PresenceInput, retained bool) (*Device, error) {
	entity, err := s.deviceService.RecordPresence(ctx, deviceID, input)
	if err != nil {
		return nil, err
	}

	online := input.Status == PresenceOnline
	timestamp := input.Timestamp.UTC()
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	s.mu.Lock()
	current := s.cache[deviceID]
	state := PresenceState{
		DeviceID:      deviceID,
		Online:        online,
		Status:        input.Status,
		LastSeen:      timestamp,
		ConnectedAt:   current.ConnectedAt,
		LastHeartbeat: current.LastHeartbeat,
		Metadata:      input.Metadata,
	}
	if online {
		connectedAt := timestamp
		state.ConnectedAt = &connectedAt
	}
	s.cache[deviceID] = state

	if online && !current.Online {
		common.ConnectedDevices.Inc()
	} else if !online && current.Online {
		common.ConnectedDevices.Dec()
	}

	s.mu.Unlock()

	source := "live"
	if retained {
		source = "retained"
	}
	log.Printf("device presence %s transition: device_id=%s status=%s timestamp=%s", source, deviceID, input.Status, timestamp.Format(time.RFC3339))

	return entity, nil
}

func (s *PresenceService) RecordHeartbeat(deviceID string, timestamp time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.cache[deviceID]
	state.DeviceID = deviceID
	state.LastHeartbeat = &timestamp
	s.cache[deviceID] = state
}

func (s *PresenceService) GetDeviceByIDOrHardwareID(ctx context.Context, idOrHardwareID string) (*Device, error) {
	// Try UUID first
	device, err := s.deviceService.GetByID(ctx, idOrHardwareID)
	if err == nil {
		return device, nil
	}

	// Try Hardware ID
	return s.deviceService.repo.GetByHardwareID(ctx, idOrHardwareID)
}

func (s *PresenceService) MarkStaleOffline(ctx context.Context, timeout time.Duration) ([]Device, error) {
	devices, err := s.deviceService.MarkStaleOffline(ctx, timeout)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, d := range devices {
		state := s.cache[d.ID]
		wasOnline := state.Online
		state.Online = false
		state.Status = PresenceOffline
		s.cache[d.ID] = state
		if wasOnline {
			common.ConnectedDevices.Dec()
		}
	}

	return devices, nil
}

func (s *PresenceService) Snapshot() map[string]PresenceState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := make(map[string]PresenceState, len(s.cache))
	for key, value := range s.cache {
		copy[key] = value
	}
	return copy
}

