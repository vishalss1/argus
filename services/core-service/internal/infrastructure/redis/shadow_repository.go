package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/vishalss1/argus/core/internal/domain/shadow"
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

var updateShadowScript = goredis.NewScript(`
	local current = redis.call('GET', KEYS[1])
	local desired_json
	local reported_json
	local version = 0

	if current then
		local rec = cjson.decode(current)
		version = rec.version or 0
		if ARGV[1] == "desired" then
			desired_json = ARGV[2]
			if type(rec.reported) == "table" and next(rec.reported) == nil then
				reported_json = "{}"
			else
				reported_json = cjson.encode(rec.reported)
			end
		else
			reported_json = ARGV[2]
			if type(rec.desired) == "table" and next(rec.desired) == nil then
				desired_json = "{}"
			else
				desired_json = cjson.encode(rec.desired)
			end
		end
	else
		if ARGV[1] == "desired" then
			desired_json = ARGV[2]
			reported_json = "{}"
		else
			desired_json = "{}"
			reported_json = ARGV[2]
		end
	end

	version = version + 1
	local dev_id = ARGV[3]
	local updated_at = ARGV[4]

	local payload = string.format('{"device_id":%s,"desired":%s,"reported":%s,"version":%d,"updated_at":%s}',
		cjson.encode(dev_id),
		desired_json,
		reported_json,
		version,
		cjson.encode(updated_at)
	)

	redis.call('SET', KEYS[1], payload)
	return payload
`)

func (r *ShadowRepository) UpdateDesired(ctx context.Context, deviceID string, state json.RawMessage) (*shadow.Shadow, error) {
	return r.update(ctx, deviceID, "desired", state)
}

func (r *ShadowRepository) UpdateReported(ctx context.Context, deviceID string, state json.RawMessage) (*shadow.Shadow, error) {
	return r.update(ctx, deviceID, "reported", state)
}

func (r *ShadowRepository) update(ctx context.Context, deviceID string, field string, state json.RawMessage) (*shadow.Shadow, error) {
	state = ensureObject(state)
	if !json.Valid(state) {
		return nil, fmt.Errorf("invalid json state")
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	key := shadowKey(deviceID)
	res, err := updateShadowScript.Run(ctx, r.client, []string{key}, field, string(state), deviceID, now).Text()
	if err != nil {
		return nil, fmt.Errorf("update shadow: %w", err)
	}

	var record shadowRecord
	if err := json.Unmarshal([]byte(res), &record); err != nil {
		return nil, fmt.Errorf("decode shadow: %w", err)
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
