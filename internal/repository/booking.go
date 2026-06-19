package repositories

import (
	"cinemabooking/internal/domain"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BookingRepository interface {
	Create(ctx context.Context, booking *domain.Booking, seats []domain.BookedSeat) error
	GetByID(ctx context.Context, id string) (*domain.Booking, error)
	ListByUser(ctx context.Context, userID string, page, limit int) ([]domain.Booking, int, error)
	FindByUserID(ctx context.Context, filter domain.BookingListFilter) ([]domain.Booking, int, error)

	StorePaymentIntent(ctx context.Context, bookingID, paymentIntentID string) error
	GetByPaymentIntentID(ctx context.Context, paymentIntentID string) (*domain.Booking, error)

	MarkConfirmed(ctx context.Context, paymentIntentID string) error
	MarkCancelled(ctx context.Context, bookingID string) error
	MarkRefunded(ctx context.Context, bookingID string) error

	GetBookedSeats(ctx context.Context, bookingId string) ([]domain.BookedSeat, error)
	UpdateStatus(ctx context.Context, bookingID string, status string) error
	ReleaseSeats(ctx context.Context, bookingID string) error
}

type bookingRepository struct {
	db *pgxpool.Pool
}

func NewBookingRepository(db *pgxpool.Pool) BookingRepository {
	return &bookingRepository{db: db}
}

func (r *bookingRepository) Create(ctx context.Context, booking *domain.Booking, seats []domain.BookedSeat) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if booking.ID == "" {
		booking.ID = uuid.NewString()
	}
	if booking.CreatedAt.IsZero() {
		booking.CreatedAt = time.Now().UTC()
	}
	if booking.Status == "" {
		booking.Status = domain.BookingStatusPending
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO bookings (
			id, user_id, showtime_id, status, total_amount,
			stripe_payment_intent_id, qr_code_url, created_at, confirmed_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`,
		booking.ID,
		booking.UserID,
		booking.ShowtimeID,
		booking.Status,
		booking.TotalAmount,
		booking.StripePaymentIntent,
		booking.QRCodeURL,
		booking.CreatedAt,
		booking.ConfirmedAt,
	)
	if err != nil {
		return err
	}

	for i := range seats {
		if seats[i].BookingID == "" {
			seats[i].BookingID = booking.ID
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO booking_seats (
				booking_id, showtime_seat_id, price
			)
			VALUES ($1,$2,$3)
		`, seats[i].BookingID, seats[i].ShowtimeSeatID, seats[i].Price)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (r *bookingRepository) GetByID(ctx context.Context, id string) (*domain.Booking, error) {
	var booking domain.Booking

	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, showtime_id, status, total_amount,
		       stripe_payment_intent_id, qr_code_url, created_at, confirmed_at
		FROM bookings
		WHERE id = $1
	`, id).Scan(
		&booking.ID,
		&booking.UserID,
		&booking.ShowtimeID,
		&booking.Status,
		&booking.TotalAmount,
		&booking.StripePaymentIntent,
		&booking.QRCodeURL,
		&booking.CreatedAt,
		&booking.ConfirmedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("booking %s not found", id)
		}
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			bs.booking_id,
			bs.showtime_seat_id,
			bs.price,
			sd.row_label,
			sd.col_number,
			sd.seat_type
		FROM booking_seats bs
		JOIN showtime_seats ss
			ON ss.id = bs.showtime_seat_id
		JOIN seat_definitions sd
			ON sd.id = ss.seat_def_id
		WHERE bs.booking_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var seat domain.BookedSeat

		err := rows.Scan(
			&seat.BookingID,
			&seat.ShowtimeSeatID,
			&seat.Price,
			&seat.RowLabel,
			&seat.ColNumber,
			&seat.SeatType,
		)
		if err != nil {
			return nil, err
		}

		booking.Seats = append(booking.Seats, seat)
	}

	return &booking, rows.Err()
}

func (r *bookingRepository) ListByUser(ctx context.Context, userID string, page, limit int) ([]domain.Booking, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM bookings
		WHERE user_id = $1
	`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, showtime_id, status, total_amount,
		       stripe_payment_intent_id, qr_code_url, created_at, confirmed_at
		FROM bookings
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	bookings := make([]domain.Booking, 0)
	for rows.Next() {
		var b domain.Booking
		if err := rows.Scan(
			&b.ID,
			&b.UserID,
			&b.ShowtimeID,
			&b.Status,
			&b.TotalAmount,
			&b.StripePaymentIntent,
			&b.QRCodeURL,
			&b.CreatedAt,
			&b.ConfirmedAt,
		); err != nil {
			return nil, 0, err
		}
		bookings = append(bookings, b)
	}

	return bookings, total, rows.Err()
}

func (r *bookingRepository) FindByUserID(ctx context.Context, filter domain.BookingListFilter) ([]domain.Booking, int, error) {
	offset := (filter.Page - 1) * filter.Limit

	rows, err := r.db.Query(ctx, `
		SELECT
			b.id, b.user_id, b.showtime_id, b.status, b.total_amount,
			b.stripe_payment_intent_id, b.qr_code_url, b.created_at, b.confirmed_at,
			m.title AS movie_title,
			m.poster_url AS movie_poster,
			t.name  AS theatre_name,
			t.city  AS theatre_city,
			s.starts_at,
			COUNT(*) OVER() AS total_count
		FROM bookings b
		JOIN showtimes st ON st.id = b.showtime_id
		JOIN movies    m  ON m.id  = st.movie_id
		JOIN halls     h  ON h.id  = st.hall_id
		JOIN theatres  t  ON t.id  = h.theatre_id
		JOIN showtimes s  ON s.id  = b.showtime_id
		WHERE b.user_id = $1
		  AND ($2 = '' OR b.status = $2)
		ORDER BY b.created_at DESC
		LIMIT $3 OFFSET $4`,
		filter.UserID, filter.Status, filter.Limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var bookings []domain.Booking
	var total int

	for rows.Next() {
		var b domain.Booking
		var showtime domain.Showtime
		var movie domain.Movie
		var theatre domain.Theatre
		var confirmedAt pgtype.Timestamptz
		var qrURL, stripePI pgtype.Text

		err := rows.Scan(
			&b.ID, &b.UserID, &b.ShowtimeID, &b.Status, &b.TotalAmount,
			&stripePI, &qrURL, &b.CreatedAt, &confirmedAt,
			&movie.Title, &movie.PosterURL,
			&theatre.Name, &theatre.City,
			&showtime.StartsAt,
			&total,
		)
		if err != nil {
			return nil, 0, err
		}

		b.StripePaymentIntent = stripePI.String
		b.QRCodeURL = qrURL.String
		if confirmedAt.Valid {
			t := confirmedAt.Time
			b.ConfirmedAt = &t
		}

		showtime.Movie = &movie
		showtime.Theatre = &theatre
		b.Showtime = &showtime
		bookings = append(bookings, b)
	}

	return bookings, total, rows.Err()
}

func (r *bookingRepository) StorePaymentIntent(ctx context.Context, bookingID, paymentIntentID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE bookings
		SET stripe_payment_intent_id = $2
		WHERE id = $1
	`, bookingID, paymentIntentID)
	return err
}

func (r *bookingRepository) GetByPaymentIntentID(ctx context.Context, paymentIntentID string) (*domain.Booking, error) {
	var booking domain.Booking

	err := r.db.QueryRow(ctx, `
		SELECT
			id,
			user_id,
			showtime_id,
			status,
			total_amount,
			stripe_payment_intent_id,
			qr_code_url,
			created_at,
			confirmed_at
		FROM bookings
		WHERE stripe_payment_intent_id = $1
	`, paymentIntentID).Scan(
		&booking.ID,
		&booking.UserID,
		&booking.ShowtimeID,
		&booking.Status,
		&booking.TotalAmount,
		&booking.StripePaymentIntent,
		&booking.QRCodeURL,
		&booking.CreatedAt,
		&booking.ConfirmedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			booking_id,
			showtime_seat_id,
			price
		FROM booking_seats
		WHERE booking_id = $1
	`, booking.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var seat domain.BookedSeat

		if err := rows.Scan(
			&seat.BookingID,
			&seat.ShowtimeSeatID,
			&seat.Price,
		); err != nil {
			return nil, err
		}

		booking.Seats = append(booking.Seats, seat)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &booking, nil
}

func (r *bookingRepository) MarkConfirmed(ctx context.Context, paymentIntentID string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
		UPDATE bookings
		SET status = $1, confirmed_at = $2
		WHERE stripe_payment_intent_id = $3 AND status != $1
	`, domain.BookingStatusConfirmed, now, paymentIntentID)
	return err
}

func (r *bookingRepository) MarkCancelled(ctx context.Context, bookingID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE bookings
		SET status = $2
		WHERE id = $1
	`, bookingID, domain.BookingStatusCancelled)
	return err
}

func (r *bookingRepository) MarkRefunded(ctx context.Context, bookingID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE bookings
		SET status = $2
		WHERE id = $1
	`, bookingID, domain.BookingStatusRefunded)
	return err
}

func (r *bookingRepository) GetBookedSeats(ctx context.Context, bookingId string) ([]domain.BookedSeat, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			booking_id,
			showtime_seat_id,
			price
		FROM booking_seats
		WHERE booking_id = $1
	`, bookingId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookedSeats []domain.BookedSeat

	for rows.Next() {
		var bookedSeat domain.BookedSeat

		err := rows.Scan(
			&bookedSeat.BookingID,
			&bookedSeat.ShowtimeSeatID,
			&bookedSeat.Price,
		)
		if err != nil {
			return nil, err
		}

		bookedSeats = append(bookedSeats, bookedSeat)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return bookedSeats, nil
}

func (r *bookingRepository) ReleaseSeats(ctx context.Context, bookingID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE showtime_seats ss
		SET status = 'available', locked_by = NULL, booking_id = NULL
		FROM booking_seats bs
		WHERE bs.booking_id = $1
		  AND bs.showtime_seat_id = ss.id`,
		bookingID,
	)
	return err
}

func (r *bookingRepository) UpdateStatus(ctx context.Context, bookingID string, status string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE bookings SET status = $1 WHERE id = $2`,
		status, bookingID,
	)
	return err
}
