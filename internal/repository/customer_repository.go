package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/models"
)

// CustomerRepository defines the interface for customer data access
type CustomerRepository interface {
	Create(ctx context.Context, customer *models.Customer) error
	GetByID(ctx context.Context, id int64) (*models.Customer, error)
	GetByPhone(ctx context.Context, phone string) (*models.Customer, error)
	List(ctx context.Context, filter models.CustomerFilter) ([]*models.Customer, int64, error)
	Update(ctx context.Context, customer *models.Customer) error
	Delete(ctx context.Context, id int64) error
}

// customerRepository implements CustomerRepository using PostgreSQL
type customerRepository struct {
	db *sql.DB
}

// NewCustomerRepository creates a new customer repository
func NewCustomerRepository(db *sql.DB) CustomerRepository {
	return &customerRepository{db: db}
}

// Create inserts a new customer
func (r *customerRepository) Create(ctx context.Context, customer *models.Customer) error {
	query := `
		INSERT INTO customers (phone, first_name, last_name, location, preferred_product)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	err := r.db.QueryRowContext(
		ctx,
		query,
		customer.Phone,
		customer.FirstName,
		customer.LastName,
		customer.Location,
		customer.PreferredProduct,
	).Scan(&customer.ID)

	if err != nil {
		return fmt.Errorf("failed to create customer: %w", err)
	}

	return nil
}

// GetByID retrieves a customer by ID
func (r *customerRepository) GetByID(ctx context.Context, id int64) (*models.Customer, error) {
	query := `
		SELECT id, phone, first_name, last_name, location, preferred_product
		FROM customers
		WHERE id = $1`

	customer := &models.Customer{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&customer.ID,
		&customer.Phone,
		&customer.FirstName,
		&customer.LastName,
		&customer.Location,
		&customer.PreferredProduct,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrNotFoundWithMsg(fmt.Sprintf("customer with ID %d not found", id))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	return customer, nil
}

// GetByPhone retrieves a customer by phone number
func (r *customerRepository) GetByPhone(ctx context.Context, phone string) (*models.Customer, error) {
	query := `
		SELECT id, phone, first_name, last_name, location, preferred_product
		FROM customers
		WHERE phone = $1`

	customer := &models.Customer{}
	err := r.db.QueryRowContext(ctx, query, phone).Scan(
		&customer.ID,
		&customer.Phone,
		&customer.FirstName,
		&customer.LastName,
		&customer.Location,
		&customer.PreferredProduct,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrNotFoundWithMsg(fmt.Sprintf("customer with phone %s not found", phone))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get customer by phone: %w", err)
	}

	return customer, nil
}

// List retrieves customers with pagination and filtering
func (r *customerRepository) List(ctx context.Context, filter models.CustomerFilter) ([]*models.Customer, int64, error) {
	// Validate and set defaults
	models.ValidateAndSetDefaults(&filter.Page, &filter.PageSize)

	// Build query with filters
	query := `
		SELECT id, phone, first_name, last_name, location, preferred_product
		FROM customers
		WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM customers WHERE 1=1`
	args := []interface{}{}
	argPos := 1

	if filter.Phone != "" {
		query += fmt.Sprintf(" AND phone LIKE $%d", argPos)
		countQuery += fmt.Sprintf(" AND phone LIKE $%d", argPos)
		args = append(args, "%"+filter.Phone+"%")
		argPos++
	}

	if filter.Location != "" {
		query += fmt.Sprintf(" AND location = $%d", argPos)
		countQuery += fmt.Sprintf(" AND location = $%d", argPos)
		args = append(args, filter.Location)
		argPos++
	}

	// Get total count
	var totalCount int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count customers: %w", err)
	}

	// Add pagination
	offset := models.CalculateOffset(filter.Page, filter.PageSize)
	query += fmt.Sprintf(" ORDER BY id DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, filter.PageSize, offset)

	// Execute query
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list customers: %w", err)
	}
	defer rows.Close()

	customers := []*models.Customer{}
	for rows.Next() {
		customer := &models.Customer{}
		err := rows.Scan(
			&customer.ID,
			&customer.Phone,
			&customer.FirstName,
			&customer.LastName,
			&customer.Location,
			&customer.PreferredProduct,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan customer: %w", err)
		}
		customers = append(customers, customer)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating customers: %w", err)
	}

	return customers, totalCount, nil
}

// Update updates an existing customer
func (r *customerRepository) Update(ctx context.Context, customer *models.Customer) error {
	query := `
		UPDATE customers
		SET phone = $1, first_name = $2, last_name = $3, location = $4, preferred_product = $5
		WHERE id = $6
		`

	result, err := r.db.ExecContext(
		ctx,
		query,
		customer.Phone,
		customer.FirstName,
		customer.LastName,
		customer.Location,
		customer.PreferredProduct,
		customer.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update customer: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrNotFoundWithMsg(fmt.Sprintf("customer with ID %d not found", customer.ID))
	}

	return nil
}

// Delete removes a customer
func (r *customerRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM customers WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete customer: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrNotFoundWithMsg(fmt.Sprintf("customer with ID %d not found", id))
	}

	return nil
}
