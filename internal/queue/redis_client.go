package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Raymond9734/campaign-messaging-backend/internal/models"
)

// redisClient implements Client using Redis
type redisClient struct {
	client    *redis.Client
	queueName string
	logger    *slog.Logger
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	URL       string
	QueueName string
}

// NewRedisClient creates a new Redis queue client
func NewRedisClient(cfg RedisConfig, logger *slog.Logger) (Client, error) {
	// Parse Redis URL
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Create Redis client
	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("connected to Redis",
		slog.String("addr", opts.Addr),
		slog.String("queue", cfg.QueueName),
	)

	return &redisClient{
		client:    client,
		queueName: cfg.QueueName,
		logger:    logger,
	}, nil
}

// Publish sends a message job to the queue
func (c *redisClient) Publish(ctx context.Context, job *models.MessageJob) error {
	// Serialize job to JSON
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Push to Redis list (LPUSH for FIFO with BRPOP)
	if err := c.client.LPush(ctx, c.queueName, data).Err(); err != nil {
		return fmt.Errorf("failed to push job to queue: %w", err)
	}

	c.logger.Debug("job published to queue",
		slog.Int64("message_id", job.OutboundMessageID),
	)

	return nil
}

// Consume receives messages from the queue and processes them with the handler
// concurrency controls how many messages can be processed simultaneously (max 5)
func (c *redisClient) Consume(ctx context.Context, handler MessageHandler, concurrency int) error {
	// Validate concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 5 {
		concurrency = 5
	}

	c.logger.Info("starting queue consumer",
		slog.String("queue", c.queueName),
		slog.Int("concurrency", concurrency),
	)

	// Semaphore to limit concurrent processing
	semaphore := make(chan struct{}, concurrency)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("consumer stopped by context, waiting for in-flight jobs to complete")
			// Wait for all in-flight jobs to complete
			for i := 0; i < concurrency; i++ {
				semaphore <- struct{}{}
			}
			c.logger.Info("all in-flight jobs completed")
			return ctx.Err()

		default:
			// Blocking pop from Redis list (blocks for 1 second if empty)
			result, err := c.client.BRPop(ctx, 1*time.Second, c.queueName).Result()
			if err != nil {
				if err == redis.Nil {
					// Timeout, no messages available - continue
					continue
				}
				if err == context.Canceled || err == context.DeadlineExceeded {
					c.logger.Info("consumer stopped by context")
					// Wait for all in-flight jobs to complete
					for i := 0; i < concurrency; i++ {
						semaphore <- struct{}{}
					}
					return err
				}
				c.logger.Error("failed to pop from queue", slog.String("error", err.Error()))
				// Sleep briefly to avoid tight loop on persistent errors
				time.Sleep(1 * time.Second)
				continue
			}

			// BRPOP returns [queueName, value]
			if len(result) < 2 {
				c.logger.Error("unexpected BRPOP result format")
				continue
			}

			// Deserialize job
			var job models.MessageJob
			if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
				c.logger.Error("failed to unmarshal job",
					slog.String("error", err.Error()),
					slog.String("data", result[1]),
				)
				continue
			}

			c.logger.Debug("job received from queue",
				slog.Int64("message_id", job.OutboundMessageID),
			)

			// Acquire semaphore slot (blocks if all slots are busy)
			semaphore <- struct{}{}

			// Process job concurrently in a goroutine
			go func(job models.MessageJob) {
				defer func() { <-semaphore }() // Release semaphore slot when done

				// Process job with handler
				if err := handler(ctx, &job); err != nil {
					c.logger.Error("handler failed to process job",
						slog.Int64("message_id", job.OutboundMessageID),
						slog.String("error", err.Error()),
					)
					// Note: Job is already popped from queue
					// Retry logic is handled by the worker/handler
				}
			}(job)
		}
	}
}

// Close closes the Redis connection
func (c *redisClient) Close() error {
	c.logger.Info("closing Redis connection")
	return c.client.Close()
}

// Health checks if Redis is healthy
func (c *redisClient) Health(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis health check failed: %w", err)
	}
	return nil
}

// QueueLength returns the number of jobs in the queue (for monitoring)
func (c *redisClient) QueueLength(ctx context.Context) (int64, error) {
	length, err := c.client.LLen(ctx, c.queueName).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get queue length: %w", err)
	}
	return length, nil
}
