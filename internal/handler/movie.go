package handlers

import (
	"cinemabooking/internal/domain"
	services "cinemabooking/internal/service"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type MovieHandler struct {
	movieService *services.MovieService
}

func NewMovieHandler(movieService *services.MovieService) *MovieHandler {
	return &MovieHandler{movieService: movieService}
}

func parsePagination(c *gin.Context) (page, limit int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "6"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return
}

func (h *MovieHandler) ListMovies(c *gin.Context) {
	page, limit := parsePagination(c)
	filter := domain.MovieFilter{
		City:     c.Query("city"),
		Genre:    c.Query("genre"),
		Language: c.Query("language"),
		Date:     c.Query("date"),
		Search:   c.Query("search"),
		Page:     page,
		Limit:    limit,
	}

	resp, err := h.movieService.ListMovies(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.OK(resp))
}

func (h *MovieHandler) GetMovie(c *gin.Context) {
	id := c.Param("id")
	movie, err := h.movieService.GetMovie(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrMovieNotFound) {
			c.JSON(http.StatusNotFound, domain.Fail[any]("movie not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.OK(gin.H{"movie": movie}))
}

func (h *MovieHandler) CreateMovie(c *gin.Context) {
	var req domain.CreateMovieRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	movie, err := h.movieService.CreateMovie(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusCreated, domain.OK(movie))
}

func (h *MovieHandler) UpdateMovie(c *gin.Context) {
	id := c.Param("id")
	var req domain.UpdateMovieRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	movie, err := h.movieService.UpdateMovie(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, services.ErrMovieNotFound) {
			c.JSON(http.StatusNotFound, domain.Fail[any]("movie not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.OK(movie))
}

func (h *MovieHandler) DeleteMovie(c *gin.Context) {
	id := c.Param("id")
	if err := h.movieService.DeleteMovie(c.Request.Context(), id); err != nil {
		if errors.Is(err, services.ErrMovieNotFound) {
			c.JSON(http.StatusNotFound, domain.Fail[any]("movie not found"))
			return
		}
		if errors.Is(err, services.ErrMovieInUse) {
			c.JSON(http.StatusConflict, domain.Fail[any]("movie has existing showtimes or reviews"))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.OK[any](nil))
}

func (h *MovieHandler) GetShowtimes(c *gin.Context) {
	id := c.Param("id")
	page, limit := parsePagination(c)
	filter := domain.ShowtimeFilter{
		Page:  page,
		Limit: limit,
		City:  c.Query("city"),
		Date:  c.Query("date"),
	}

	showtimes, err := h.movieService.GetMovieShowtimes(c.Request.Context(), id, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.OK(showtimes))
}

func (h *MovieHandler) AddReview(c *gin.Context) {
	movieID := c.Param("id")

	userID, ok := c.Get("user_id") // from auth middleware
	if !ok {
		c.JSON(http.StatusUnauthorized, domain.Fail[any]("authentication required"))
		return
	}

	var req domain.CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	review, err := h.movieService.AddReview(c.Request.Context(), userID.(string), movieID, &req)
	if err != nil {
		if errors.Is(err, services.ErrMovieNotFound) {
			c.JSON(http.StatusNotFound, domain.Fail[any]("movie not found"))
			return
		}
		if errors.Is(err, services.ErrAlreadyReviewed) {
			c.JSON(http.StatusConflict, domain.Fail[any]("you have already reviewed this movie"))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusCreated, domain.OK(review))
}

func (h *MovieHandler) ListReviews(c *gin.Context) {
	movieID := c.Param("id")
	page, limit := parsePagination(c)

	reviews, err := h.movieService.ListReviews(c.Request.Context(), movieID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.OK(reviews))
}
