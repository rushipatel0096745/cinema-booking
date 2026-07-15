package handlers

import (
	"cinemabooking/internal/domain"
	services "cinemabooking/internal/service"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TheatreHandler struct {
	theatreService *services.TheatreService
}

func NewTheatreHandler(theatreService *services.TheatreService) *TheatreHandler {
	return &TheatreHandler{theatreService: theatreService}
}

func (h *TheatreHandler) ListTheatres(c *gin.Context) {
	page, limit := parsePagination(c)
	filter := domain.TheatreFilter{
		City:  c.Query("city"),
		Page:  page,
		Limit: limit,
	}

	theatres, total, err := h.theatreService.ListTheatres(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.OKList(theatres, domain.NewMeta(page, limit, total)))
}

func (h *TheatreHandler) ListCities(c *gin.Context) {
	cities, err := h.theatreService.ListCities(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.OK(cities))
}

func (h *TheatreHandler) GetTheatre(c *gin.Context) {
	id := c.Param("id")
	theatre, err := h.theatreService.GetTheatre(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrTheatreNotFound) {
			c.JSON(http.StatusNotFound, domain.Fail[any]("theatre not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.OK(theatre))
}

func (h *TheatreHandler) GetHalls(c *gin.Context) {
	id := c.Param("id")
	halls, err := h.theatreService.GetHalls(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrTheatreNotFound) {
			c.JSON(http.StatusNotFound, domain.Fail[any]("theatre not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.OK(halls))
}

func (h *TheatreHandler) CreateTheatre(c *gin.Context) {
	var req domain.CreateTheatreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	theatre, err := h.theatreService.CreateTheatre(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusCreated, domain.OK(theatre))
}

func (h *TheatreHandler) CreateHall(c *gin.Context) {
	theatreID := c.Param("id")
	var req domain.CreateHallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	hall, err := h.theatreService.CreateHall(c.Request.Context(), theatreID, &req)
	if err != nil {
		if errors.Is(err, services.ErrTheatreNotFound) {
			c.JSON(http.StatusNotFound, domain.Fail[any]("theatre not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusCreated, domain.OK(hall))
}
