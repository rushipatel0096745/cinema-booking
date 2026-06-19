// package repositories

// import (
// 	"cinemabooking/internal/domain"
// 	"context"

// 	"github.com/jackc/pgx/v5/pgxpool"
// )

// type ShowtimeRepository interface {
// 	FindByID(ctx context.Context, id string) (*domain.Showtime, error)
// 	FindAll(ctx context.Context, filter domain.ShowtimeFilter) ([]domain.Showtime, int, error)
// 	Create(ctx context.Context, showtime *domain.Showtime) (*domain.Showtime, error)
// 	Update(ctx context.Context, id string, showtime *domain.Showtime) (*domain.Showtime, error)
// 	Delete(ctx context.Context, id string) error
// 	GenerateSeats(ctx context.Context, showtimeID string, hallID string, basePrice float64) error
// }

// type showtimeRepository struct {
// 	db *pgxpool.Pool
// }

// func NewShowtimeRepository(db *pgxpool.Pool) *showtimeRepository {
// 	return &showtimeRepository{db: db}
// }

package repositories

import (
	"cinemabooking/internal/domain"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ShowtimeRepository struct {
	db *pgxpool.Pool
}

func NewShowtimeRepository(db *pgxpool.Pool) *ShowtimeRepository {
	return &ShowtimeRepository{db: db}
}

func (r *ShowtimeRepository) FindByID(ctx context.Context, id string) (*domain.Showtime, error) {
	var st domain.Showtime
	err := r.db.QueryRow(ctx, `
		SELECT s.id, s.movie_id, s.hall_id, s.starts_at, s.ends_at, s.base_price, s.is_active,
		       COUNT(*) FILTER (WHERE ss.status = 'available') AS avail_seats,
		       COUNT(*) FILTER (WHERE ss.status = 'booked') AS booked_seats
		FROM showtimes s
		LEFT JOIN showtime_seats ss ON ss.showtime_id = s.id
		WHERE s.id = $1
		GROUP BY s.id`, id,
	).Scan(&st.ID, &st.MovieID, &st.HallID, &st.StartsAt, &st.EndsAt, &st.BasePrice, &st.IsActive,
		&st.AvailSeats, &st.BookedSeats)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &st, nil
}

func (r *ShowtimeRepository) FindByIDWithDetails(ctx context.Context, id string) (*domain.Showtime, error) {
	var st domain.Showtime
	var movie domain.Movie
	var hall domain.Hall
	var theatre domain.Theatre

	// nullable columns
	var (
		moviePoster  pgtype.Text
		movieTrailer pgtype.Text
		theatreAddr  pgtype.Text
	)

	err := r.db.QueryRow(ctx, `
		SELECT
			s.id, s.movie_id, s.hall_id, s.starts_at, s.ends_at, s.base_price, s.is_active,
			COUNT(*) FILTER (WHERE ss.status = 'available') AS avail_seats,
			COUNT(*) FILTER (WHERE ss.status = 'booked')    AS booked_seats,
			m.id, m.title, m.description, m.duration_mins, m.language, m.poster_url, m.trailer_url, m.rating,
			h.id, h.name, h.total_rows, h.total_cols,
			t.id, t.name, t.city, t.address
		FROM showtimes s
		LEFT JOIN showtime_seats ss ON ss.showtime_id = s.id
		JOIN movies   m ON m.id = s.movie_id
		JOIN halls    h ON h.id = s.hall_id
		JOIN theatres t ON t.id = h.theatre_id
		WHERE s.id = $1
		GROUP BY s.id, m.id, h.id, t.id`, id,
	).Scan(
		&st.ID, &st.MovieID, &st.HallID, &st.StartsAt, &st.EndsAt, &st.BasePrice, &st.IsActive,
		&st.AvailSeats, &st.BookedSeats,
		&movie.ID, &movie.Title, &movie.Description, &movie.DurationMin, &movie.Language, &moviePoster, &movieTrailer, &movie.Rating,
		&hall.ID, &hall.Name, &hall.TotalRows, &hall.TotalCols,
		&theatre.ID, &theatre.Name, &theatre.City, &theatreAddr,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	movie.PosterURL = moviePoster.String
	movie.TrailerURL = movieTrailer.String
	theatre.Address = theatreAddr.String
	hall.TotalSeats = hall.TotalRows * hall.TotalCols

	st.Movie = &movie
	st.Hall = &hall
	st.Theatre = &theatre

	return &st, nil
}

// FindAll defaults to active showtimes only (s.is_active = TRUE) — flag if you
// want an admin path that can also see deactivated ones.
func (r *ShowtimeRepository) FindAll(ctx context.Context, filter domain.ShowtimeFilter) ([]domain.Showtime, int, error) {
	var (
		conditions = []string{"s.is_active = TRUE"}
		args       []any
		joins      string
	)
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if filter.MovieID != "" {
		conditions = append(conditions, fmt.Sprintf("s.movie_id = %s", arg(filter.MovieID)))
	}
	if filter.City != "" {
		joins = `JOIN halls h ON h.id = s.hall_id JOIN theatres t ON t.id = h.theatre_id`
		conditions = append(conditions, fmt.Sprintf("t.city = %s", arg(filter.City)))
	}
	if filter.Date != "" {
		conditions = append(conditions, fmt.Sprintf("s.starts_at::date = %s::date", arg(filter.Date)))
	}

	page, limit := filter.Page, filter.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	limitArg := arg(limit)
	offsetArg := arg((page - 1) * limit)

	query := fmt.Sprintf(`
		SELECT s.id, s.movie_id, s.hall_id, s.starts_at, s.ends_at, s.base_price, s.is_active,
		       COUNT(*) FILTER (WHERE ss.status = 'available') AS avail_seats,
		       COUNT(*) FILTER (WHERE ss.status = 'booked') AS booked_seats,
		       COUNT(*) OVER() AS total
		FROM showtimes s
		LEFT JOIN showtime_seats ss ON ss.showtime_id = s.id
		%s
		WHERE %s
		GROUP BY s.id
		ORDER BY s.starts_at ASC
		LIMIT %s OFFSET %s`,
		joins, strings.Join(conditions, " AND "), limitArg, offsetArg,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		showtimes []domain.Showtime
		total     int
	)
	for rows.Next() {
		var st domain.Showtime
		if err := rows.Scan(&st.ID, &st.MovieID, &st.HallID, &st.StartsAt, &st.EndsAt,
			&st.BasePrice, &st.IsActive, &st.AvailSeats, &st.BookedSeats, &total); err != nil {
			return nil, 0, err
		}
		showtimes = append(showtimes, st)
	}
	return showtimes, total, rows.Err()
}

func (r *ShowtimeRepository) Create(ctx context.Context, showtime *domain.Showtime) (*domain.Showtime, error) {
	err := r.db.QueryRow(ctx, `
		INSERT INTO showtimes (movie_id, hall_id, starts_at, ends_at, base_price, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		showtime.MovieID, showtime.HallID, showtime.StartsAt, showtime.EndsAt,
		showtime.BasePrice, showtime.IsActive,
	).Scan(&showtime.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, ErrNotFound // movie_id or hall_id doesn't exist
		}
		return nil, err
	}
	return showtime, nil
}

func (r *ShowtimeRepository) Update(ctx context.Context, id string, showtime *domain.Showtime) (*domain.Showtime, error) {
	err := r.db.QueryRow(ctx, `
		UPDATE showtimes
		SET movie_id = $1, hall_id = $2, starts_at = $3, ends_at = $4, base_price = $5, is_active = $6
		WHERE id = $7
		RETURNING id`,
		showtime.MovieID, showtime.HallID, showtime.StartsAt, showtime.EndsAt,
		showtime.BasePrice, showtime.IsActive, id,
	).Scan(&showtime.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return showtime, nil
}

func (r *ShowtimeRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM showtimes WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ShowtimeRepository) GenerateSeats(ctx context.Context, showtimeID string, hallID string, basePrice float64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO showtime_seats (showtime_id, seat_def_id, price, status)
		SELECT $1, sd.id,
		       CASE sd.seat_type
		           WHEN 'premium' THEN $2 * 1.5
		           WHEN 'recliner' THEN $2 * 2
		           ELSE $2
		       END,
		       'available'
		FROM seat_definitions sd
		WHERE sd.hall_id = $3
		ON CONFLICT (showtime_id, seat_def_id) DO NOTHING`,
		showtimeID, basePrice, hallID,
	)
	return err
}

func (r *ShowtimeRepository) FindSeatMap(ctx context.Context, showtimeID string) (*domain.SeatMap, error) {
	rows, err := r.db.Query(ctx, `
        SELECT
            ss.id,
            ss.status,
            ss.price,
            sd.row_label,
            sd.col_number,
            sd.seat_type
        FROM showtime_seats ss
        JOIN seat_definitions sd ON sd.id = ss.seat_def_id
        WHERE ss.showtime_id = $1
        ORDER BY sd.row_label, sd.col_number`,
		showtimeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// row label → index in Rows slice
	rowIndex := make(map[string]int)
	seatMap := &domain.SeatMap{ShowtimeID: showtimeID}

	for rows.Next() {
		var (
			id       string
			status   string
			price    float64
			rowLabel string
			colNum   int
			seatType string
		)

		if err := rows.Scan(&id, &status, &price, &rowLabel, &colNum, &seatType); err != nil {
			return nil, err
		}

		cell := domain.SeatCell{
			ID:     id,
			Col:    colNum,
			Type:   seatType,
			Status: status,
			Price:  price,
		}

		idx, exists := rowIndex[rowLabel]
		if !exists {
			seatMap.Rows = append(seatMap.Rows, domain.SeatRow{Label: rowLabel})
			idx = len(seatMap.Rows) - 1
			rowIndex[rowLabel] = idx
		}

		seatMap.Rows[idx].Seats = append(seatMap.Rows[idx].Seats, cell)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return seatMap, nil
}

func (r *ShowtimeRepository) HasConflict(ctx context.Context, hallID string, startsAt, endsAt time.Time, excludeID *string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM showtimes
			WHERE hall_id = $1
			AND ($2::uuid IS NULL OR id != $2::uuid)
			AND is_active = TRUE
			AND starts_at < $4
			AND ends_at > $3
		)`,
		hallID, excludeID, startsAt, endsAt,
	).Scan(&exists)
	return exists, err
}

func (r *ShowtimeRepository) HasConfirmedBookings(ctx context.Context, showtimeID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM bookings
            WHERE showtime_id = $1
              AND status      = 'confirmed'
        )`,
		showtimeID,
	).Scan(&exists)
	return exists, err
}
