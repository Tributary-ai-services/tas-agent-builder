-- Migration: 007_fix_execution_foreign_key.sql
-- Description: Fix foreign key constraint on ab_agent_executions to reference agent_builder.agents
-- Author: Claude Code
-- Created: 2026-01-28

BEGIN;

-- Drop the incorrect foreign key constraint that references public.ab_agents
ALTER TABLE public.ab_agent_executions
    DROP CONSTRAINT IF EXISTS fk_ab_agent_executions_agent;

-- Add the correct foreign key constraint that references agent_builder.agents
ALTER TABLE public.ab_agent_executions
    ADD CONSTRAINT fk_ab_agent_executions_agent
    FOREIGN KEY (agent_id)
    REFERENCES agent_builder.agents(id)
    ON DELETE CASCADE;

-- Also fix the ab_agent_usage_stats foreign key if it exists
ALTER TABLE public.ab_agent_usage_stats
    DROP CONSTRAINT IF EXISTS fk_ab_agent_usage_stats_agent;

ALTER TABLE public.ab_agent_usage_stats
    ADD CONSTRAINT fk_ab_agent_usage_stats_agent
    FOREIGN KEY (agent_id)
    REFERENCES agent_builder.agents(id)
    ON DELETE CASCADE;

-- Add comment to document the fix
COMMENT ON CONSTRAINT fk_ab_agent_executions_agent ON public.ab_agent_executions
    IS 'Foreign key to agent_builder.agents - fixed from incorrect reference to public.ab_agents';

COMMIT;
