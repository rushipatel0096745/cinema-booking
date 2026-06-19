package handlers

import (
	"cinemabooking/internal/domain"
	services "cinemabooking/internal/service"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type BookingHandler struct {
	bookingService *services.BookingService
}

func NewBookingHandler(bookingService *services.BookingService) *BookingHandler {
	return &BookingHandler{
		bookingService: bookingService,
	}
}

func (h *BookingHandler) LockSeats(c *gin.Context) {
	var req domain.LockSeatsRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		handleError(c, err)
		return
	}

	userID := c.GetString("user_id")

	res, err := h.bookingService.LockSeats(
		c.Request.Context(),
		userID,
		req,
	)
	if err != nil {
		// response.Error(c, http.StatusBadRequest, err.Error())

		var seatErr *domain.SeatUnavailableError

		if errors.As(err, &seatErr) {

			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": seatErr.Error(),
				"seats":   seatErr.Seats,
			})

			return
		}

		handleError(c, err)
		return
	}

	// response.Success(c, http.StatusOK, res)
	c.JSON(http.StatusOK, domain.OK(res))
}

func (h *BookingHandler) CreateBooking(c *gin.Context) {
	userID := c.GetString("user_id")

	var req domain.CreateBookingRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	booking, err := h.bookingService.CreateBooking(
		c.Request.Context(),
		userID,
		req.ShowtimeID,
		req.SeatIDs,
	)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.OK(booking))
}

// GET /api/bookings
func (h *BookingHandler) GetUserBookings(c *gin.Context) {
	userID := c.GetString("user_id")

	filter := domain.BookingListFilter{
		UserID: userID,
		Status: c.Query("status"),
		Page:   queryInt(c, "page", 1),
		Limit:  queryInt(c, "limit", 10),
	}

	bookings, total, err := h.bookingService.GetUserBookings(
		c.Request.Context(),
		filter,
	)
	if err != nil {
		handleError(c, err)
		return
	}

	filter.Page, filter.Limit = domain.NormalisePage(filter.Page, filter.Limit)
	c.JSON(http.StatusOK, domain.OKList(bookings, domain.NewMeta(filter.Page, filter.Limit, total)))
}

// GET /api/bookings/:id
func (h *BookingHandler) GetBooking(c *gin.Context) {
	userID := c.GetString("user_id")
	bookingID := c.Param("id")

	booking, err := h.bookingService.GetBooking(
		c.Request.Context(),
		bookingID,
		userID,
	)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.OK(booking))
}

// POST /api/bookings/:id/cancel
func (h *BookingHandler) CancelBooking(c *gin.Context) {
	userID := c.GetString("user_id")
	bookingID := c.Param("id")

	var req domain.CancelBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	if err := h.bookingService.CancelBooking(
		c.Request.Context(),
		bookingID,
		userID,
		req.Reason,
	); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.OK[any](nil))
}
