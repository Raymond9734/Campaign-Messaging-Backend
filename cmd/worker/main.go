package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/config"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/db"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/models"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/queue"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/repository"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/worker"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	logger.Info("starting SMSLeopard worker")

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
	messageRepo := repository.NewOutboundMessageRepository(database.DB)
	campaignRepo := repository.NewCampaignRepository(database.DB)
	customerRepo := repository.NewCustomerRepository(database.DB)

	// Initialize mock sender (92% success rate)
	sender := worker.NewMockSender(0.92)

	// Initialize message processor
	processor := worker.NewMessageProcessor(
		messageRepo,
		campaignRepo,
		customerRepo,
		sender,
		cfg.Worker.MaxRetryCount,
		logger,
	)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start consuming messages
	consumerErrors := make(chan error, 1)
	go func() {
		logger.Info("starting message consumer",
			slog.Int("max_retry_count", cfg.Worker.MaxRetryCount),
		)

		// Define message handler
		handler := func(ctx context.Context, job *models.MessageJob) error {
			return processor.Process(ctx, job)
		}

		// Start consuming
		consumerErrors <- queueClient.Consume(ctx, handler)
	}()

	// Wait for interrupt signal or consumer error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-consumerErrors:
		if err != nil && err != context.Canceled {
			logger.Error("consumer error", slog.String("error", err.Error()))
			os.Exit(1)
		}

	case sig := <-quit:
		logger.Info("shutting down worker", slog.String("signal", sig.String()))

		// Cancel context to stop consumer
		cancel()

		// Give consumer time to finish current job
		time.Sleep(5 * time.Second)

		logger.Info("worker stopped gracefully")
	}
}
