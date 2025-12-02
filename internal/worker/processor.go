package worker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/models"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/repository"
)

// MessageProcessor processes message jobs from the queue
type MessageProcessor struct {
	messageRepo  repository.OutboundMessageRepository
	campaignRepo repository.CampaignRepository
	customerRepo repository.CustomerRepository
	sender       MessageSender
	maxRetries   int
	logger       *slog.Logger
}

// NewMessageProcessor creates a new message processor
func NewMessageProcessor(
	messageRepo repository.OutboundMessageRepository,
	campaignRepo repository.CampaignRepository,
	customerRepo repository.CustomerRepository,
	sender MessageSender,
	maxRetries int,
	logger *slog.Logger,
) *MessageProcessor {
	return &MessageProcessor{
		messageRepo:  messageRepo,
		campaignRepo: campaignRepo,
		customerRepo: customerRepo,
		sender:       sender,
		maxRetries:   maxRetries,
		logger:       logger,
	}
}

// Process handles a single message job
func (p *MessageProcessor) Process(ctx context.Context, job *models.MessageJob) error {
	// Fetch the outbound message from database
	message, err := p.messageRepo.GetByID(ctx, job.OutboundMessageID)
	if err != nil {
		p.logger.Error("failed to fetch message",
			slog.Int64("message_id", job.OutboundMessageID),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to fetch message: %w", err)
	}

	// Fetch campaign to get channel information
	campaign, err := p.campaignRepo.GetByID(ctx, message.CampaignID)
	if err != nil {
		p.logger.Error("failed to fetch campaign",
			slog.Int64("campaign_id", message.CampaignID),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to fetch campaign: %w", err)
	}

	// Fetch customer to get phone number
	customer, err := p.customerRepo.GetByID(ctx, message.CustomerID)
	if err != nil {
		p.logger.Error("failed to fetch customer",
			slog.Int64("customer_id", message.CustomerID),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to fetch customer: %w", err)
	}

	p.logger.Info("processing message",
		slog.Int64("message_id", message.ID),
		slog.Int64("campaign_id", campaign.ID),
		slog.String("customer_phone", customer.Phone),
		slog.String("channel", campaign.Channel),
	)

	// Attempt to send the message
	err = p.sender.Send(ctx, campaign.Channel, customer.Phone, message.RenderedContent)

	if err != nil {
		// Sending failed
		p.logger.Warn("message send failed",
			slog.Int64("message_id", message.ID),
			slog.Int("retry_count", message.RetryCount),
			slog.String("error", err.Error()),
		)

		return p.handleFailure(ctx, message, err)
	}

	// Sending succeeded
	p.logger.Info("message sent successfully",
		slog.Int64("message_id", message.ID),
		slog.String("customer_phone", customer.Phone),
	)

	return p.handleSuccess(ctx, message)
}

// handleSuccess updates message status to sent
func (p *MessageProcessor) handleSuccess(ctx context.Context, message *models.OutboundMessage) error {
	err := p.messageRepo.UpdateStatus(ctx, message.ID, models.MessageStatusSent, nil)
	if err != nil {
		p.logger.Error("failed to update message status to sent",
			slog.Int64("message_id", message.ID),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to update message status: %w", err)
	}

	return nil
}

// handleFailure handles send failures with retry logic
func (p *MessageProcessor) handleFailure(ctx context.Context, message *models.OutboundMessage, sendErr error) error {
	// Increment retry count
	if err := p.messageRepo.IncrementRetryCount(ctx, message.ID); err != nil {
		p.logger.Error("failed to increment retry count",
			slog.Int64("message_id", message.ID),
			slog.String("error", err.Error()),
		)
		return err
	}

	// Check if we've exceeded max retries
	if message.RetryCount+1 >= p.maxRetries {
		// Max retries reached - mark as permanently failed
		p.logger.Error("message permanently failed after max retries",
			slog.Int64("message_id", message.ID),
			slog.Int("retry_count", message.RetryCount+1),
			slog.Int("max_retries", p.maxRetries),
		)

		errMsg := fmt.Sprintf("max retries exceeded: %s", sendErr.Error())
		if err := p.messageRepo.UpdateStatus(ctx, message.ID, models.MessageStatusFailed, &errMsg); err != nil {
			p.logger.Error("failed to update message status to failed",
				slog.Int64("message_id", message.ID),
				slog.String("error", err.Error()),
			)
			return err
		}

		return nil // Job processed (albeit failed)
	}

	// Still have retries left - update status but keep as failed for retry
	p.logger.Info("message will be retried",
		slog.Int64("message_id", message.ID),
		slog.Int("retry_count", message.RetryCount+1),
		slog.Int("max_retries", p.maxRetries),
	)

	errMsg := sendErr.Error()
	if err := p.messageRepo.UpdateStatus(ctx, message.ID, models.MessageStatusFailed, &errMsg); err != nil {
		p.logger.Error("failed to update message status",
			slog.Int64("message_id", message.ID),
			slog.String("error", err.Error()),
		)
		return err
	}

	// Return error so worker can potentially requeue if needed
	// Note: In this simple implementation, we don't auto-requeue
	// In production, you might add the job back to queue with exponential backoff
	return fmt.Errorf("send failed, retry %d/%d: %w", message.RetryCount+1, p.maxRetries, sendErr)
}
