-- Migration: Add internal agents support
-- Description: Adds is_internal column to identify system agents available to all users

-- Add is_internal column to agents table
ALTER TABLE agent_builder.agents
ADD COLUMN IF NOT EXISTS is_internal BOOLEAN DEFAULT FALSE;

-- Create index for internal agents lookup (commonly queried)
CREATE INDEX IF NOT EXISTS idx_agents_is_internal
ON agent_builder.agents(is_internal) WHERE is_internal = TRUE;

-- Create composite index for internal + public agents (community tab queries)
CREATE INDEX IF NOT EXISTS idx_agents_community
ON agent_builder.agents(is_internal, is_public, status)
WHERE (is_internal = TRUE OR is_public = TRUE) AND deleted_at IS NULL;

-- Add comment for documentation
COMMENT ON COLUMN agent_builder.agents.is_internal IS 'Identifies system agents that are available to all users across all spaces. Internal agents cannot be modified or deleted by regular users.';

-- Create a view for internal agents (system tools)
CREATE OR REPLACE VIEW agent_builder.internal_agents_view AS
SELECT
    a.id,
    a.name,
    a.description,
    a.system_prompt,
    a.llm_config,
    a.status,
    a.tags,
    a.total_executions,
    a.avg_response_time_ms,
    a.created_at,
    a.updated_at
FROM agent_builder.agents a
WHERE a.is_internal = TRUE
  AND a.deleted_at IS NULL
ORDER BY a.name ASC;

-- Grant usage on the view
COMMENT ON VIEW agent_builder.internal_agents_view IS 'View for listing internal system agents (e.g., Prompt Assistant)';
