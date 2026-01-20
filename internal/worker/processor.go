package worker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Raymond9734/campaign-messaging-backend/internal/models"
	"github.com/Raymond9734/campaign-messaging-backend/internal/repository"
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

	// Check if all messages for this campaign are complete
	p.updateCampaignStatusIfComplete(ctx, message.CampaignID)

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

		// Check if all messages for this campaign are complete
		p.updateCampaignStatusIfComplete(ctx, message.CampaignID)

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
	// In production, we might add the job back to queue with exponential backoff
	return fmt.Errorf("send failed, retry %d/%d: %w", message.RetryCount+1, p.maxRetries, sendErr)
}

// updateCampaignStatusIfComplete checks if all messages for a campaign are complete
// and updates the campaign status accordingly
func (p *MessageProcessor) updateCampaignStatusIfComplete(ctx context.Context, campaignID int64) {
	// Get campaign with stats
	campaign, err := p.campaignRepo.GetWithStats(ctx, campaignID)
	if err != nil {
		p.logger.Error("failed to get campaign stats",
			slog.Int64("campaign_id", campaignID),
			slog.String("error", err.Error()),
		)
		return
	}

	// Check if all messages are complete (no pending messages)
	if campaign.Stats.Pending > 0 {
		p.logger.Info("campaign still has pending messages",
			slog.Int64("campaign_id", campaignID),
			slog.Int64("pending", campaign.Stats.Pending),
		)
		return
	}

	// All messages complete - determine final status
	var newStatus string
	if campaign.Stats.Failed > 0 && campaign.Stats.Sent == 0 {
		// All messages failed
		newStatus = models.CampaignStatusFailed
	} else {
		// At least some messages sent successfully
		newStatus = models.CampaignStatusSent
	}

	// Only update if status changed
	if campaign.Status == newStatus {
		return
	}

	// Update campaign status
	err = p.campaignRepo.UpdateStatus(ctx, campaignID, newStatus)
	if err != nil {
		p.logger.Error("failed to update campaign status",
			slog.Int64("campaign_id", campaignID),
			slog.String("new_status", newStatus),
			slog.String("error", err.Error()),
		)
		return
	}

	p.logger.Info("campaign status updated",
		slog.Int64("campaign_id", campaignID),
		slog.String("status", newStatus),
		slog.Int64("total", campaign.Stats.Total),
		slog.Int64("sent", campaign.Stats.Sent),
		slog.Int64("failed", campaign.Stats.Failed),
	)
}
