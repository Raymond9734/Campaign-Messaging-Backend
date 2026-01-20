-- CampaignManager System - Seed Data
-- Inserts sample customers and campaigns for testing

-- ========================================
-- Seed Customers (15 records)
-- ========================================
INSERT INTO customers (phone, first_name, last_name, location, preferred_product) VALUES
    ('+254712345001', 'Alice', 'Mwangi', 'Nairobi', 'Running Shoes'),
    ('+254712345002', 'Bob', 'Ochieng', 'Mombasa', 'Winter Jacket'),
    ('+254712345003', 'Carol', 'Kamau', 'Kisumu', 'Laptop Stand'),
    ('+254712345004', 'David', 'Njoroge', 'Nakuru', 'Wireless Headphones'),
    ('+254712345005', 'Emma', 'Wanjiru', 'Eldoret', 'Yoga Mat'),
    ('+254712345006', 'Frank', 'Mutua', 'Nairobi', 'Coffee Maker'),
    ('+254712345007', 'Grace', 'Achieng', 'Mombasa', 'Smart Watch'),
    ('+254712345008', 'Henry', 'Kipchoge', 'Kisumu', 'Running Shoes'),
    ('+254712345009', 'Ivy', 'Chepkoech', 'Nakuru', 'Fitness Tracker'),
    ('+254712345010', 'Jack', 'Otieno', 'Eldoret', 'Bluetooth Speaker'),
    ('+254712345011', 'Karen', 'Wairimu', 'Nairobi', 'Office Chair'),
    ('+254712345012', 'Leo', 'Kimani', 'Mombasa', 'Desk Lamp'),
    ('+254712345013', 'Mary', 'Adhiambo', 'Kisumu', 'Water Bottle'),
    ('+254712345014', 'Nathan', 'Kariuki', 'Nakuru', 'Backpack'),
    ('+254712345015', 'Olivia', 'Chelangat', 'Eldoret', 'Phone Case');

-- ========================================
-- Seed Campaigns (3 records)
-- ========================================
INSERT INTO campaigns (name, channel, status, base_template, scheduled_at) VALUES
    (
        'Summer Sale 2025',
        'sms',
        'draft',
        'Hi {first_name}, check out our amazing deals on {preferred_product} in {location}! Limited time offer.',
        NULL
    ),
    (
        'Holiday Special - WhatsApp',
        'whatsapp',
        'scheduled',
        'Hello {first_name}! Special holiday discount on {preferred_product}. Shop now and save big!',
        '2025-12-15 10:00:00'
    ),
    (
        'New Product Launch',
        'sms',
        'sent',
        '{first_name}, exciting news! The {preferred_product} you love is back in stock. Order now!',
        NULL
    );

-- ========================================
-- Seed Outbound Messages (Sample for campaign 3)
-- ========================================
-- Create some messages for the "sent" campaign to demonstrate stats
INSERT INTO outbound_messages (campaign_id, customer_id, status, rendered_content, retry_count) VALUES
    (3, 1, 'sent', 'Alice, exciting news! The Running Shoes you love is back in stock. Order now!', 0),
    (3, 2, 'sent', 'Bob, exciting news! The Winter Jacket you love is back in stock. Order now!', 0),
    (3, 3, 'sent', 'Carol, exciting news! The Laptop Stand you love is back in stock. Order now!', 0),
    (3, 4, 'failed', 'David, exciting news! The Wireless Headphones you love is back in stock. Order now!', 3),
    (3, 5, 'sent', 'Emma, exciting news! The Yoga Mat you love is back in stock. Order now!', 0),
    (3, 6, 'sent', 'Frank, exciting news! The Coffee Maker you love is back in stock. Order now!', 0),
    (3, 7, 'pending', 'Grace, exciting news! The Smart Watch you love is back in stock. Order now!', 0),
    (3, 8, 'sent', 'Henry, exciting news! The Running Shoes you love is back in stock. Order now!', 0);

-- Update campaign status statistics
UPDATE campaigns SET updated_at = CURRENT_TIMESTAMP WHERE id = 3;

-- ========================================
-- Verify Seed Data
-- ========================================
-- Log seed data summary
DO $$
DECLARE
    customer_count INTEGER;
    campaign_count INTEGER;
    message_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO customer_count FROM customers;
    SELECT COUNT(*) INTO campaign_count FROM campaigns;
    SELECT COUNT(*) INTO message_count FROM outbound_messages;

    RAISE NOTICE 'Seed data completed:';
    RAISE NOTICE '  Customers: %', customer_count;
    RAISE NOTICE '  Campaigns: %', campaign_count;
    RAISE NOTICE '  Outbound Messages: %', message_count;
END $$;

-- Track seed version
INSERT INTO schema_version (version, description) VALUES (2, 'Seed data: 15 customers, 3 campaigns, 8 outbound messages');
