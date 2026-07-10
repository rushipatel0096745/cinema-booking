package repositories

import (
	"cinemabooking/internal/domain"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// var ErrNotFound = errors.New("not found")

type MovieRepository struct {
	db *pgxpool.Pool
}

func NewMovieRepository(db *pgxpool.Pool) *MovieRepository {
	return &MovieRepository{db: db}
}

const movieColumns = `id, title, description, duration_mins, genre, language,
	poster_url, backdrop_path, trailer_url, release_date, rating, tmdb_id, created_at`

func scanMovie(row pgx.Row, m *domain.Movie) error {
	return row.Scan(
		&m.ID, &m.Title, &m.Description, &m.DurationMin, &m.Genre, &m.Language,
		&m.PosterURL, &m.BackdropURL, &m.TrailerURL, &m.ReleaseDate, &m.Rating, &m.TmdbID, &m.CreatedAt,
	)
}

func (r *MovieRepository) FindAll(ctx context.Context, page, limit int) ([]domain.Movie, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+movieColumns+` FROM movies ORDER BY release_date DESC LIMIT $1 OFFSET $2`,
		limit, (page-1)*limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	movies := make([]domain.Movie, 0, limit)
	for rows.Next() {
		var m domain.Movie
		if err := scanMovie(rows, &m); err != nil {
			return nil, err
		}
		movies = append(movies, m)
	}
	return movies, rows.Err()
}

func (r *MovieRepository) FindByID(ctx context.Context, id string) (*domain.Movie, error) {
	movie := &domain.Movie{}
	row := r.db.QueryRow(ctx, `SELECT `+movieColumns+` FROM movies WHERE id = $1`, id)
	if err := scanMovie(row, movie); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return movie, nil
}

// FindWithFilters supports genre/language/search directly on movies, and
// city/date by checking for matching showtimes via an EXISTS subquery.
// Assumes halls.theatre_id -> theatres.id and theatres.city — adjust if your
// schema names these differently.
func (r *MovieRepository) FindWithFilters(ctx context.Context, f domain.MovieFilter) ([]domain.Movie, int, error) {
	var (
		conditions []string
		args       []any
	)
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if f.Genre != "" {
		conditions = append(conditions, fmt.Sprintf("%s = ANY(genre)", arg(f.Genre)))
	}
	if f.Language != "" {
		conditions = append(conditions, fmt.Sprintf("language = %s", arg(f.Language)))
	}
	if f.Search != "" {
		conditions = append(conditions, fmt.Sprintf("title ILIKE %s", arg("%"+f.Search+"%")))
	}
	if f.City != "" || f.Date != "" {
		var sub []string
		sub = append(sub, "showtimes.movie_id = movies.id", "showtimes.is_active = TRUE")
		if f.City != "" {
			sub = append(sub, fmt.Sprintf(`EXISTS (
				SELECT 1 FROM halls
				JOIN theatres ON theatres.id = halls.theatre_id
				WHERE halls.id = showtimes.hall_id AND theatres.city = %s
			)`, arg(f.City)))
		}
		if f.Date != "" {
			sub = append(sub, fmt.Sprintf("showtimes.starts_at::date = %s::date", arg(f.Date)))
		}
		conditions = append(conditions, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM showtimes WHERE %s)", strings.Join(sub, " AND "),
		))
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	page, limit := f.Page, f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	limitArg := arg(limit)
	offsetArg := arg((page - 1) * limit)

	query := fmt.Sprintf(
		`SELECT %s, COUNT(*) OVER() AS total FROM movies %s
         ORDER BY release_date DESC LIMIT %s OFFSET %s`,
		movieColumns, where, limitArg, offsetArg,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		movies []domain.Movie
		total  int
	)
	for rows.Next() {
		var m domain.Movie
		if err := rows.Scan(
			&m.ID, &m.Title, &m.Description, &m.DurationMin, &m.Genre, &m.Language,
			&m.PosterURL, &m.BackdropURL, &m.TrailerURL, &m.ReleaseDate, &m.Rating, &m.TmdbID, &m.CreatedAt,
			&total,
		); err != nil {
			return nil, 0, err
		}
		movies = append(movies, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return movies, total, nil
}

func (r *MovieRepository) Create(ctx context.Context, movie *domain.Movie) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO movies (title, description, duration_mins, genre, language, poster_url, backdrop_url, trailer_url, release_date, tmdb_id)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
         RETURNING id, rating, created_at`,
		movie.Title, movie.Description, movie.DurationMin, movie.Genre, movie.Language,
		movie.PosterURL, movie.BackdropURL, movie.TrailerURL, movie.ReleaseDate, movie.TmdbID,
	).Scan(&movie.ID, &movie.Rating, &movie.CreatedAt)
}

func (r *MovieRepository) Update(ctx context.Context, movie *domain.Movie) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE movies SET title = $1, description = $2, duration_mins = $3, genre = $4,
            language = $5, poster_url = $6, trailer_url = $7, release_date = $8
         WHERE id = $9`,
		movie.Title, movie.Description, movie.DurationMin, movie.Genre, movie.Language,
		movie.PosterURL, movie.TrailerURL, movie.ReleaseDate, movie.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MovieRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM movies WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// FindShowtimes returns active upcoming showtimes for a movie, with hall/theatre
// names attached for display. Adjust join columns if halls/theatres differ.
func (r *MovieRepository) FindShowtimes(ctx context.Context, movieID string, filter domain.ShowtimeFilter) ([]domain.Showtime, error) {

	_, err := r.FindByID(ctx, movieID)
	if err != nil {
		return nil, errors.New("Movie not found")
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			s.id, s.movie_id, s.hall_id, s.starts_at, s.ends_at, s.base_price, s.is_active,
			h.id, h.theatre_id, h.name, h.total_rows, h.total_cols,
			t.id, t.name, t.city, t.address, t.lat, t.lng,
			COUNT(ss.id) FILTER (WHERE ss.status = 'available') AS available_seats,
			COUNT(ss.id) FILTER (WHERE ss.status = 'booked')    AS booked_seats
		FROM showtimes s
		JOIN halls h ON h.id = s.hall_id
		JOIN theatres t ON t.id = h.theatre_id
		LEFT JOIN showtime_seats ss ON ss.showtime_id = s.id
		WHERE s.movie_id = $1
		  AND s.is_active = TRUE
		  AND s.starts_at > NOW()
		  AND ($2 = '' OR t.city ILIKE $2)
		  AND ($3 = '' OR DATE(s.starts_at) = $3::date)
		GROUP BY s.id, h.id, t.id
		ORDER BY t.city, s.starts_at ASC`,
		movieID, filter.City, filter.Date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var showtimes []domain.Showtime
	for rows.Next() {
		var st domain.Showtime
		var hall domain.Hall
		var theatre domain.Theatre

		if err := rows.Scan(
			&st.ID, &st.MovieID, &st.HallID, &st.StartsAt, &st.EndsAt, &st.BasePrice, &st.IsActive,
			&hall.ID, &hall.TheatreID, &hall.Name, &hall.TotalRows, &hall.TotalCols,
			&theatre.ID, &theatre.Name, &theatre.City, &theatre.Address, &theatre.Lat, &theatre.Lng,
			&st.AvailSeats, &st.BookedSeats,
		); err != nil {
			return nil, err
		}

		hall.TotalSeats = hall.TotalRows * hall.TotalCols
		st.Hall = &hall
		st.Theatre = &theatre
		showtimes = append(showtimes, st)
	}
	return showtimes, rows.Err()
}

// reviews
var ErrAlreadyReviewed = errors.New("user has already reviewed this movie")

func (r *MovieRepository) AddReview(ctx context.Context, review *domain.Review) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO reviews (user_id, movie_id, rating, body)
         VALUES ($1, $2, $3, $4)
         RETURNING id, created_at`,
		review.UserID, review.MovieID, review.Rating, review.Body,
	).Scan(&review.ID, &review.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return ErrAlreadyReviewed
			case "23503":
				return ErrNotFound
			}
		}
		return err
	}
	return nil
}

func (r *MovieRepository) FindReviews(ctx context.Context, movieID string, page, limit int) ([]domain.Review, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT r.id, r.user_id, r.movie_id, r.rating, r.body, r.created_at,
		       u.name, u.avatar_url
		FROM reviews r
		JOIN users u ON u.id = r.user_id
		WHERE r.movie_id = $1
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3`,
		movieID, limit, (page-1)*limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []domain.Review
	for rows.Next() {
		var rv domain.Review
		if err := rows.Scan(
			&rv.ID, &rv.UserID, &rv.MovieID, &rv.Rating, &rv.Body, &rv.CreatedAt,
			&rv.UserName, &rv.AvatarURL,
		); err != nil {
			return nil, err
		}
		reviews = append(reviews, rv)
	}
	return reviews, rows.Err()
}
