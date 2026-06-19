package handlers

import (
	"cinemabooking/internal/domain"
	services "cinemabooking/internal/service"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

type WebhookHandler struct {
	bookingService *services.BookingService
}

func NewWebhookHandler(
	bookingService *services.BookingService,
) *WebhookHandler {
	return &WebhookHandler{
		bookingService: bookingService,
	}
}

func (h *WebhookHandler) StripeWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	var event domain.StripeWebhookEvent

	if err := json.NewDecoder(c.Request.Body).Decode(&event); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	switch event.Type {

	case "payment_intent.succeeded":

		err := h.bookingService.HandlePaymentSuccess(
			ctx,
			event.Data.Object.ID,
		)
		if err != nil {
			// handleError(c, err)
			slog.Error("handling payment success",
				"payment_intent", event.Data.Object.ID,
				"error", err.Error(),
			)
			return
		}

	case "payment_intent.payment_failed":
		err := h.bookingService.HandlePaymentFailed(ctx, event.Data.Object.ID)
		if err != nil {
			// handleError(c, err)
			slog.Error("handling payment failed",
				"payment_intent", event.Data.Object.ID,
				"error", err.Error(),
			)
			return
		}
	}

	c.JSON(http.StatusOK, domain.OK[any](nil))
}
