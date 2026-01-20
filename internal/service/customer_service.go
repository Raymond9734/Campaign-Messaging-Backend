package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Raymond9734/campaign-messaging-backend/internal/models"
	"github.com/Raymond9734/campaign-messaging-backend/internal/repository"
)

// CustomerService handles customer business logic
type CustomerService interface {
	Create(ctx context.Context, customer *models.Customer) (*models.Customer, error)
	GetByID(ctx context.Context, id int64) (*models.Customer, error)
	GetByPhone(ctx context.Context, phone string) (*models.Customer, error)
	List(ctx context.Context, filter models.CustomerFilter) ([]*models.Customer, models.PaginationResult, error)
	Update(ctx context.Context, customer *models.Customer) (*models.Customer, error)
	Delete(ctx context.Context, id int64) error
}

type customerService struct {
	customerRepo repository.CustomerRepository
	logger       *slog.Logger
}

// NewCustomerService creates a new customer service
func NewCustomerService(
	customerRepo repository.CustomerRepository,
	logger *slog.Logger,
) CustomerService {
	return &customerService{
		customerRepo: customerRepo,
		logger:       logger,
	}
}

// Create creates a new customer
func (s *customerService) Create(ctx context.Context, customer *models.Customer) (*models.Customer, error) {
	// Validate customer
	if err := customer.Validate(); err != nil {
		return nil, err
	}

	// Check if customer with phone already exists
	existing, err := s.customerRepo.GetByPhone(ctx, customer.Phone)
	if err == nil && existing != nil {
		return nil, models.ErrConflictWithMsg(
			fmt.Sprintf("customer with phone %s already exists", customer.Phone),
		)
	}

	// Create customer
	if err := s.customerRepo.Create(ctx, customer); err != nil {
		s.logger.Error("failed to create customer",
			slog.String("phone", customer.Phone),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	s.logger.Info("customer created",
		slog.Int64("customer_id", customer.ID),
		slog.String("phone", customer.Phone),
	)

	return customer, nil
}

// GetByID retrieves a customer by ID
func (s *customerService) GetByID(ctx context.Context, id int64) (*models.Customer, error) {
	customer, err := s.customerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return customer, nil
}

// GetByPhone retrieves a customer by phone number
func (s *customerService) GetByPhone(ctx context.Context, phone string) (*models.Customer, error) {
	customer, err := s.customerRepo.GetByPhone(ctx, phone)
	if err != nil {
		return nil, err
	}

	return customer, nil
}

// List retrieves customers with pagination
func (s *customerService) List(ctx context.Context, filter models.CustomerFilter) ([]*models.Customer, models.PaginationResult, error) {
	customers, totalCount, err := s.customerRepo.List(ctx, filter)
	if err != nil {
		return nil, models.PaginationResult{}, fmt.Errorf("failed to list customers: %w", err)
	}

	// Validate and set defaults for pagination
	models.ValidateAndSetDefaults(&filter.Page, &filter.PageSize)

	pagination := models.NewPaginationResult(filter.Page, filter.PageSize, totalCount)

	return customers, pagination, nil
}

// Update updates an existing customer
func (s *customerService) Update(ctx context.Context, customer *models.Customer) (*models.Customer, error) {
	// Validate customer
	if err := customer.Validate(); err != nil {
		return nil, err
	}

	// Update customer
	if err := s.customerRepo.Update(ctx, customer); err != nil {
		s.logger.Error("failed to update customer",
			slog.Int64("customer_id", customer.ID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to update customer: %w", err)
	}

	s.logger.Info("customer updated",
		slog.Int64("customer_id", customer.ID),
	)

	return customer, nil
}

// Delete removes a customer
func (s *customerService) Delete(ctx context.Context, id int64) error {
	if err := s.customerRepo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete customer",
			slog.Int64("customer_id", id),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to delete customer: %w", err)
	}

	s.logger.Info("customer deleted",
		slog.Int64("customer_id", id),
	)

	return nil
}
