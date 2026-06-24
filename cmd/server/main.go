package main

import (
	"cinemabooking/internal/cache"
	"cinemabooking/internal/config"
	"cinemabooking/internal/db"
	handlers "cinemabooking/internal/handler"
	"cinemabooking/internal/middleware"
	"cinemabooking/internal/pkg/mailer"
	"cinemabooking/internal/pkg/qr"
	"cinemabooking/internal/pkg/storage"
	"cinemabooking/internal/ws"
	"log/slog"
	"os"

	"cinemabooking/internal/payment"
	repositories "cinemabooking/internal/repository"
	services "cinemabooking/internal/service"
	"log"
	"strconv"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {

	cfg := config.Load()

	logger := slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	)

	slog.SetDefault(logger)

	// connect to database
	pool := db.Connect()
	defer pool.Close()

	// redis connection
	dbNum, err := strconv.Atoi(cfg.RedisDB)
	if err != nil {
		log.Fatalf("invalid Redis DB number: %v", err)
	}
	redisClient, err := cache.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, dbNum, cfg.RedisUseTLS)
	if err != nil {
		log.Fatalf("connecting to redis: %v", err)
	}
	defer redisClient.Close()

	// stripe client
	stripeClient := payment.NewStripeClient(cfg.StripeSecretKey)

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

	// showtime service
	showtimeRepo := repositories.NewShowtimeRepository(pool)
	showtimeService := services.NewShowtimeService(showtimeRepo, movieRepo, theatreRepo)
	showtimeHandler := handlers.NewShowtimeHandler(showtimeService)

	// booking service
	bookingRepo := repositories.NewBookingRepository(pool)
	showtimeSeatRepo := repositories.NewShowtimeSeatRepository(pool)
	lockRepo := repositories.NewSeatLockRepository(redisClient)

	// mailer service
	mailerSvc := mailer.New(
		cfg.ResendApiKey,
		cfg.FromEmail,
		"CinemaBook",
	)

	// qr service
	qrService := qr.NewQrService(cfg.QrSecret)

	// storage service
	storageSvc, err := storage.New(
		cfg.CloudinaryCloudName,
		cfg.CloudinaryApiKey,
		cfg.CloudinaryApiSecret,
	)
	if err != nil {
		log.Fatalf("storage init: %v", err)
	}

	// ws hub
	hub := ws.NewHub()
	go hub.Run()

	wsHandler := ws.NewHandler(hub)

	skipTLS := os.Getenv("APP_ENV") == "development"

	bookingService := services.NewBookingService(
		bookingRepo,
		*userRepo,
		*showtimeRepo,
		showtimeSeatRepo,
		lockRepo,
		stripeClient,
		cfg.StripePublishableKey,
		services.NewStripeService(cfg.StripeSecretKey, cfg.StripePublishableKey, skipTLS),
		hub,
		mailerSvc,
		qrService,
		storageSvc,
	)

	bookingHandler := handlers.NewBookingHandler(
		bookingService,
	)

	// stripe
	stripeWebhookHandler := handlers.NewWebhookHandler(bookingService, cfg.StripeWebhookSecret)

	r := gin.Default()

	// CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:5173", // Vite
			"http://localhost:3000", // React
		},
		AllowMethods: []string{
			"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Authorization",
		},
		ExposeHeaders: []string{
			"Content-Length",
		},
		AllowCredentials: true,
	}))

	// stripe
	r.POST("/api/webhook/stripe", stripeWebhookHandler.StripeWebhook)

	// verify qr code GET /api/verify/:bookingID?sig=...&uid=...&sid=...&exp=...
	r.GET("/api/verify/:bookingId", bookingHandler.VerifyTicket)

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

	r.GET("/ws/showtimes/:id", wsHandler.ServeWS)

	// protected routes
	api := r.Group("/api", middleware.AuthMiddleware(authService))
	{
		api.GET("/me", authHandler.Me)

		// for movie
		api.GET("/movies", movieHandler.ListMovies)
		api.GET("/movies/:id", movieHandler.GetMovie)
		api.POST("/movies", movieHandler.CreateMovie)
		api.PUT("/movies/:id", movieHandler.UpdateMovie)
		api.DELETE("/movies/:id", movieHandler.DeleteMovie)
		api.GET("/movies/:id/showtimes", movieHandler.GetShowtimes)

		// for reviews
		api.POST("/movies/:id/reviews", movieHandler.AddReview)
		api.GET("/movies/:id/reviews", movieHandler.ListReviews)

		// for theatres
		api.GET("/theatres", theatreHandler.ListTheatres)
		api.GET("/theatres/:id", theatreHandler.GetTheatre)
		api.GET("/theatres/:id/halls", theatreHandler.GetHalls)
		api.POST("/theatres", theatreHandler.CreateTheatre)
		api.POST("/theatres/:id/halls", theatreHandler.CreateHall)
		// upadate and delete later for theatre

		// Showtimes
		api.GET("/showtimes", showtimeHandler.ListShowtimes)
		api.GET("/showtimes/:id", showtimeHandler.GetShowtime)
		api.GET("/showtimes/:id/seats", showtimeHandler.GetSeatMap)
		api.POST("/showtimes", showtimeHandler.CreateShowtime)
		api.PUT("/showtimes/:id", showtimeHandler.UpdateShowtime)
		api.DELETE("/showtimes/:id", showtimeHandler.DeleteShowtime)

		// Booking
		api.GET("/bookings", bookingHandler.GetUserBookings)
		api.GET("/bookings/:id", bookingHandler.GetBooking)
		api.POST("/bookings/lock-seats", bookingHandler.LockSeats)
		api.POST("/bookings", bookingHandler.CreateBooking)
		api.POST("/bookings/:id/cancel", bookingHandler.CancelBooking)
	}

	if err := r.Run(); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
