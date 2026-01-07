-- Migration: 002_create_agent_executions_table.sql
-- Description: Create table for tracking agent execution history and results
-- Author: Agent Builder Team  
-- Created: 2024-01-15

BEGIN;

-- Create agent_executions table
CREATE TABLE IF NOT EXISTS agent_executions (
    -- Primary identification
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL,
    user_id UUID NOT NULL,
    
    -- Execution context
    session_id UUID,
    execution_context JSONB DEFAULT '{}',
    
    -- Input and output data (stored as JSONB for flexibility)
    input_data JSONB NOT NULL,
    output_data JSONB,
    
    -- Execution status and metrics
    status VARCHAR(50) NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'failed', 'timeout', 'cancelled')),
    total_duration_ms INTEGER CHECK (total_duration_ms >= 0),
    token_usage INTEGER CHECK (token_usage >= 0),
    cost_usd DECIMAL(10,6) CHECK (cost_usd >= 0),
    
    -- LLM Router integration data
    router_provider VARCHAR(100),
    router_model VARCHAR(100), 
    routing_strategy VARCHAR(50),
    routing_reason TEXT,
    
    -- Knowledge and memory integration
    knowledge_items_used INTEGER DEFAULT 0 CHECK (knowledge_items_used >= 0),
    memory_items_used INTEGER DEFAULT 0 CHECK (memory_items_used >= 0),
    
    -- Error handling
    error_message TEXT,
    error_code VARCHAR(100),
    retry_count INTEGER DEFAULT 0 CHECK (retry_count >= 0),
    
    -- Execution chain tracking for debugging
    execution_steps JSONB DEFAULT '[]',
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for common query patterns
CREATE INDEX idx_agent_executions_agent_id ON agent_executions(agent_id);
CREATE INDEX idx_agent_executions_user_id ON agent_executions(user_id);
CREATE INDEX idx_agent_executions_session_id ON agent_executions(session_id);
CREATE INDEX idx_agent_executions_status ON agent_executions(status);
CREATE INDEX idx_agent_executions_created_at ON agent_executions(created_at DESC);
CREATE INDEX idx_agent_executions_agent_created ON agent_executions(agent_id, created_at DESC);
CREATE INDEX idx_agent_executions_user_created ON agent_executions(user_id, created_at DESC);
CREATE INDEX idx_agent_executions_running ON agent_executions(status, created_at) WHERE status = 'running';

-- Create GIN indexes for JSONB columns
CREATE INDEX idx_agent_executions_input_data ON agent_executions USING GIN (input_data);
CREATE INDEX idx_agent_executions_output_data ON agent_executions USING GIN (output_data);
CREATE INDEX idx_agent_executions_execution_steps ON agent_executions USING GIN (execution_steps);

-- Add constraint comments
COMMENT ON TABLE agent_executions IS 'Table storing execution history and results for agent runs';
COMMENT ON COLUMN agent_executions.id IS 'Unique identifier for this execution';
COMMENT ON COLUMN agent_executions.agent_id IS 'Reference to the agent that was executed';
COMMENT ON COLUMN agent_executions.user_id IS 'User who triggered this execution';
COMMENT ON COLUMN agent_executions.session_id IS 'Session identifier for grouping related executions';
COMMENT ON COLUMN agent_executions.execution_context IS 'Additional context data for the execution';
COMMENT ON COLUMN agent_executions.input_data IS 'JSON containing input message and parameters';
COMMENT ON COLUMN agent_executions.output_data IS 'JSON containing response content and metadata';
COMMENT ON COLUMN agent_executions.status IS 'Current execution status';
COMMENT ON COLUMN agent_executions.total_duration_ms IS 'Total execution time in milliseconds';
COMMENT ON COLUMN agent_executions.token_usage IS 'Number of tokens consumed by LLM';
COMMENT ON COLUMN agent_executions.cost_usd IS 'Cost in USD for this execution';
COMMENT ON COLUMN agent_executions.router_provider IS 'LLM provider used (e.g., openai, anthropic)';
COMMENT ON COLUMN agent_executions.router_model IS 'Specific model used (e.g., gpt-4o, claude-3-5-sonnet)';
COMMENT ON COLUMN agent_executions.routing_strategy IS 'Strategy used by router (cost, performance, etc.)';
COMMENT ON COLUMN agent_executions.routing_reason IS 'Explanation of why this route was chosen';
COMMENT ON COLUMN agent_executions.knowledge_items_used IS 'Number of knowledge base items retrieved';
COMMENT ON COLUMN agent_executions.memory_items_used IS 'Number of memory items retrieved';
COMMENT ON COLUMN agent_executions.error_message IS 'Error message if execution failed';
COMMENT ON COLUMN agent_executions.error_code IS 'Error code for categorizing failures';
COMMENT ON COLUMN agent_executions.retry_count IS 'Number of retry attempts made';
COMMENT ON COLUMN agent_executions.execution_steps IS 'JSON array of execution steps for debugging';

COMMIT;