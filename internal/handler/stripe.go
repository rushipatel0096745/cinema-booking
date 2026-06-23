package handlers

import (
	"cinemabooking/internal/domain"
	services "cinemabooking/internal/service"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/webhook"
)

type WebhookHandler struct {
	bookingService      *services.BookingService
	stripeWebhookSecret string
}

func NewWebhookHandler(
	bookingService *services.BookingService,
	stripeWebhookSecret string,
) *WebhookHandler {
	return &WebhookHandler{
		bookingService:      bookingService,
		stripeWebhookSecret: stripeWebhookSecret,
	}
}

func (h *WebhookHandler) StripeWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	// read raw body first (needed for signature verification)
	payload, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 65536))
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any]("failed to read body"))
		return
	}

	// verify stripe signature
	event, err := webhook.ConstructEvent(
		payload,
		c.GetHeader("Stripe-Signature"),
		h.stripeWebhookSecret,
	)
	if err != nil {
		slog.Error("stripe signature verification failed", "error", err)
		c.JSON(http.StatusBadRequest, domain.Fail[any]("invalid signature"))
		return
	}

	switch event.Type {
	case "payment_intent.succeeded":
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			c.JSON(http.StatusBadRequest, domain.Fail[any]("failed to parse payment intent"))
			return
		}
		if err := h.bookingService.HandlePaymentSuccess(ctx, pi.ID); err != nil {
			slog.Error("handling payment success", "payment_intent", pi.ID, "error", err)
		}

	case "payment_intent.payment_failed":
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			c.JSON(http.StatusBadRequest, domain.Fail[any]("failed to parse payment intent"))
			return
		}
		if err := h.bookingService.HandlePaymentFailed(ctx, pi.ID); err != nil {
			slog.Error("handling payment failed", "payment_intent", pi.ID, "error", err)
		}
	}

	c.JSON(http.StatusOK, domain.OK[any](nil))
}

// func (h *WebhookHandler) StripeWebhook(c *gin.Context) {
// 	ctx := c.Request.Context()

// 	var event domain.StripeWebhookEvent

// 	if err := json.NewDecoder(c.Request.Body).Decode(&event); err != nil {
// 		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
// 		return
// 	}

// 	switch event.Type {

// 	case "payment_intent.succeeded":

// 		err := h.bookingService.HandlePaymentSuccess(
// 			ctx,
// 			event.Data.Object.ID,
// 		)
// 		if err != nil {
// 			// handleError(c, err)
// 			slog.Error("handling payment success",
// 				"payment_intent", event.Data.Object.ID,
// 				"error", err.Error(),
// 			)
// 			return
// 		}

// 	case "payment_intent.payment_failed":
// 		err := h.bookingService.HandlePaymentFailed(ctx, event.Data.Object.ID)
// 		if err != nil {
// 			// handleError(c, err)
// 			slog.Error("handling payment failed",
// 				"payment_intent", event.Data.Object.ID,
// 				"error", err.Error(),
// 			)
// 			return
// 		}
// 	}

// 	c.JSON(http.StatusOK, domain.OK[any](nil))
// }
