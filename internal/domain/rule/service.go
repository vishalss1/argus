package rule

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*Rule, error) {
	rule, err := buildRule(input)
	if err != nil {
		return nil, err
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}
	rule.ID = id

	return s.repo.CreateRule(ctx, rule)
}

func (s *Service) List(ctx context.Context) ([]Rule, error) {
	return s.repo.ListRules(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (*Rule, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("rule id is required")
	}

	return s.repo.GetRule(ctx, id)
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*Rule, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("rule id is required")
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, errors.New("name cannot be empty")
		}
		input.Name = &name
	}
	if input.Metric != nil {
		metric := strings.TrimSpace(*input.Metric)
		if metric == "" {
			return nil, errors.New("metric cannot be empty")
		}
		input.Metric = &metric
	}
	if input.Operator != nil {
		operator := strings.TrimSpace(*input.Operator)
		if !validOperator(operator) {
			return nil, errors.New("operator must be one of >, >=, <, <=, ==, !=")
		}
		input.Operator = &operator
	}

	return s.repo.UpdateRule(ctx, id, input)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("rule id is required")
	}

	return s.repo.DeleteRule(ctx, id)
}

func (s *Service) ListAlerts(ctx context.Context) ([]Alert, error) {
	return s.repo.ListAlerts(ctx)
}

func (s *Service) EvaluateTelemetry(ctx context.Context, event telemetry.Telemetry) ([]Alert, error) {
	metrics, err := numericMetrics(event.Metrics)
	if err != nil {
		return nil, err
	}

	rules, err := s.repo.ListEnabledRules(ctx)
	if err != nil {
		return nil, err
	}

	alerts := make([]Alert, 0)
	for _, rule := range rules {
		observed, ok := metrics[rule.Metric]
		if !ok || !matches(rule.Operator, observed, rule.Threshold) {
			continue
		}

		id, err := newID()
		if err != nil {
			return nil, err
		}

		alert, err := s.repo.CreateAlert(ctx, Alert{
			ID:            id,
			RuleID:        rule.ID,
			DeviceID:      event.DeviceID,
			TelemetryID:   event.ID,
			Metric:        rule.Metric,
			Operator:      rule.Operator,
			Threshold:     rule.Threshold,
			ObservedValue: observed,
			Message:       fmt.Sprintf("%s: %s %s %.4g matched observed %.4g", rule.Name, rule.Metric, rule.Operator, rule.Threshold, observed),
		})
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, *alert)
	}

	return alerts, nil
}

func buildRule(input CreateInput) (Rule, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Rule{}, errors.New("name is required")
	}
	metric := strings.TrimSpace(input.Metric)
	if metric == "" {
		return Rule{}, errors.New("metric is required")
	}
	operator := strings.TrimSpace(input.Operator)
	if !validOperator(operator) {
		return Rule{}, errors.New("operator must be one of >, >=, <, <=, ==, !=")
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	return Rule{
		Name:      name,
		Metric:    metric,
		Operator:  operator,
		Threshold: input.Threshold,
		Enabled:   enabled,
	}, nil
}

func validOperator(operator string) bool {
	switch operator {
	case OperatorGreaterThan, OperatorGreaterThanOrEqual, OperatorLessThan, OperatorLessThanOrEqual, OperatorEqual, OperatorNotEqual:
		return true
	default:
		return false
	}
}

func numericMetrics(raw json.RawMessage) (map[string]float64, error) {
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("decode telemetry metrics: %w", err)
	}

	metrics := make(map[string]float64, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case float64:
			metrics[key] = typed
		}
	}

	return metrics, nil
}

func matches(operator string, observed float64, threshold float64) bool {
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

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	encoded := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}
