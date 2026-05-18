package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/vishalss1/argus/internal/domain/shadow"
)

const shadowKeyPrefix = "argus:shadow:"

type ShadowRepository struct {
	client *goredis.Client
}

type shadowRecord struct {
	DeviceID  string          `json:"device_id"`
	Desired   json.RawMessage `json:"desired"`
	Reported  json.RawMessage `json:"reported"`
	Version   int64           `json:"version"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func NewShadowRepository(client *Client) *ShadowRepository {
	return &ShadowRepository{client: client.client}
}

func (r *ShadowRepository) Get(ctx context.Context, deviceID string) (*shadow.Shadow, error) {
	record, err := r.getRecord(ctx, deviceID)
	if err != nil {
		return nil, err
	}

	return record.toDomain(), nil
}

func (r *ShadowRepository) UpdateDesired(ctx context.Context, deviceID string, state json.RawMessage) (*shadow.Shadow, error) {
	return r.update(ctx, deviceID, func(record *shadowRecord) {
		record.Desired = state
	})
}

func (r *ShadowRepository) UpdateReported(ctx context.Context, deviceID string, state json.RawMessage) (*shadow.Shadow, error) {
	return r.update(ctx, deviceID, func(record *shadowRecord) {
		record.Reported = state
	})
}

func (r *ShadowRepository) update(ctx context.Context, deviceID string, apply func(*shadowRecord)) (*shadow.Shadow, error) {
	record, err := r.getRecord(ctx, deviceID)
	if errors.Is(err, shadow.ErrShadowNotFound) {
		record = &shadowRecord{
			DeviceID:  deviceID,
			Desired:   json.RawMessage(`{}`),
			Reported:  json.RawMessage(`{}`),
			UpdatedAt: time.Now().UTC(),
		}
	} else if err != nil {
		return nil, err
	}

	apply(record)
	record.Version++
	record.UpdatedAt = time.Now().UTC()

	payload, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("encode shadow: %w", err)
	}
	if err := r.client.Set(ctx, shadowKey(deviceID), payload, 0).Err(); err != nil {
		return nil, fmt.Errorf("set shadow: %w", err)
	}

	return record.toDomain(), nil
}

func (r *ShadowRepository) getRecord(ctx context.Context, deviceID string) (*shadowRecord, error) {
	value, err := r.client.Get(ctx, shadowKey(deviceID)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, shadow.ErrShadowNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get shadow: %w", err)
	}

	var record shadowRecord
	if err := json.Unmarshal(value, &record); err != nil {
		return nil, fmt.Errorf("decode shadow: %w", err)
	}

	return &record, nil
}

func shadowKey(deviceID string) string {
	return shadowKeyPrefix + deviceID
}

func (s shadowRecord) toDomain() *shadow.Shadow {
	desired := ensureObject(s.Desired)
	reported := ensureObject(s.Reported)

	return &shadow.Shadow{
		DeviceID:  s.DeviceID,
		Desired:   desired,
		Reported:  reported,
		Drift:     !jsonEqual(desired, reported),
		Version:   s.Version,
		UpdatedAt: s.UpdatedAt,
	}
}

func ensureObject(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}

	return value
}

func jsonEqual(left, right json.RawMessage) bool {
	var leftValue any
	var rightValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		return false
	}
	if err := json.Unmarshal(right, &rightValue); err != nil {
		return false
	}

	return reflect.DeepEqual(leftValue, rightValue)
}
