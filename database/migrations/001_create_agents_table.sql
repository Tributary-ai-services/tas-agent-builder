-- Migration: 001_create_agents_table.sql
-- Description: Create table for storing AI agent configurations and metadata
-- Author: Agent Builder Team
-- Created: 2024-01-15

BEGIN;

-- Create agents table in agent_builder schema
CREATE TABLE IF NOT EXISTS agent_builder.agents (
    -- Primary identification
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL CHECK (char_length(name) >= 1),
    description TEXT,
    
    -- Agent configuration
    system_prompt TEXT NOT NULL CHECK (char_length(system_prompt) >= 1),
    
    -- LLM configuration stored as JSONB for flexibility
    llm_config JSONB NOT NULL DEFAULT '{}',
    
    -- Ownership and multi-tenancy
    owner_id UUID NOT NULL,
    space_id UUID NOT NULL,
    tenant_id VARCHAR(255) NOT NULL,
    
    -- Agent status and visibility
    status VARCHAR(50) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'disabled')),
    space_type VARCHAR(50) NOT NULL CHECK (space_type IN ('personal', 'organization')),
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    is_template BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- Knowledge and capabilities
    notebook_ids JSONB DEFAULT '[]',
    enable_knowledge BOOLEAN NOT NULL DEFAULT TRUE,
    enable_memory BOOLEAN NOT NULL DEFAULT TRUE,
    
    -- Runtime tags for categorization and search
    tags JSONB DEFAULT '[]',
    
    -- Performance and usage metrics (updated from agent_usage_stats)
    total_executions INTEGER DEFAULT 0 CHECK (total_executions >= 0),
    total_cost_usd DECIMAL(10,6) DEFAULT 0.000000 CHECK (total_cost_usd >= 0),
    avg_response_time_ms INTEGER DEFAULT 0 CHECK (avg_response_time_ms >= 0),
    last_executed_at TIMESTAMP WITH TIME ZONE,
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for query performance
CREATE INDEX idx_agents_owner_id ON agent_builder.agents(owner_id);
CREATE INDEX idx_agents_space_id ON agent_builder.agents(space_id);
CREATE INDEX idx_agents_tenant_id ON agent_builder.agents(tenant_id);
CREATE INDEX idx_agents_status ON agent_builder.agents(status);
CREATE INDEX idx_agents_owner_status ON agent_builder.agents(owner_id, status);
CREATE INDEX idx_agents_space_status ON agent_builder.agents(space_id, status);
CREATE INDEX idx_agents_created_at ON agent_builder.agents(created_at DESC);
CREATE INDEX idx_agents_last_executed ON agent_builder.agents(last_executed_at DESC NULLS LAST);
CREATE INDEX idx_agents_llm_config ON agent_builder.agents USING GIN (llm_config);
CREATE INDEX idx_agents_notebook_ids ON agent_builder.agents USING GIN (notebook_ids);

-- Add constraint comments
COMMENT ON TABLE agent_builder.agents IS 'AI agents with their configuration and metadata';
COMMENT ON COLUMN agent_builder.agents.id IS 'Unique identifier for the agent';
COMMENT ON COLUMN agent_builder.agents.name IS 'Human-readable name for the agent';
COMMENT ON COLUMN agent_builder.agents.description IS 'Optional description of what the agent does';
COMMENT ON COLUMN agent_builder.agents.system_prompt IS 'System prompt that defines the agent behavior';
COMMENT ON COLUMN agent_builder.agents.llm_config IS 'LLM configuration including provider, model, and parameters';
COMMENT ON COLUMN agent_builder.agents.owner_id IS 'User who owns this agent';
COMMENT ON COLUMN agent_builder.agents.space_id IS 'Space where this agent belongs';
COMMENT ON COLUMN agent_builder.agents.tenant_id IS 'Tenant for multi-tenancy';
COMMENT ON COLUMN agent_builder.agents.status IS 'Current status: draft, published, disabled';
COMMENT ON COLUMN agent_builder.agents.space_type IS 'Type of space: personal, organization';
COMMENT ON COLUMN agent_builder.agents.is_public IS 'Whether the agent is publicly accessible';
COMMENT ON COLUMN agent_builder.agents.is_template IS 'Whether this agent serves as a template';
COMMENT ON COLUMN agent_builder.agents.notebook_ids IS 'Array of notebook UUIDs for knowledge retrieval';
COMMENT ON COLUMN agent_builder.agents.enable_knowledge IS 'Whether knowledge retrieval is enabled';
COMMENT ON COLUMN agent_builder.agents.enable_memory IS 'Whether conversation memory is enabled';
COMMENT ON COLUMN agent_builder.agents.total_executions IS 'Total number of times this agent has been executed';
COMMENT ON COLUMN agent_builder.agents.total_cost_usd IS 'Total cost in USD for all agent executions';
COMMENT ON COLUMN agent_builder.agents.avg_response_time_ms IS 'Average response time in milliseconds';
COMMENT ON COLUMN agent_builder.agents.last_executed_at IS 'Timestamp of last execution';

-- Create trigger to automatically update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_agents_updated_at 
    BEFORE UPDATE ON agent_builder.agents 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

COMMIT;