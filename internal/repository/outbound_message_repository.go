package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Raymond9734/campaign-messaging-backend/internal/models"
)

// OutboundMessageRepository defines the interface for outbound message data access
type OutboundMessageRepository interface {
	Create(ctx context.Context, message *models.OutboundMessage) error
	CreateBatch(ctx context.Context, messages []*models.OutboundMessage) error
	GetByID(ctx context.Context, id int64) (*models.OutboundMessage, error)
	List(ctx context.Context, filter models.OutboundMessageFilter) ([]*models.OutboundMessage, int64, error)
	Update(ctx context.Context, message *models.OutboundMessage) error
	UpdateStatus(ctx context.Context, id int64, status string, lastError *string) error
	GetPendingMessages(ctx context.Context, limit int) ([]*models.OutboundMessage, error)
	IncrementRetryCount(ctx context.Context, id int64) error
}

// outboundMessageRepository implements OutboundMessageRepository using PostgreSQL
type outboundMessageRepository struct {
	db *sql.DB
}

// NewOutboundMessageRepository creates a new outbound message repository
func NewOutboundMessageRepository(db *sql.DB) OutboundMessageRepository {
	return &outboundMessageRepository{db: db}
}

// Create inserts a new outbound message
func (r *outboundMessageRepository) Create(ctx context.Context, message *models.OutboundMessage) error {
	query := `
		INSERT INTO outbound_messages (campaign_id, customer_id, status, rendered_content, last_error, retry_count)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(
		ctx,
		query,
		message.CampaignID,
		message.CustomerID,
		message.Status,
		message.RenderedContent,
		message.LastError,
		message.RetryCount,
	).Scan(&message.ID, &message.CreatedAt, &message.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create outbound message: %w", err)
	}

	return nil
}

// CreateBatch inserts multiple outbound messages in a single transaction
func (r *outboundMessageRepository) CreateBatch(ctx context.Context, messages []*models.OutboundMessage) error {
	if len(messages) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // Rollback is safe to call even after Commit
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO outbound_messages (campaign_id, customer_id, status, rendered_content, retry_count)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, message := range messages {
		err := stmt.QueryRowContext(
			ctx,
			message.CampaignID,
			message.CustomerID,
			message.Status,
			message.RenderedContent,
			message.RetryCount,
		).Scan(&message.ID, &message.CreatedAt, &message.UpdatedAt)

		if err != nil {
			return fmt.Errorf("failed to insert message: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetByID retrieves an outbound message by ID
func (r *outboundMessageRepository) GetByID(ctx context.Context, id int64) (*models.OutboundMessage, error) {
	query := `
		SELECT id, campaign_id, customer_id, status, rendered_content, last_error, retry_count, created_at, updated_at
		FROM outbound_messages
		WHERE id = $1`

	message := &models.OutboundMessage{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&message.ID,
		&message.CampaignID,
		&message.CustomerID,
		&message.Status,
		&message.RenderedContent,
		&message.LastError,
		&message.RetryCount,
		&message.CreatedAt,
		&message.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrNotFoundWithMsg(fmt.Sprintf("outbound message with ID %d not found", id))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get outbound message: %w", err)
	}

	return message, nil
}

// List retrieves outbound messages with pagination and filtering
func (r *outboundMessageRepository) List(ctx context.Context, filter models.OutboundMessageFilter) ([]*models.OutboundMessage, int64, error) {
	// Validate and set defaults
	models.ValidateAndSetDefaults(&filter.Page, &filter.PageSize)

	// Build query with filters
	query := `
		SELECT id, campaign_id, customer_id, status, rendered_content, last_error, retry_count, created_at, updated_at
		FROM outbound_messages
		WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM outbound_messages WHERE 1=1`
	args := []interface{}{}
	argPos := 1

	if filter.CampaignID > 0 {
		query += fmt.Sprintf(" AND campaign_id = $%d", argPos)
		countQuery += fmt.Sprintf(" AND campaign_id = $%d", argPos)
		args = append(args, filter.CampaignID)
		argPos++
	}

	if filter.CustomerID > 0 {
		query += fmt.Sprintf(" AND customer_id = $%d", argPos)
		countQuery += fmt.Sprintf(" AND customer_id = $%d", argPos)
		args = append(args, filter.CustomerID)
		argPos++
	}

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argPos)
		countQuery += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, filter.Status)
		argPos++
	}

	// Get total count
	var totalCount int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count outbound messages: %w", err)
	}

	// Add pagination
	offset := models.CalculateOffset(filter.Page, filter.PageSize)
	query += fmt.Sprintf(" ORDER BY id DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, filter.PageSize, offset)

	// Execute query
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list outbound messages: %w", err)
	}
	defer rows.Close()

	messages := []*models.OutboundMessage{}
	for rows.Next() {
		message := &models.OutboundMessage{}
		err := rows.Scan(
			&message.ID,
			&message.CampaignID,
			&message.CustomerID,
			&message.Status,
			&message.RenderedContent,
			&message.LastError,
			&message.RetryCount,
			&message.CreatedAt,
			&message.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan outbound message: %w", err)
		}
		messages = append(messages, message)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating outbound messages: %w", err)
	}

	return messages, totalCount, nil
}

// Update updates an existing outbound message
func (r *outboundMessageRepository) Update(ctx context.Context, message *models.OutboundMessage) error {
	query := `
		UPDATE outbound_messages
		SET status = $1, rendered_content = $2, last_error = $3, retry_count = $4
		WHERE id = $5
		RETURNING updated_at`

	err := r.db.QueryRowContext(
		ctx,
		query,
		message.Status,
		message.RenderedContent,
		message.LastError,
		message.RetryCount,
		message.ID,
	).Scan(&message.UpdatedAt)

	if err == sql.ErrNoRows {
		return models.ErrNotFoundWithMsg(fmt.Sprintf("outbound message with ID %d not found", message.ID))
	}
	if err != nil {
		return fmt.Errorf("failed to update outbound message: %w", err)
	}

	return nil
}

// UpdateStatus updates the status and error message of an outbound message
func (r *outboundMessageRepository) UpdateStatus(ctx context.Context, id int64, status string, lastError *string) error {
	query := `
		UPDATE outbound_messages
		SET status = $1, last_error = $2
		WHERE id = $3`

	result, err := r.db.ExecContext(ctx, query, status, lastError, id)
	if err != nil {
		return fmt.Errorf("failed to update outbound message status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrNotFoundWithMsg(fmt.Sprintf("outbound message with ID %d not found", id))
	}

	return nil
}

// GetPendingMessages retrieves pending messages for worker processing
func (r *outboundMessageRepository) GetPendingMessages(ctx context.Context, limit int) ([]*models.OutboundMessage, error) {
	query := `
		SELECT id, campaign_id, customer_id, status, rendered_content, last_error, retry_count, created_at, updated_at
		FROM outbound_messages
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending messages: %w", err)
	}
	defer rows.Close()

	messages := []*models.OutboundMessage{}
	for rows.Next() {
		message := &models.OutboundMessage{}
		err := rows.Scan(
			&message.ID,
			&message.CampaignID,
			&message.CustomerID,
			&message.Status,
			&message.RenderedContent,
			&message.LastError,
			&message.RetryCount,
			&message.CreatedAt,
			&message.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pending message: %w", err)
		}
		messages = append(messages, message)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pending messages: %w", err)
	}

	return messages, nil
}

// IncrementRetryCount increments the retry count for a message
func (r *outboundMessageRepository) IncrementRetryCount(ctx context.Context, id int64) error {
	query := `
		UPDATE outbound_messages
		SET retry_count = retry_count + 1
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to increment retry count: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrNotFoundWithMsg(fmt.Sprintf("outbound message with ID %d not found", id))
	}

	return nil
}
