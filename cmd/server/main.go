package main

import (
	"cinemabooking/internal/config"
	"cinemabooking/internal/db"
	handlers "cinemabooking/internal/handler"
	"cinemabooking/internal/middleware"
	repositories "cinemabooking/internal/repository"
	services "cinemabooking/internal/service"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {

	cfg := config.Load()

	// connect to database
	pool := db.Connect()
	defer pool.Close()

	// user auth service
	userRepo := repositories.NewUserRepository(pool)
	authService := services.NewAuthService(userRepo, cfg)
	authHandler := handlers.NewAuthHandler(authService, cfg, userRepo)

	// movie service
	movieRepo := repositories.NewMovieRepository(pool)
	movieService := services.NewMovieService(movieRepo)
	movieHandler := handlers.NewMovieHandler(movieService)

	// theatre service
	theatreRepo := repositories.NewTheatreRepository(pool)
	theatreService := services.NewTheatreService(theatreRepo)
	theatreHandler := handlers.NewTheatreHandler(theatreService)

	r := gin.Default()

	// auth routes
	auth := r.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.Refresh)
		auth.POST("/logout", authHandler.Logout)
		auth.GET("/google/login", authHandler.GoogleLogin)
		auth.GET("/google/callback", authHandler.GoogleCallback)
	}

	// protected routes
	api := r.Group("/api", middleware.AuthMiddleware(authService))
	{
		api.GET("/me", authHandler.Me)

		// for movie
		api.GET("/movies", movieHandler.ListMovies)
		api.GET("/movies/:id", movieHandler.GetMovie)
		// api.POST("/movies", movieHandler.CreateMovie)
		// api.PUT("/movies/:id", movieHandler.UpdateMovie)
		// api.DELETE("/movies/:id", movieHandler.DeleteMovie)
		api.GET("/movies/:id/showtimes", movieHandler.GetShowtimes)

		// for reviews
		api.POST("/movies/:id/reviews", movieHandler.AddReview)
		api.GET("/movies/:id/reviews", movieHandler.ListReviews)

		// for theatres
		api.GET("/theatres", theatreHandler.ListTheatres)
		api.GET("/theatres/:id", theatreHandler.GetTheatre)
		api.GET("/theatres/:id/halls", theatreHandler.GetHalls)
		// api.POST("/theatres", theatreHandler.CreateTheatre)
		// api.POST("/theatres/:id/halls", theatreHandler.CreateHall)

	}

	if err := r.Run(); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
