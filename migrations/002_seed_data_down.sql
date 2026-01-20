-- CampaignManager System - Rollback Seed Data
-- Removes all seed data inserted by 002_seed_data_up.sql

-- Delete in reverse order due to foreign key constraints
DELETE FROM outbound_messages WHERE campaign_id IN (1, 2, 3);
DELETE FROM campaigns WHERE id IN (1, 2, 3);
DELETE FROM customers WHERE phone LIKE '+25471234500_';

-- Remove seed version entry
DELETE FROM schema_version WHERE version = 2;

-- Reset sequences to start from 1
ALTER SEQUENCE customers_id_seq RESTART WITH 1;
ALTER SEQUENCE campaigns_id_seq RESTART WITH 1;
ALTER SEQUENCE outbound_messages_id_seq RESTART WITH 1;

-- Log rollback
DO $$
BEGIN
    RAISE NOTICE 'Seed data rolled back successfully';
END $$;
