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
	Create(ctx context.Context, theatre *domain.Theatre) (*domain.Theatre, error)
	CreateHall(ctx context.Context, hall *domain.Hall) (*domain.Hall, error)
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
