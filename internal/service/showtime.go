package services

import (
	"context"
	"fmt"
	"time"

	"cinemabooking/internal/domain"
	repository "cinemabooking/internal/repository"
)

type ShowtimeService struct {
	repo        repository.ShowtimeRepository
	movieRepo   repository.MovieRepository
	theatreRepo repository.TheatreRepository
}

func NewShowtimeService(
	repo *repository.ShowtimeRepository,
	movieRepo *repository.MovieRepository,
	theatreRepo repository.TheatreRepository,
) *ShowtimeService {
	return &ShowtimeService{
		repo:        *repo,
		movieRepo:   *movieRepo,
		theatreRepo: theatreRepo,
	}
}

// GetShowtime returns a single showtime with movie, hall and theatre populated.
func (s *ShowtimeService) GetShowtime(ctx context.Context, id string) (*domain.Showtime, error) {
	showtime, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return showtime, nil
}

// ListShowtimes returns paginated showtimes with optional filters.
func (s *ShowtimeService) ListShowtimes(ctx context.Context, filter domain.ShowtimeFilter) ([]domain.Showtime, int, error) {
	filter.Page, filter.Limit = domain.NormalisePage(filter.Page, filter.Limit)

	showtimes, total, err := s.repo.FindAll(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	return showtimes, total, nil
}

// CreateShowtime validates the request, creates the showtime, then auto-generates
// showtime_seats from the hall's seat_definitions in the same logical operation.
func (s *ShowtimeService) CreateShowtime(ctx context.Context, req domain.CreateShowtimeRequest) (*domain.Showtime, error) {
	// validate movie exists
	movie, err := s.movieRepo.FindByID(ctx, req.MovieID)
	if err != nil {
		return nil, fmt.Errorf("movie not found: %w", err)
	}

	// validate hall exists and belongs to a real theatre
	hall, err := s.theatreRepo.FindHallByID(ctx, req.HallID)
	if err != nil {
		return nil, fmt.Errorf("hall not found: %w", err)
	}

	// parse and validate times
	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		return nil, domain.NewAppError(400, "invalid starts_at format, use RFC3339 e.g. 2026-06-18T10:00:00Z")
	}
	if startsAt.Before(time.Now()) {
		return nil, domain.NewAppError(400, "starts_at must be in the future")
	}

	// derive ends_at from movie duration
	endsAt := startsAt.Add(time.Duration(movie.DurationMin) * time.Minute)

	// check for hall conflicts — no overlapping showtimes in the same hall
	conflict, err := s.repo.HasConflict(ctx, req.HallID, startsAt, endsAt, nil)
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, domain.NewAppError(409, fmt.Sprintf(
			"hall '%s' already has a showtime between %s and %s",
			hall.Name,
			startsAt.Format("15:04"),
			endsAt.Format("15:04"),
		))
	}

	showtime, err := s.repo.Create(ctx, &domain.Showtime{
		MovieID:   req.MovieID,
		HallID:    req.HallID,
		StartsAt:  startsAt,
		EndsAt:    endsAt,
		BasePrice: req.BasePrice,
		IsActive:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("creating showtime: %w", err)
	}

	fmt.Printf("showtime.ID = %q\n", showtime.ID)
	fmt.Printf("hall.ID     = %q\n", hall.ID)

	// auto-generate showtime_seats from hall seat_definitions
	if err := s.repo.GenerateSeats(ctx, showtime.ID, hall.ID, req.BasePrice); err != nil {
		return nil, fmt.Errorf("generating seats: %w", err)
	}

	return showtime, nil
}

// UpdateShowtime applies partial updates — only non-zero fields are changed.
// Recalculates ends_at only when starts_at actually differs from the current value.
// Short-circuits with no DB write when nothing changed.
func (s *ShowtimeService) UpdateShowtime(ctx context.Context, id string, req domain.UpdateShowtimeRequest) (*domain.Showtime, error) {
	showtime, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// block updates to showtimes that have already started
	if showtime.HasStarted() {
		return nil, domain.NewAppError(409, "cannot update a showtime that has already started")
	}

	changed := false

	if req.StartsAt != "" {
		startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
		if err != nil {
			return nil, domain.NewAppError(400, "invalid starts_at format, use RFC3339")
		}
		if startsAt.Before(time.Now()) {
			return nil, domain.NewAppError(400, "starts_at must be in the future")
		}

		// Only do the expensive movie-fetch + conflict check when the time actually changed
		if !startsAt.Equal(showtime.StartsAt) {
			movie, err := s.movieRepo.FindByID(ctx, showtime.MovieID)
			if err != nil {
				return nil, err
			}

			newEndsAt := startsAt.Add(time.Duration(movie.DurationMin) * time.Minute)

			conflict, err := s.repo.HasConflict(ctx, showtime.HallID, startsAt, newEndsAt, &id)
			if err != nil {
				return nil, err
			}
			if conflict {
				return nil, domain.NewAppError(409, "hall has a conflicting showtime in the new time slot")
			}

			showtime.StartsAt = startsAt
			showtime.EndsAt = newEndsAt
			changed = true
		}
	}

	if req.BasePrice > 0 && req.BasePrice != showtime.BasePrice {
		showtime.BasePrice = req.BasePrice
		changed = true
	}

	if req.IsActive != nil && *req.IsActive != showtime.IsActive {
		showtime.IsActive = *req.IsActive
		changed = true
	}

	// Nothing actually changed — return current state without a DB write
	if !changed {
		return showtime, nil
	}

	updated, err := s.repo.Update(ctx, id, showtime)
	if err != nil {
		return nil, fmt.Errorf("updating showtime: %w", err)
	}

	return updated, nil
}

// DeleteShowtime blocks deletion if confirmed bookings exist.
func (s *ShowtimeService) DeleteShowtime(ctx context.Context, id string) error {
	showtime, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if showtime.HasStarted() {
		return domain.NewAppError(409, "cannot delete a showtime that has already started")
	}

	hasBookings, err := s.repo.HasConfirmedBookings(ctx, id)
	if err != nil {
		return err
	}
	if hasBookings {
		return domain.NewAppError(409, "cannot delete a showtime with confirmed bookings")
	}

	return s.repo.Delete(ctx, id)
}

// GetSeatMap returns the full seat grid for a showtime ready for the frontend seat picker.
func (s *ShowtimeService) GetSeatMap(ctx context.Context, showtimeID string) (*domain.SeatMap, error) {
	_, err := s.repo.FindByID(ctx, showtimeID)
	if err != nil {
		return nil, err
	}

	seatMap, err := s.repo.FindSeatMap(ctx, showtimeID)
	if err != nil {
		return nil, fmt.Errorf("fetching seat map: %w", err)
	}

	return seatMap, nil
}
