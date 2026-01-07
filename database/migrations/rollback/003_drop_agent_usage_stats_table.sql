-- Rollback Migration: 003_drop_agent_usage_stats_table.sql
-- Description: Rollback the agent_usage_stats table creation
-- Author: Agent Builder Team
-- Created: 2024-01-15

BEGIN;

-- Drop trigger first (depends on agent_executions table)
DROP TRIGGER IF EXISTS trigger_agent_executions_stats_update ON agent_executions;

-- Drop functions
DROP FUNCTION IF EXISTS trigger_update_agent_usage_stats();
DROP FUNCTION IF EXISTS update_agent_usage_stats(UUID);

-- Drop indexes
DROP INDEX IF EXISTS idx_agent_usage_model_stats;
DROP INDEX IF EXISTS idx_agent_usage_provider_stats;
DROP INDEX IF EXISTS idx_agent_usage_stats_updated_at;
DROP INDEX IF EXISTS idx_agent_usage_stats_last_execution;
DROP INDEX IF EXISTS idx_agent_usage_stats_success_rate;
DROP INDEX IF EXISTS idx_agent_usage_stats_total_cost;
DROP INDEX IF EXISTS idx_agent_usage_stats_total_executions;

-- Drop table
DROP TABLE IF EXISTS agent_usage_stats;

COMMIT;