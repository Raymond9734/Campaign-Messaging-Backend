package models

import (
	"fmt"
	"time"
)

// Campaign status constants
const (
	CampaignStatusDraft     = "draft"
	CampaignStatusScheduled = "scheduled"
	CampaignStatusSending   = "sending"
	CampaignStatusSent      = "sent"
	CampaignStatusFailed    = "failed"
)

// Campaign channel constants
const (
	ChannelSMS      = "sms"
	ChannelWhatsApp = "whatsapp"
)

// Campaign represents a messaging campaign
type Campaign struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Channel      string     `json:"channel"`
	Status       string     `json:"status"`
	BaseTemplate string     `json:"base_template"`
	ScheduledAt  *time.Time `json:"scheduled_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// CampaignFilter holds filtering options for listing campaigns
type CampaignFilter struct {
	Channel  string
	Status   string
	Page     int
	PageSize int
}

// CampaignStats holds statistics for a campaign
type CampaignStats struct {
	Total   int64 `json:"total"`
	Pending int64 `json:"pending"`
	Sent    int64 `json:"sent"`
	Failed  int64 `json:"failed"`
}

// CampaignWithStats combines campaign details with statistics
type CampaignWithStats struct {
	Campaign
	Stats CampaignStats `json:"stats"`
}

// Validate performs validation on campaign data
func (c *Campaign) Validate() error {
	if c.Name == "" {
		return ErrInvalidInput("name is required")
	}
	if c.Channel == "" {
		return ErrInvalidInput("channel is required")
	}
	if !IsValidChannel(c.Channel) {
		return ErrInvalidInput(fmt.Sprintf("invalid channel: %s (must be 'sms' or 'whatsapp')", c.Channel))
	}
	if c.BaseTemplate == "" {
		return ErrInvalidInput("base_template is required")
	}
	if c.Status != "" && !IsValidCampaignStatus(c.Status) {
		return ErrInvalidInput(fmt.Sprintf("invalid status: %s", c.Status))
	}
	return nil
}

// IsValidChannel checks if the channel is valid
func IsValidChannel(channel string) bool {
	return channel == ChannelSMS || channel == ChannelWhatsApp
}

// IsValidCampaignStatus checks if the campaign status is valid
func IsValidCampaignStatus(status string) bool {
	switch status {
	case CampaignStatusDraft, CampaignStatusScheduled, CampaignStatusSending, CampaignStatusSent, CampaignStatusFailed:
		return true
	default:
		return false
	}
}

// CanBeSent checks if a campaign can be sent
// This provides idempotency: once a campaign is "sending", "sent", or "failed",
// it cannot be sent again, preventing duplicate sends if API is called multiple times
func (c *Campaign) CanBeSent() bool {
	return c.Status == CampaignStatusDraft || c.Status == CampaignStatusScheduled
}
