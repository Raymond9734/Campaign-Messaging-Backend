-- SMSLeopard Backend Challenge - Rollback Initial Schema
-- Drops all tables and functions created in 001_initial_schema_up.sql

-- Drop triggers first
DROP TRIGGER IF EXISTS update_customers_updated_at ON customers;
DROP TRIGGER IF EXISTS update_campaigns_updated_at ON campaigns;
DROP TRIGGER IF EXISTS update_outbound_messages_updated_at ON outbound_messages;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables (CASCADE will drop associated indexes and constraints)
DROP TABLE IF EXISTS outbound_messages CASCADE;
DROP TABLE IF EXISTS campaigns CASCADE;
DROP TABLE IF EXISTS customers CASCADE;
DROP TABLE IF EXISTS schema_version CASCADE;

-- Drop extension if no longer needed (optional, comment out if other schemas use it)
-- DROP EXTENSION IF EXISTS "uuid-ossp";
