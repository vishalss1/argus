package app

import (
	"context"

	devicedomain "github.com/vishalss1/argus/internal/domain/device"
	telemetrydomain "github.com/vishalss1/argus/internal/domain/telemetry"
	transportws "github.com/vishalss1/argus/internal/transport/websocket"
)

type realtimePublisher struct {
	hub *transportws.Hub
}

func (p *realtimePublisher) PublishDeviceUpdate(ctx context.Context, entity devicedomain.Device) {
	p.hub.Broadcast("device_update", entity)
}

func (p *realtimePublisher) PublishDevicePresence(ctx context.Context, event devicedomain.PresenceEvent) {
	p.hub.BroadcastPayload(event)
}

func (p *realtimePublisher) PublishTelemetry(ctx context.Context, entity telemetrydomain.Telemetry) {
	p.hub.Broadcast("telemetry", entity)
}
