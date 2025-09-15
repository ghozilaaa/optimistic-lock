-- Initialize database for optimistic locking demo
-- This script runs automatically when the PostgreSQL container starts

-- Create the database if it doesn't exist (optional, since POSTGRES_DB handles this)
-- CREATE DATABASE IF NOT EXISTS optimistic_lock;

-- You can add initial data here if needed
-- For example, insert some sample balance records:
/*
INSERT INTO balances (amount, version) VALUES 
    (1000, 1),
    (2500, 1),
    (500, 1);
*/

-- Enable logging for better debugging (optional)
-- ALTER SYSTEM SET log_statement = 'all';
-- SELECT pg_reload_conf();

-- Print confirmation
SELECT 'Database initialized successfully' as message;