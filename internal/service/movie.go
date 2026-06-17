package services

import (
	"cinemabooking/internal/domain"
	repositories "cinemabooking/internal/repository"
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

var (
	ErrMovieNotFound = errors.New("movie not found")
	ErrMovieInUse    = errors.New("movie has existing showtimes or reviews and cannot be deleted")
)

type MovieService struct {
	repo *repositories.MovieRepository
}

func NewMovieService(repo *repositories.MovieRepository) *MovieService {
	return &MovieService{repo: repo}
}

func (s *MovieService) ListMovies(ctx context.Context, filter domain.MovieFilter) (*domain.MovieListResponse, error) {
	movies, total, err := s.repo.FindWithFilters(ctx, filter)
	if err != nil {
		return nil, err
	}
	page, limit := filter.Page, filter.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	return &domain.MovieListResponse{Movies: movies, Total: total, Page: page, Limit: limit}, nil
}

func (s *MovieService) GetMovie(ctx context.Context, id string) (*domain.Movie, error) {
	movie, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrMovieNotFound
		}
		return nil, err
	}
	return movie, nil
}

func (s *MovieService) CreateMovie(ctx context.Context, req *domain.CreateMovieRequest) (*domain.Movie, error) {
	releaseDate, err := time.Parse("2006-01-02", req.ReleaseDate)
	if err != nil {
		return nil, fmt.Errorf("invalid release_date: %w", err)
	}

	movie := &domain.Movie{
		Title:       req.Title,
		Description: req.Description,
		DurationMin: req.DurationMin,
		Genre:       req.Genre,
		Language:    req.Language,
		PosterURL:   req.PosterURL,
		TrailerURL:  req.TrailerURL,
		ReleaseDate: releaseDate,
		TmdbID:      req.TmdbID,
	}
	if err := s.repo.Create(ctx, movie); err != nil {
		return nil, err
	}
	return movie, nil
}

func (s *MovieService) UpdateMovie(ctx context.Context, id string, req *domain.UpdateMovieRequest) (*domain.Movie, error) {
	movie, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrMovieNotFound
		}
		return nil, err
	}

	if req.Title != "" {
		movie.Title = req.Title
	}
	if req.Description != "" {
		movie.Description = req.Description
	}
	if req.DurationMin != 0 {
		movie.DurationMin = req.DurationMin
	}
	if len(req.Genre) > 0 {
		movie.Genre = req.Genre
	}
	if req.Language != "" {
		movie.Language = req.Language
	}
	if req.PosterURL != "" {
		movie.PosterURL = req.PosterURL
	}
	if req.TrailerURL != "" {
		movie.TrailerURL = req.TrailerURL
	}
	if req.ReleaseDate != "" {
		releaseDate, err := time.Parse("2006-01-02", req.ReleaseDate)
		if err != nil {
			return nil, fmt.Errorf("invalid release_date: %w", err)
		}
		movie.ReleaseDate = releaseDate
	}

	if err := s.repo.Update(ctx, movie); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrMovieNotFound
		}
		return nil, err
	}
	return movie, nil
}

func (s *MovieService) DeleteMovie(ctx context.Context, id string) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrMovieNotFound
		}
		if errors.Is(err, repositories.ErrMovieInUse) {
			return ErrMovieInUse
		}
		return err
	}
	return nil
}

// func (s *MovieService) GetShowtimes(ctx context.Context, movieID string) ([]domain.Showtime, error) {
// 	return s.repo.FindShowtimes(ctx, movieID)
// }

// service/movie.go
func (s *MovieService) GetMovieShowtimes(ctx context.Context, movieID string, filter domain.ShowtimeFilter) (*domain.MovieShowtimesResponse, error) {
	showtimes, err := s.repo.FindShowtimes(ctx, movieID, filter)
	if err != nil {
		return nil, err
	}

	// group: city → theatre → []showtime
	cityMap := make(map[string]map[string]*domain.TheatreShowtimes)

	for _, st := range showtimes {
		city := st.Theatre.City
		theatreID := st.Theatre.ID

		if cityMap[city] == nil {
			cityMap[city] = make(map[string]*domain.TheatreShowtimes)
		}
		if cityMap[city][theatreID] == nil {
			cityMap[city][theatreID] = &domain.TheatreShowtimes{
				Theatre: *st.Theatre,
			}
		}
		cityMap[city][theatreID].Showtimes = append(cityMap[city][theatreID].Showtimes, st)
	}

	// flatten to ordered slice
	var cities []domain.CityShowtimes
	for city, theatreMap := range cityMap {
		var theatres []domain.TheatreShowtimes
		for _, ts := range theatreMap {
			theatres = append(theatres, *ts)
		}
		cities = append(cities, domain.CityShowtimes{City: city, Theatres: theatres})
	}

	// sort cities alphabetically for consistent response
	sort.Slice(cities, func(i, j int) bool {
		return cities[i].City < cities[j].City
	})

	if cities == nil {
		cities = []domain.CityShowtimes{}
	}

	return &domain.MovieShowtimesResponse{
		MovieID: movieID,
		Cities:  cities,
	}, nil
}

var ErrAlreadyReviewed = errors.New("you have already reviewed this movie")

func (s *MovieService) AddReview(ctx context.Context, userID, movieID string, req *domain.CreateReviewRequest) (*domain.Review, error) {
	review := &domain.Review{UserID: userID, MovieID: movieID, Rating: req.Rating, Body: req.Body}
	if err := s.repo.AddReview(ctx, review); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrMovieNotFound
		}
		if errors.Is(err, repositories.ErrAlreadyReviewed) {
			return nil, ErrAlreadyReviewed
		}
		return nil, err
	}
	return review, nil
}

func (s *MovieService) ListReviews(ctx context.Context, movieID string, page, limit int) ([]domain.Review, error) {
	return s.repo.FindReviews(ctx, movieID, page, limit)
}
