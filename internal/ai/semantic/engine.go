package semantic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vishalss1/argus/internal/domain/event"
	"github.com/vishalss1/argus/internal/domain/telemetry"
	"github.com/vishalss1/argus/internal/infrastructure/ai"
)

type Engine struct {
	eventRepo event.Repository
}

func NewEngine(eventRepo event.Repository) *Engine {
	return &Engine{
		eventRepo: eventRepo,
	}
}

func (e *Engine) AnalyzeTelemetry(ctx context.Context, t telemetry.Telemetry) ([]event.Event, error) {
	var metrics map[string]interface{}
	if err := json.Unmarshal(t.Metrics, &metrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	var events []event.Event

	// Thermal Anomaly Rule (supports 'temperature' or 'temp_c')
	temp, hasTemp := metrics["temperature"].(float64)
	if !hasTemp {
		temp, hasTemp = metrics["temp_c"].(float64)
	}

	if hasTemp {
		if temp > 89 {
			events = append(events, event.Event{
				ID:              uuid.New().String(),
				DeviceID:        t.DeviceID,
				Type:            "thermal_anomaly",
				Severity:        event.SeverityCritical,
				Title:           "Critical Thermal Anomaly",
				Summary:         fmt.Sprintf("High temperature detected: %.2f°C", temp),
				Source:          "semantic_engine",
				ConfidenceScore: 1.0,
				Metadata:        t.Metrics,
				CreatedAt:       time.Now(),
			})
		} else if temp > 75 {
			events = append(events, event.Event{
				ID:              uuid.New().String(),
				DeviceID:        t.DeviceID,
				Type:            "thermal_anomaly",
				Severity:        event.SeverityWarning,
				Title:           "Elevated Temperature",
				Summary:         fmt.Sprintf("Temperature is rising: %.2f°C", temp),
				Source:          "semantic_engine",
				ConfidenceScore: 0.8,
				Metadata:        t.Metrics,
				CreatedAt:       time.Now(),
			})
		}
	}

	// Memory Pressure Rule (supports 'ram_usage' or 'free_heap' calculation)
	ram, hasRam := metrics["ram_usage"].(float64)
	if !hasRam {
		// If no percentage is provided, look for free_heap (common in ESP32)
		// We'll treat low heap as pressure
		if heap, ok := metrics["free_heap"].(float64); ok && heap < 50000 {
			ram = 95.0 // Simulate high usage for the rule
			hasRam = true
		}
	}

	if hasRam {
		if ram > 94 {
			events = append(events, event.Event{
				ID:              uuid.New().String(),
				DeviceID:        t.DeviceID,
				Type:            "resource_pressure",
				Severity:        event.SeverityCritical,
				Title:           "Critical Memory Pressure",
				Summary:         fmt.Sprintf("Extremely high memory usage detected (RAM: %.2f%% or Low Heap)", ram),
				Source:          "semantic_engine",
				ConfidenceScore: 1.0,
				Metadata:        t.Metrics,
				CreatedAt:       time.Now(),
			})
		} else if ram > 80 {
			events = append(events, event.Event{
				ID:              uuid.New().String(),
				DeviceID:        t.DeviceID,
				Type:            "resource_pressure",
				Severity:        event.SeverityWarning,
				Title:           "High Memory Usage",
				Summary:         fmt.Sprintf("High RAM usage detected: %.2f%%", ram),
				Source:          "semantic_engine",
				ConfidenceScore: 0.9,
				Metadata:        t.Metrics,
				CreatedAt:       time.Now(),
			})
		}
	}

	// Persist generated events
	for _, ev := range events {
		if _, err := e.eventRepo.Create(ctx, ev); err != nil {
			// Log error but continue
			fmt.Printf("failed to persist event: %v\n", err)
		} else {
			ai.EventsGeneratedTotal.WithLabelValues(ev.Type, string(ev.Severity)).Inc()
		}
	}

	return events, nil
}
