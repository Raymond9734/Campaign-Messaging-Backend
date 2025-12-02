package service

import (
	"time"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/models"
)

// CreateCampaignRequest represents a request to create a campaign
type CreateCampaignRequest struct {
	Name         string     `json:"name"`
	Channel      string     `json:"channel"`
	BaseTemplate string     `json:"base_template"`
	ScheduledAt  *time.Time `json:"scheduled_at,omitempty"`
}

// Validate performs validation on the create campaign request
func (r *CreateCampaignRequest) Validate() error {
	if r.Name == "" {
		return models.ErrInvalidInput("name is required")
	}
	if r.Channel == "" {
		return models.ErrInvalidInput("channel is required")
	}
	if !models.IsValidChannel(r.Channel) {
		return models.ErrInvalidInput("invalid channel (must be 'sms' or 'whatsapp')")
	}
	if r.BaseTemplate == "" {
		return models.ErrInvalidInput("base_template is required")
	}
	return nil
}

// SendCampaignRequest represents a request to send a campaign
type SendCampaignRequest struct {
	CustomerIDs []int64 `json:"customer_ids"`
}

// Validate performs validation on the send campaign request
func (r *SendCampaignRequest) Validate() error {
	if len(r.CustomerIDs) == 0 {
		return models.ErrInvalidInput("customer_ids is required and cannot be empty")
	}
	return nil
}

// SendCampaignResult represents the result of sending a campaign
type SendCampaignResult struct {
	CampaignID     int64  `json:"campaign_id"`
	MessagesQueued int    `json:"messages_queued"`
	Status         string `json:"status"`
}

// PreviewRequest represents a request to preview a personalized message
type PreviewRequest struct {
	CustomerID       int64   `json:"customer_id"`
	OverrideTemplate *string `json:"override_template,omitempty"`
}

// Validate performs validation on the preview request
func (r *PreviewRequest) Validate() error {
	if r.CustomerID <= 0 {
		return models.ErrInvalidInput("customer_id is required")
	}
	return nil
}

// PreviewResult represents the result of a personalized preview
type PreviewResult struct {
	RenderedMessage string           `json:"rendered_message"`
	UsedTemplate    string           `json:"used_template"`
	Customer        *CustomerPreview `json:"customer"`
}

// CustomerPreview contains minimal customer info for preview
type CustomerPreview struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

// CampaignListResult represents paginated campaign list results
type CampaignListResult struct {
	Data       []*models.Campaign       `json:"data"`
	Pagination models.PaginationResult  `json:"pagination"`
}
