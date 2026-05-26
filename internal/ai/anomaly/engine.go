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
	onAnomaly      func(ctx context.Context, a anomalydomain.Anomaly)
}

func NewEngine(onAnomaly func(ctx context.Context, a anomalydomain.Anomaly)) *Engine {
	return &Engine{
		thermalZScores: make(map[string]*ZScore),
		onAnomaly:      onAnomaly,
	}
}

func (e *Engine) Analyze(ctx context.Context, t telemetry.Telemetry) error {
	var metrics map[string]interface{}
	if err := json.Unmarshal(t.Metrics, &metrics); err != nil {
		return err
	}

	// Thermal Anomaly Detection using Z-Score
	if temp, ok := metrics["temperature"].(float64); ok {
		e.mu.Lock()
		z, ok := e.thermalZScores[t.DeviceID]
		if !ok {
			z = NewZScore(10)
			e.thermalZScores[t.DeviceID] = z
		}
		score := z.Push(temp)
		e.mu.Unlock()

		if math.Abs(score) > 3.0 {
			// Detected anomaly
			a := anomalydomain.Anomaly{
				ID:              uuid.New().String(),
				DeviceID:        t.DeviceID,
				Type:            anomalydomain.AnomalyTypeThermal,
				Severity:        "warning",
				Title:           "Statistical Thermal Anomaly",
				Summary:         fmt.Sprintf("Temperature deviation detected. Z-score: %.2f", score),
				ConfidenceScore: 0.9,
				Metadata:        t.Metrics,
				DetectedAt:      time.Now(),
				CreatedAt:       time.Now(),
			}
			if e.onAnomaly != nil {
				e.onAnomaly(ctx, a)
			}
		}
	}

	return nil
}
