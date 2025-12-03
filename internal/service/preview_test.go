package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/models"
)

func TestCampaignService_PreviewPersonalized(t *testing.T) {
	tests := []struct {
		name            string
		campaign        *models.Campaign
		customer        *models.Customer
		overrideTemplate *string
		wantMessage     string
		wantTemplate    string
		wantErr         bool
	}{
		{
			name: "use campaign template",
			campaign: &models.Campaign{
				ID:           1,
				Name:         "Test Campaign",
				BaseTemplate: "Hi {first_name}, check {preferred_product}!",
			},
			customer: &models.Customer{
				ID:               1,
				FirstName:        "Alice",
				PreferredProduct: "Running Shoes",
			},
			overrideTemplate: nil,
			wantMessage:      "Hi Alice, check Running Shoes!",
			wantTemplate:     "Hi {first_name}, check {preferred_product}!",
			wantErr:          false,
		},
		{
			name: "use override template",
			campaign: &models.Campaign{
				ID:           1,
				BaseTemplate: "Old template",
			},
			customer: &models.Customer{
				ID:        1,
				FirstName: "Bob",
				Location:  "Nairobi",
			},
			overrideTemplate: stringPtr("Hello {first_name} from {location}!"),
			wantMessage:      "Hello Bob from Nairobi!",
			wantTemplate:     "Hello {first_name} from {location}!",
			wantErr:          false,
		},
		{
			name: "empty override template uses campaign template",
			campaign: &models.Campaign{
				ID:           1,
				BaseTemplate: "Campaign: {first_name}",
			},
			customer: &models.Customer{
				ID:        1,
				FirstName: "Charlie",
			},
			overrideTemplate: stringPtr(""),
			wantMessage:      "Campaign: Charlie",
			wantTemplate:     "Campaign: {first_name}",
			wantErr:          false,
		},
		{
			name: "missing customer fields filled with empty string",
			campaign: &models.Campaign{
				ID:           1,
				BaseTemplate: "Hi {first_name} {last_name}, {preferred_product}",
			},
			customer: &models.Customer{
				ID:               1,
				FirstName:        "David",
				LastName:         "",
				PreferredProduct: "",
			},
			overrideTemplate: nil,
			wantMessage:      "Hi David , ",
			wantTemplate:     "Hi {first_name} {last_name}, {preferred_product}",
			wantErr:          false,
		},
		{
			name: "all placeholders used",
			campaign: &models.Campaign{
				ID:           1,
				BaseTemplate: "{first_name} {last_name}, {location}, {preferred_product}, {phone}",
			},
			customer: &models.Customer{
				ID:               1,
				FirstName:        "Eve",
				LastName:         "Mwangi",
				Location:         "Mombasa",
				PreferredProduct: "Laptop",
				Phone:            "+254712345678",
			},
			overrideTemplate: nil,
			wantMessage:      "Eve Mwangi, Mombasa, Laptop, +254712345678",
			wantTemplate:     "{first_name} {last_name}, {location}, {preferred_product}, {phone}",
			wantErr:          false,
		},
		{
			name: "special characters in customer data",
			campaign: &models.Campaign{
				ID:           1,
				BaseTemplate: "Hello {first_name}!",
			},
			customer: &models.Customer{
				ID:        1,
				FirstName: "O'Brien",
			},
			overrideTemplate: nil,
			wantMessage:      "Hello O'Brien!",
			wantTemplate:     "Hello {first_name}!",
			wantErr:          false,
		},
		{
			name: "unicode characters",
			campaign: &models.Campaign{
				ID:           1,
				BaseTemplate: "مرحبا {first_name}",
			},
			customer: &models.Customer{
				ID:        1,
				FirstName: "محمد",
			},
			overrideTemplate: nil,
			wantMessage:      "مرحبا محمد",
			wantTemplate:     "مرحبا {first_name}",
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockCampaignRepo := &mockCampaignRepository{
				campaigns: []*models.Campaign{tt.campaign},
			}

			mockCustomerRepo := &mockCustomerRepository{
				customers: map[int64]*models.Customer{
					tt.customer.ID: tt.customer,
				},
			}

			templateSvc := NewTemplateService()
			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

			svc := &campaignService{
				campaignRepo: mockCampaignRepo,
				customerRepo: mockCustomerRepo,
				templateSvc:  templateSvc,
				logger:       logger,
			}

			// Create request
			req := &PreviewRequest{
				CustomerID:       tt.customer.ID,
				OverrideTemplate: tt.overrideTemplate,
			}

			// Test
			result, err := svc.PreviewPersonalized(context.Background(), tt.campaign.ID, req)

			if (err != nil) != tt.wantErr {
				t.Errorf("PreviewPersonalized() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// Verify rendered message
			if result.RenderedMessage != tt.wantMessage {
				t.Errorf("RenderedMessage = %v, want %v", result.RenderedMessage, tt.wantMessage)
			}

			// Verify used template
			if result.UsedTemplate != tt.wantTemplate {
				t.Errorf("UsedTemplate = %v, want %v", result.UsedTemplate, tt.wantTemplate)
			}

			// Verify customer data (should only have id and first_name)
			if result.Customer.ID != tt.customer.ID {
				t.Errorf("Customer.ID = %v, want %v", result.Customer.ID, tt.customer.ID)
			}
			if result.Customer.FirstName != tt.customer.FirstName {
				t.Errorf("Customer.FirstName = %v, want %v", result.Customer.FirstName, tt.customer.FirstName)
			}
		})
	}
}

func TestCampaignService_PreviewPersonalized_Errors(t *testing.T) {
	tests := []struct {
		name         string
		campaignID   int64
		customerID   int64
		setupMocks   func() (*mockCampaignRepository, *mockCustomerRepository)
		wantErrType  string
	}{
		{
			name:       "campaign not found",
			campaignID: 999,
			customerID: 1,
			setupMocks: func() (*mockCampaignRepository, *mockCustomerRepository) {
				return &mockCampaignRepository{
						campaigns: []*models.Campaign{},
					}, &mockCustomerRepository{
						customers: map[int64]*models.Customer{
							1: {ID: 1, FirstName: "Alice"},
						},
					}
			},
			wantErrType: "not_found",
		},
		{
			name:       "customer not found",
			campaignID: 1,
			customerID: 999,
			setupMocks: func() (*mockCampaignRepository, *mockCustomerRepository) {
				return &mockCampaignRepository{
						campaigns: []*models.Campaign{
							{ID: 1, BaseTemplate: "test"},
						},
					}, &mockCustomerRepository{
						customers: map[int64]*models.Customer{},
					}
			},
			wantErrType: "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCampaignRepo, mockCustomerRepo := tt.setupMocks()

			svc := &campaignService{
				campaignRepo: mockCampaignRepo,
				customerRepo: mockCustomerRepo,
				templateSvc:  NewTemplateService(),
				logger:       slog.New(slog.NewJSONHandler(os.Stdout, nil)),
			}

			req := &PreviewRequest{
				CustomerID: tt.customerID,
			}

			_, err := svc.PreviewPersonalized(context.Background(), tt.campaignID, req)

			if err == nil {
				t.Errorf("PreviewPersonalized() error = nil, want error")
				return
			}

			// Check error type
			if tt.wantErrType == "not_found" {
				var appErr *models.AppError
				if !errors.As(err, &appErr) || appErr.Code != "NOT_FOUND" {
					t.Errorf("PreviewPersonalized() error type = %T, want AppError with NOT_FOUND code", err)
				}
			}
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// mockCustomerRepository for preview tests
type mockCustomerRepository struct {
	customers map[int64]*models.Customer
}

func (m *mockCustomerRepository) GetByID(ctx context.Context, id int64) (*models.Customer, error) {
	customer, ok := m.customers[id]
	if !ok {
		return nil, models.ErrNotFoundWithMsg("customer not found")
	}
	return customer, nil
}

func (m *mockCustomerRepository) Create(ctx context.Context, customer *models.Customer) error {
	return nil
}
func (m *mockCustomerRepository) List(ctx context.Context, filter models.CustomerFilter) ([]*models.Customer, int64, error) {
	return nil, 0, nil
}
func (m *mockCustomerRepository) Update(ctx context.Context, customer *models.Customer) error {
	return nil
}
func (m *mockCustomerRepository) Delete(ctx context.Context, id int64) error {
	return nil
}
func (m *mockCustomerRepository) GetByPhone(ctx context.Context, phone string) (*models.Customer, error) {
	return nil, models.ErrNotFoundWithMsg("not implemented")
}
