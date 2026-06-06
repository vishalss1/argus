package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/vishalss1/argus/internal/ai/memory"
	commanddomain "github.com/vishalss1/argus/internal/domain/command"
	devicedomain "github.com/vishalss1/argus/internal/domain/device"
	eventdomain "github.com/vishalss1/argus/internal/domain/event"
	otadomain "github.com/vishalss1/argus/internal/domain/ota"
	telemetrydomain "github.com/vishalss1/argus/internal/domain/telemetry"
	"github.com/vishalss1/argus/internal/infrastructure/redis"
	transportws "github.com/vishalss1/argus/internal/transport/websocket"
)

type realtimePublisher struct {
	hub         *transportws.Hub
	eventRepo   eventdomain.Repository
	embedSvc    *memory.EmbeddingService
	redisClient *redis.Client
	deviceRepo  devicedomain.Repository
}

func (p *realtimePublisher) PublishDeviceUpdate(ctx context.Context, entity devicedomain.Device) {
	p.hub.Broadcast("device_update", entity)
}

func (p *realtimePublisher) PublishDevicePresence(ctx context.Context, event devicedomain.PresenceEvent) {
	p.hub.BroadcastPayload(event)

	// Connectivity Failure tracking: when device goes offline
	if event.Status == "offline" && p.eventRepo != nil {
		if p.deviceRepo != nil {
			if _, err := p.deviceRepo.GetByID(ctx, event.DeviceID); err != nil {
				log.Printf("[REALTIME PUBLISHER] Unknown device %s, auto-provisioning...", event.DeviceID)
				autoDev := devicedomain.Device{
					ID:              event.DeviceID,
					Name:            fmt.Sprintf("Auto-Provisioned Device %s", event.DeviceID),
					Type:            "unknown",
					FirmwareVersion: "unknown",
					Status:          "offline",
					Metadata:        json.RawMessage(`{}`),
				}
				if _, err := p.deviceRepo.Create(ctx, autoDev); err != nil {
					log.Printf("[REALTIME PUBLISHER] Failed to auto-provision device %s: %v", event.DeviceID, err)
					log.Printf("[REALTIME PUBLISHER] Skipping offline event logging for unknown device %s: %v", event.DeviceID, err)
					return
				}
			}
		}

		cooldownKey := fmt.Sprintf("device_offline_cooldown:%s", event.DeviceID)
		if p.redisClient != nil {
			allowed := p.redisClient.Client().SetNX(ctx, cooldownKey, "1", 15*time.Minute).Val()
			if !allowed {
				return // Deduplicate flapping events
			}
		}

		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		newEvent := eventdomain.Event{
			ID:              uuid.New().String(),
			DeviceID:        event.DeviceID,
			Type:            "connectivity_failure",
			Severity:        eventdomain.SeverityWarning,
			Title:           "Device Offline",
			Summary:         fmt.Sprintf("Device %s went offline at %s", event.DeviceID, event.Timestamp),
			Source:          "presence_monitor",
			ConfidenceScore: 1.0,
			Metadata:        json.RawMessage(`{}`),
			CreatedAt:       time.Now().UTC(),
		}

		created, err := p.eventRepo.Create(bgCtx, newEvent)
		if err != nil {
			log.Printf("[CONNECTIVITY FAILURE] Failed to log offline event for device %s: %v", event.DeviceID, err)
			return
		}

		if p.embedSvc != nil {
			p.embedSvc.EnqueueEvent(*created)
		}
	}
}

func (p *realtimePublisher) PublishTelemetry(ctx context.Context, entity telemetrydomain.Telemetry) {
	p.hub.Broadcast("telemetry", entity)
}

func (p *realtimePublisher) PublishCommandUpdate(ctx context.Context, entity commanddomain.Command) {
	p.hub.Broadcast("command_update", entity)
}

func (p *realtimePublisher) PublishOTAEvent(ctx context.Context, eventType string, deployment otadomain.Deployment) {
	p.hub.Broadcast(eventType, deployment)
}

