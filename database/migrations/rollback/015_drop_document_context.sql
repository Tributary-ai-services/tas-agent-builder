-- Rollback Migration: 015_drop_document_context.sql
-- Description: Remove document context configuration and agent type columns
-- Author: TAS Agent Builder Team
-- Created: 2025-02-03

BEGIN;

-- Drop indexes first
DROP INDEX IF EXISTS agent_builder.idx_agents_document_context;
DROP INDEX IF EXISTS agent_builder.idx_agents_type_status;
DROP INDEX IF EXISTS agent_builder.idx_agents_type;

-- Drop columns
ALTER TABLE agent_builder.agents DROP COLUMN IF EXISTS document_context;
ALTER TABLE agent_builder.agents DROP COLUMN IF EXISTS type;

COMMIT;
