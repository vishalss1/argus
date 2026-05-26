package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
	"github.com/vishalss1/argus/internal/domain/incident"
)

type Manager struct {
	contextService *ctxdomain.Service
}

func NewManager(contextService *ctxdomain.Service) *Manager {
	return &Manager{
		contextService: contextService,
	}
}

func (m *Manager) SummarizeIncident(ctx context.Context, inc incident.Incident) error {
	data, err := json.Marshal(map[string]interface{}{
		"incident_id": inc.ID,
		"title":       inc.Title,
		"severity":    inc.Severity,
		"event_count": len(inc.EventIDs),
		"duration":    m.calculateDuration(inc),
	})
	if err != nil {
		return fmt.Errorf("marshal incident data: %w", err)
	}

	for _, deviceID := range inc.DeviceIDs {
		_, err := m.contextService.RecordMemory(ctx, ctxdomain.OperationalMemory{
			DeviceID:  &deviceID,
			Type:      ctxdomain.MemoryTypeIncident,
			Summary:   fmt.Sprintf("Incident Resolved: %s", inc.Title),
			Data:      data,
			Timestamp: time.Now(),
		})
		if err != nil {
			return fmt.Errorf("record memory for device %s: %w", deviceID, err)
		}
	}

	return nil
}

func (m *Manager) calculateDuration(inc incident.Incident) string {
	if inc.ResolvedAt == nil {
		return "ongoing"
	}
	return inc.ResolvedAt.Sub(inc.StartedAt).String()
}
