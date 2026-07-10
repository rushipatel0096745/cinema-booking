package services

import (
	"cinemabooking/internal/domain"
	"cinemabooking/internal/pkg/mailer"
	"cinemabooking/internal/pkg/qr"
	"cinemabooking/internal/pkg/storage"
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
	qrService      *qr.QrService
	storageService *storage.Service
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
	qrService *qr.QrService,
	storageService *storage.Service,
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
		qrService:        qrService,
		storageService:   storageService,
	}
}

func (s *BookingService) GetUserBookings(ctx context.Context, filter domain.BookingListFilter) ([]domain.Booking, int, error) {
	filter.Page, filter.Limit = domain.NormalisePage(filter.Page, filter.Limit)
	return s.bookingRepo.FindByUserID(ctx, filter)
}

func (s *BookingService) GetBookingById(ctx context.Context, bookingId string) (*domain.Booking, error) {
	return s.bookingRepo.GetByID(ctx, bookingId)
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

	// Sync with Stripe if the booking is pending and has a PaymentIntent ID
	if booking.Status == domain.BookingStatusPending && booking.StripePaymentIntent != "" {
		slog.Info("syncing booking status with stripe", "booking_id", booking.ID, "payment_intent_id", booking.StripePaymentIntent)
		intent, err := s.stripeService.GetPaymentIntent(ctx, booking.StripePaymentIntent)
		if err != nil {
			slog.Error("failed to retrieve payment intent from stripe", "booking_id", booking.ID, "error", err)
		} else if intent != nil {
			slog.Info("retrieved stripe payment intent status", "booking_id", booking.ID, "status", intent.Status)
			if intent.Status == "succeeded" {
				if err := s.HandlePaymentSuccess(ctx, booking.StripePaymentIntent); err != nil {
					slog.Error("failed to handle payment success from stripe sync", "booking_id", booking.ID, "error", err)
				} else {
					slog.Info("successfully confirmed booking from stripe sync", "booking_id", booking.ID)
					if reloaded, err := s.bookingRepo.GetByID(ctx, bookingID); err == nil {
						booking = reloaded
					}
				}
			} else if intent.Status == "canceled" {
				if err := s.HandlePaymentFailed(ctx, booking.StripePaymentIntent); err != nil {
					slog.Error("failed to handle payment failure from stripe sync", "booking_id", booking.ID, "error", err)
				} else {
					slog.Info("successfully cancelled booking from stripe sync", "booking_id", booking.ID)
					if reloaded, err := s.bookingRepo.GetByID(ctx, bookingID); err == nil {
						booking = reloaded
					}
				}
			}
		}
	}

	showtime, err := s.showtimeRepo.FindByIDWithDetails(ctx, booking.ShowtimeID)
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

	showtime, err := s.showtimeRepo.FindByIDWithDetails(ctx, booking.ShowtimeID)
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

// func (s *BookingService) CreateBooking(ctx context.Context, userID string, showtimeID string, seatIDs []string) (*domain.CreateBookingResponse, error) {

// 	for _, seatID := range seatIDs {

// 		owner, err := s.lockRepo.GetLockOwner(
// 			ctx,
// 			showtimeID,
// 			seatID,
// 		)
// 		if err != nil {
// 			return nil, err
// 		}

// 		if owner != userID {
// 			return nil, errors.New("seat lock expired or not owned by user")
// 		}
// 	}

// 	// Load seats
// 	seats, err := s.showtimeSeatRepo.GetByIDs(
// 		ctx,
// 		showtimeID,
// 		seatIDs,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}

// 	if len(seats) != len(seatIDs) {
// 		return nil, errors.New("one or more seats not found")
// 	}

// 	var total float64

// 	bookedSeats := make(
// 		[]domain.BookedSeat,
// 		0,
// 		len(seats),
// 	)

// 	for _, seat := range seats {

// 		total += seat.Price

// 		bookedSeats = append(
// 			bookedSeats,
// 			domain.BookedSeat{
// 				ShowtimeSeatID: seat.ID,
// 				Price:          seat.Price,
// 				RowLabel:       seat.RowLabel,
// 				ColNumber:      seat.ColNumber,
// 				SeatType:       seat.SeatType,
// 			},
// 		)
// 	}

// 	booking := &domain.Booking{
// 		ID:          uuid.NewString(),
// 		UserID:      userID,
// 		ShowtimeID:  showtimeID,
// 		Status:      domain.BookingStatusPending,
// 		TotalAmount: total,
// 		CreatedAt:   time.Now(),
// 	}

// 	for i := range bookedSeats {
// 		bookedSeats[i].BookingID = booking.ID
// 	}

// 	// Save booking + booking seats
// 	if err := s.bookingRepo.Create(
// 		ctx,
// 		booking,
// 		bookedSeats,
// 	); err != nil {
// 		return nil, err
// 	}

// 	// Create Stripe PaymentIntent
// 	paymentResp, err := s.stripeService.CreatePaymentIntent(
// 		ctx,
// 		booking,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Store PaymentIntent ID
// 	if err := s.bookingRepo.StorePaymentIntent(
// 		ctx,
// 		booking.ID,
// 		paymentResp.PaymentIntentID,
// 	); err != nil {
// 		return nil, err
// 	}

// 	return paymentResp, nil
// }

func (s *BookingService) CreateBooking(ctx context.Context, userID string, showtimeID string, seatIDs []string) (*domain.CreateBookingResponse, error) {

	// verify all seat locks are owned by this user
	for _, seatID := range seatIDs {
		owner, err := s.lockRepo.GetLockOwner(ctx, showtimeID, seatID)
		if err != nil {
			return nil, err
		}
		if owner != userID {
			return nil, domain.ErrLockExpired
		}
	}

	// load seats
	seats, err := s.showtimeSeatRepo.GetByIDs(ctx, showtimeID, seatIDs)
	if err != nil {
		return nil, err
	}
	if len(seats) != len(seatIDs) {
		return nil, errors.New("one or more seats not found")
	}

	// build price summary
	summary := &domain.PriceSummary{}
	bookedSeats := make([]domain.BookedSeat, 0, len(seats))

	for _, seat := range seats {
		summary.Items = append(summary.Items, domain.PriceLineItem{
			ShowtimeSeatID: seat.ID,
			RowLabel:       seat.RowLabel,
			ColNumber:      seat.ColNumber,
			SeatType:       seat.SeatType,
			BasePrice:      seat.Price,
		})
		bookedSeats = append(bookedSeats, domain.BookedSeat{
			ShowtimeSeatID: seat.ID,
			Price:          seat.Price,
			RowLabel:       seat.RowLabel,
			ColNumber:      seat.ColNumber,
			SeatType:       seat.SeatType,
		})
	}

	// compute subtotal + convenience fee + GST in one call
	summary.ComputeTotal()

	booking := &domain.Booking{
		ID:             uuid.NewString(),
		UserID:         userID,
		ShowtimeID:     showtimeID,
		Status:         domain.BookingStatusPending,
		Subtotal:       summary.SubTotal,
		ConvenienceFee: summary.ConvenienceFee,
		Taxes:          summary.Taxes,
		TotalAmount:    summary.Total,
		CreatedAt:      time.Now(),
	}

	for i := range bookedSeats {
		bookedSeats[i].BookingID = booking.ID
	}

	// save booking + seats
	if err := s.bookingRepo.Create(ctx, booking, bookedSeats); err != nil {
		return nil, err
	}

	// create Stripe PaymentIntent with grand total
	paymentResp, err := s.stripeService.CreatePaymentIntent(ctx, booking)
	if err != nil {
		// clean up orphan booking
		_ = s.bookingRepo.Delete(ctx, booking.ID)
		return nil, fmt.Errorf("creating payment intent: %w", err)
	}

	// store PaymentIntent ID
	if err := s.bookingRepo.StorePaymentIntent(ctx, booking.ID, paymentResp.PaymentIntentID); err != nil {
		_ = s.bookingRepo.Delete(ctx, booking.ID)
		return nil, fmt.Errorf("storing payment intent: %w", err)
	}

	return &domain.CreateBookingResponse{
		BookingID:            booking.ID,
		ClientSecret:         paymentResp.ClientSecret,
		Currency:             "inr",
		StripePublishableKey: paymentResp.StripePublishableKey,
		Breakdown:            summary,
	}, nil
}

// func (s *BookingService) HandlePaymentSuccess(ctx context.Context, paymentIntentID string) error {

// 	booking, err := s.bookingRepo.GetByPaymentIntentID(ctx, paymentIntentID)
// 	if err != nil {
// 		return err
// 	}

// 	if booking == nil {
// 		return fmt.Errorf(
// 			"booking not found for payment intent %s",
// 			paymentIntentID,
// 		)
// 	}

// 	// webhook retry protection
// 	if booking.Status == "confirmed" {
// 		return nil
// 	}

// 	seats, err := s.bookingRepo.GetBookedSeats(ctx, booking.ID)
// 	if err != nil {
// 		return err
// 	}

// 	seatIDs := make([]string, 0, len(seats))

// 	for _, seat := range seats {
// 		seatIDs = append(seatIDs, seat.ShowtimeSeatID)
// 	}

// 	s.hub.Broadcast(booking.ShowtimeID, domain.SeatStatusEvent{
// 		Type:       "seats_booked",
// 		ShowtimeID: booking.ShowtimeID,
// 		SeatIDs:    seatIDs,
// 		Status:     domain.SeatStatusBooked,
// 	})

// 	showtime, err := s.showtimeRepo.FindByIDWithDetails(ctx, booking.ShowtimeID)
// 	if err != nil {
// 		return fmt.Errorf("loading showtime details: %w", err)
// 	}

// 	// load user for email
// 	user, err := s.userRepo.FindByID(ctx, booking.UserID)
// 	if err != nil {
// 		return fmt.Errorf("loading user for email: %w", err)
// 	}

// 	// generate QR PNG bytes
// 	fmt.Println("generating qr for booking..........")
// 	qrPNG, err := s.qrService.Generate(
// 		booking.ID,
// 		booking.UserID,
// 		booking.ShowtimeID,
// 		showtime.EndsAt.Unix(),
// 	)
// 	if err != nil {
// 		return fmt.Errorf("generating qr: %w", err)
// 	}

// 	fmt.Println("uploading qr for booking..........")
// 	qrUrl, err := s.storageService.UploadQR(ctx, booking.ID, qrPNG)
// 	if err != nil {
// 		return fmt.Errorf("uploading qr: %w", err)
// 	}

// 	fmt.Print("updating seats status in showtime..........")
// 	_, err = s.showtimeSeatRepo.MarkBooked(ctx, booking.ShowtimeID, seatIDs)
// 	if err != nil {
// 		return err
// 	}

// 	fmt.Print("updating booking status in bookings..........")
// 	err = s.bookingRepo.MarkConfirmed(ctx, paymentIntentID, qrUrl)
// 	if err != nil {
// 		return err
// 	}

// 	fmt.Println("releasing seats in redis.........")
// 	err = s.lockRepo.ReleaseSeats(ctx, booking.ShowtimeID, seatIDs)
// 	if err != nil {
// 		return err
// 	}

// 	go func() {
// 		err := s.mailer.SendBookingConfirmed(context.Background(), domain.BookingConfirmedPayload{
// 			User:        user.PublicProfile(),
// 			Booking:     *booking,
// 			Showtime:    *showtime,
// 			Movie:       *showtime.Movie,
// 			Theatre:     *showtime.Theatre,
// 			Hall:        *showtime.Hall,
// 			Seats:       booking.Seats,
// 			QRCodeURL:   qrUrl,
// 			TotalAmount: booking.TotalAmount,
// 		})
// 		if err != nil {
// 			slog.Error("sending booking confirmed email",
// 				"booking_id", booking.ID,
// 				"error", err,
// 			)
// 		}
// 	}()

// 	return nil
// }

func (s *BookingService) HandlePaymentSuccess(ctx context.Context, paymentIntentID string) error {
	booking, err := s.bookingRepo.GetByPaymentIntentID(ctx, paymentIntentID)
	if err != nil {
		return err
	}
	if booking.Status == domain.BookingStatusConfirmed {
		return nil // idempotency — already processed
	}

	// 1. confirm booking + mark seats in DB first — fast operations
	if err := s.bookingRepo.MarkConfirmed(ctx, paymentIntentID, ""); err != nil {
		return fmt.Errorf("confirming booking: %w", err)
	}

	seats, err := s.bookingRepo.GetBookedSeats(ctx, booking.ID)
	if err != nil {
		return err
	}
	seatIDs := make([]string, 0, len(seats))
	for _, seat := range seats {
		seatIDs = append(seatIDs, seat.ShowtimeSeatID)
	}

	if _, err := s.showtimeSeatRepo.MarkBooked(ctx, booking.ShowtimeID, seatIDs); err != nil {
		return err
	}

	if err := s.lockRepo.ReleaseSeats(ctx, booking.ShowtimeID, seatIDs); err != nil {
		return err
	}

	// broadcast immediately
	s.hub.Broadcast(booking.ShowtimeID, domain.SeatStatusEvent{
		Type:       "seats_booked",
		ShowtimeID: booking.ShowtimeID,
		SeatIDs:    seatIDs,
		Status:     domain.SeatStatusBooked,
	})

	// 2. QR gen + upload + email in background — these are slow
	go func() {
		bgCtx := context.Background()

		showtime, err := s.showtimeRepo.FindByIDWithDetails(bgCtx, booking.ShowtimeID)
		if err != nil {
			slog.Error("loading showtime for qr", "error", err)
			return
		}

		qrPNG, err := s.qrService.Generate(
			booking.ID, booking.UserID,
			booking.ShowtimeID, showtime.EndsAt.Unix(),
		)
		if err != nil {
			slog.Error("generating qr", "booking_id", booking.ID, "error", err)
			return
		}

		qrURL, err := s.storageService.UploadQR(bgCtx, booking.ID, qrPNG)
		if err != nil {
			slog.Error("uploading qr", "booking_id", booking.ID, "error", err)
			return
		}

		// update qr_code_url now that we have it
		if err := s.bookingRepo.UpdateQRURL(bgCtx, booking.ID, qrURL); err != nil {
			slog.Error("updating qr url", "booking_id", booking.ID, "error", err)
			return
		}

		user, err := s.userRepo.FindByID(bgCtx, booking.UserID)
		if err != nil {
			slog.Error("loading user for email", "error", err)
			return
		}

		if err := s.mailer.SendBookingConfirmed(bgCtx, domain.BookingConfirmedPayload{
			User:        user.PublicProfile(),
			Booking:     *booking,
			Showtime:    *showtime,
			Movie:       *showtime.Movie,
			Theatre:     *showtime.Theatre,
			Hall:        *showtime.Hall,
			Seats:       booking.Seats,
			QRCodeURL:   qrURL,
			TotalAmount: booking.TotalAmount,
		}); err != nil {
			slog.Error("sending confirmation email", "booking_id", booking.ID, "error", err)
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

func (s *BookingService) VerifyQR(payload qr.Payload) bool {
	return s.qrService.Verify(payload)
}
