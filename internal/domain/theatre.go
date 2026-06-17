package domain

// Theatre is a cinema complex that may contain multiple halls.
type Theatre struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	City    string   `json:"city"`
	Address string   `json:"address,omitempty"`
	Lat     *float64 `json:"lat,omitempty"`
	Lng     *float64 `json:"lng,omitempty"`
	// Populated on demand
	Halls []Hall `json:"halls"`
}

// Hall is a single screen/auditorium within a theatre.
type Hall struct {
	ID         string `json:"id"`
	TheatreID  string `json:"theatre_id"`
	Name       string `json:"name"` // "IMAX Screen", "Hall 2"
	TotalRows  int    `json:"total_rows"`
	TotalCols  int    `json:"total_cols"`
	TotalSeats int    `json:"total_seats"` // TotalRows × TotalCols
}

// SeatDefinition describes a physical seat in a hall (static, never changes).
type SeatDefinition struct {
	ID        string `json:"id"`
	HallID    string `json:"hall_id"`
	RowLabel  string `json:"row_label"`  // "A", "B" …
	ColNumber int    `json:"col_number"` // 1-based
	SeatType  string `json:"seat_type"`  // standard | premium | recliner
}

// Seat type constants
const (
	SeatTypeStandard = "standard"
	SeatTypePremium  = "premium"
	SeatTypeRecliner = "recliner"
)

// ──────────────────────────────────────────────
// Request / Response DTOs
// ──────────────────────────────────────────────

type CreateTheatreRequest struct {
	Name    string  `json:"name"    binding:"required,min=2,max=200"`
	City    string  `json:"city"    binding:"required"`
	Address string  `json:"address" binding:"omitempty"`
	Lat     float64 `json:"lat"     binding:"omitempty"`
	Lng     float64 `json:"lng"     binding:"omitempty"`
}

type UpdateTheatreRequest struct {
	Name    string  `json:"name"    binding:"omitempty,min=2,max=200"`
	City    string  `json:"city"    binding:"omitempty"`
	Address string  `json:"address" binding:"omitempty"`
	Lat     float64 `json:"lat"     binding:"omitempty"`
	Lng     float64 `json:"lng"     binding:"omitempty"`
}

type CreateHallRequest struct {
	Name      string `json:"name"       binding:"required,min=1,max=100"`
	TotalRows int    `json:"total_rows" binding:"required,min=1,max=50"`
	TotalCols int    `json:"total_cols" binding:"required,min=1,max=50"`
}

// TheatreFilter is used when listing theatres.
type TheatreFilter struct {
	City  string
	Page  int
	Limit int
}
