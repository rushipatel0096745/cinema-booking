package domain

import "time"

// ──────────────────────────────────────────────
// Admin analytics / reporting DTOs
// ──────────────────────────────────────────────

// RevenueReport is the admin revenue breakdown for a date range.
type RevenueReport struct {
	From         time.Time        `json:"from"`
	To           time.Time        `json:"to"`
	TotalRev     float64          `json:"total_revenue"`
	TotalTickets int              `json:"total_tickets"`
	ByMovie      []MovieRevenue   `json:"by_movie"`
	ByTheatre    []TheatreRevenue `json:"by_theatre"`
	Daily        []DailyRevenue   `json:"daily"`
}

// MovieRevenue is a single row in the per-movie breakdown.
type MovieRevenue struct {
	MovieID string  `json:"movie_id"`
	Title   string  `json:"title"`
	Tickets int     `json:"tickets"`
	Revenue float64 `json:"revenue"`
}

// TheatreRevenue is per-theatre.
type TheatreRevenue struct {
	TheatreID string  `json:"theatre_id"`
	Name      string  `json:"name"`
	City      string  `json:"city"`
	Tickets   int     `json:"tickets"`
	Revenue   float64 `json:"revenue"`
}

// DailyRevenue is one row of the time-series chart.
type DailyRevenue struct {
	Date    string  `json:"date"` // "2006-01-02"
	Revenue float64 `json:"revenue"`
	Tickets int     `json:"tickets"`
}

// OccupancyReport shows seat fill rates across showtimes.
type OccupancyReport struct {
	From         time.Time           `json:"from"`
	To           time.Time           `json:"to"`
	AvgOccupancy float64             `json:"avg_occupancy_pct"`
	ByShowtime   []ShowtimeOccupancy `json:"by_showtime"`
}

// ShowtimeOccupancy is a single showtime's fill rate.
type ShowtimeOccupancy struct {
	ShowtimeID   string    `json:"showtime_id"`
	MovieTitle   string    `json:"movie_title"`
	HallName     string    `json:"hall_name"`
	StartsAt     time.Time `json:"starts_at"`
	TotalSeats   int       `json:"total_seats"`
	BookedSeats  int       `json:"booked_seats"`
	OccupancyPct float64   `json:"occupancy_pct"`
}

// ──────────────────────────────────────────────
// Admin filter DTOs
// ──────────────────────────────────────────────

// DateRangeFilter is used by revenue and occupancy report endpoints.
type DateRangeFilter struct {
	From  string `form:"from"  binding:"required"` // "2006-01-02"
	To    string `form:"to"    binding:"required"`
	City  string `form:"city"`
	Page  int    `form:"page"`
	Limit int    `form:"limit"`
}

// AdminBookingFilter extends BookingListFilter with admin-only fields.
type AdminBookingFilter struct {
	BookingListFilter
	TheatreID string
	MovieID   string
	From      string
	To        string
}

// DashboardSummary is the data shown on the admin home dashboard.
type DashboardSummary struct {
	TotalRevToday     float64   `json:"total_revenue_today"`
	TotalTicketsToday int       `json:"total_tickets_today"`
	ActiveShowtimes   int       `json:"active_showtimes"`
	PendingBookings   int       `json:"pending_bookings"`
	TotalUsers        int       `json:"total_users"`
	RecentBookings    []Booking `json:"recent_bookings"`
}
