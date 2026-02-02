-- Migration: 000_create_schema.sql
-- Description: Create agent_builder schema in shared database
-- Author: Agent Builder Team
-- Created: 2024-01-15

BEGIN;

-- Create schema for agent builder service
CREATE SCHEMA IF NOT EXISTS agent_builder;

-- Grant permissions to tasuser
GRANT USAGE ON SCHEMA agent_builder TO tasuser;
GRANT CREATE ON SCHEMA agent_builder TO tasuser;
GRANT ALL ON ALL TABLES IN SCHEMA agent_builder TO tasuser;
GRANT ALL ON ALL SEQUENCES IN SCHEMA agent_builder TO tasuser;

-- Set default permissions for future tables
ALTER DEFAULT PRIVILEGES IN SCHEMA agent_builder GRANT ALL ON TABLES TO tasuser;
ALTER DEFAULT PRIVILEGES IN SCHEMA agent_builder GRANT ALL ON SEQUENCES TO tasuser;

COMMIT;