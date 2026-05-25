package device

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type fakeRepository struct {
	byHardwareID map[string]*Device
	created      *Device
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{byHardwareID: make(map[string]*Device)}
}

func (r *fakeRepository) Create(ctx context.Context, entity Device) (*Device, error) {
	now := time.Now().UTC()
	entity.CreatedAt = now
	entity.UpdatedAt = now
	r.created = &entity

	var metadata struct {
		HardwareID string `json:"hardware_id"`
	}
	if err := json.Unmarshal(entity.Metadata, &metadata); err == nil && metadata.HardwareID != "" {
		r.byHardwareID[metadata.HardwareID] = &entity
	}

	return &entity, nil
}

func (r *fakeRepository) List(ctx context.Context) ([]Device, error) {
	return nil, nil
}

func (r *fakeRepository) GetByID(ctx context.Context, id string) (*Device, error) {
	return nil, ErrDeviceNotFound
}

func (r *fakeRepository) GetByHardwareID(ctx context.Context, hardwareID string) (*Device, error) {
	entity, ok := r.byHardwareID[hardwareID]
	if !ok {
		return nil, ErrDeviceNotFound
	}
	return entity, nil
}

func (r *fakeRepository) Update(ctx context.Context, id string, input UpdateInput) (*Device, error) {
	return nil, ErrDeviceNotFound
}

func (r *fakeRepository) UpdateHeartbeat(ctx context.Context, id string, status string) (*Device, error) {
	return nil, ErrDeviceNotFound
}

func (r *fakeRepository) MarkStaleOffline(ctx context.Context, timeout time.Duration) ([]Device, error) {
	return nil, nil
}

func (r *fakeRepository) Delete(ctx context.Context, id string) error {
	return nil
}

func TestProvisionCreatesDeviceWithHardwareMetadata(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(repo)
	service.SetProvisioningConfig(ProvisioningConfig{
		MQTTBrokerURL:        "tcp://broker:1883",
		MQTTTelemetryPattern: "argus/devices/+/telemetry",
	})

	response, err := service.Provision(context.Background(), ProvisionInput{
		HardwareID:      "mac-1",
		DeviceType:      "esp32",
		FirmwareVersion: "1.0.0",
		Capabilities:    json.RawMessage(`{"ota":true}`),
	})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	if response.DeviceUUID == "" {
		t.Fatal("expected device uuid")
	}
	if response.MQTTBrokerURL != "tcp://broker:1883" {
		t.Fatalf("unexpected broker url: %s", response.MQTTBrokerURL)
	}
	if response.MQTTTelemetryTopic != "argus/devices/"+response.DeviceUUID+"/telemetry" {
		t.Fatalf("unexpected telemetry topic: %s", response.MQTTTelemetryTopic)
	}
	if response.MQTTCommandTopic != "argus/devices/"+response.DeviceUUID+"/commands" {
		t.Fatalf("unexpected command topic: %s", response.MQTTCommandTopic)
	}

	var metadata map[string]any
	if err := json.Unmarshal(repo.created.Metadata, &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if metadata["hardware_id"] != "mac-1" {
		t.Fatalf("expected hardware_id metadata, got %#v", metadata)
	}
	if _, ok := metadata["capabilities"].(map[string]any); !ok {
		t.Fatalf("expected capabilities metadata, got %#v", metadata)
	}
}

func TestProvisionReturnsExistingDeviceForHardwareID(t *testing.T) {
	repo := newFakeRepository()
	existing := &Device{
		ID:              "device-1",
		Name:            "mac-1",
		Type:            "esp32",
		FirmwareVersion: "1.0.0",
		Status:          "offline",
		Metadata:        json.RawMessage(`{"hardware_id":"mac-1"}`),
	}
	repo.byHardwareID["mac-1"] = existing
	service := NewService(repo)

	response, err := service.Provision(context.Background(), ProvisionInput{
		HardwareID: "mac-1",
		DeviceType: "esp32",
	})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	if response.DeviceUUID != existing.ID {
		t.Fatalf("expected existing device id %q, got %q", existing.ID, response.DeviceUUID)
	}
	if repo.created != nil {
		t.Fatal("expected no new device to be created")
	}
}

func TestProvisionValidatesInput(t *testing.T) {
	service := NewService(newFakeRepository())

	_, err := service.Provision(context.Background(), ProvisionInput{DeviceType: "esp32"})
	if err == nil || err.Error() != "hardware id is required" {
		t.Fatalf("expected hardware id validation error, got %v", err)
	}

	_, err = service.Provision(context.Background(), ProvisionInput{HardwareID: "mac-1"})
	if err == nil || err.Error() != "device type is required" {
		t.Fatalf("expected device type validation error, got %v", err)
	}
}
