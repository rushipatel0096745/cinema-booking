// package services

// import (
// 	domain "cinemabooking/internal/domain"
// 	"context"
// 	 "crypto/tls"
//     "net/http"

// 	"github.com/stripe/stripe-go/v85"
// )

// type StripeService interface {
// 	CreatePaymentIntent(
// 		ctx context.Context,
// 		booking *domain.Booking,
// 	) (*domain.CreateBookingResponse, error)
// }

// type stripeService struct {
// 	client         *stripe.Client
// 	publishableKey string
// }

// func NewStripeService(
// 	client *stripe.Client,
// 	publishableKey string,
// ) StripeService {
// 	return &stripeService{
// 		client:         client,
// 		publishableKey: publishableKey,
// 	}
// }

// func (s *stripeService) CreatePaymentIntent(ctx context.Context, booking *domain.Booking) (*domain.CreateBookingResponse, error) {
// 	params := &stripe.PaymentIntentCreateParams{
// 		Amount:   stripe.Int64(int64(booking.TotalAmount * 100)),
// 		Currency: stripe.String("inr"),
// 	}

// 	params.Metadata = map[string]string{
// 		"booking_id": booking.ID,
// 		"user_id":    booking.UserID,
// 	}

// 	intent, err := s.client.V1PaymentIntents.Create(
// 		ctx,
// 		params,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &domain.CreateBookingResponse{
// 		BookingID:            booking.ID,
// 		PaymentIntentID:      intent.ID,
// 		ClientSecret:         intent.ClientSecret,
// 		TotalAmount:          booking.TotalAmount,
// 		Currency:             "inr",
// 		StripePublishableKey: s.publishableKey,
// 	}, nil
// }

package services

import (
	domain "cinemabooking/internal/domain"
	"context"
	"crypto/tls"
	"net/http"

	"github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/client"
)

type StripeService interface {
	CreatePaymentIntent(
		ctx context.Context,
		booking *domain.Booking,
	) (*domain.CreateBookingResponse, error)
	GetPaymentIntent(
		ctx context.Context,
		paymentIntentID string,
	) (*stripe.PaymentIntent, error)
}

type stripeService struct {
	client         *client.API
	publishableKey string
}

func NewStripeService(secretKey string, publishableKey string, skipTLS bool) StripeService {
	httpClient := &http.Client{}

	if skipTLS {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // dev only — macOS Tahoe beta TLS bug
			},
		}
	}

	backends := stripe.NewBackends(httpClient)
	sc := &client.API{}
	sc.Init(secretKey, &stripe.Backends{
		API:     backends.API,
		Connect: backends.Connect,
		Uploads: backends.Uploads,
	})

	return &stripeService{
		client:         sc,
		publishableKey: publishableKey,
	}
}

func (s *stripeService) CreatePaymentIntent(ctx context.Context, booking *domain.Booking) (*domain.CreateBookingResponse, error) {
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(int64(booking.TotalAmount * 100)),
		Currency: stripe.String("inr"),
		Params: stripe.Params{
			Metadata: map[string]string{
				"booking_id": booking.ID,
				"user_id":    booking.UserID,
			},
		},
	}

	intent, err := s.client.PaymentIntents.New(params)
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

func (s *stripeService) GetPaymentIntent(ctx context.Context, paymentIntentID string) (*stripe.PaymentIntent, error) {
	intent, err := s.client.PaymentIntents.Get(paymentIntentID, nil)
	if err != nil {
		return nil, err
	}
	return intent, nil
}
