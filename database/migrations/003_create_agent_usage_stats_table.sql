-- Migration: 003_create_agent_usage_stats_table.sql
-- Description: Create table for tracking aggregated agent usage statistics and metrics
-- Author: Agent Builder Team
-- Created: 2024-01-15

BEGIN;

-- Create agent_usage_stats table
CREATE TABLE IF NOT EXISTS agent_usage_stats (
    -- Primary key is agent_id since this is one row per agent
    agent_id UUID PRIMARY KEY,
    
    -- Execution counts
    total_executions INTEGER DEFAULT 0 CHECK (total_executions >= 0),
    successful_executions INTEGER DEFAULT 0 CHECK (successful_executions >= 0),
    failed_executions INTEGER DEFAULT 0 CHECK (failed_executions >= 0),
    
    -- Cost and token metrics
    total_cost_usd DECIMAL(10,6) DEFAULT 0.000000 CHECK (total_cost_usd >= 0),
    total_tokens_used BIGINT DEFAULT 0 CHECK (total_tokens_used >= 0),
    avg_cost_per_execution DECIMAL(10,6) DEFAULT 0.000000 CHECK (avg_cost_per_execution >= 0),
    
    -- Performance metrics
    avg_response_time_ms INTEGER DEFAULT 0 CHECK (avg_response_time_ms >= 0),
    min_response_time_ms INTEGER CHECK (min_response_time_ms >= 0),
    max_response_time_ms INTEGER CHECK (max_response_time_ms >= 0),
    p95_response_time_ms INTEGER CHECK (p95_response_time_ms >= 0),
    
    -- Time-based activity counters (reset periodically)
    executions_today INTEGER DEFAULT 0 CHECK (executions_today >= 0),
    executions_this_week INTEGER DEFAULT 0 CHECK (executions_this_week >= 0),
    executions_this_month INTEGER DEFAULT 0 CHECK (executions_this_month >= 0),
    
    cost_today DECIMAL(8,6) DEFAULT 0.000000 CHECK (cost_today >= 0),
    cost_this_week DECIMAL(8,6) DEFAULT 0.000000 CHECK (cost_this_week >= 0),
    cost_this_month DECIMAL(8,6) DEFAULT 0.000000 CHECK (cost_this_month >= 0),
    
    -- Success and error rates
    success_rate DECIMAL(5,4) DEFAULT 0.0000 CHECK (success_rate >= 0 AND success_rate <= 1.0),
    error_rate DECIMAL(5,4) DEFAULT 0.0000 CHECK (error_rate >= 0 AND error_rate <= 1.0),
    
    -- Knowledge and memory usage
    avg_knowledge_items_used DECIMAL(8,2) DEFAULT 0.00,
    avg_memory_items_used DECIMAL(8,2) DEFAULT 0.00,
    
    -- Provider usage distribution (JSONB for flexibility)
    provider_usage_stats JSONB DEFAULT '{}',
    model_usage_stats JSONB DEFAULT '{}',
    
    -- Timestamps for tracking data freshness
    last_execution_at TIMESTAMP WITH TIME ZONE,
    stats_last_updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    daily_stats_reset_at DATE DEFAULT CURRENT_DATE,
    weekly_stats_reset_at DATE DEFAULT CURRENT_DATE,
    monthly_stats_reset_at DATE DEFAULT CURRENT_DATE
);

-- Create indexes for query performance
CREATE INDEX idx_agent_usage_stats_total_executions ON agent_usage_stats(total_executions DESC);
CREATE INDEX idx_agent_usage_stats_total_cost ON agent_usage_stats(total_cost_usd DESC);
CREATE INDEX idx_agent_usage_stats_success_rate ON agent_usage_stats(success_rate DESC);
CREATE INDEX idx_agent_usage_stats_last_execution ON agent_usage_stats(last_execution_at DESC NULLS LAST);
CREATE INDEX idx_agent_usage_stats_updated_at ON agent_usage_stats(stats_last_updated_at DESC);

-- GIN indexes for JSONB columns
CREATE INDEX idx_agent_usage_provider_stats ON agent_usage_stats USING GIN (provider_usage_stats);
CREATE INDEX idx_agent_usage_model_stats ON agent_usage_stats USING GIN (model_usage_stats);

-- Add constraint comments
COMMENT ON TABLE agent_usage_stats IS 'Aggregated usage statistics and metrics for each agent';
COMMENT ON COLUMN agent_usage_stats.agent_id IS 'Reference to the agent (FK to agents.id)';
COMMENT ON COLUMN agent_usage_stats.total_executions IS 'Total number of executions across all time';
COMMENT ON COLUMN agent_usage_stats.successful_executions IS 'Number of successful executions';
COMMENT ON COLUMN agent_usage_stats.failed_executions IS 'Number of failed executions';
COMMENT ON COLUMN agent_usage_stats.total_cost_usd IS 'Total cost in USD across all executions';
COMMENT ON COLUMN agent_usage_stats.total_tokens_used IS 'Total tokens consumed across all executions';
COMMENT ON COLUMN agent_usage_stats.avg_cost_per_execution IS 'Average cost per execution';
COMMENT ON COLUMN agent_usage_stats.avg_response_time_ms IS 'Average response time in milliseconds';
COMMENT ON COLUMN agent_usage_stats.success_rate IS 'Success rate as decimal (0.0 to 1.0)';
COMMENT ON COLUMN agent_usage_stats.error_rate IS 'Error rate as decimal (0.0 to 1.0)';
COMMENT ON COLUMN agent_usage_stats.provider_usage_stats IS 'JSON object with provider usage distribution';
COMMENT ON COLUMN agent_usage_stats.model_usage_stats IS 'JSON object with model usage distribution';

-- Create function to update agent usage stats from executions
CREATE OR REPLACE FUNCTION update_agent_usage_stats(p_agent_id UUID)
RETURNS void AS $$
BEGIN
    INSERT INTO agent_usage_stats (agent_id) VALUES (p_agent_id)
    ON CONFLICT (agent_id) DO NOTHING;
    
    UPDATE agent_usage_stats SET
        total_executions = (
            SELECT COUNT(*) FROM agent_executions WHERE agent_id = p_agent_id
        ),
        successful_executions = (
            SELECT COUNT(*) FROM agent_executions 
            WHERE agent_id = p_agent_id AND status = 'completed'
        ),
        failed_executions = (
            SELECT COUNT(*) FROM agent_executions 
            WHERE agent_id = p_agent_id AND status IN ('failed', 'timeout')
        ),
        total_cost_usd = COALESCE((
            SELECT SUM(cost_usd) FROM agent_executions 
            WHERE agent_id = p_agent_id AND cost_usd IS NOT NULL
        ), 0),
        total_tokens_used = COALESCE((
            SELECT SUM(token_usage) FROM agent_executions 
            WHERE agent_id = p_agent_id AND token_usage IS NOT NULL
        ), 0),
        avg_response_time_ms = COALESCE((
            SELECT AVG(total_duration_ms)::INTEGER FROM agent_executions 
            WHERE agent_id = p_agent_id AND total_duration_ms IS NOT NULL
        ), 0),
        min_response_time_ms = (
            SELECT MIN(total_duration_ms) FROM agent_executions 
            WHERE agent_id = p_agent_id AND total_duration_ms IS NOT NULL
        ),
        max_response_time_ms = (
            SELECT MAX(total_duration_ms) FROM agent_executions 
            WHERE agent_id = p_agent_id AND total_duration_ms IS NOT NULL
        ),
        last_execution_at = (
            SELECT MAX(created_at) FROM agent_executions WHERE agent_id = p_agent_id
        ),
        stats_last_updated_at = NOW()
    WHERE agent_id = p_agent_id;
    
    -- Calculate derived metrics
    UPDATE agent_usage_stats SET
        success_rate = CASE 
            WHEN total_executions > 0 THEN successful_executions::DECIMAL / total_executions 
            ELSE 0.0 
        END,
        error_rate = CASE 
            WHEN total_executions > 0 THEN failed_executions::DECIMAL / total_executions 
            ELSE 0.0 
        END,
        avg_cost_per_execution = CASE 
            WHEN total_executions > 0 THEN total_cost_usd / total_executions 
            ELSE 0.0 
        END
    WHERE agent_id = p_agent_id;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to automatically update stats when executions change
CREATE OR REPLACE FUNCTION trigger_update_agent_usage_stats()
RETURNS TRIGGER AS $$
BEGIN
    -- Update stats for the affected agent
    PERFORM update_agent_usage_stats(COALESCE(NEW.agent_id, OLD.agent_id));
    
    -- Also update the agents table summary fields
    UPDATE agents SET
        total_executions = aus.total_executions,
        total_cost_usd = aus.total_cost_usd,
        avg_response_time_ms = aus.avg_response_time_ms,
        last_executed_at = aus.last_execution_at
    FROM agent_usage_stats aus
    WHERE agents.id = aus.agent_id 
    AND aus.agent_id = COALESCE(NEW.agent_id, OLD.agent_id);
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_agent_executions_stats_update
    AFTER INSERT OR UPDATE OR DELETE ON agent_executions
    FOR EACH ROW
    EXECUTE FUNCTION trigger_update_agent_usage_stats();

COMMIT;