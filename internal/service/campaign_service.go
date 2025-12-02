package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/models"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/queue"
	"github.com/Raymond9734/smsleopard-backend-challenge/internal/repository"
)

// CampaignService handles campaign business logic
type CampaignService interface {
	Create(ctx context.Context, req *CreateCampaignRequest) (*models.Campaign, error)
	GetByID(ctx context.Context, id int64) (*models.CampaignWithStats, error)
	List(ctx context.Context, filter models.CampaignFilter) (*CampaignListResult, error)
	SendCampaign(ctx context.Context, campaignID int64, req *SendCampaignRequest) (*SendCampaignResult, error)
	PreviewPersonalized(ctx context.Context, campaignID int64, req *PreviewRequest) (*PreviewResult, error)
}

type campaignService struct {
	campaignRepo repository.CampaignRepository
	customerRepo repository.CustomerRepository
	messageRepo  repository.OutboundMessageRepository
	templateSvc  TemplateService
	queueClient  queue.Client
	logger       *slog.Logger
}

// NewCampaignService creates a new campaign service
func NewCampaignService(
	campaignRepo repository.CampaignRepository,
	customerRepo repository.CustomerRepository,
	messageRepo repository.OutboundMessageRepository,
	templateSvc TemplateService,
	queueClient queue.Client,
	logger *slog.Logger,
) CampaignService {
	return &campaignService{
		campaignRepo: campaignRepo,
		customerRepo: customerRepo,
		messageRepo:  messageRepo,
		templateSvc:  templateSvc,
		queueClient:  queueClient,
		logger:       logger,
	}
}

// Create creates a new campaign
func (s *campaignService) Create(ctx context.Context, req *CreateCampaignRequest) (*models.Campaign, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Validate template syntax
	if err := s.templateSvc.ValidateTemplate(req.BaseTemplate); err != nil {
		return nil, err
	}

	// Determine initial status
	status := models.CampaignStatusDraft
	if req.ScheduledAt != nil {
		status = models.CampaignStatusScheduled
	}

	// Create campaign
	campaign := &models.Campaign{
		Name:         req.Name,
		Channel:      req.Channel,
		Status:       status,
		BaseTemplate: req.BaseTemplate,
		ScheduledAt:  req.ScheduledAt,
	}

	if err := s.campaignRepo.Create(ctx, campaign); err != nil {
		s.logger.Error("failed to create campaign",
			slog.String("error", err.Error()),
			slog.String("name", req.Name),
		)
		return nil, fmt.Errorf("failed to create campaign: %w", err)
	}

	s.logger.Info("campaign created",
		slog.Int64("campaign_id", campaign.ID),
		slog.String("name", campaign.Name),
		slog.String("status", campaign.Status),
	)

	return campaign, nil
}

// GetByID retrieves a campaign with statistics
func (s *campaignService) GetByID(ctx context.Context, id int64) (*models.CampaignWithStats, error) {
	campaign, err := s.campaignRepo.GetWithStats(ctx, id)
	if err != nil {
		return nil, err
	}

	return campaign, nil
}

// List retrieves campaigns with pagination
func (s *campaignService) List(ctx context.Context, filter models.CampaignFilter) (*CampaignListResult, error) {
	campaigns, totalCount, err := s.campaignRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list campaigns: %w", err)
	}

	// Validate and set defaults for pagination
	models.ValidateAndSetDefaults(&filter.Page, &filter.PageSize)

	pagination := models.NewPaginationResult(filter.Page, filter.PageSize, totalCount)

	return &CampaignListResult{
		Data:       campaigns,
		Pagination: pagination,
	}, nil
}

// SendCampaign sends a campaign to specified customers
func (s *campaignService) SendCampaign(ctx context.Context, campaignID int64, req *SendCampaignRequest) (*SendCampaignResult, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Get campaign
	campaign, err := s.campaignRepo.GetByID(ctx, campaignID)
	if err != nil {
		return nil, err
	}

	// Check if campaign can be sent
	if !campaign.CanBeSent() {
		return nil, models.ErrConflictWithMsg(
			fmt.Sprintf("campaign with status '%s' cannot be sent", campaign.Status),
		)
	}

	// Create outbound messages for each customer
	messages := make([]*models.OutboundMessage, 0, len(req.CustomerIDs))
	for _, customerID := range req.CustomerIDs {
		// Get customer
		customer, err := s.customerRepo.GetByID(ctx, customerID)
		if err != nil {
			s.logger.Warn("customer not found, skipping",
				slog.Int64("customer_id", customerID),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Render message content
		renderedContent, err := s.templateSvc.Render(campaign.BaseTemplate, customer)
		if err != nil {
			s.logger.Error("failed to render template",
				slog.Int64("campaign_id", campaignID),
				slog.Int64("customer_id", customerID),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Create outbound message
		message := &models.OutboundMessage{
			CampaignID:      campaign.ID,
			CustomerID:      customer.ID,
			Status:          models.MessageStatusPending,
			RenderedContent: renderedContent,
			RetryCount:      0,
		}

		messages = append(messages, message)
	}

	if len(messages) == 0 {
		return nil, models.ErrInvalidInput("no valid customers found to send messages")
	}

	// Batch create messages
	if err := s.messageRepo.CreateBatch(ctx, messages); err != nil {
		s.logger.Error("failed to create messages",
			slog.Int64("campaign_id", campaignID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to create messages: %w", err)
	}

	// Queue messages for sending
	queuedCount := 0
	for _, message := range messages {
		job := &models.MessageJob{
			OutboundMessageID: message.ID,
		}

		if err := s.queueClient.Publish(ctx, job); err != nil {
			s.logger.Error("failed to queue message",
				slog.Int64("message_id", message.ID),
				slog.String("error", err.Error()),
			)
			continue
		}
		queuedCount++
	}

	// Update campaign status to sending
	if err := s.campaignRepo.UpdateStatus(ctx, campaign.ID, models.CampaignStatusSending); err != nil {
		s.logger.Error("failed to update campaign status",
			slog.Int64("campaign_id", campaignID),
			slog.String("error", err.Error()),
		)
		// Don't fail the request if status update fails
	}

	s.logger.Info("campaign sent",
		slog.Int64("campaign_id", campaignID),
		slog.Int("messages_queued", queuedCount),
	)

	return &SendCampaignResult{
		CampaignID:     campaign.ID,
		MessagesQueued: queuedCount,
		Status:         models.CampaignStatusSending,
	}, nil
}

// PreviewPersonalized generates a preview of a personalized message
func (s *campaignService) PreviewPersonalized(ctx context.Context, campaignID int64, req *PreviewRequest) (*PreviewResult, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Get campaign
	campaign, err := s.campaignRepo.GetByID(ctx, campaignID)
	if err != nil {
		return nil, err
	}

	// Get customer
	customer, err := s.customerRepo.GetByID(ctx, req.CustomerID)
	if err != nil {
		return nil, err
	}

	// Determine which template to use
	templateToUse := campaign.BaseTemplate
	if req.OverrideTemplate != nil && *req.OverrideTemplate != "" {
		templateToUse = *req.OverrideTemplate

		// Validate override template
		if err := s.templateSvc.ValidateTemplate(templateToUse); err != nil {
			return nil, err
		}
	}

	// Render message
	renderedMessage, err := s.templateSvc.Render(templateToUse, customer)
	if err != nil {
		return nil, fmt.Errorf("failed to render message: %w", err)
	}

	return &PreviewResult{
		RenderedMessage: renderedMessage,
		UsedTemplate:    templateToUse,
		Customer: &CustomerPreview{
			ID:        customer.ID,
			FirstName: customer.FirstName,
			LastName:  customer.LastName,
			Phone:     customer.Phone,
		},
	}, nil
}
