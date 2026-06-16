package domain

import "time"

// Movie represents a film in the catalogue.
type Movie struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	DurationMin int       `json:"duration_mins"`
	Genre       []string  `json:"genre"`
	Language    string    `json:"language"`
	PosterURL   string    `json:"poster_url,omitempty"`
	TrailerURL  string    `json:"trailer_url,omitempty"`
	ReleaseDate time.Time `json:"release_date"`
	Rating      float64   `json:"rating"`
	TmdbID      string    `json:"tmdb_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Review holds a user's rating and comment for a movie.
type Review struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	MovieID   string    `json:"movie_id"`
	Rating    int       `json:"rating"` // 1–5
	Body      string    `json:"body,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	// Populated via JOIN when listing reviews
	UserName  string `json:"user_name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// ──────────────────────────────────────────────
// Request / Response DTOs
// ──────────────────────────────────────────────

type CreateMovieRequest struct {
	Title       string   `json:"title"        binding:"required,min=1,max=200"`
	Description string   `json:"description"  binding:"required"`
	DurationMin int      `json:"duration_mins" binding:"required,min=1"`
	Genre       []string `json:"genre"        binding:"required,min=1"`
	Language    string   `json:"language"     binding:"required"`
	PosterURL   string   `json:"poster_url"   binding:"omitempty,url"`
	TrailerURL  string   `json:"trailer_url"  binding:"omitempty,url"`
	ReleaseDate string   `json:"release_date" binding:"required"` // "2006-01-02"
	TmdbID      string   `json:"tmdb_id"      binding:"omitempty"`
}

type UpdateMovieRequest struct {
	Title       string   `json:"title"         binding:"omitempty,min=1,max=200"`
	Description string   `json:"description"   binding:"omitempty"`
	DurationMin int      `json:"duration_mins"  binding:"omitempty,min=1"`
	Genre       []string `json:"genre"         binding:"omitempty,min=1"`
	Language    string   `json:"language"      binding:"omitempty"`
	PosterURL   string   `json:"poster_url"    binding:"omitempty,url"`
	TrailerURL  string   `json:"trailer_url"   binding:"omitempty,url"`
	ReleaseDate string   `json:"release_date"  binding:"omitempty"`
}

type MovieFilter struct {
	City     string // filters by city of theatres showing this movie
	Genre    string
	Language string
	Date     string // "2006-01-02"
	Search   string // full-text against title
	Page     int
	Limit    int
}

type MovieListResponse struct {
	Movies []Movie `json:"movies"`
	Total  int     `json:"total"`
	Page   int     `json:"page"`
	Limit  int     `json:"limit"`
}

type CreateReviewRequest struct {
	Rating int    `json:"rating" binding:"required,min=1,max=5"`
	Body   string `json:"body"   binding:"omitempty,max=1000"`
}
