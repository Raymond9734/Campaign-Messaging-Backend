package service

import (
	"context"
	"testing"

	"github.com/Raymond9734/campaign-messaging-backend/internal/models"
)

// MockCampaignRepository for testing
type mockCampaignRepository struct {
	campaigns []*models.Campaign
}

func (m *mockCampaignRepository) Create(ctx context.Context, campaign *models.Campaign) error {
	campaign.ID = int64(len(m.campaigns) + 1)
	m.campaigns = append(m.campaigns, campaign)
	return nil
}

func (m *mockCampaignRepository) GetByID(ctx context.Context, id int64) (*models.Campaign, error) {
	for _, c := range m.campaigns {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, models.ErrNotFoundWithMsg("campaign not found")
}

func (m *mockCampaignRepository) GetWithStats(ctx context.Context, id int64) (*models.CampaignWithStats, error) {
	campaign, err := m.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &models.CampaignWithStats{
		ID:           campaign.ID,
		Name:         campaign.Name,
		Channel:      campaign.Channel,
		Status:       campaign.Status,
		BaseTemplate: campaign.BaseTemplate,
		ScheduledAt:  campaign.ScheduledAt,
		CreatedAt:    campaign.CreatedAt,
		Stats: models.CampaignStats{
			Total:   0,
			Pending: 0,
			Sending: 0,
			Sent:    0,
			Failed:  0,
		},
	}, nil
}

func (m *mockCampaignRepository) List(ctx context.Context, filter models.CampaignFilter) ([]*models.Campaign, int64, error) {
	// Apply filters
	filtered := []*models.Campaign{}
	for _, c := range m.campaigns {
		if filter.Channel != "" && c.Channel != filter.Channel {
			continue
		}
		if filter.Status != "" && c.Status != filter.Status {
			continue
		}
		filtered = append(filtered, c)
	}

	totalCount := int64(len(filtered))

	// Apply pagination
	models.ValidateAndSetDefaults(&filter.Page, &filter.PageSize)
	offset := models.CalculateOffset(filter.Page, filter.PageSize)

	start := offset
	if start > len(filtered) {
		start = len(filtered)
	}

	end := start + filter.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], totalCount, nil
}

func (m *mockCampaignRepository) Update(ctx context.Context, campaign *models.Campaign) error {
	for i, c := range m.campaigns {
		if c.ID == campaign.ID {
			m.campaigns[i] = campaign
			return nil
		}
	}
	return models.ErrNotFoundWithMsg("campaign not found")
}

func (m *mockCampaignRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	for _, c := range m.campaigns {
		if c.ID == id {
			c.Status = status
			return nil
		}
	}
	return models.ErrNotFoundWithMsg("campaign not found")
}

func (m *mockCampaignRepository) Delete(ctx context.Context, id int64) error {
	for i, c := range m.campaigns {
		if c.ID == id {
			m.campaigns = append(m.campaigns[:i], m.campaigns[i+1:]...)
			return nil
		}
	}
	return models.ErrNotFoundWithMsg("campaign not found")
}

func TestCampaignService_List_Pagination(t *testing.T) {
	tests := []struct {
		name            string
		totalCampaigns  int
		page            int
		pageSize        int
		wantCount       int
		wantTotalCount  int64
		wantTotalPages  int
	}{
		{
			name:            "first page with default page size (20)",
			totalCampaigns:  50,
			page:            1,
			pageSize:        20,
			wantCount:       20,
			wantTotalCount:  50,
			wantTotalPages:  3,
		},
		{
			name:            "second page",
			totalCampaigns:  50,
			page:            2,
			pageSize:        20,
			wantCount:       20,
			wantTotalCount:  50,
			wantTotalPages:  3,
		},
		{
			name:            "last page (partial)",
			totalCampaigns:  50,
			page:            3,
			pageSize:        20,
			wantCount:       10,
			wantTotalCount:  50,
			wantTotalPages:  3,
		},
		{
			name:            "page beyond last (empty)",
			totalCampaigns:  50,
			page:            10,
			pageSize:        20,
			wantCount:       0,
			wantTotalCount:  50,
			wantTotalPages:  3,
		},
		{
			name:            "small page size",
			totalCampaigns:  25,
			page:            1,
			pageSize:        5,
			wantCount:       5,
			wantTotalCount:  25,
			wantTotalPages:  5,
		},
		{
			name:            "page size larger than total",
			totalCampaigns:  10,
			page:            1,
			pageSize:        50,
			wantCount:       10,
			wantTotalCount:  10,
			wantTotalPages:  1,
		},
		{
			name:            "zero page defaults to 1",
			totalCampaigns:  30,
			page:            0,
			pageSize:        10,
			wantCount:       10,
			wantTotalCount:  30,
			wantTotalPages:  3,
		},
		{
			name:            "negative page defaults to 1",
			totalCampaigns:  30,
			page:            -1,
			pageSize:        10,
			wantCount:       10,
			wantTotalCount:  30,
			wantTotalPages:  3,
		},
		{
			name:            "zero page size defaults to 20",
			totalCampaigns:  50,
			page:            1,
			pageSize:        0,
			wantCount:       20,
			wantTotalCount:  50,
			wantTotalPages:  3,
		},
		{
			name:            "page size over 100 capped at 100",
			totalCampaigns:  150,
			page:            1,
			pageSize:        200,
			wantCount:       100,
			wantTotalCount:  150,
			wantTotalPages:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock repository with campaigns
			mockRepo := &mockCampaignRepository{
				campaigns: make([]*models.Campaign, tt.totalCampaigns),
			}

			for i := 0; i < tt.totalCampaigns; i++ {
				mockRepo.campaigns[i] = &models.Campaign{
					ID:           int64(i + 1),
					Name:         "Campaign " + string(rune(i+1)),
					Channel:      "sms",
					Status:       "draft",
					BaseTemplate: "test",
				}
			}

			// Create service with mock
			svc := &campaignService{
				campaignRepo: mockRepo,
			}

			// Test pagination
			filter := models.CampaignFilter{
				Page:     tt.page,
				PageSize: tt.pageSize,
			}

			result, err := svc.List(context.Background(), filter)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}

			// Verify count
			if len(result.Data) != tt.wantCount {
				t.Errorf("List() returned %d campaigns, want %d", len(result.Data), tt.wantCount)
			}

			// Verify total count
			if result.Pagination.TotalCount != tt.wantTotalCount {
				t.Errorf("List() TotalCount = %d, want %d", result.Pagination.TotalCount, tt.wantTotalCount)
			}

			// Verify total pages
			if result.Pagination.TotalPages != tt.wantTotalPages {
				t.Errorf("List() TotalPages = %d, want %d", result.Pagination.TotalPages, tt.wantTotalPages)
			}

			// Verify page number
			expectedPage := tt.page
			if tt.page <= 0 {
				expectedPage = 1
			}
			if result.Pagination.Page != expectedPage {
				t.Errorf("List() Page = %d, want %d", result.Pagination.Page, expectedPage)
			}

			// Verify page size
			expectedPageSize := tt.pageSize
			if tt.pageSize <= 0 {
				expectedPageSize = 20
			} else if tt.pageSize > 100 {
				expectedPageSize = 100
			}
			if result.Pagination.PageSize != expectedPageSize {
				t.Errorf("List() PageSize = %d, want %d", result.Pagination.PageSize, expectedPageSize)
			}
		})
	}
}

func TestCampaignService_List_Filtering(t *testing.T) {
	tests := []struct {
		name       string
		campaigns  []*models.Campaign
		filter     models.CampaignFilter
		wantCount  int
	}{
		{
			name: "filter by channel sms",
			campaigns: []*models.Campaign{
				{ID: 1, Channel: "sms", Status: "draft"},
				{ID: 2, Channel: "whatsapp", Status: "draft"},
				{ID: 3, Channel: "sms", Status: "draft"},
			},
			filter: models.CampaignFilter{
				Channel:  "sms",
				Page:     1,
				PageSize: 20,
			},
			wantCount: 2,
		},
		{
			name: "filter by status sent",
			campaigns: []*models.Campaign{
				{ID: 1, Channel: "sms", Status: "draft"},
				{ID: 2, Channel: "sms", Status: "sent"},
				{ID: 3, Channel: "sms", Status: "sent"},
			},
			filter: models.CampaignFilter{
				Status:   "sent",
				Page:     1,
				PageSize: 20,
			},
			wantCount: 2,
		},
		{
			name: "filter by both channel and status",
			campaigns: []*models.Campaign{
				{ID: 1, Channel: "sms", Status: "draft"},
				{ID: 2, Channel: "sms", Status: "sent"},
				{ID: 3, Channel: "whatsapp", Status: "sent"},
			},
			filter: models.CampaignFilter{
				Channel:  "sms",
				Status:   "sent",
				Page:     1,
				PageSize: 20,
			},
			wantCount: 1,
		},
		{
			name: "no filters (all campaigns)",
			campaigns: []*models.Campaign{
				{ID: 1, Channel: "sms", Status: "draft"},
				{ID: 2, Channel: "whatsapp", Status: "sent"},
				{ID: 3, Channel: "sms", Status: "failed"},
			},
			filter: models.CampaignFilter{
				Page:     1,
				PageSize: 20,
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockCampaignRepository{
				campaigns: tt.campaigns,
			}

			svc := &campaignService{
				campaignRepo: mockRepo,
			}

			result, err := svc.List(context.Background(), tt.filter)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}

			if len(result.Data) != tt.wantCount {
				t.Errorf("List() returned %d campaigns, want %d", len(result.Data), tt.wantCount)
			}
		})
	}
}

func TestCampaignService_List_Stability(t *testing.T) {
	// Test that pagination is stable (ORDER BY id DESC)
	mockRepo := &mockCampaignRepository{
		campaigns: []*models.Campaign{
			{ID: 1, Channel: "sms", Status: "draft", Name: "Campaign 1"},
			{ID: 2, Channel: "sms", Status: "draft", Name: "Campaign 2"},
			{ID: 3, Channel: "sms", Status: "draft", Name: "Campaign 3"},
			{ID: 4, Channel: "sms", Status: "draft", Name: "Campaign 4"},
			{ID: 5, Channel: "sms", Status: "draft", Name: "Campaign 5"},
		},
	}

	svc := &campaignService{
		campaignRepo: mockRepo,
	}

	// Fetch page 1
	result1, err := svc.List(context.Background(), models.CampaignFilter{
		Page:     1,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Fetch page 2
	result2, err := svc.List(context.Background(), models.CampaignFilter{
		Page:     2,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Verify no overlap (stability check)
	page1IDs := make(map[int64]bool)
	for _, c := range result1.Data {
		page1IDs[c.ID] = true
	}

	for _, c := range result2.Data {
		if page1IDs[c.ID] {
			t.Errorf("Found duplicate campaign ID %d across pages (pagination not stable)", c.ID)
		}
	}
}
