package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/config"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/db"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/handler"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/queue"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/repository"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/service"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	logger.Info("starting SMSLeopard API server")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Connect to database
	database, err := db.New(db.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	})
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer database.Close()

	logger.Info("connected to database")

	// Connect to Redis queue
	queueClient, err := queue.NewRedisClient(queue.RedisConfig{
		URL:       cfg.Queue.RedisURL,
		QueueName: cfg.Queue.QueueName,
	}, logger)
	if err != nil {
		logger.Error("failed to connect to Redis", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer queueClient.Close()

	logger.Info("connected to Redis queue")

	// Initialize repositories
	customerRepo := repository.NewCustomerRepository(database.DB)
	campaignRepo := repository.NewCampaignRepository(database.DB)
	messageRepo := repository.NewOutboundMessageRepository(database.DB)

	// Initialize services
	templateSvc := service.NewTemplateService()
	customerSvc := service.NewCustomerService(customerRepo, logger)
	messageSvc := service.NewMessageService(messageRepo, logger)

	campaignSvc := service.NewCampaignService(
		campaignRepo,
		customerRepo,
		messageRepo,
		templateSvc,
		queueClient,
		logger,
	)

	// Initialize handlers
	campaignHandler := handler.NewCampaignHandler(campaignSvc, logger)
	healthHandler := handler.NewHealthHandler(database.DB, queueClient, logger)

	// Setup router
	r := chi.NewRouter()

	// Apply middleware
	r.Use(handler.RecoveryMiddleware(logger))
	r.Use(handler.LoggingMiddleware(logger))
	r.Use(handler.CORSMiddleware)

	// Register routes
	r.Get("/health", healthHandler.Health)

	r.Route("/campaigns", func(r chi.Router) {
		r.Post("/", campaignHandler.CreateCampaign)
		r.Get("/", campaignHandler.ListCampaigns)
		r.Get("/{id}", campaignHandler.GetCampaign)
		r.Post("/{id}/send", campaignHandler.SendCampaign)
		r.Post("/{id}/personalized-preview", campaignHandler.PreviewPersonalized)
	})

	// Create server
	addr := fmt.Sprintf(":%d", cfg.API.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("API server listening", slog.String("addr", addr))
		serverErrors <- server.ListenAndServe()
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Error("server error", slog.String("error", err.Error()))
		os.Exit(1)

	case sig := <-quit:
		logger.Info("shutting down server", slog.String("signal", sig.String()))

		// Graceful shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("server shutdown failed", slog.String("error", err.Error()))
			os.Exit(1)
		}

		logger.Info("server stopped gracefully")
	}

	// Suppress unused variable warnings
	_ = customerSvc
	_ = messageSvc
}
