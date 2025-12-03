package worker

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/models"
)

// Mock repositories for testing
type mockOutboundMessageRepo struct {
	messages map[int64]*models.OutboundMessage
	updates  []statusUpdate
}

type statusUpdate struct {
	id        int64
	status    string
	lastError *string
}

func (m *mockOutboundMessageRepo) GetByID(ctx context.Context, id int64) (*models.OutboundMessage, error) {
	msg, ok := m.messages[id]
	if !ok {
		return nil, models.ErrNotFoundWithMsg("message not found")
	}
	return msg, nil
}

func (m *mockOutboundMessageRepo) UpdateStatus(ctx context.Context, id int64, status string, lastError *string) error {
	msg, ok := m.messages[id]
	if !ok {
		return models.ErrNotFoundWithMsg("message not found")
	}
	msg.Status = status
	msg.LastError = lastError
	m.updates = append(m.updates, statusUpdate{id, status, lastError})
	return nil
}

func (m *mockOutboundMessageRepo) IncrementRetryCount(ctx context.Context, id int64) error {
	msg, ok := m.messages[id]
	if !ok {
		return models.ErrNotFoundWithMsg("message not found")
	}
	msg.RetryCount++
	return nil
}

// Unused methods for interface compliance
func (m *mockOutboundMessageRepo) Create(ctx context.Context, message *models.OutboundMessage) error {
	return nil
}
func (m *mockOutboundMessageRepo) CreateBatch(ctx context.Context, messages []*models.OutboundMessage) error {
	return nil
}
func (m *mockOutboundMessageRepo) List(ctx context.Context, filter models.OutboundMessageFilter) ([]*models.OutboundMessage, int64, error) {
	return nil, 0, nil
}
func (m *mockOutboundMessageRepo) Update(ctx context.Context, message *models.OutboundMessage) error {
	return nil
}
func (m *mockOutboundMessageRepo) GetPendingMessages(ctx context.Context, limit int) ([]*models.OutboundMessage, error) {
	return nil, nil
}

type mockCampaignRepo struct {
	campaigns map[int64]*models.CampaignWithStats
}

func (m *mockCampaignRepo) GetByID(ctx context.Context, id int64) (*models.Campaign, error) {
	campaign, ok := m.campaigns[id]
	if !ok {
		return nil, models.ErrNotFoundWithMsg("campaign not found")
	}
	return &models.Campaign{
		ID:           campaign.ID,
		Name:         campaign.Name,
		Channel:      campaign.Channel,
		Status:       campaign.Status,
		BaseTemplate: campaign.BaseTemplate,
	}, nil
}

func (m *mockCampaignRepo) GetWithStats(ctx context.Context, id int64) (*models.CampaignWithStats, error) {
	campaign, ok := m.campaigns[id]
	if !ok {
		return nil, models.ErrNotFoundWithMsg("campaign not found")
	}
	return campaign, nil
}

func (m *mockCampaignRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	campaign, ok := m.campaigns[id]
	if !ok {
		return models.ErrNotFoundWithMsg("campaign not found")
	}
	campaign.Status = status
	return nil
}

// Unused methods for interface compliance
func (m *mockCampaignRepo) Create(ctx context.Context, campaign *models.Campaign) error {
	return nil
}
func (m *mockCampaignRepo) List(ctx context.Context, filter models.CampaignFilter) ([]*models.Campaign, int64, error) {
	return nil, 0, nil
}
func (m *mockCampaignRepo) Update(ctx context.Context, campaign *models.Campaign) error {
	return nil
}
func (m *mockCampaignRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

type mockCustomerRepo struct {
	customers map[int64]*models.Customer
}

func (m *mockCustomerRepo) GetByID(ctx context.Context, id int64) (*models.Customer, error) {
	customer, ok := m.customers[id]
	if !ok {
		return nil, models.ErrNotFoundWithMsg("customer not found")
	}
	return customer, nil
}

// Unused methods for interface compliance
func (m *mockCustomerRepo) Create(ctx context.Context, customer *models.Customer) error {
	return nil
}
func (m *mockCustomerRepo) List(ctx context.Context, filter models.CustomerFilter) ([]*models.Customer, int64, error) {
	return nil, 0, nil
}
func (m *mockCustomerRepo) Update(ctx context.Context, customer *models.Customer) error {
	return nil
}
func (m *mockCustomerRepo) Delete(ctx context.Context, id int64) error {
	return nil
}
func (m *mockCustomerRepo) GetByPhone(ctx context.Context, phone string) (*models.Customer, error) {
	for _, customer := range m.customers {
		if customer.Phone == phone {
			return customer, nil
		}
	}
	return nil, models.ErrNotFoundWithMsg("customer not found")
}

type testMockSender struct {
	shouldFail bool
	calls      []sendCall
}

type sendCall struct {
	channel string
	phone   string
	content string
}

func (m *testMockSender) Send(ctx context.Context, channel, phone, content string) error {
	m.calls = append(m.calls, sendCall{channel, phone, content})
	if m.shouldFail {
		return errors.New("mock sender failed: simulated network error")
	}
	return nil
}

func TestMessageProcessor_Process_Success(t *testing.T) {
	messageRepo := &mockOutboundMessageRepo{
		messages: map[int64]*models.OutboundMessage{
			1: {
				ID:              1,
				CampaignID:      1,
				CustomerID:      1,
				Status:          models.MessageStatusPending,
				RenderedContent: "Hi Alice, check out Running Shoes!",
				RetryCount:      0,
			},
		},
		updates: []statusUpdate{},
	}

	campaignRepo := &mockCampaignRepo{
		campaigns: map[int64]*models.CampaignWithStats{
			1: {
				ID:      1,
				Name:    "Test Campaign",
				Channel: "sms",
				Status:  "sending",
				Stats: models.CampaignStats{
					Total:   1,
					Pending: 0, // All complete
					Sending: 0,
					Sent:    0,
					Failed:  0,
				},
			},
		},
	}

	customerRepo := &mockCustomerRepo{
		customers: map[int64]*models.Customer{
			1: {
				ID:        1,
				Phone:     "+254712345001",
				FirstName: "Alice",
			},
		},
	}

	sender := &testMockSender{shouldFail: false}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	processor := NewMessageProcessor(messageRepo, campaignRepo, customerRepo, sender, 3, logger)

	job := &models.MessageJob{OutboundMessageID: 1}

	err := processor.Process(context.Background(), job)
	if err != nil {
		t.Errorf("Process() error = %v, want nil", err)
	}

	// Verify message status updated to "sent"
	if len(messageRepo.updates) != 1 {
		t.Fatalf("Expected 1 status update, got %d", len(messageRepo.updates))
	}
	if messageRepo.updates[0].status != models.MessageStatusSent {
		t.Errorf("Message status = %s, want %s", messageRepo.updates[0].status, models.MessageStatusSent)
	}

	// Verify sender was called
	if len(sender.calls) != 1 {
		t.Fatalf("Expected 1 sender call, got %d", len(sender.calls))
	}
	if sender.calls[0].channel != "sms" {
		t.Errorf("Sender channel = %s, want sms", sender.calls[0].channel)
	}
	if sender.calls[0].phone != "+254712345001" {
		t.Errorf("Sender phone = %s, want +254712345001", sender.calls[0].phone)
	}
}

func TestMessageProcessor_Process_Failure_WithRetries(t *testing.T) {
	tests := []struct {
		name           string
		retryCount     int
		maxRetries     int
		wantStatus     string
		wantRetryCount int
		wantErr        bool
	}{
		{
			name:           "first failure, retry available",
			retryCount:     0,
			maxRetries:     3,
			wantStatus:     models.MessageStatusFailed,
			wantRetryCount: 1,
			wantErr:        true,
		},
		{
			name:           "at max retries, becomes permanent",
			retryCount:     2,
			maxRetries:     3,
			wantStatus:     models.MessageStatusFailed,
			wantRetryCount: 3,
			wantErr:        false, // Permanent failure, no error returned
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messageRepo := &mockOutboundMessageRepo{
				messages: map[int64]*models.OutboundMessage{
					1: {
						ID:              1,
						CampaignID:      1,
						CustomerID:      1,
						Status:          models.MessageStatusPending,
						RenderedContent: "Hi Alice!",
						RetryCount:      tt.retryCount,
					},
				},
				updates: []statusUpdate{},
			}

			campaignRepo := &mockCampaignRepo{
				campaigns: map[int64]*models.CampaignWithStats{
					1: {
						ID:      1,
						Channel: "sms",
						Status:  "sending",
						Stats: models.CampaignStats{
							Total:   1,
							Pending: 0,
							Sending: 0,
							Sent:    0,
							Failed:  0,
						},
					},
				},
			}

			customerRepo := &mockCustomerRepo{
				customers: map[int64]*models.Customer{
					1: {ID: 1, Phone: "+254712345001"},
				},
			}

			sender := &testMockSender{shouldFail: true}

			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			processor := NewMessageProcessor(messageRepo, campaignRepo, customerRepo, sender, tt.maxRetries, logger)

			job := &models.MessageJob{OutboundMessageID: 1}

			err := processor.Process(context.Background(), job)

			if (err != nil) != tt.wantErr {
				t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify status updated
			if len(messageRepo.updates) == 0 {
				t.Fatal("Expected status update, got none")
			}

			lastUpdate := messageRepo.updates[len(messageRepo.updates)-1]
			if lastUpdate.status != tt.wantStatus {
				t.Errorf("Message status = %s, want %s", lastUpdate.status, tt.wantStatus)
			}

			// Verify retry count incremented
			if messageRepo.messages[1].RetryCount != tt.wantRetryCount {
				t.Errorf("RetryCount = %d, want %d", messageRepo.messages[1].RetryCount, tt.wantRetryCount)
			}

			// Verify error message populated
			if lastUpdate.lastError == nil {
				t.Error("lastError is nil, want error message")
			}
		})
	}
}

func TestMessageProcessor_Process_CampaignStatusUpdate(t *testing.T) {
	tests := []struct {
		name              string
		initialStatus     string
		pendingCount      int64
		sentCount         int64
		failedCount       int64
		wantCampaignStatus string
	}{
		{
			name:               "all messages sent successfully",
			initialStatus:      "sending",
			pendingCount:       0,
			sentCount:          5,
			failedCount:        0,
			wantCampaignStatus: "sent",
		},
		{
			name:               "some messages failed, some sent",
			initialStatus:      "sending",
			pendingCount:       0,
			sentCount:          3,
			failedCount:        2,
			wantCampaignStatus: "sent", // At least some sent
		},
		{
			name:               "all messages failed",
			initialStatus:      "sending",
			pendingCount:       0,
			sentCount:          0,
			failedCount:        5,
			wantCampaignStatus: "failed",
		},
		{
			name:               "still has pending messages",
			initialStatus:      "sending",
			pendingCount:       3,
			sentCount:          2,
			failedCount:        0,
			wantCampaignStatus: "sending", // Should not change
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messageRepo := &mockOutboundMessageRepo{
				messages: map[int64]*models.OutboundMessage{
					1: {
						ID:              1,
						CampaignID:      1,
						CustomerID:      1,
						Status:          models.MessageStatusPending,
						RenderedContent: "test",
						RetryCount:      0,
					},
				},
				updates: []statusUpdate{},
			}

			campaignRepo := &mockCampaignRepo{
				campaigns: map[int64]*models.CampaignWithStats{
					1: {
						ID:      1,
						Channel: "sms",
						Status:  tt.initialStatus,
						Stats: models.CampaignStats{
							Total:   tt.pendingCount + tt.sentCount + tt.failedCount,
							Pending: tt.pendingCount,
							Sending: 0,
							Sent:    tt.sentCount,
							Failed:  tt.failedCount,
						},
					},
				},
			}

			customerRepo := &mockCustomerRepo{
				customers: map[int64]*models.Customer{
					1: {ID: 1, Phone: "+254712345001"},
				},
			}

			sender := &testMockSender{shouldFail: false}

			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			processor := NewMessageProcessor(messageRepo, campaignRepo, customerRepo, sender, 3, logger)

			job := &models.MessageJob{OutboundMessageID: 1}
			_ = processor.Process(context.Background(), job)

			// Verify campaign status
			if campaignRepo.campaigns[1].Status != tt.wantCampaignStatus {
				t.Errorf("Campaign status = %s, want %s", campaignRepo.campaigns[1].Status, tt.wantCampaignStatus)
			}
		})
	}
}

func TestMockSender_Send(t *testing.T) {
	tests := []struct {
		name      string
		successRate float64
		iterations int
		wantSuccessRange [2]int // min, max expected successes
	}{
		{
			name:             "100% success rate",
			successRate:      1.0,
			iterations:       10,
			wantSuccessRange: [2]int{10, 10},
		},
		{
			name:             "92% success rate (default)",
			successRate:      0.92,
			iterations:       100,
			wantSuccessRange: [2]int{85, 100}, // Allow some variance
		},
		{
			name:             "50% success rate",
			successRate:      0.5,
			iterations:       100,
			wantSuccessRange: [2]int{35, 65}, // Allow variance around 50%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := NewMockSender(tt.successRate)
			successes := 0

			for i := 0; i < tt.iterations; i++ {
				err := sender.Send(context.Background(), "sms", "+254712345001", "test message")
				if err == nil {
					successes++
				}
			}

			if successes < tt.wantSuccessRange[0] || successes > tt.wantSuccessRange[1] {
				t.Errorf("MockSender successes = %d, want between %d and %d",
					successes, tt.wantSuccessRange[0], tt.wantSuccessRange[1])
			}
		})
	}
}
