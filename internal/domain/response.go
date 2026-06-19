package domain

import "net/http"

// ──────────────────────────────────────────────
// Standard API envelope
// ──────────────────────────────────────────────

// Response is the generic JSON envelope for every API response.
//
//	{ "success": true,  "data": { ... }, "meta": { ... } }
//	{ "success": false, "error": "seat already locked" }
//
// Frontend can always check `success` before reading `data`.
type Response[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data"`
	Error   string `json:"error,omitempty"`
	Meta    *Meta  `json:"meta,omitempty"`
}

// Meta carries pagination info when the response is a list.
type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// NewMeta builds a Meta from the common page/limit/total triple.
func NewMeta(page, limit, total int) *Meta {
	pages := total / limit
	if total%limit != 0 {
		pages++
	}
	return &Meta{Page: page, Limit: limit, Total: total, TotalPages: pages}
}

// OK wraps data in a success envelope (no meta).
func OK[T any](data T) Response[T] {
	return Response[T]{Success: true, Data: data}
}

// OKList wraps a list response with pagination meta.
func OKList[T any](data T, meta *Meta) Response[T] {
	return Response[T]{Success: true, Data: data, Meta: meta}
}

// Fail wraps an error message in a failure envelope.
func Fail[T any](msg string) Response[T] {
	return Response[T]{Success: false, Error: msg}
}

// ──────────────────────────────────────────────
// Typed application errors
// ──────────────────────────────────────────────

// AppError is an error with an associated HTTP status code.
// Services return these; handlers convert them to JSON.
type AppError struct {
	Code    int
	Message string
}

func (e *AppError) Error() string { return e.Message }

// Pre-defined sentinel errors handlers can match with errors.Is / type assert.
var (
	ErrNotFound     = &AppError{Code: http.StatusNotFound, Message: "resource not found"}
	ErrUnauthorised = &AppError{Code: http.StatusUnauthorized, Message: "unauthorised"}
	ErrForbidden    = &AppError{Code: http.StatusForbidden, Message: "forbidden"}
	ErrBadRequest   = &AppError{Code: http.StatusBadRequest, Message: "bad request"}
	ErrConflict     = &AppError{Code: http.StatusConflict, Message: "conflict"}
	ErrInternal     = &AppError{Code: http.StatusInternalServerError, Message: "internal server error"}

	// Domain-specific
	ErrSeatAlreadyLocked  = &AppError{Code: http.StatusConflict, Message: "one or more seats are already locked or booked"}
	ErrBookingNotFound    = &AppError{Code: http.StatusConflict, Message: "Booking not found"}
	ErrSeatNotAvailable   = &AppError{Code: http.StatusConflict, Message: "seat is not available"}
	ErrLockExpired        = &AppError{Code: http.StatusGone, Message: "seat lock has expired, please select again"}
	ErrShowtimeStarted    = &AppError{Code: http.StatusConflict, Message: "showtime has already started"}
	ErrCancelWindowPassed = &AppError{Code: http.StatusConflict, Message: "cancellation window has passed (must be 2h before showtime)"}
	ErrDuplicateReview    = &AppError{Code: http.StatusConflict, Message: "you have already reviewed this movie"}
	ErrInvalidCredentials = &AppError{Code: http.StatusUnauthorized, Message: "invalid email or password"}
	ErrTokenExpired       = &AppError{Code: http.StatusUnauthorized, Message: "token has expired"}
	ErrEmailTaken         = &AppError{Code: http.StatusConflict, Message: "email is already registered"}
)

// NewAppError creates a custom AppError with a specific status and message.
func NewAppError(code int, msg string) *AppError {
	return &AppError{Code: code, Message: msg}
}

// ──────────────────────────────────────────────
// Pagination defaults
// ──────────────────────────────────────────────

const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

// NormalisePage clamps page and limit to sane values.
func NormalisePage(page, limit int) (int, int) {
	if page < 1 {
		page = DefaultPage
	}
	if limit < 1 || limit > MaxLimit {
		limit = DefaultLimit
	}
	return page, limit
}
