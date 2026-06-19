package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"cinemabooking/internal/domain"
	services "cinemabooking/internal/service"

	"github.com/gin-gonic/gin"
)

type ShowtimeHandler struct {
	service *services.ShowtimeService
}

func NewShowtimeHandler(service *services.ShowtimeService) *ShowtimeHandler {
	return &ShowtimeHandler{service: service}
}

// handleError is a helper function to handle errors uniformly.
func handleError(c *gin.Context, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.Code, domain.Fail[any](appErr.Message))
		return
	}
	// c.JSON(http.StatusInternalServerError, domain.Fail[any]("internal server error"))
	c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
}


func queryInt(c *gin.Context, key string, defaultVal int) int {
	if val, err := strconv.Atoi(c.Query(key)); err == nil {
		return val
	}
	return defaultVal
}

// GET /api/v1/showtimes?city=&date=&movie_id=&page=&limit=
func (h *ShowtimeHandler) ListShowtimes(c *gin.Context) {
	filter := domain.ShowtimeFilter{
		MovieID: c.Query("movie_id"),
		City:    c.Query("city"),
		Date:    c.Query("date"),
	}

	if page, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil {
		filter.Page = page
	}
	if limit, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil {
		filter.Limit = limit
	}

	showtimes, total, err := h.service.ListShowtimes(c.Request.Context(), filter)
	if err != nil {
		handleError(c, err)
		return
	}

	filter.Page, filter.Limit = domain.NormalisePage(filter.Page, filter.Limit)
	c.JSON(http.StatusOK, domain.OKList(showtimes, domain.NewMeta(filter.Page, filter.Limit, total)))
}

// GET /api/v1/showtimes/:id
func (h *ShowtimeHandler) GetShowtime(c *gin.Context) {
	id := c.Param("id")

	showtime, err := h.service.GetShowtime(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.OK(showtime))
}

// GET /api/v1/showtimes/:id/seats
func (h *ShowtimeHandler) GetSeatMap(c *gin.Context) {
	showtimeID := c.Param("id")

	seatMap, err := h.service.GetSeatMap(c.Request.Context(), showtimeID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.OK(seatMap))
}

// POST /api/v1/showtimes  [admin]
func (h *ShowtimeHandler) CreateShowtime(c *gin.Context) {
	var req domain.CreateShowtimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	showtime, err := h.service.CreateShowtime(c.Request.Context(), req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, domain.OK(showtime))
}

// PUT /api/v1/showtimes/:id  [admin]
func (h *ShowtimeHandler) UpdateShowtime(c *gin.Context) {
	id := c.Param("id")

	var req domain.UpdateShowtimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	showtime, err := h.service.UpdateShowtime(c.Request.Context(), id, req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.OK(showtime))
}

// DELETE /api/v1/showtimes/:id  [admin]
func (h *ShowtimeHandler) DeleteShowtime(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteShowtime(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.OK[any](nil))
}
