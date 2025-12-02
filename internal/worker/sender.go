package worker

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// MessageSender defines the interface for sending messages
type MessageSender interface {
	Send(ctx context.Context, channel, phone, content string) error
}

// mockSender simulates message sending with 90-95% success rate
type mockSender struct {
	successRate float64
	minDelay    time.Duration
	maxDelay    time.Duration
}

// NewMockSender creates a new mock message sender
// successRate: probability of success (0.0 to 1.0), default 0.92 (92%)
func NewMockSender(successRate float64) MessageSender {
	if successRate <= 0 || successRate > 1.0 {
		successRate = 0.92 // Default 92% success rate
	}

	return &mockSender{
		successRate: successRate,
		minDelay:    50 * time.Millisecond,  // Simulate network latency
		maxDelay:    200 * time.Millisecond,
	}
}

// Send simulates sending a message
func (s *mockSender) Send(ctx context.Context, channel, phone, content string) error {
	// Simulate network delay
	delay := s.minDelay + time.Duration(rand.Int63n(int64(s.maxDelay-s.minDelay)))

	select {
	case <-time.After(delay):
		// Continue
	case <-ctx.Done():
		return ctx.Err()
	}

	// Randomly fail based on success rate
	if rand.Float64() > s.successRate {
		return fmt.Errorf("mock sender failed: simulated network error")
	}

	// Success
	return nil
}
