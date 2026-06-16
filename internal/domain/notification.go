package domain

import "time"

// Notification channel constants
const (
	ChannelEmail = "email"
	ChannelPush  = "push"
	ChannelSMS   = "sms"
)

// Notification type constants
const (
	NotifTypeBookingConfirmed  = "booking_confirmed"
	NotifTypeBookingCancelled  = "booking_cancelled"
	NotifTypePaymentFailed     = "payment_failed"
	NotifTypeRefundIssued      = "refund_issued"
	NotifTypeShowtimeReminder  = "showtime_reminder"  // sent 2h before showtime
)

// Notification is a record of every notification dispatched to a user.
type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	Channel   string    `json:"channel"`
	Subject   string    `json:"subject,omitempty"` // email subject line
	Body      string    `json:"body"`
	IsRead    bool      `json:"is_read"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// ──────────────────────────────────────────────
// Payload types passed to the notification service
// ──────────────────────────────────────────────

// BookingConfirmedPayload carries everything needed to render the
// "booking confirmed" email / push notification.
type BookingConfirmedPayload struct {
	User        UserProfile
	Booking     Booking
	Showtime    Showtime
	Movie       Movie
	Theatre     Theatre
	Hall        Hall
	Seats       []BookedSeat
	QRCodeURL   string
	TotalAmount float64
}

// BookingCancelledPayload is used for both cancellation and refund notices.
type BookingCancelledPayload struct {
	User          UserProfile
	Booking       Booking
	Movie         Movie
	RefundAmount  float64
	RefundDays    int // "within N business days"
}

// ShowtimeReminderPayload drives the pre-show reminder.
type ShowtimeReminderPayload struct {
	User      UserProfile
	Booking   Booking
	Showtime  Showtime
	Movie     Movie
	Theatre   Theatre
	Seats     []BookedSeat
}

// PushToken stores a device push token for a user (FCM or APNs).
type PushToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	Platform  string    `json:"platform"` // ios | android
	CreatedAt time.Time `json:"created_at"`
}

// RegisterPushTokenRequest is sent by the mobile app after obtaining a token.
type RegisterPushTokenRequest struct {
	Token    string `json:"token"    binding:"required"`
	Platform string `json:"platform" binding:"required,oneof=ios android"`
}