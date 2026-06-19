package repositories

import (
	"context"

	"cinemabooking/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ShowtimeSeatRepository interface {
	GetByIDs(ctx context.Context, showtimeID string, seatIDs []string) ([]domain.ShowtimeSeat, error)

	GetTotalAmount(ctx context.Context, showtimeID string, seatIDs []string) (float64, error)

	MarkBooked(ctx context.Context, showtimeID string, seatIDs []string) (int64, error)
}

type showtimeSeatRepository struct {
	db *pgxpool.Pool
}

func NewShowtimeSeatRepository(
	db *pgxpool.Pool,
) ShowtimeSeatRepository {
	return &showtimeSeatRepository{
		db: db,
	}
}

func (r *showtimeSeatRepository) GetByIDs(ctx context.Context, showtimeID string, seatIDs []string) ([]domain.ShowtimeSeat, error) {

	rows, err := r.db.Query(ctx, `
		SELECT
			ss.id,
			ss.showtime_id,
			ss.seat_def_id,
			ss.status,
			COALESCE(ss.locked_by::text, ''),
			ss.price,

			sd.row_label,
			sd.col_number,
			sd.seat_type
		FROM showtime_seats ss
		JOIN seat_definitions sd
			ON sd.id = ss.seat_def_id
		WHERE ss.showtime_id = $1
		AND ss.id = ANY($2)
	`, showtimeID, seatIDs)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seats []domain.ShowtimeSeat

	for rows.Next() {
		var seat domain.ShowtimeSeat

		err := rows.Scan(
			&seat.ID,
			&seat.ShowtimeID,
			&seat.SeatDefID,
			&seat.Status,
			&seat.LockedBy,
			&seat.Price,
			&seat.RowLabel,
			&seat.ColNumber,
			&seat.SeatType,
		)
		if err != nil {
			return nil, err
		}

		seats = append(seats, seat)
	}

	return seats, rows.Err()
}

func (r *showtimeSeatRepository) GetTotalAmount(ctx context.Context, showtimeID string, seatIDs []string) (float64, error) {
	var total float64

	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(price), 0)
		FROM showtime_seats
		WHERE showtime_id = $1
		  AND id = ANY($2)
	`,
		showtimeID,
		seatIDs,
	).Scan(&total)

	return total, err
}

func (r *showtimeSeatRepository) MarkBooked(ctx context.Context, showtimeID string, seatIDs []string) (int64, error) {

	tag, err := r.db.Exec(ctx, `
		UPDATE showtime_seats
		SET
			status = 'booked',
			locked_by = NULL
		WHERE showtime_id = $1
		  AND id = ANY($2)
		  AND status != 'booked'
	`, showtimeID, seatIDs)

	if err != nil {
		return 0, err
	}

	return tag.RowsAffected(), nil
}

