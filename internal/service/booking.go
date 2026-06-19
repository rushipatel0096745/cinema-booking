package services

import (
	"cinemabooking/internal/domain"
	"cinemabooking/internal/pkg/mailer"
	repositories "cinemabooking/internal/repository"
	"cinemabooking/internal/ws"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v85"
)

type BookingService struct {
	bookingRepo      repositories.BookingRepository
	userRepo         repositories.UserRepository
	showtimeRepo     repositories.ShowtimeRepository
	showtimeSeatRepo repositories.ShowtimeSeatRepository
	lockRepo         repositories.SeatLockRepository
	mailer           *mailer.Service

	stripeClient   *stripe.Client
	publishableKey string
	stripeService  StripeService
	hub            *ws.Hub
}

func NewBookingService(
	bookingRepo repositories.BookingRepository,
	userRepo repositories.UserRepository,
	showtimeRepo repositories.ShowtimeRepository,
	showtimeSeatRepo repositories.ShowtimeSeatRepository,
	lockRepo repositories.SeatLockRepository,
	stripeClient *stripe.Client,
	publishableKey string,
	stripeService StripeService,
	hub *ws.Hub,
	mailer *mailer.Service,
) *BookingService {
	return &BookingService{
		bookingRepo:      bookingRepo,
		userRepo:         userRepo,
		showtimeRepo:     showtimeRepo,
		showtimeSeatRepo: showtimeSeatRepo,
		lockRepo:         lockRepo,
		stripeClient:     stripeClient,
		publishableKey:   publishableKey,
		stripeService:    stripeService,
		hub:              hub,
		mailer:           mailer,
	}
}

func (s *BookingService) GetUserBookings(ctx context.Context, filter domain.BookingListFilter) ([]domain.Booking, int, error) {
	filter.Page, filter.Limit = domain.NormalisePage(filter.Page, filter.Limit)
	return s.bookingRepo.FindByUserID(ctx, filter)
}

func (s *BookingService) GetBooking(ctx context.Context, bookingID string, userID string) (*domain.Booking, error) {
	booking, err := s.bookingRepo.GetByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	// users can only see their own bookings — admins bypass this in a separate handler
	if booking.UserID != userID {
		return nil, domain.ErrForbidden
	}

	showtime, err := s.showtimeRepo.FindByID(ctx, booking.ShowtimeID)
	if err != nil {
		return nil, domain.NewAppError(http.StatusNotFound, "showtime not found")
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, domain.NewAppError(http.StatusNotFound, "user not found")
	}

	booking.Showtime = showtime
	booking.User = &domain.UserProfile{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
		Role:      user.Role,
	}

	return booking, nil
}

func (s *BookingService) CancelBooking(ctx context.Context, bookingID string, userID string, reason string) error {
	booking, err := s.bookingRepo.GetByID(ctx, bookingID)
	if err != nil {
		return err
	}

	if booking.UserID != userID {
		return domain.ErrForbidden
	}

	if booking.Status != domain.BookingStatusConfirmed {
		return domain.NewAppError(http.StatusConflict, "only confirmed bookings can be cancelled")
	}

	showtime, err := s.showtimeRepo.FindByID(ctx, booking.ShowtimeID)
	if err != nil {
		return domain.NewAppError(http.StatusNotFound, "showtime not found")
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return domain.NewAppError(http.StatusNotFound, "user not found")
	}

	booking.Showtime = showtime
	booking.User = &domain.UserProfile{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
		Role:      user.Role,
	}

	if !booking.IsCancellable() {
		return domain.ErrCancelWindowPassed
	}

	// release seats back to available
	if err := s.bookingRepo.ReleaseSeats(ctx, bookingID); err != nil {
		return fmt.Errorf("releasing seats: %w", err)
	}

	// update booking status
	if err := s.bookingRepo.UpdateStatus(ctx, bookingID, domain.BookingStatusCancelled); err != nil {
		return fmt.Errorf("updating booking status: %w", err)
	}

	seatIDs := make([]string, 0, len(booking.Seats))

	for _, seat := range booking.Seats {
		seatIDs = append(seatIDs, seat.ShowtimeSeatID)
	}

	s.hub.Broadcast(booking.ShowtimeID, domain.SeatStatusEvent{
		Type:       "seats_released",
		ShowtimeID: booking.ShowtimeID,
		SeatIDs:    seatIDs,
		Status:     domain.SeatStatusAvailable,
	})

	// send cancellation email — non-blocking
	go func() {
		err := s.mailer.SendBookingCancelled(context.Background(), domain.BookingCancelledPayload{
			User:         user.PublicProfile(),
			Booking:      *booking,
			Movie:        *showtime.Movie,
			RefundAmount: booking.TotalAmount,
			RefundDays:   5,
		})
		if err != nil {
			slog.Error("sending booking cancelled email",
				"booking_id", booking.ID,
				"error", err,
			)
		}
	}()

	return nil
}

func (s *BookingService) LockSeats(ctx context.Context, userID string, req domain.LockSeatsRequest) (*domain.LockSeatsResponse, error) {

	seats, err := s.showtimeSeatRepo.GetByIDs(ctx, req.ShowtimeID, req.SeatIDs)
	if err != nil {
		return nil, err
	}

	// ensure all requested seats exist
	if len(seats) != len(req.SeatIDs) {
		return nil, errors.New("one or more seats not found")
	}

	var bookedSeats []string
	var total float64

	for _, seat := range seats {
		fmt.Printf(
			"Seat=%s%d ID=%s Status=%s LockedBy=%s\n",
			seat.RowLabel,
			seat.ColNumber,
			seat.ID,
			seat.Status,
			seat.LockedBy,
		)
		if seat.Status == "booked" {
			bookedSeats = append(
				bookedSeats,
				fmt.Sprintf("%s%d",
					seat.RowLabel,
					seat.ColNumber,
				),
			)
			continue
		}

		total += seat.Price
	}

	if len(bookedSeats) > 0 {
		return nil, &domain.SeatUnavailableError{
			Seats: bookedSeats,
		}
	}

	lockTTL := 10 * time.Minute

	err = s.lockRepo.LockSeats(
		ctx,
		req.ShowtimeID,
		req.SeatIDs,
		userID,
		lockTTL,
	)
	if err != nil {
		return nil, err
	}

	// broadcast to all viewers of this showtime
	s.hub.Broadcast(req.ShowtimeID, domain.SeatStatusEvent{
		Type:       "seats_locked",
		ShowtimeID: req.ShowtimeID,
		SeatIDs:    req.SeatIDs,
		Status:     domain.SeatStatusLocked,
	})

	return &domain.LockSeatsResponse{
		SeatIDs:       req.SeatIDs,
		TotalAmount:   total,
		LockExpiresAt: time.Now().Add(lockTTL),
	}, nil
}

func (s *BookingService) CreateBooking(ctx context.Context, userID string, showtimeID string, seatIDs []string) (*domain.CreateBookingResponse, error) {

	for _, seatID := range seatIDs {

		owner, err := s.lockRepo.GetLockOwner(
			ctx,
			showtimeID,
			seatID,
		)
		if err != nil {
			return nil, err
		}

		if owner != userID {
			return nil, errors.New("seat lock expired or not owned by user")
		}
	}

	// Load seats
	seats, err := s.showtimeSeatRepo.GetByIDs(
		ctx,
		showtimeID,
		seatIDs,
	)
	if err != nil {
		return nil, err
	}

	if len(seats) != len(seatIDs) {
		return nil, errors.New("one or more seats not found")
	}

	var total float64

	bookedSeats := make(
		[]domain.BookedSeat,
		0,
		len(seats),
	)

	for _, seat := range seats {

		total += seat.Price

		bookedSeats = append(
			bookedSeats,
			domain.BookedSeat{
				ShowtimeSeatID: seat.ID,
				Price:          seat.Price,
				RowLabel:       seat.RowLabel,
				ColNumber:      seat.ColNumber,
				SeatType:       seat.SeatType,
			},
		)
	}

	booking := &domain.Booking{
		ID:          uuid.NewString(),
		UserID:      userID,
		ShowtimeID:  showtimeID,
		Status:      domain.BookingStatusPending,
		TotalAmount: total,
		CreatedAt:   time.Now(),
	}

	for i := range bookedSeats {
		bookedSeats[i].BookingID = booking.ID
	}

	// Save booking + booking seats
	if err := s.bookingRepo.Create(
		ctx,
		booking,
		bookedSeats,
	); err != nil {
		return nil, err
	}

	// Create Stripe PaymentIntent
	paymentResp, err := s.stripeService.CreatePaymentIntent(
		ctx,
		booking,
	)
	if err != nil {
		return nil, err
	}

	// Store PaymentIntent ID
	if err := s.bookingRepo.StorePaymentIntent(
		ctx,
		booking.ID,
		paymentResp.PaymentIntentID,
	); err != nil {
		return nil, err
	}

	return paymentResp, nil
}

func (s *BookingService) HandlePaymentSuccess(ctx context.Context, paymentIntentID string) error {

	booking, err := s.bookingRepo.GetByPaymentIntentID(ctx, paymentIntentID)
	if err != nil {
		return err
	}

	if booking == nil {
		return fmt.Errorf(
			"booking not found for payment intent %s",
			paymentIntentID,
		)
	}

	// webhook retry protection
	if booking.Status == "confirmed" {
		return nil
	}

	seats, err := s.bookingRepo.GetBookedSeats(ctx, booking.ID)
	if err != nil {
		return err
	}

	seatIDs := make([]string, 0, len(seats))

	for _, seat := range seats {
		seatIDs = append(seatIDs, seat.ShowtimeSeatID)
	}

	_, err = s.showtimeSeatRepo.MarkBooked(ctx, booking.ShowtimeID, seatIDs)
	if err != nil {
		return err
	}

	err = s.bookingRepo.MarkConfirmed(ctx, paymentIntentID)
	if err != nil {
		return err
	}

	err = s.lockRepo.ReleaseSeats(ctx, booking.ShowtimeID, seatIDs)
	if err != nil {
		return err
	}

	s.hub.Broadcast(booking.ShowtimeID, domain.SeatStatusEvent{
		Type:       "seats_booked",
		ShowtimeID: booking.ShowtimeID,
		SeatIDs:    seatIDs,
		Status:     domain.SeatStatusBooked,
	})

	showtime, err := s.showtimeRepo.FindByIDWithDetails(ctx, booking.ShowtimeID)
	if err != nil {
		return fmt.Errorf("loading showtime details: %w", err)
	}

	// load user for email
	user, err := s.userRepo.FindByID(ctx, booking.UserID)
	if err != nil {
		return fmt.Errorf("loading user for email: %w", err)
	}

	go func() {
		err := s.mailer.SendBookingConfirmed(context.Background(), domain.BookingConfirmedPayload{
			User:        user.PublicProfile(),
			Booking:     *booking,
			Showtime:    *showtime,
			Movie:       *showtime.Movie,
			Theatre:     *showtime.Theatre,
			Hall:        *showtime.Hall,
			Seats:       booking.Seats,
			QRCodeURL:   "https://example.com/qr/" + booking.ID, //later immplement QR code genearation
			TotalAmount: booking.TotalAmount,
		})
		if err != nil {
			slog.Error("sending booking confirmed email",
				"booking_id", booking.ID,
				"error", err,
			)
		}
	}()

	return nil
}

func (s *BookingService) HandlePaymentFailed(ctx context.Context, paymentIntentID string) error {
	booking, err := s.bookingRepo.GetByPaymentIntentID(
		ctx,
		paymentIntentID,
	)
	if err != nil {
		return err
	}

	if err := s.bookingRepo.ReleaseSeats(ctx, booking.ID); err != nil {
		return fmt.Errorf("releasing seats after payment failure: %w", err)
	}

	return s.bookingRepo.UpdateStatus(ctx, booking.ID, domain.BookingStatusCancelled)
}
