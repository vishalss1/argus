package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vishalss1/argus/telemetry/internal/domain/device"
	"github.com/vishalss1/argus/telemetry/internal/domain/fleet"
	"github.com/vishalss1/argus/telemetry/internal/domain/severity"
	"go.opentelemetry.io/otel"
)

type FleetService struct {
	client     *Client
	deviceRepo device.Repository
}

func NewFleetService(client *Client, deviceRepo device.Repository) *FleetService {
	return &FleetService{
		client:     client,
		deviceRepo: deviceRepo,
	}
}

type redisIncident struct {
	DeviceID     string `json:"device_id"`
	Metric       string `json:"metric"`
	IncidentType string `json:"incident_type"`
	Severity     string `json:"severity"`
}

func (s *FleetService) GetStats(ctx context.Context, workspaceID string) (*fleet.Stats, error) {
	tracer := otel.Tracer("telemetry-service")
	ctx, span := tracer.Start(ctx, "fleet_service.GetStats")
	defer span.End()

	if workspaceID == "" {
		return &fleet.Stats{WorstSeverity: severity.Healthy}, nil
	}

	// 1. Get all device IDs in the workspace
	deviceIDs, err := s.deviceRepo.GetWorkspaceDevices(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace devices: %w", err)
	}

	rdb := s.client.Client()

	// 2. Get active session ID for this workspace
	sessionKey := fmt.Sprintf("workspace:%s:active_session", workspaceID)
	sessionID, _ := rdb.Get(ctx, sessionKey).Result()

	stats := &fleet.Stats{
		TotalDevices:  len(deviceIDs),
		WorstSeverity: severity.Healthy,
	}

	// 3. Check status and incidents for each device
	for _, devID := range deviceIDs {
		// Calculate online/offline
		online := false
		if sessionID != "" {
			// If there's an active session, check if device has session state in Redis
			devStateKey := fmt.Sprintf("session:%s:device:%s:state", sessionID, devID)
			exists, _ := rdb.Exists(ctx, devStateKey).Result()
			if exists > 0 {
				online = true
			}
		}

		// If not found in session state, check the DB device status
		if !online {
			dev, err := s.deviceRepo.GetByID(ctx, devID)
			if err == nil && dev != nil && dev.Status == "online" {
				online = true
			}
		}

		if online {
			stats.OnlineDevices++
		}

		// Count incidents for this device in the active session
		if sessionID != "" {
			deviceIncidentsSetKey := fmt.Sprintf("session:%s:device:%s:incidents", sessionID, devID)
			incidentKeys, _ := rdb.SMembers(ctx, deviceIncidentsSetKey).Result()
			if len(incidentKeys) > 0 {
				values, err := rdb.MGet(ctx, incidentKeys...).Result()
				if err == nil {
					for _, val := range values {
						raw, ok := val.(string)
						if !ok || raw == "" {
							continue
						}
						var inc redisIncident
						if json.Unmarshal([]byte(raw), &inc) == nil {
							stats.ActiveIncidents++
							sev := severity.Parse(inc.Severity)
							if sev == severity.Warning {
								stats.WarningIncidents++
							} else if sev == severity.Critical {
								stats.CriticalIncidents++
							}
							if sev > stats.WorstSeverity {
								stats.WorstSeverity = sev
							}
						}
					}
				}
			}
		}
	}

	return stats, nil
}

func (s *FleetService) GetHealthSummary(ctx context.Context, workspaceID string) (*fleet.HealthSummary, error) {
	tracer := otel.Tracer("telemetry-service")
	ctx, span := tracer.Start(ctx, "fleet_service.GetHealthSummary")
	defer span.End()

	stats, err := s.GetStats(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	// Return basic summary without top incident types parsing to save time
	return &fleet.HealthSummary{
		Stats: *stats,
	}, nil
}

func (s *FleetService) GetWorstDevices(ctx context.Context, workspaceID string, limit int) ([]fleet.DeviceBrief, error) {
	return nil, nil
}

func (s *FleetService) GetRecentIncidents(ctx context.Context, workspaceID string, limit int) ([]fleet.IncidentBrief, error) {
	if workspaceID == "" {
		return nil, nil
	}

	rdb := s.client.Client()

	sessionKey := fmt.Sprintf("workspace:%s:active_session", workspaceID)
	sessionID, _ := rdb.Get(ctx, sessionKey).Result()
	if sessionID == "" {
		return nil, nil
	}

	deviceIDs, err := s.deviceRepo.GetWorkspaceDevices(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace devices: %w", err)
	}

	var incidentKeys []string
	for _, devID := range deviceIDs {
		deviceIncidentsSetKey := fmt.Sprintf("session:%s:device:%s:incidents", sessionID, devID)
		keys, err := rdb.SMembers(ctx, deviceIncidentsSetKey).Result()
		if err == nil && len(keys) > 0 {
			incidentKeys = append(incidentKeys, keys...)
		}
	}

	var incidents []fleet.IncidentBrief
	if len(incidentKeys) > 0 {
		vals, err := rdb.MGet(ctx, incidentKeys...).Result()
		if err == nil {
			for _, v := range vals {
				if vStr, ok := v.(string); ok && vStr != "" {
					var inc struct {
						DeviceID     string    `json:"device_id"`
						Metric       string    `json:"metric"`
						IncidentType string    `json:"incident_type"`
						Severity     string    `json:"severity"`
					}
					if json.Unmarshal([]byte(vStr), &inc) == nil {
						incidents = append(incidents, fleet.IncidentBrief{
							DeviceID:     inc.DeviceID,
							Metric:       inc.Metric,
							IncidentType: inc.IncidentType,
							Severity:     severity.Parse(inc.Severity),
						})
						if limit > 0 && len(incidents) >= limit {
							break
						}
					}
				}
			}
		}
	}
	return incidents, nil
}
