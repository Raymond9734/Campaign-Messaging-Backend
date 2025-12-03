package queue

import (
	"context"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/models"
)

// Client defines the interface for queue operations
type Client interface {
	// Publish sends a message job to the queue
	Publish(ctx context.Context, job *models.MessageJob) error

	// Consume receives messages from the queue and processes them with the handler
	// concurrency controls how many messages can be processed simultaneously
	Consume(ctx context.Context, handler MessageHandler, concurrency int) error

	// Close closes the queue connection
	Close() error

	// Health checks if the queue is healthy
	Health(ctx context.Context) error
}

// MessageHandler is a function that processes a message job
type MessageHandler func(ctx context.Context, job *models.MessageJob) error
