package models

import "time"

// Outbound message status constants
const (
	MessageStatusPending = "pending"
	MessageStatusSent    = "sent"
	MessageStatusFailed  = "failed"
)

// OutboundMessage represents a message to be sent to a customer
type OutboundMessage struct {
	ID              int64     `json:"id"`
	CampaignID      int64     `json:"campaign_id"`
	CustomerID      int64     `json:"customer_id"`
	Status          string    `json:"status"`
	RenderedContent string    `json:"rendered_content"`
	LastError       *string   `json:"last_error,omitempty"`
	RetryCount      int       `json:"retry_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// OutboundMessageFilter holds filtering options for listing messages
type OutboundMessageFilter struct {
	CampaignID int64
	CustomerID int64
	Status     string
	Page       int
	PageSize   int
}

// MessageJob represents a job to be queued for processing
type MessageJob struct {
	OutboundMessageID int64 `json:"outbound_message_id"`
}

// IsValidMessageStatus checks if the message status is valid
func IsValidMessageStatus(status string) bool {
	switch status {
	case MessageStatusPending, MessageStatusSent, MessageStatusFailed:
		return true
	default:
		return false
	}
}

// CanRetry checks if a message can be retried
func (m *OutboundMessage) CanRetry(maxRetries int) bool {
	return m.Status == MessageStatusFailed && m.RetryCount < maxRetries
}
