package correlation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vishalss1/argus/internal/domain/event"
	"github.com/vishalss1/argus/internal/domain/incident"
	"github.com/vishalss1/argus/internal/infrastructure/ai"
)

type Engine struct {
	incidentService *incident.Service
	eventRepo      event.Repository
}

func NewEngine(incidentService *incident.Service, eventRepo event.Repository) *Engine {
	return &Engine{
		incidentService: incidentService,
		eventRepo:      eventRepo,
	}
}

func (e *Engine) Correlate(ctx context.Context, ev event.Event) error {
	// Look for existing open incidents for this device
	incidents, err := e.incidentService.ListIncidents(ctx)
	if err != nil {
		return fmt.Errorf("list incidents: %w", err)
	}

	var targetIncident *incident.Incident
	for _, inc := range incidents {
		if inc.Status == incident.StatusOpen {
			for _, dID := range inc.DeviceIDs {
				if dID == ev.DeviceID {
					// Check if the event is related to this incident
					// For now, let's use a simple time window of 1 hour
					if time.Since(inc.UpdatedAt) < 1*time.Hour {
						targetIncident = &inc
						break
					}
				}
			}
		}
		if targetIncident != nil {
			break
		}
	}

	if targetIncident != nil {
		return e.incidentService.AddEventToIncident(ctx, targetIncident.ID, ev.ID)
	}

	// Rule: Connectivity Instability
	if ev.Type == "connectivity_instability" {
		events, err := e.eventRepo.ListByDevice(ctx, ev.DeviceID)
		if err == nil {
			recent := 0
			for _, prev := range events {
				if prev.Type == "connectivity_instability" && time.Since(prev.CreatedAt) < 15*time.Minute {
					recent++
				}
			}
			if recent >= 3 {
				_, err := e.incidentService.CreateIncident(ctx, incident.Incident{
					Title:     "Connectivity Flapping",
					Summary:   "Device is repeatedly connecting and disconnecting, suggesting unstable network conditions.",
					Severity:  "warning",
					DeviceIDs: []string{ev.DeviceID},
					EventIDs:  []string{ev.ID},
					StartedAt: time.Now(),
				})
				if err == nil {
					ai.IncidentsCreatedTotal.Inc()
				}
				return err
			}
		}
	}

	// Rule: Resource Pressure
	if ev.Type == "resource_pressure" && ev.Severity == event.SeverityCritical {
		_, err := e.incidentService.CreateIncident(ctx, incident.Incident{
			Title:     "Critical Resource Depletion",
			Summary:   "Device is reporting critical memory or CPU pressure, which may lead to crashes or instability.",
			Severity:  "critical",
			DeviceIDs: []string{ev.DeviceID},
			EventIDs:  []string{ev.ID},
			StartedAt: time.Now(),
		})
		if err == nil {
			ai.IncidentsCreatedTotal.Inc()
		}
		return err
	}

	// Default Rule: Recurring events
	events, err := e.eventRepo.ListByDevice(ctx, ev.DeviceID)
	if err != nil {
		return fmt.Errorf("list events by device: %w", err)
	}

	recentEvents := []event.Event{}
	for _, prevEv := range events {
		if prevEv.Type == ev.Type && time.Since(prevEv.CreatedAt) < 30*time.Minute {
			recentEvents = append(recentEvents, prevEv)
		}
	}

	if len(recentEvents) >= 3 {
		// Create a new incident
		newInc := incident.Incident{
			ID:        uuid.New().String(),
			Title:     fmt.Sprintf("Recurring %s on device %s", ev.Type, ev.DeviceID),
			Summary:   fmt.Sprintf("Detected %d occurrences of %s within 30 minutes", len(recentEvents), ev.Type),
			Severity:  string(ev.Severity),
			DeviceIDs: []string{ev.DeviceID},
			EventIDs:  []string{ev.ID},
			StartedAt: recentEvents[len(recentEvents)-1].CreatedAt,
		}
		for _, re := range recentEvents {
			newInc.EventIDs = append(newInc.EventIDs, re.ID)
		}

		_, err := e.incidentService.CreateIncident(ctx, newInc)
		if err == nil {
			ai.IncidentsCreatedTotal.Inc()
		}
		return err
	}

	return nil
}
