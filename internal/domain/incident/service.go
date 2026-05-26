package incident

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo       Repository
	OnCreated  func(ctx context.Context, inc Incident)
	OnResolved func(ctx context.Context, inc Incident)
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateIncident(ctx context.Context, inc Incident) (*Incident, error) {
	if inc.ID == "" {
		inc.ID = uuid.New().String()
	}
	if inc.CreatedAt.IsZero() {
		inc.CreatedAt = time.Now()
	}
	inc.UpdatedAt = inc.CreatedAt
	inc.Status = StatusOpen

	created, err := s.repo.Create(ctx, inc)
	if err != nil {
		return nil, err
	}

	if s.OnCreated != nil {
		s.OnCreated(ctx, *created)
	}

	return created, nil
}

func (s *Service) GetIncident(ctx context.Context, id string) (*Incident, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) ListIncidents(ctx context.Context) ([]Incident, error) {
	return s.repo.List(ctx)
}

func (s *Service) ResolveIncident(ctx context.Context, id string) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return err
	}

	if err := s.repo.Resolve(ctx, id); err != nil {
		return err
	}

	if s.OnResolved != nil {
		resolved, _ := s.repo.GetByID(ctx, id)
		if resolved != nil {
			s.OnResolved(ctx, *resolved)
		}
	}

	return nil
}

func (s *Service) AddEventToIncident(ctx context.Context, incidentID string, eventID string) error {
	inc, err := s.repo.GetByID(ctx, incidentID)
	if err != nil {
		return err
	}

	for _, id := range inc.EventIDs {
		if id == eventID {
			return nil
		}
	}

	inc.EventIDs = append(inc.EventIDs, eventID)
	inc.UpdatedAt = time.Now()
	_, err = s.repo.Update(ctx, *inc)
	return err
}
