package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Raymond9734/campaign-messaging-backend/internal/models"
)

// CampaignRepository defines the interface for campaign data access
type CampaignRepository interface {
	Create(ctx context.Context, campaign *models.Campaign) error
	GetByID(ctx context.Context, id int64) (*models.Campaign, error)
	GetWithStats(ctx context.Context, id int64) (*models.CampaignWithStats, error)
	List(ctx context.Context, filter models.CampaignFilter) ([]*models.Campaign, int64, error)
	Update(ctx context.Context, campaign *models.Campaign) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	Delete(ctx context.Context, id int64) error
}

// campaignRepository implements CampaignRepository using PostgreSQL
type campaignRepository struct {
	db *sql.DB
}

// NewCampaignRepository creates a new campaign repository
func NewCampaignRepository(db *sql.DB) CampaignRepository {
	return &campaignRepository{db: db}
}

// Create inserts a new campaign
func (r *campaignRepository) Create(ctx context.Context, campaign *models.Campaign) error {
	query := `
		INSERT INTO campaigns (name, channel, status, base_template, scheduled_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(
		ctx,
		query,
		campaign.Name,
		campaign.Channel,
		campaign.Status,
		campaign.BaseTemplate,
		campaign.ScheduledAt,
	).Scan(&campaign.ID, &campaign.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create campaign: %w", err)
	}

	return nil
}

// GetByID retrieves a campaign by ID
func (r *campaignRepository) GetByID(ctx context.Context, id int64) (*models.Campaign, error) {
	query := `
		SELECT id, name, channel, status, base_template, scheduled_at, created_at
		FROM campaigns
		WHERE id = $1`

	campaign := &models.Campaign{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&campaign.ID,
		&campaign.Name,
		&campaign.Channel,
		&campaign.Status,
		&campaign.BaseTemplate,
		&campaign.ScheduledAt,
		&campaign.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrNotFoundWithMsg(fmt.Sprintf("campaign with ID %d not found", id))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}

	return campaign, nil
}

// GetWithStats retrieves a campaign with message statistics
func (r *campaignRepository) GetWithStats(ctx context.Context, id int64) (*models.CampaignWithStats, error) {
	// Get campaign
	campaign, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get stats
	statsQuery := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			0 as sending,
			COUNT(*) FILTER (WHERE status = 'sent') as sent,
			COUNT(*) FILTER (WHERE status = 'failed') as failed
		FROM outbound_messages
		WHERE campaign_id = $1`

	var stats models.CampaignStats
	err = r.db.QueryRowContext(ctx, statsQuery, id).Scan(
		&stats.Total,
		&stats.Pending,
		&stats.Sending,
		&stats.Sent,
		&stats.Failed,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get campaign stats: %w", err)
	}

	return &models.CampaignWithStats{
		ID:           campaign.ID,
		Name:         campaign.Name,
		Channel:      campaign.Channel,
		Status:       campaign.Status,
		BaseTemplate: campaign.BaseTemplate,
		ScheduledAt:  campaign.ScheduledAt,
		CreatedAt:    campaign.CreatedAt,
		Stats:        stats,
	}, nil
}

// List retrieves campaigns with pagination and filtering
func (r *campaignRepository) List(ctx context.Context, filter models.CampaignFilter) ([]*models.Campaign, int64, error) {
	// Validate and set defaults
	models.ValidateAndSetDefaults(&filter.Page, &filter.PageSize)

	// Build query with filters
	query := `
		SELECT id, name, channel, status, base_template, scheduled_at, created_at
		FROM campaigns
		WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM campaigns WHERE 1=1`
	args := []interface{}{}
	argPos := 1

	if filter.Channel != "" {
		query += fmt.Sprintf(" AND channel = $%d", argPos)
		countQuery += fmt.Sprintf(" AND channel = $%d", argPos)
		args = append(args, filter.Channel)
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
		return nil, 0, fmt.Errorf("failed to count campaigns: %w", err)
	}

	// Add pagination with stable ordering (id DESC for consistency)
	offset := models.CalculateOffset(filter.Page, filter.PageSize)
	query += fmt.Sprintf(" ORDER BY id DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, filter.PageSize, offset)

	// Execute query
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list campaigns: %w", err)
	}
	defer rows.Close()

	campaigns := []*models.Campaign{}
	for rows.Next() {
		campaign := &models.Campaign{}
		err := rows.Scan(
			&campaign.ID,
			&campaign.Name,
			&campaign.Channel,
			&campaign.Status,
			&campaign.BaseTemplate,
			&campaign.ScheduledAt,
			&campaign.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan campaign: %w", err)
		}
		campaigns = append(campaigns, campaign)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating campaigns: %w", err)
	}

	return campaigns, totalCount, nil
}

// Update updates an existing campaign
func (r *campaignRepository) Update(ctx context.Context, campaign *models.Campaign) error {
	query := `
		UPDATE campaigns
		SET name = $1, channel = $2, status = $3, base_template = $4, scheduled_at = $5
		WHERE id = $6
		`

	result, err := r.db.ExecContext(
		ctx,
		query,
		campaign.Name,
		campaign.Channel,
		campaign.Status,
		campaign.BaseTemplate,
		campaign.ScheduledAt,
		campaign.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update campaign: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrNotFoundWithMsg(fmt.Sprintf("campaign with ID %d not found", campaign.ID))
	}

	return nil
}

// UpdateStatus updates only the status of a campaign
func (r *campaignRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := `
		UPDATE campaigns
		SET status = $1
		WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update campaign status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrNotFoundWithMsg(fmt.Sprintf("campaign with ID %d not found", id))
	}

	return nil
}

// Delete removes a campaign
func (r *campaignRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM campaigns WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete campaign: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrNotFoundWithMsg(fmt.Sprintf("campaign with ID %d not found", id))
	}

	return nil
}
