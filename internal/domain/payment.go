package domain

import "time"

// Payment status constants — mirrors Stripe's PaymentIntent statuses
// but kept as our own so we're not coupled to Stripe's exact strings.
const (
	PaymentStatusPending   = "pending"
	PaymentStatusSucceeded = "succeeded"
	PaymentStatusFailed    = "failed"
	PaymentStatusRefunded  = "refunded"
)

// Payment is an internal record of every payment attempt.
// Created when we create a Stripe PaymentIntent, updated by webhooks.
type Payment struct {
	ID                  string     `json:"id"`
	BookingID           string     `json:"booking_id"`
	UserID              string     `json:"user_id"`
	Amount              float64    `json:"amount"`
	Currency            string     `json:"currency"` // "inr"
	Status              string     `json:"status"`
	StripePaymentIntent string     `json:"stripe_payment_intent_id"`
	StripeChargeID      string     `json:"stripe_charge_id,omitempty"`
	FailureReason       string     `json:"failure_reason,omitempty"`
	RefundedAt          *time.Time `json:"refunded_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// Refund represents a refund issued against a payment.
type Refund struct {
	ID             string    `json:"id"`
	PaymentID      string    `json:"payment_id"`
	BookingID      string    `json:"booking_id"`
	Amount         float64   `json:"amount"`
	StripeRefundID string    `json:"stripe_refund_id"`
	Reason         string    `json:"reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// ──────────────────────────────────────────────
// Price breakdown — returned to frontend before payment
// ──────────────────────────────────────────────

// PriceSummary gives an itemised breakdown of what the user will pay.
// Frontend renders this as the order summary before confirming.
type PriceSummary struct {
	Items          []PriceLineItem `json:"items"`
	SubTotal       float64         `json:"sub_total"`
	ConvenienceFee float64         `json:"convenience_fee"`
	Taxes          float64         `json:"taxes"`
	Total          float64         `json:"total"`
	Currency       string          `json:"currency"`
}

// PriceLineItem is one seat in the summary.
type PriceLineItem struct {
	ShowtimeSeatID string  `json:"showtime_seat_id"`
	RowLabel       string  `json:"row_label"`
	ColNumber      int     `json:"col_number"`
	SeatType       string  `json:"seat_type"`
	BasePrice      float64 `json:"base_price"`
}

// ComputeTotal applies convenience fee (2%) and GST (18%) to the subtotal.
func (p *PriceSummary) ComputeTotal() {
	var sub float64
	for _, item := range p.Items {
		sub += item.BasePrice
	}
	p.SubTotal = round2(sub)
	p.ConvenienceFee = round2(sub * 0.02)
	p.Taxes = round2((sub + p.ConvenienceFee) * 0.18)
	p.Total = round2(p.SubTotal + p.ConvenienceFee + p.Taxes)
	p.Currency = "inr"
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
