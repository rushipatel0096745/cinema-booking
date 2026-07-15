package repositories

import (
	"cinemabooking/internal/domain"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TheatreRepository interface {
	FindAll(ctx context.Context, filter domain.TheatreFilter) ([]domain.Theatre, int, error)
	FindByID(ctx context.Context, id string) (*domain.Theatre, error)
	FindHalls(ctx context.Context, theatreID string) ([]domain.Hall, error)
	FindHallByID(ctx context.Context, hallID string) (*domain.Hall, error)
	FindAllCities(ctx context.Context) ([]string, error)
	Create(ctx context.Context, theatre *domain.Theatre) (*domain.Theatre, error)
	CreateHall(ctx context.Context, hall *domain.Hall) (*domain.Hall, error)
	GenerateSeatDefinitions(ctx context.Context, hallID string, totalRows int, totalCols int) error
}

type theatreRepository struct {
	db *pgxpool.Pool
}

func NewTheatreRepository(db *pgxpool.Pool) *theatreRepository {
	return &theatreRepository{db: db}
}

func (r *theatreRepository) FindAll(ctx context.Context, filter domain.TheatreFilter) ([]domain.Theatre, int, error) {
	query := `SELECT id, name, city, address, lat, lng, COUNT(*) OVER() AS total FROM theatres`
	var args []any
	if filter.City != "" {
		args = append(args, filter.City)
		query += fmt.Sprintf(` WHERE city = $%d`, len(args))
	}
	query += ` ORDER BY name ASC`

	page, limit := filter.Page, filter.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	args = append(args, limit, (page-1)*limit)
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)-1, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	theatres := []domain.Theatre{}
	var total int
	for rows.Next() {
		var t domain.Theatre
		t.Halls = []domain.Hall{} // always send [] not null
		if err := rows.Scan(&t.ID, &t.Name, &t.City, &t.Address, &t.Lat, &t.Lng, &total); err != nil {
			return nil, 0, err
		}
		theatres = append(theatres, t)
	}
	return theatres, total, rows.Err()
}

func (r *theatreRepository) FindByID(ctx context.Context, id string) (*domain.Theatre, error) {
	t := &domain.Theatre{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, city, address, lat, lng FROM theatres WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.City, &t.Address, &t.Lat, &t.Lng)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return t, nil
}

func (r *theatreRepository) FindAllCities(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT city FROM theatres ORDER BY city ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cities := []string{}
	for rows.Next() {
		var city string
		if err := rows.Scan(&city); err != nil {
			return nil, err
		}
		cities = append(cities, city)
	}
	return cities, rows.Err()
}

func (r *theatreRepository) FindHalls(ctx context.Context, theatreID string) ([]domain.Hall, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, theatre_id, name, total_rows, total_cols
         FROM halls WHERE theatre_id = $1 ORDER BY name ASC`,
		theatreID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	halls := []domain.Hall{}
	for rows.Next() {
		var h domain.Hall
		if err := rows.Scan(&h.ID, &h.TheatreID, &h.Name, &h.TotalRows, &h.TotalCols); err != nil {
			return nil, err
		}
		h.TotalSeats = h.TotalRows * h.TotalCols
		halls = append(halls, h)
	}
	return halls, rows.Err()
}

func (r *theatreRepository) FindHallByID(ctx context.Context, hallID string) (*domain.Hall, error) {
	hall := &domain.Hall{}
	err := r.db.QueryRow(ctx, `
        SELECT id, theatre_id, name, total_rows, total_cols
        FROM halls WHERE id = $1`, hallID,
	).Scan(&hall.ID, &hall.TheatreID, &hall.Name, &hall.TotalRows, &hall.TotalCols)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	hall.TotalSeats = hall.TotalRows * hall.TotalCols
	return hall, nil
}

func (r *theatreRepository) Create(ctx context.Context, theatre *domain.Theatre) (*domain.Theatre, error) {
	err := r.db.QueryRow(ctx,
		`INSERT INTO theatres (name, city, address, lat, lng)
         VALUES ($1, $2, $3, $4, $5)
         RETURNING id`,
		theatre.Name, theatre.City, theatre.Address, theatre.Lat, theatre.Lng,
	).Scan(&theatre.ID)
	if err != nil {
		return nil, err
	}
	return theatre, nil
}

func (r *theatreRepository) CreateHall(ctx context.Context, hall *domain.Hall) (*domain.Hall, error) {
	err := r.db.QueryRow(ctx,
		`INSERT INTO halls (theatre_id, name, total_rows, total_cols)
         VALUES ($1, $2, $3, $4)
         RETURNING id`,
		hall.TheatreID, hall.Name, hall.TotalRows, hall.TotalCols,
	).Scan(&hall.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	hall.TotalSeats = hall.TotalRows * hall.TotalCols
	return hall, nil
}

func (r *theatreRepository) GenerateSeatDefinitions(ctx context.Context, hallID string, totalRows int, totalCols int) error {
	totalSeats := totalRows * totalCols

	// Pre-allocate slices — one entry per seat
	hallIDs := make([]string, 0, totalSeats)
	rowLabels := make([]string, 0, totalSeats)
	colNumbers := make([]int, 0, totalSeats)
	seatTypes := make([]string, 0, totalSeats)

	for r := 0; r < totalRows; r++ {
		rowLabel := string(rune('A' + r))
		st := seatTypeForRow(rowLabel)
		for col := 1; col <= totalCols; col++ {
			hallIDs = append(hallIDs, hallID)
			rowLabels = append(rowLabels, rowLabel)
			colNumbers = append(colNumbers, col)
			seatTypes = append(seatTypes, st)
		}
	}

	// Single round-trip: UNNEST expands the arrays into rows server-side
	_, err := r.db.Exec(ctx, `
		INSERT INTO seat_definitions (hall_id, row_label, col_number, seat_type)
		SELECT
			UNNEST($1::uuid[]),
			UNNEST($2::text[]),
			UNNEST($3::int[]),
			UNNEST($4::text[])
		ON CONFLICT (hall_id, row_label, col_number) DO NOTHING`,
		hallIDs, rowLabels, colNumbers, seatTypes,
	)
	if err != nil {
		return fmt.Errorf("generating seat definitions: %w", err)
	}
	return nil
}

func seatTypeForRow(rowLabel string) string {
	switch rowLabel {
	case "A", "B":
		return domain.SeatTypeRecliner
	case "C", "D":
		return domain.SeatTypePremium
	default:
		return domain.SeatTypeStandard
	}
}
