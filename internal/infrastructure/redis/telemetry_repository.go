package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type TelemetryRepository struct {
	client *Client
	ttl    time.Duration
}

func NewTelemetryRepository(client *Client, ttl time.Duration) *TelemetryRepository {
	return &TelemetryRepository{
		client: client,
		ttl:    ttl,
	}
}

func (r *TelemetryRepository) SetLatest(ctx context.Context, deviceID string, t telemetry.Telemetry) error {
	key := fmt.Sprintf("device:%s:latest", deviceID)
	
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal telemetry: %w", err)
	}

	if err := r.client.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}

func (r *TelemetryRepository) SessionTrackPipeline(ctx context.Context, pipe goredis.Pipeliner, sessionID, deviceID string, nowUnix int64, latestPayload []byte) error {
	devsKey := fmt.Sprintf("session:%s:devices", sessionID)
	stateKey := fmt.Sprintf("session:%s:device:%s:state", sessionID, deviceID)
	latestKey := fmt.Sprintf("device:%s:latest", deviceID)

	// Since we are optimizing for readability and simplicity at 1K devices,
	// we will just use native pipeline commands rather than a complex Lua script.
	// The stopped check is handled asynchronously by the drain window in manager.go.
	pipe.Set(ctx, latestKey, latestPayload, r.ttl)
	pipe.HSet(ctx, stateKey, "last_seen", nowUnix)
	pipe.HIncrBy(ctx, stateKey, "sample_count", 1)
	
	// Add to session devices. If it's the first time (SAdd returns 1), we could initialize more, 
	// but pipeline commands don't return until Exec. We will rely on HSetNX to lazily init.
	pipe.SAdd(ctx, devsKey, deviceID)
	pipe.HSetNX(ctx, stateKey, "first_seen", nowUnix)
	pipe.HSetNX(ctx, stateKey, "warning_incidents_count", 0)
	pipe.HSetNX(ctx, stateKey, "critical_incidents_count", 0)
	pipe.HSetNX(ctx, stateKey, "worst_severity", "healthy")

	return nil
}

func (r *TelemetryRepository) GetLatest(ctx context.Context, deviceID string) (*telemetry.Telemetry, error) {
	key := fmt.Sprintf("device:%s:latest", deviceID)
	
	data, err := r.client.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var t telemetry.Telemetry
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("unmarshal telemetry: %w", err)
	}

	return &t, nil
}

func (r *TelemetryRepository) GetAllLatest(ctx context.Context) ([]telemetry.Telemetry, error) {
	var cursor uint64
	var allTelemetry []telemetry.Telemetry

	for {
		keys, nextCursor, err := r.client.client.Scan(ctx, cursor, "device:*:latest", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("redis scan: %w", err)
		}

		for _, key := range keys {
			data, err := r.client.client.Get(ctx, key).Bytes()
			if err != nil {
				continue // key might have expired
			}
			var t telemetry.Telemetry
			if err := json.Unmarshal(data, &t); err == nil {
				allTelemetry = append(allTelemetry, t)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return allTelemetry, nil
}
