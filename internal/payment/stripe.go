package payment

import "github.com/stripe/stripe-go/v85"

func NewStripeClient(secretKey string) *stripe.Client {
	return stripe.NewClient(secretKey)
}
