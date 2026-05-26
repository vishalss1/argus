package anomaly

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	anomalydomain "github.com/vishalss1/argus/internal/domain/anomaly"
	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type Engine struct {
	mu             sync.Mutex
	thermalZScores map[string]*ZScore
	thermalEWMAs   map[string]*EWMA
	reconnectZScores map[string]*ZScore
	onAnomaly      func(ctx context.Context, a anomalydomain.Anomaly)
}

func NewEngine(onAnomaly func(ctx context.Context, a anomalydomain.Anomaly)) *Engine {
	return &Engine{
		thermalZScores:   make(map[string]*ZScore),
		thermalEWMAs:     make(map[string]*EWMA),
		reconnectZScores: make(map[string]*ZScore),
		onAnomaly:        onAnomaly,
	}
}

func (e *Engine) Analyze(ctx context.Context, t telemetry.Telemetry) error {
	var metrics map[string]interface{}
	if err := json.Unmarshal(t.Metrics, &metrics); err != nil {
		return err
	}

	// 1. Thermal Anomaly Detection (Z-Score for Spikes)
	if temp, ok := metrics["temperature"].(float64); ok {
		e.mu.Lock()
		z, ok := e.thermalZScores[t.DeviceID]
		if !ok {
			z = NewZScore(10)
			e.thermalZScores[t.DeviceID] = z
		}
		score := z.Push(temp)

		// 2. Thermal Drift Detection (EWMA)
		ewma, ok := e.thermalEWMAs[t.DeviceID]
		if !ok {
			ewma = NewEWMA(0.1) // alpha = 0.1 for slow moving average
			e.thermalEWMAs[t.DeviceID] = ewma
		}
		avg := ewma.Update(temp)
		e.mu.Unlock()

		if math.Abs(score) > 3.0 {
			e.trigger(ctx, anomalydomain.Anomaly{
				ID:              uuid.New().String(),
				DeviceID:        t.DeviceID,
				Type:            anomalydomain.AnomalyTypeThermal,
				Severity:        "warning",
				Title:           "Statistical Thermal Spike",
				Summary:         fmt.Sprintf("Temperature spike detected. Z-score: %.2f (Value: %.2f°C)", score, temp),
				ConfidenceScore: 0.9,
				Metadata:        t.Metrics,
				DetectedAt:      time.Now(),
				CreatedAt:       time.Now(),
			})
		}

		// Detect drift: if current value is significantly different from EWMA but Z-score is low (steady change)
		if math.Abs(temp-avg) > 10.0 && math.Abs(score) < 2.0 {
			e.trigger(ctx, anomalydomain.Anomaly{
				ID:              uuid.New().String(),
				DeviceID:        t.DeviceID,
				Type:            anomalydomain.AnomalyTypeThermal,
				Severity:        "info",
				Title:           "Thermal Drift Detected",
				Summary:         fmt.Sprintf("Temperature is drifting from historical average. Average: %.2f°C, Current: %.2f°C", avg, temp),
				ConfidenceScore: 0.7,
				Metadata:        t.Metrics,
				DetectedAt:      time.Now(),
				CreatedAt:       time.Now(),
			})
		}
	}

	// 3. Resource Pressure Detection (Z-Score)
	if _, ok := metrics["ram_usage"].(float64); ok {
		// Generic spike detection for RAM could be added here similar to thermal
	}

	return nil
}

func (e *Engine) AnalyzeConnectivity(ctx context.Context, deviceID string, reconnectCount int) {
	e.mu.Lock()
	z, ok := e.reconnectZScores[deviceID]
	if !ok {
		z = NewZScore(5)
		e.reconnectZScores[deviceID] = z
	}
	score := z.Push(float64(reconnectCount))
	e.mu.Unlock()

	if score > 2.5 {
		e.trigger(ctx, anomalydomain.Anomaly{
			ID:              uuid.New().String(),
			DeviceID:        deviceID,
			Type:            anomalydomain.AnomalyTypeConnectivity,
			Severity:        "critical",
			Title:           "Reconnect Burst Detected",
			Summary:         fmt.Sprintf("Abnormal frequency of reconnects detected. Z-score: %.2f", score),
			ConfidenceScore: 0.95,
			Metadata:        json.RawMessage(fmt.Sprintf(`{"reconnect_count": %d}`, reconnectCount)),
			DetectedAt:      time.Now(),
			CreatedAt:       time.Now(),
		})
	}
}

func (e *Engine) trigger(ctx context.Context, a anomalydomain.Anomaly) {
	if e.onAnomaly != nil {
		e.onAnomaly(ctx, a)
	}
}
