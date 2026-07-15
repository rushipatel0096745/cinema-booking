package services

import (
	"cinemabooking/internal/domain"
	repositories "cinemabooking/internal/repository"
	"context"
	"errors"
	"fmt"
)

var ErrTheatreNotFound = errors.New("theatre not found")

type TheatreService struct {
	repo repositories.TheatreRepository
}

func NewTheatreService(repo repositories.TheatreRepository) *TheatreService {
	return &TheatreService{repo: repo}
}

// ListTheatres returns paginated theatres, filtered by city.
// Halls are NOT included here — they are loaded on-demand via GetTheatre.
func (s *TheatreService) ListTheatres(ctx context.Context, filter domain.TheatreFilter) ([]domain.Theatre, int, error) {
	return s.repo.FindAll(ctx, filter)
}

// ListCities returns all unique cities where theatres are located.
func (s *TheatreService) ListCities(ctx context.Context) ([]string, error) {
	return s.repo.FindAllCities(ctx)
}

// GetTheatre returns the theatre with its halls attached — this is the "on demand"
// case the domain.Theatre comment refers to. ListTheatres deliberately doesn't do this,
// to avoid an N+1 query when listing many theatres at once.
func (s *TheatreService) GetTheatre(ctx context.Context, id string) (*domain.Theatre, error) {
	theatre, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrTheatreNotFound
		}
		return nil, err
	}

	halls, err := s.repo.FindHalls(ctx, theatre.ID)
	if err != nil {
		return nil, err
	}
	theatre.Halls = halls
	return theatre, nil
}

func (s *TheatreService) GetHalls(ctx context.Context, theatreID string) ([]domain.Hall, error) {
	if _, err := s.repo.FindByID(ctx, theatreID); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrTheatreNotFound
		}
		return nil, err
	}
	return s.repo.FindHalls(ctx, theatreID)
}

func (s *TheatreService) CreateTheatre(ctx context.Context, req *domain.CreateTheatreRequest) (*domain.Theatre, error) {
	theatre := &domain.Theatre{
		Name:    req.Name,
		City:    req.City,
		Address: req.Address,
	}
	if req.Lat != 0 {
		lat := req.Lat
		theatre.Lat = &lat
	}
	if req.Lng != 0 {
		lng := req.Lng
		theatre.Lng = &lng
	}
	return s.repo.Create(ctx, theatre)
}

func (s *TheatreService) CreateHall(ctx context.Context, theatreID string, req *domain.CreateHallRequest) (*domain.Hall, error) {
	hall := &domain.Hall{
		TheatreID: theatreID,
		Name:      req.Name,
		TotalRows: req.TotalRows,
		TotalCols: req.TotalCols,
	}
	created, err := s.repo.CreateHall(ctx, hall)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrTheatreNotFound
		}
		return nil, err
	}

	// auto-generate seat definitions immediately after hall creation
	if err := s.repo.GenerateSeatDefinitions(
		ctx,
		created.ID,
		created.TotalRows,
		created.TotalCols,
	); err != nil {
		return nil, fmt.Errorf("generating seat definitions: %w", err)
	}

	return created, nil
}
