package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/vishalss1/argus/telemetry/internal/domain/device"
	"github.com/vishalss1/argus/telemetry/internal/domain/fleet"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"github.com/vishalss1/argus/telemetry/internal/domain/severity"
)

type DeviceService struct {
	client *Client
}

func NewDeviceService(client *Client) *DeviceService {
	return &DeviceService{client: client}
}

func (s *DeviceService) GetSnapshot(ctx context.Context, deviceID string) (*device.Snapshot, error) {
	tracer := otel.Tracer("telemetry-service")
	ctx, span := tracer.Start(ctx, "device_service.GetSnapshot")
	span.SetAttributes(attribute.String("device_id", deviceID))
	defer span.End()

	rdb := s.client.Client()
	wsKey := fmt.Sprintf("device:%s:workspace", deviceID)
	workspaceID, _ := rdb.Get(ctx, wsKey).Result()

	var sessionID string
	if workspaceID != "" {
		sessionKey := fmt.Sprintf("workspace:%s:active_session", workspaceID)
		sessionID, _ = rdb.Get(ctx, sessionKey).Result()
	}

	devStateKey := fmt.Sprintf("session:%s:device:%s:state", sessionID, deviceID)
	stateMap, _ := rdb.HGetAll(ctx, devStateKey).Result()

	deviceStatus := "offline"
	var lastSeen time.Time
	if len(stateMap) > 0 {
		deviceStatus = "online"
		if lUnix, err := strconv.ParseInt(stateMap["last_seen"], 10, 64); err == nil {
			lastSeen = time.Unix(lUnix, 0).UTC()
		}
	}

	latestKey := fmt.Sprintf("device:%s:latest", deviceID)
	latestJSON, _ := rdb.Get(ctx, latestKey).Result()
	latestMetricsMap := make(map[string]float64)

	if latestJSON != "" {
		var t struct {
			Metrics json.RawMessage `json:"metrics"`
		}
		if json.Unmarshal([]byte(latestJSON), &t) == nil {
			var vals map[string]interface{}
			if json.Unmarshal(t.Metrics, &vals) == nil {
				for k, v := range vals {
					if f, ok := v.(float64); ok {
						latestMetricsMap[k] = f
					}
				}
			}
		}
	}

	var activeIncidentTypes []string
	var totalIncidents int = 0
	if sessionID != "" {
		deviceIncidentsSetKey := fmt.Sprintf("session:%s:device:%s:incidents", sessionID, deviceID)
		targetKeys, _ := rdb.SMembers(ctx, deviceIncidentsSetKey).Result()
		totalIncidents = len(targetKeys)
		if len(targetKeys) > 0 {
			vals, err := rdb.MGet(ctx, targetKeys...).Result()
			if err == nil {
				for _, v := range vals {
					if vStr, ok := v.(string); ok && vStr != "" {
						var inc struct {
							IncidentType string `json:"incident_type"`
						}
						if json.Unmarshal([]byte(vStr), &inc) == nil {
							activeIncidentTypes = append(activeIncidentTypes, inc.IncidentType)
						}
					}
				}
			}
		}
	}

	return &device.Snapshot{
		DeviceID:               deviceID,
		Status:                 deviceStatus,
		LastSeen:               lastSeen,
		LatestMetrics:          latestMetricsMap,
		ActiveIncidentTypes:    activeIncidentTypes,
		TotalIncidentsRecorded: totalIncidents,
	}, nil
}

func (s *DeviceService) GetState(ctx context.Context, deviceID string) (*device.State, error) {
	tracer := otel.Tracer("telemetry-service")
	ctx, span := tracer.Start(ctx, "device_service.GetState")
	span.SetAttributes(attribute.String("device_id", deviceID))
	defer span.End()

	rdb := s.client.Client()
	wsKey := fmt.Sprintf("device:%s:workspace", deviceID)
	workspaceID, _ := rdb.Get(ctx, wsKey).Result()

	var sessionID string
	if workspaceID != "" {
		sessionKey := fmt.Sprintf("workspace:%s:active_session", workspaceID)
		sessionID, _ = rdb.Get(ctx, sessionKey).Result()
	}

	devStateKey := fmt.Sprintf("session:%s:device:%s:state", sessionID, deviceID)
	stateMap, _ := rdb.HGetAll(ctx, devStateKey).Result()

	deviceStatus := "offline"
	var lastSeen time.Time
	if len(stateMap) > 0 {
		deviceStatus = "online"
		if lUnix, err := strconv.ParseInt(stateMap["last_seen"], 10, 64); err == nil {
			lastSeen = time.Unix(lUnix, 0).UTC()
		}
	}

	return &device.State{
		Status:   deviceStatus,
		LastSeen: lastSeen,
	}, nil
}

func (s *DeviceService) GetIncidents(ctx context.Context, deviceID string, limit int) ([]fleet.IncidentBrief, error) {
	tracer := otel.Tracer("telemetry-service")
	ctx, span := tracer.Start(ctx, "device_service.GetIncidents")
	span.SetAttributes(attribute.String("device_id", deviceID))
	defer span.End()

	rdb := s.client.Client()
	wsKey := fmt.Sprintf("device:%s:workspace", deviceID)
	workspaceID, _ := rdb.Get(ctx, wsKey).Result()

	var sessionID string
	if workspaceID != "" {
		sessionKey := fmt.Sprintf("workspace:%s:active_session", workspaceID)
		sessionID, _ = rdb.Get(ctx, sessionKey).Result()
	}

	var incidents []fleet.IncidentBrief
	if sessionID != "" {
		deviceIncidentsSetKey := fmt.Sprintf("session:%s:device:%s:incidents", sessionID, deviceID)
		targetKeys, _ := rdb.SMembers(ctx, deviceIncidentsSetKey).Result()
		if len(targetKeys) > 0 {
			vals, err := rdb.MGet(ctx, targetKeys...).Result()
			if err == nil {
				for _, v := range vals {
					if vStr, ok := v.(string); ok && vStr != "" {
						var inc struct {
							DeviceID     string `json:"device_id"`
							Metric       string `json:"metric"`
							IncidentType string `json:"incident_type"`
							Severity     string `json:"severity"`
						}
						if json.Unmarshal([]byte(vStr), &inc) == nil {
							incidents = append(incidents, fleet.IncidentBrief{
								DeviceID:     inc.DeviceID,
								Metric:       inc.Metric,
								IncidentType: inc.IncidentType,
								Severity:     severity.Parse(inc.Severity),
							})
						}
					}
				}
			}
		}
	}
	return incidents, nil
}

func (s *DeviceService) GetMetrics(ctx context.Context, deviceID string) (*device.Metrics, error) {
	tracer := otel.Tracer("telemetry-service")
	ctx, span := tracer.Start(ctx, "device_service.GetMetrics")
	span.SetAttributes(attribute.String("device_id", deviceID))
	defer span.End()

	rdb := s.client.Client()
	latestKey := fmt.Sprintf("device:%s:latest", deviceID)
	latestJSON, _ := rdb.Get(ctx, latestKey).Result()
	latestMetricsMap := make(map[string]float64)

	if latestJSON != "" {
		var t struct {
			Metrics json.RawMessage `json:"metrics"`
		}
		if json.Unmarshal([]byte(latestJSON), &t) == nil {
			var vals map[string]interface{}
			if json.Unmarshal(t.Metrics, &vals) == nil {
				for k, v := range vals {
					if f, ok := v.(float64); ok {
						latestMetricsMap[k] = f
					}
				}
			}
		}
	}

	return &device.Metrics{
		Latest: latestMetricsMap,
	}, nil
}
