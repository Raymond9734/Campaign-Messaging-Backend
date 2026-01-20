-- CampaignManager System - Initial Schema
-- Creates tables: customers, campaigns, outbound_messages

-- Enable extension for better timestamp handling
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ========================================
-- Table: customers
-- ========================================
CREATE TABLE IF NOT EXISTS customers (
    id BIGSERIAL PRIMARY KEY,
    phone VARCHAR(20) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    location VARCHAR(100),
    preferred_product VARCHAR(100)
);

-- Index for phone lookups (used in customer searches)
CREATE INDEX idx_customers_phone ON customers(phone);

COMMENT ON TABLE customers IS 'Stores customer information for campaign targeting';
COMMENT ON COLUMN customers.phone IS 'Customer phone number (E.164 format recommended)';
COMMENT ON COLUMN customers.preferred_product IS 'Product preference for personalization';

-- ========================================
-- Table: campaigns
-- ========================================
CREATE TABLE IF NOT EXISTS campaigns (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    channel VARCHAR(20) NOT NULL CHECK (channel IN ('sms', 'whatsapp')),
    status VARCHAR(20) NOT NULL CHECK (status IN ('draft', 'scheduled', 'sending', 'sent', 'failed')),
    base_template TEXT NOT NULL,
    scheduled_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for filtering by status (common query pattern)
CREATE INDEX idx_campaigns_status ON campaigns(status);

-- Index for filtering by channel
CREATE INDEX idx_campaigns_channel ON campaigns(channel);

-- Index for pagination with stable ordering (id DESC)
CREATE INDEX idx_campaigns_id_desc ON campaigns(id DESC);

-- Composite index for scheduled campaigns that need to be sent
CREATE INDEX idx_campaigns_scheduled ON campaigns(status, scheduled_at)
    WHERE status = 'scheduled' AND scheduled_at IS NOT NULL;

COMMENT ON TABLE campaigns IS 'Stores campaign definitions and metadata';
COMMENT ON COLUMN campaigns.base_template IS 'Message template with placeholders like {first_name}';
COMMENT ON COLUMN campaigns.status IS 'Campaign lifecycle: draft -> scheduled/sending -> sent/failed';

-- ========================================
-- Table: outbound_messages
-- ========================================
CREATE TABLE IF NOT EXISTS outbound_messages (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    customer_id BIGINT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'sent', 'failed')),
    rendered_content TEXT NOT NULL,
    last_error TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Composite index for campaign statistics queries
CREATE INDEX idx_outbound_messages_campaign_status ON outbound_messages(campaign_id, status);

-- Index for worker queue processing (fetch pending messages)
CREATE INDEX idx_outbound_messages_worker_queue ON outbound_messages(status, created_at)
    WHERE status = 'pending';

-- Index for retry logic (failed messages with retry count)
CREATE INDEX idx_outbound_messages_retry ON outbound_messages(status, retry_count)
    WHERE status = 'failed';

-- Composite index for customer message history
CREATE INDEX idx_outbound_messages_customer ON outbound_messages(customer_id, created_at DESC);

COMMENT ON TABLE outbound_messages IS 'Stores individual messages to be sent as part of campaigns';
COMMENT ON COLUMN outbound_messages.rendered_content IS 'Final personalized message after template rendering';
COMMENT ON COLUMN outbound_messages.retry_count IS 'Number of send attempts (max 3)';

-- ========================================
-- Trigger: Update updated_at timestamp (only for outbound_messages)
-- ========================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_outbound_messages_updated_at BEFORE UPDATE ON outbound_messages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ========================================
-- Schema Version Tracking (Simple)
-- ========================================
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);

INSERT INTO schema_version (version, description) VALUES (1, 'Initial schema with customers, campaigns, outbound_messages');
