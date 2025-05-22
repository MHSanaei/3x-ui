-- =============================================================================
-- 3X-UI PostgreSQL Database Initialization Script
-- =============================================================================
-- This script is automatically executed when PostgreSQL container starts
-- for the first time

-- Create extensions if needed
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_stat_statements";

-- Set timezone
SET timezone = 'UTC';

-- Create additional indexes for better performance (will be created by GORM if needed)
-- These are just examples, actual tables will be created by the application

-- Grant additional permissions to the user
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO x_ui;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO x_ui;
GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public TO x_ui;

-- Set default privileges for future objects
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO x_ui;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO x_ui;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON FUNCTIONS TO x_ui;

-- Optimize PostgreSQL settings for 3X-UI workload
-- These settings are also configured in docker-compose.postgresql.yml
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
ALTER SYSTEM SET log_statement = 'mod';
ALTER SYSTEM SET log_min_duration_statement = 1000;
ALTER SYSTEM SET log_line_prefix = '%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h ';

-- Create a simple health check function
CREATE OR REPLACE FUNCTION health_check()
RETURNS TEXT AS $$
BEGIN
    RETURN 'OK - ' || current_timestamp;
END;
$$ LANGUAGE plpgsql;

-- Log initialization completion
DO $$
BEGIN
    RAISE NOTICE '3X-UI PostgreSQL database initialized successfully at %', current_timestamp;
END $$; 