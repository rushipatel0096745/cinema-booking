package services

import (
	domain "cinemabooking/internal/domain"
	"context"

	"github.com/stripe/stripe-go/v85"
)

type StripeService interface {
	CreatePaymentIntent(
		ctx context.Context,
		booking *domain.Booking,
	) (*domain.CreateBookingResponse, error)
}

type stripeService struct {
	client         *stripe.Client
	publishableKey string
}

func NewStripeService(
	client *stripe.Client,
	publishableKey string,
) StripeService {
	return &stripeService{
		client:         client,
		publishableKey: publishableKey,
	}
}

func (s *stripeService) CreatePaymentIntent(ctx context.Context, booking *domain.Booking) (*domain.CreateBookingResponse, error) {
	params := &stripe.PaymentIntentCreateParams{
		Amount:   stripe.Int64(int64(booking.TotalAmount * 100)),
		Currency: stripe.String("inr"),
	}

	params.Metadata = map[string]string{
		"booking_id": booking.ID,
		"user_id":    booking.UserID,
	}

	intent, err := s.client.V1PaymentIntents.Create(
		ctx,
		params,
	)
	if err != nil {
		return nil, err
	}

	return &domain.CreateBookingResponse{
		BookingID:            booking.ID,
		PaymentIntentID:      intent.ID,
		ClientSecret:         intent.ClientSecret,
		TotalAmount:          booking.TotalAmount,
		Currency:             "inr",
		StripePublishableKey: s.publishableKey,
	}, nil
}
