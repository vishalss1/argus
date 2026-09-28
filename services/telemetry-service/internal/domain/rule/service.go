package rule

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
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
