package rule

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

const ruleCacheTTL = 30 * time.Second

// CooldownChecker reports whether an alert for (rule, device) may be raised now.
type CooldownChecker func(ctx context.Context, ruleID, deviceID string) bool

type cachedRules struct {
	rules     []Rule
	expiresAt time.Time
}

// Evaluator matches incoming telemetry against a workspace's enabled rules.
// Rules are cached briefly so the ingestion path does not query Postgres per message.
type Evaluator struct {
	repo     Repository
	cooldown CooldownChecker

	mu    sync.Mutex
	cache map[string]cachedRules
}

func NewEvaluator(repo Repository, cooldown CooldownChecker) *Evaluator {
	return &Evaluator{repo: repo, cooldown: cooldown, cache: make(map[string]cachedRules)}
}

func (e *Evaluator) rulesFor(ctx context.Context, workspaceID string) ([]Rule, error) {
	e.mu.Lock()
	entry, ok := e.cache[workspaceID]
	e.mu.Unlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.rules, nil
	}

	rules, err := e.repo.ListEnabledRulesByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	e.cache[workspaceID] = cachedRules{rules: rules, expiresAt: time.Now().Add(ruleCacheTTL)}
	e.mu.Unlock()
	return rules, nil
}

// Evaluate returns the alerts raised by one telemetry sample. Only rules owned by
// workspaceID are considered, so rules never fire across tenants.
func (e *Evaluator) Evaluate(ctx context.Context, workspaceID, deviceID, telemetryID string, rawMetrics json.RawMessage) ([]Alert, error) {
	if workspaceID == "" {
		return nil, nil
	}

	metrics, err := numericMetrics(rawMetrics)
	if err != nil {
		return nil, err
	}
	if len(metrics) == 0 {
		return nil, nil
	}

	rules, err := e.rulesFor(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	var alerts []Alert
	for _, r := range rules {
		observed, ok := metrics[r.Metric]
		if !ok || !matches(r.Operator, observed, r.Threshold) {
			continue
		}
		if e.cooldown != nil && !e.cooldown(ctx, r.ID, deviceID) {
			continue
		}

		alert := Alert{
			ID:            uuid.New().String(),
			RuleID:        r.ID,
			DeviceID:      deviceID,
			WorkspaceID:   workspaceID,
			Metric:        r.Metric,
			Operator:      r.Operator,
			Threshold:     r.Threshold,
			ObservedValue: observed,
			Severity:      "warning",
			Message:       fmt.Sprintf("%s: %s %s %.4g matched observed %.4g", r.Name, r.Metric, r.Operator, r.Threshold, observed),
			CreatedAt:     time.Now().UTC(),
		}
		if telemetryID != "" {
			id := telemetryID
			alert.TelemetryID = &id
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

func numericMetrics(raw json.RawMessage) (map[string]float64, error) {
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("decode telemetry metrics: %w", err)
	}

	metrics := make(map[string]float64, len(values))
	for key, value := range values {
		if typed, ok := value.(float64); ok {
			metrics[key] = typed
		}
	}
	return metrics, nil
}

func matches(operator string, observed, threshold float64) bool {
	switch operator {
	case OperatorGreaterThan:
		return observed > threshold
	case OperatorGreaterThanOrEqual:
		return observed >= threshold
	case OperatorLessThan:
		return observed < threshold
	case OperatorLessThanOrEqual:
		return observed <= threshold
	case OperatorEqual:
		return observed == threshold
	case OperatorNotEqual:
		return observed != threshold
	default:
		return false
	}
}
