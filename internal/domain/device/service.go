package device

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	repo               Repository
	publisher          EventPublisher
	provisioningConfig ProvisioningConfig
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		provisioningConfig: ProvisioningConfig{
			MQTTTelemetryPattern: "devices/+/telemetry",
			SamplingIntervalMS:   5000,
			HeartbeatIntervalMS:  30000,
		},
	}
}

type EventPublisher interface {
	PublishDeviceUpdate(ctx context.Context, entity Device)
	PublishDevicePresence(ctx context.Context, event PresenceEvent)
}

func (s *Service) SetEventPublisher(publisher EventPublisher) {
	s.publisher = publisher
}

func (s *Service) SetProvisioningConfig(config ProvisioningConfig) {
	if strings.TrimSpace(config.MQTTBrokerURL) != "" {
		s.provisioningConfig.MQTTBrokerURL = strings.TrimSpace(config.MQTTBrokerURL)
	}
	if strings.TrimSpace(config.MQTTTelemetryPattern) != "" {
		s.provisioningConfig.MQTTTelemetryPattern = strings.TrimSpace(config.MQTTTelemetryPattern)
	}
	if config.SamplingIntervalMS > 0 {
		s.provisioningConfig.SamplingIntervalMS = config.SamplingIntervalMS
	}
	if config.HeartbeatIntervalMS > 0 {
		s.provisioningConfig.HeartbeatIntervalMS = config.HeartbeatIntervalMS
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*Device, error) {
	id, err := newDeviceID()
	if err != nil {
		return nil, err
	}

	device := Device{
		ID:              id,
		Name:            strings.TrimSpace(input.Name),
		Type:            strings.TrimSpace(input.Type),
		FirmwareVersion: strings.TrimSpace(input.FirmwareVersion),
		Status:          strings.TrimSpace(input.Status),
		Metadata:        input.Metadata,
	}

	if device.Name == "" {
		return nil, errors.New("name is required")
	}
	if device.Type == "" {
		return nil, errors.New("type is required")
	}
	if device.Status == "" {
		device.Status = "offline"
	}
	if len(device.Metadata) == 0 {
		device.Metadata = json.RawMessage(`{}`)
	}
	if !json.Valid(device.Metadata) {
		return nil, errors.New("metadata must be valid JSON")
	}

	return s.repo.Create(ctx, device)
}

func (s *Service) List(ctx context.Context) ([]Device, error) {
	return s.repo.List(ctx)
}

func (s *Service) GetByID(ctx context.Context, id string) (*Device, error) {
	return s.repo.GetByID(ctx, strings.TrimSpace(id))
}

func (s *Service) Provision(ctx context.Context, input ProvisionInput) (*ProvisionResponse, error) {
	hardwareID := strings.TrimSpace(input.HardwareID)
	deviceType := strings.TrimSpace(input.DeviceType)
	firmwareVersion := strings.TrimSpace(input.FirmwareVersion)

	if hardwareID == "" {
		return nil, errors.New("hardware id is required")
	}
	if deviceType == "" {
		return nil, errors.New("device type is required")
	}

	if len(input.Capabilities) > 0 && !json.Valid(input.Capabilities) {
		return nil, errors.New("capabilities must be valid JSON")
	}

	existing, err := s.repo.GetByHardwareID(ctx, hardwareID)
	if err == nil {
		return s.provisionResponse(existing), nil
	}
	if !errors.Is(err, ErrDeviceNotFound) {
		return nil, err
	}

	metadata, err := provisioningMetadata(hardwareID, input.Capabilities)
	if err != nil {
		return nil, err
	}

	entity, err := s.Create(ctx, CreateInput{
		Name:            hardwareID,
		Type:            deviceType,
		FirmwareVersion: firmwareVersion,
		Status:          "offline",
		Metadata:        metadata,
	})
	if err != nil {
		existing, lookupErr := s.repo.GetByHardwareID(ctx, hardwareID)
		if lookupErr == nil {
			return s.provisionResponse(existing), nil
		}
		return nil, err
	}

	if s.publisher != nil {
		s.publisher.PublishDeviceUpdate(ctx, *entity)
	}

	return s.provisionResponse(entity), nil
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*Device, error) {
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, errors.New("name cannot be empty")
		}
		input.Name = &name
	}
	if input.Type != nil {
		deviceType := strings.TrimSpace(*input.Type)
		if deviceType == "" {
			return nil, errors.New("type cannot be empty")
		}
		input.Type = &deviceType
	}
	if input.FirmwareVersion != nil {
		firmwareVersion := strings.TrimSpace(*input.FirmwareVersion)
		input.FirmwareVersion = &firmwareVersion
	}
	if input.Status != nil {
		status := strings.TrimSpace(*input.Status)
		if status == "" {
			return nil, errors.New("status cannot be empty")
		}
		input.Status = &status
	}
	if input.Metadata != nil && !json.Valid(*input.Metadata) {
		return nil, errors.New("metadata must be valid JSON")
	}

	return s.repo.Update(ctx, strings.TrimSpace(id), input)
}

func (s *Service) RecordHeartbeat(ctx context.Context, id string, input HeartbeatInput) (*Device, error) {
	deviceID := strings.TrimSpace(id)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "online"
	}

	entity, err := s.repo.UpdateHeartbeat(ctx, deviceID, status)
	if err != nil {
		return nil, err
	}

	return entity, nil
}

func (s *Service) RecordPresence(ctx context.Context, id string, input PresenceInput) (*Device, error) {
	deviceID := strings.TrimSpace(id)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	status := PresenceStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	if status != PresenceOnline && status != PresenceOffline {
		return nil, errors.New("presence status must be online or offline")
	}

	timestamp := input.Timestamp.UTC()
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	entity, err := s.repo.UpdatePresence(ctx, deviceID, string(status), timestamp)
	if err != nil {
		return nil, err
	}
	if s.publisher != nil {
		s.publisher.PublishDevicePresence(ctx, PresenceEvent{
			Type:      "device_presence",
			DeviceID:  deviceID,
			Status:    string(status),
			Timestamp: timestamp.Format(time.RFC3339),
		})
		s.publisher.PublishDeviceUpdate(ctx, *entity)
	}

	return entity, nil
}

func (s *Service) MarkStaleOffline(ctx context.Context, timeout time.Duration) ([]Device, error) {
	if timeout <= 0 {
		return nil, errors.New("heartbeat timeout must be positive")
	}

	devices, err := s.repo.MarkStaleOffline(ctx, timeout)
	if err != nil {
		return nil, err
	}
	if s.publisher != nil {
		for _, entity := range devices {
			timestamp := entity.UpdatedAt.UTC()
			s.publisher.PublishDevicePresence(ctx, PresenceEvent{
				Type:      "device_presence",
				DeviceID:  entity.ID,
				Status:    string(PresenceOffline),
				Timestamp: timestamp.Format(time.RFC3339),
			})
			s.publisher.PublishDeviceUpdate(ctx, entity)
		}
	}

	return devices, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, strings.TrimSpace(id))
}

func (s *Service) provisionResponse(entity *Device) *ProvisionResponse {
	deviceID := entity.ID
	return &ProvisionResponse{
		DeviceUUID:          deviceID,
		MQTTBrokerURL:       s.provisioningConfig.MQTTBrokerURL,
		MQTTTelemetryTopic:  deviceTopic(s.provisioningConfig.MQTTTelemetryPattern, deviceID),
		MQTTCommandTopic:    fmt.Sprintf("devices/%s/commands", deviceID),
		SamplingIntervalMS:  s.provisioningConfig.SamplingIntervalMS,
		HeartbeatIntervalMS: s.provisioningConfig.HeartbeatIntervalMS,
	}
}

func provisioningMetadata(hardwareID string, capabilities json.RawMessage) (json.RawMessage, error) {
	metadata := map[string]any{
		"hardware_id": hardwareID,
	}
	if len(capabilities) > 0 {
		var value any
		if err := json.Unmarshal(capabilities, &value); err != nil {
			return nil, fmt.Errorf("decode capabilities: %w", err)
		}
		metadata["capabilities"] = value
	}

	payload, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("encode provisioning metadata: %w", err)
	}

	return payload, nil
}

func deviceTopic(pattern string, deviceID string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Sprintf("devices/%s/telemetry", deviceID)
	}
	if strings.Contains(pattern, "+") {
		return strings.Replace(pattern, "+", deviceID, 1)
	}
	return pattern
}

func newDeviceID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate device id: %w", err)
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	encoded := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}
