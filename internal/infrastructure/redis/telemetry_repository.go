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

func (r *TelemetryRepository) SetLatestPipeline(ctx context.Context, pipe goredis.Pipeliner, deviceID string, t telemetry.Telemetry) error {
	key := fmt.Sprintf("device:%s:latest", deviceID)

	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal telemetry: %w", err)
	}

	pipe.Set(ctx, key, data, r.ttl)
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
