package domain

import (
	// "cinemabooking/internal/domain"
	"fmt"
	"time"
)

// Booking status constants
const (
	BookingStatusPending   = "pending"
	BookingStatusConfirmed = "confirmed"
	BookingStatusCancelled = "cancelled"
	BookingStatusRefunded  = "refunded"
)

// Booking represents a ticket purchase (one booking = one or more seats).
type Booking struct {
	ID                  string     `json:"id"`
	UserID              string     `json:"user_id"`
	ShowtimeID          string     `json:"showtime_id"`
	Status              string     `json:"status"`
	// TotalAmount         float64    `json:"total_amount"`
	StripePaymentIntent string     `json:"stripe_payment_intent_id,omitempty"`
	QRCodeURL           string     `json:"qr_code_url,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	ConfirmedAt         *time.Time `json:"confirmed_at,omitempty"`
	// Populated via JOINs
	Seats    []BookedSeat `json:"seats,omitempty"`
	Showtime *Showtime    `json:"showtime,omitempty"`
	User     *UserProfile `json:"user,omitempty"`
	Subtotal       float64 `json:"subtotal"`
	ConvenienceFee float64 `json:"convenience_fee"`
	Taxes          float64 `json:"taxes"`
	TotalAmount    float64 `json:"total_amount"` // grand total
}

// IsCancellable returns true if the booking can still be cancelled.
// Business rule: cancellable up to 2 hours before the show.
// func (b *Booking) IsCancellable() bool {
// 	if b.Status != BookingStatusConfirmed {
// 		return false
// 	}
// 	if b.Showtime == nil {
// 		return false
// 	}
// 	return time.Now().Before(b.Showtime.StartsAt.Add(-2 * time.Hour))
// }

func (b *Booking) IsCancellable() bool {
	if b.Status != BookingStatusConfirmed {
		fmt.Println("status not confirmed")
		return false
	}

	if b.Showtime == nil {
		fmt.Println("showtime nil")
		return false
	}

	fmt.Println("showtime starts at:", b.Showtime.StartsAt)
	fmt.Println("cutoff:", b.Showtime.StartsAt.Add(-2*time.Hour))
	fmt.Println("now:", time.Now())

	return time.Now().Before(b.Showtime.StartsAt.Add(-2 * time.Hour))
}

// BookedSeat is the join between a booking and a specific showtime seat.
type BookedSeat struct {
	BookingID      string  `json:"booking_id"`
	ShowtimeSeatID string  `json:"showtime_seat_id"`
	Price          float64 `json:"price"`
	// Populated via JOIN
	RowLabel  string `json:"row_label"`
	ColNumber int    `json:"col_number"`
	SeatType  string `json:"seat_type"`
}

// ──────────────────────────────────────────────
// Request / Response DTOs
// ──────────────────────────────────────────────

// LockSeatsRequest is the first step — client locks chosen seats before payment.
type LockSeatsRequest struct {
	ShowtimeID string   `json:"showtime_id" binding:"required,uuid"`
	SeatIDs    []string `json:"seat_ids"    binding:"required,min=1,max=10,dive,uuid"`
}

// LockSeatsResponse returns the payment intent and how long the lock lasts.
type LockSeatsResponse struct {
	LockExpiresAt time.Time `json:"lock_expires_at"` // now + 10 min
	SeatIDs       []string  `json:"seat_ids"`
	TotalAmount   float64   `json:"total_amount"`
}

// CreateBookingRequest is sent after seat locking to initiate payment.
type CreateBookingRequest struct {
	ShowtimeID string   `json:"showtime_id" binding:"required,uuid"`
	SeatIDs    []string `json:"seat_ids"    binding:"required,min=1,max=10,dive,uuid"`
}

// CreateBookingResponse returns the new booking plus the Stripe client secret
// so the frontend can complete payment using Stripe.js / React Native Stripe SDK.
type CreateBookingResponse struct {
	BookingID            string  `json:"booking_id"`
	PaymentIntentID      string  `json:"payment_intent_id"`      // Stripe PaymentIntent ID (e.g. pi_...)
	ClientSecret         string  `json:"client_secret"`          // Stripe PaymentIntent client_secret
	TotalAmount          float64 `json:"total_amount"`           // e.g. 1250.0
	Currency             string  `json:"currency"`               // "inr"
	StripePublishableKey string  `json:"stripe_publishable_key"` // your publishable key for the frontend
	Breakdown            *PriceSummary `json:"breakdown"`
}

// CancelBookingRequest carries optional cancellation reason.
type CancelBookingRequest struct {
	Reason string `json:"reason" binding:"omitempty,max=500"`
}

// BookingListFilter for admin or per-user listing.
type BookingListFilter struct {
	UserID     string
	ShowtimeID string
	Status     string
	Page       int
	Limit      int
}

// BookingListResponse wraps paginated booking results.
type BookingListResponse struct {
	Bookings []Booking `json:"bookings"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	Limit    int       `json:"limit"`
}

// ──────────────────────────────────────────────
// Stripe webhook payload (internal use)
// ──────────────────────────────────────────────

// StripeWebhookEvent is parsed from Stripe's raw webhook body.
// Only the fields we act on are mapped here.
type StripeWebhookEvent struct {
	Type string                 `json:"type"` // "payment_intent.succeeded" etc.
	Data StripeWebhookEventData `json:"data"`
}

type StripeWebhookEventData struct {
	Object StripePaymentIntentObject `json:"object"`
}

type StripePaymentIntentObject struct {
	ID       string            `json:"id"`     // pi_xxx
	Status   string            `json:"status"` // succeeded | requires_payment_method | canceled
	Amount   int64             `json:"amount"` // in smallest currency unit (paise for INR)
	Currency string            `json:"currency"`
	Metadata map[string]string `json:"metadata"` // booking_id stored here
}
