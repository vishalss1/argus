package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vishalss1/argus/internal/domain/finding"
	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type Engine struct {
	onFinding func(ctx context.Context, f finding.Finding)
}

func NewEngine(onFinding func(ctx context.Context, f finding.Finding)) *Engine {
	return &Engine{
		onFinding: onFinding,
	}
}

func (e *Engine) Analyze(ctx context.Context, t telemetry.Telemetry) error {
	var metrics map[string]interface{}
	if err := json.Unmarshal(t.Metrics, &metrics); err != nil {
		return err
	}

	// 1. Calculate Health & Risk Scores
	healthScore := 100
	riskScore := 0.0
	var issues []string

	// Temperature Impact
	temp, hasTemp := metrics["temperature"].(float64)
	if !hasTemp {
		temp, hasTemp = metrics["temp_c"].(float64)
	}
	if hasTemp {
		if temp > 85 {
			healthScore -= 40
			riskScore += 0.6
			issues = append(issues, "Critical thermal stress")
		} else if temp > 70 {
			healthScore -= 15
			riskScore += 0.2
			issues = append(issues, "Elevated temperature")
		}
	}

	// RAM Impact
	ram, hasRam := metrics["ram_usage"].(float64)
	if !hasRam {
		if heap, ok := metrics["free_heap"].(float64); ok && heap < 30000 {
			ram = 98.0
			hasRam = true
		}
	}
	if hasRam {
		if ram > 95 {
			healthScore -= 30
			riskScore += 0.4
			issues = append(issues, "Critical memory exhaustion")
		} else if ram > 85 {
			healthScore -= 10
			riskScore += 0.1
			issues = append(issues, "High memory pressure")
		}
	}

	// Connectivity Impact (RSSI)
	if rssi, ok := metrics["wifi_rssi"].(float64); ok {
		if rssi < -85 {
			healthScore -= 20
			riskScore += 0.3
			issues = append(issues, "Poor signal quality")
		}
	}

	// Clamp scores
	if healthScore < 0 {
		healthScore = 0
	}
	if riskScore > 1.0 {
		riskScore = 1.0
	}

	// Determine Severity
	severity := "info"
	if riskScore > 0.7 {
		severity = "critical"
	} else if riskScore > 0.4 {
		severity = "warning"
	}

	summary := "System healthy"
	if len(issues) > 0 {
		summary = fmt.Sprintf("Issues detected: %s", stringsJoin(issues, ", "))
	}

	// 2. Trigger Finding
	if e.onFinding != nil {
		e.onFinding(ctx, finding.Finding{
			ID:          uuid.New().String(),
			DeviceID:    t.DeviceID,
			RiskScore:   riskScore,
			HealthScore: healthScore,
			Severity:    severity,
			Summary:     summary,
			Metadata:    t.Metrics,
			CreatedAt:   time.Now(),
		})
	}

	return nil
}

func stringsJoin(s []string, sep string) string {
	if len(s) == 0 {
		return ""
	}
	res := s[0]
	for i := 1; i < len(s); i++ {
		res += sep + s[i]
	}
	return res
}
