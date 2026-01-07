-- Rollback Migration: 001_drop_agents_table.sql
-- Description: Rollback the agents table creation
-- Author: Agent Builder Team
-- Created: 2024-01-15

BEGIN;

-- Drop trigger first
DROP TRIGGER IF EXISTS update_agents_updated_at ON agent_builder.agents;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_agents_notebook_ids;
DROP INDEX IF EXISTS idx_agents_llm_config;
DROP INDEX IF EXISTS idx_agents_last_executed;
DROP INDEX IF EXISTS idx_agents_created_at;
DROP INDEX IF EXISTS idx_agents_space_status;
DROP INDEX IF EXISTS idx_agents_owner_status;
DROP INDEX IF EXISTS idx_agents_status;
DROP INDEX IF EXISTS idx_agents_tenant_id;
DROP INDEX IF EXISTS idx_agents_space_id;
DROP INDEX IF EXISTS idx_agents_owner_id;

-- Drop table
DROP TABLE IF EXISTS agent_builder.agents;

COMMIT;