-- Rollback Migration: 002_drop_agent_executions_table.sql
-- Description: Rollback the agent_executions table creation
-- Author: Agent Builder Team
-- Created: 2024-01-15

BEGIN;

-- Drop indexes
DROP INDEX IF EXISTS idx_agent_executions_execution_steps;
DROP INDEX IF EXISTS idx_agent_executions_output_data;
DROP INDEX IF EXISTS idx_agent_executions_input_data;
DROP INDEX IF EXISTS idx_agent_executions_running;
DROP INDEX IF EXISTS idx_agent_executions_user_created;
DROP INDEX IF EXISTS idx_agent_executions_agent_created;
DROP INDEX IF EXISTS idx_agent_executions_created_at;
DROP INDEX IF EXISTS idx_agent_executions_status;
DROP INDEX IF EXISTS idx_agent_executions_session_id;
DROP INDEX IF EXISTS idx_agent_executions_user_id;
DROP INDEX IF EXISTS idx_agent_executions_agent_id;

-- Drop table
DROP TABLE IF EXISTS agent_executions;

COMMIT;