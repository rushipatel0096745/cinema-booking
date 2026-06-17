package domain

import (
	"time"
)

// Showtime is a scheduled screening of a movie in a hall.
type Showtime struct {
	ID        string    `json:"id"`
	MovieID   string    `json:"movie_id"`
	HallID    string    `json:"hall_id"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	BasePrice float64   `json:"base_price"`
	IsActive  bool      `json:"is_active"`
	// Populated via JOINs
	Movie       *Movie   `json:"movie,omitempty"`
	Hall        *Hall    `json:"hall,omitempty"`
	Theatre     *Theatre `json:"theatre,omitempty"`
	AvailSeats  int      `json:"available_seats,omitempty"`
	BookedSeats int      `json:"booked_seats,omitempty"`
}

type MovieShowtimesResponse struct {
    MovieID string          `json:"movie_id"`
    Cities  []CityShowtimes `json:"cities"`
}

type CityShowtimes struct {
    City     string             `json:"city"`
    Theatres []TheatreShowtimes `json:"theatres"`
}

type TheatreShowtimes struct {
    Theatre   Theatre  `json:"theatre"`
    Showtimes []Showtime `json:"showtimes"`
}

// HasStarted reports whether the showtime has already begun.
func (s *Showtime) HasStarted() bool { return time.Now().After(s.StartsAt) }

// ──────────────────────────────────────────────
// Seat Status values
// ──────────────────────────────────────────────

const (
	SeatStatusAvailable = "available"
	SeatStatusLocked    = "locked"
	SeatStatusBooked    = "booked"
)

// ShowtimeSeat is an instance of a seat for a specific showtime.
// One row per (showtime, seat_definition) pair.
type ShowtimeSeat struct {
	ID         string  `json:"id"`
	ShowtimeID string  `json:"showtime_id"`
	SeatDefID  string  `json:"seat_def_id"`
	Status     string  `json:"status"` // available | locked | booked
	LockedBy   string  `json:"locked_by,omitempty"`
	Price      float64 `json:"price"`
	// Populated via JOIN with seat_definitions
	RowLabel  string `json:"row_label"`
	ColNumber int    `json:"col_number"`
	SeatType  string `json:"seat_type"`
}

// ──────────────────────────────────────────────
// Seat map — structured for direct frontend use
// ──────────────────────────────────────────────

// SeatMap is the full seat grid for a showtime, ready to render.
type SeatMap struct {
	ShowtimeID string    `json:"showtime_id"`
	Rows       []SeatRow `json:"rows"`
}

// SeatRow is one row of seats (e.g. row "C").
type SeatRow struct {
	Label string     `json:"label"` // "A", "B" …
	Seats []SeatCell `json:"seats"`
}

// SeatCell is a single seat in the grid.
// IsAisle=true means render a gap — no seat to book.
type SeatCell struct {
	ID      string  `json:"id"` // showtime_seat.id — use this when locking
	Col     int     `json:"col"`
	Type    string  `json:"type"`   // standard | premium | recliner
	Status  string  `json:"status"` // available | locked | booked
	Price   float64 `json:"price"`
	IsAisle bool    `json:"is_aisle"` // true → render a visual gap, no ID
}

// ──────────────────────────────────────────────
// WebSocket event broadcasted to all viewers
// ──────────────────────────────────────────────

// SeatStatusEvent is sent over WebSocket whenever seat statuses change.
type SeatStatusEvent struct {
	Type       string   `json:"type"` // seats_locked | seats_released | seats_booked
	ShowtimeID string   `json:"showtime_id"`
	SeatIDs    []string `json:"seat_ids"` // showtime_seat IDs
	Status     string   `json:"status"`   // new status value
}

// ──────────────────────────────────────────────
// Request / Response DTOs
// ──────────────────────────────────────────────

type CreateShowtimeRequest struct {
	MovieID   string  `json:"movie_id"   binding:"required,uuid"`
	HallID    string  `json:"hall_id"    binding:"required,uuid"`
	StartsAt  string  `json:"starts_at"  binding:"required"` // RFC3339
	BasePrice float64 `json:"base_price" binding:"required,gt=0"`
}

type UpdateShowtimeRequest struct {
	StartsAt  string  `json:"starts_at"  binding:"omitempty"`
	BasePrice float64 `json:"base_price" binding:"omitempty,gt=0"`
	IsActive  *bool   `json:"is_active"  binding:"omitempty"`
}

type ShowtimeFilter struct {
	MovieID string
	City    string
	Date    string // "2006-01-02"
	Page    int
	Limit   int
}

type ShowtimeListResponse struct {
	Showtimes []Showtime `json:"showtimes"`
	Total     int        `json:"total"`
	Page      int        `json:"page"`
	Limit     int        `json:"limit"`
}
