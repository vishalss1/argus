package grpc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/vishalss1/argus/telemetry/internal/domain/device"
	pb "github.com/vishalss1/argus/shared/proto/core"
	redisinfra "github.com/vishalss1/argus/telemetry/internal/infrastructure/redis"
)

type cacheEntry struct {
	dev       *device.Device
	expiresAt time.Time
}

type DeviceRepository struct {
	coreClient  *CoreClient
	redisClient *redisinfra.Client
	cache       map[string]cacheEntry
	cacheMu     sync.RWMutex
}

func NewDeviceRepository(coreClient *CoreClient, redisClient *redisinfra.Client) *DeviceRepository {
	return &DeviceRepository{
		coreClient:  coreClient,
		redisClient: redisClient,
		cache:       make(map[string]cacheEntry),
	}
}

func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "circuit breaker") ||
		strings.Contains(errStr, "open") ||
		strings.Contains(errStr, "unavailable") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "desc = transport") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "Service Unavailable")
}

func (r *DeviceRepository) GetByID(ctx context.Context, id string) (*device.Device, error) {
	// 1. Check cache first
	r.cacheMu.RLock()
	entry, found := r.cache[id]
	r.cacheMu.RUnlock()

	if found && time.Now().Before(entry.expiresAt) {
		return entry.dev, nil
	}

	// 2. Cache miss: fallback to gRPC with bounded retries (3 attempts)
	var resp *pb.DeviceContextResponse
	var err error
	backoff := 500 * time.Millisecond

	for i := 0; i < 3; i++ {
		resp, err = r.coreClient.Client().GetDeviceContext(ctx, &pb.GetDeviceContextRequest{
			DeviceId: id,
		})
		if err == nil {
			break
		}

		if !isTransientError(err) {
			return nil, fmt.Errorf("core grpc GetDeviceContext: %w", err)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}

	if err != nil {
		return nil, fmt.Errorf("core grpc GetDeviceContext (after 3 retries): %w", err)
	}

	var lastSeen *time.Time
	if resp.LastSeen != nil {
		t := resp.LastSeen.AsTime()
		lastSeen = &t
	}

	var workspaceID *string
	if resp.WorkspaceId != "" {
		workspaceID = &resp.WorkspaceId
	}

	deviceEntity := &device.Device{
		ID:              resp.DeviceId,
		Name:            resp.Name,
		Type:            resp.Type,
		FirmwareVersion: resp.FirmwareVersion,
		Status:          resp.Status,
		WorkspaceID:     workspaceID,
		LastSeen:        lastSeen,
		CreatedAt:       resp.CreatedAt.AsTime(),
		UpdatedAt:       resp.UpdatedAt.AsTime(),
	}

	// 3. Cache the response with 5 minutes TTL
	r.cacheMu.Lock()
	r.cache[id] = cacheEntry{
		dev:       deviceEntity,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	r.cacheMu.Unlock()

	return deviceEntity, nil
}

func (r *DeviceRepository) Search(ctx context.Context, terms []string, limit int) ([]device.Device, error) {
	if len(terms) == 0 {
		return nil, nil
	}

	rdb := r.redisClient.Client()
	var cursor uint64
	var keys []string
	for {
		scanKeys, nextCursor, err := rdb.Scan(ctx, cursor, "device:*:latest", 100).Result()
		if err == nil {
			keys = append(keys, scanKeys...)
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	var matchedDevices []device.Device
	for _, key := range keys {
		parts := strings.Split(key, ":")
		if len(parts) < 2 {
			continue
		}
		deviceID := parts[1]

		dev, err := r.GetByID(ctx, deviceID)
		if err != nil {
			continue
		}

		matches := false
		for _, term := range terms {
			termLower := strings.ToLower(term)
			if strings.Contains(strings.ToLower(dev.ID), termLower) ||
				strings.Contains(strings.ToLower(dev.Name), termLower) ||
				strings.Contains(strings.ToLower(dev.Type), termLower) {
				matches = true
				break
			}
		}

		if matches {
			matchedDevices = append(matchedDevices, *dev)
			if len(matchedDevices) >= limit {
				break
			}
		}
	}

	return matchedDevices, nil
}

func (r *DeviceRepository) GetWorkspaceDevices(ctx context.Context, workspaceID string) ([]string, error) {
	if workspaceID == "" {
		return nil, nil
	}

	var resp *pb.GetWorkspaceDevicesResponse
	var err error
	backoff := 500 * time.Millisecond

	for i := 0; i < 3; i++ {
		resp, err = r.coreClient.Client().GetWorkspaceDevices(ctx, &pb.GetWorkspaceDevicesRequest{
			WorkspaceId: workspaceID,
		})
		if err == nil {
			break
		}

		if !isTransientError(err) {
			return nil, fmt.Errorf("core grpc GetWorkspaceDevices: %w", err)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}

	if err != nil {
		return nil, fmt.Errorf("core grpc GetWorkspaceDevices (after 3 retries): %w", err)
	}

	return resp.DeviceIds, nil
}

