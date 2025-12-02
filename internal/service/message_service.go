package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/models"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/repository"
)

// MessageService handles outbound message business logic
type MessageService interface {
	GetByID(ctx context.Context, id int64) (*models.OutboundMessage, error)
	UpdateStatus(ctx context.Context, id int64, status string, lastError *string) error
	IncrementRetryCount(ctx context.Context, id int64) error
	GetPendingMessages(ctx context.Context, limit int) ([]*models.OutboundMessage, error)
}

type messageService struct {
	messageRepo repository.OutboundMessageRepository
	logger      *slog.Logger
}

// NewMessageService creates a new message service
func NewMessageService(
	messageRepo repository.OutboundMessageRepository,
	logger *slog.Logger,
) MessageService {
	return &messageService{
		messageRepo: messageRepo,
		logger:      logger,
	}
}

// GetByID retrieves a message by ID
func (s *messageService) GetByID(ctx context.Context, id int64) (*models.OutboundMessage, error) {
	message, err := s.messageRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return message, nil
}

// UpdateStatus updates the status of a message
func (s *messageService) UpdateStatus(ctx context.Context, id int64, status string, lastError *string) error {
	if !models.IsValidMessageStatus(status) {
		return models.ErrInvalidInput(fmt.Sprintf("invalid status: %s", status))
	}

	if err := s.messageRepo.UpdateStatus(ctx, id, status, lastError); err != nil {
		s.logger.Error("failed to update message status",
			slog.Int64("message_id", id),
			slog.String("status", status),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to update message status: %w", err)
	}

	s.logger.Debug("message status updated",
		slog.Int64("message_id", id),
		slog.String("status", status),
	)

	return nil
}

// IncrementRetryCount increments the retry count for a message
func (s *messageService) IncrementRetryCount(ctx context.Context, id int64) error {
	if err := s.messageRepo.IncrementRetryCount(ctx, id); err != nil {
		s.logger.Error("failed to increment retry count",
			slog.Int64("message_id", id),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to increment retry count: %w", err)
	}

	return nil
}

// GetPendingMessages retrieves pending messages for processing
func (s *messageService) GetPendingMessages(ctx context.Context, limit int) ([]*models.OutboundMessage, error) {
	messages, err := s.messageRepo.GetPendingMessages(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending messages: %w", err)
	}

	return messages, nil
}
