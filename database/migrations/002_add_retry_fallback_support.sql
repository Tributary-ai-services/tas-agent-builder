-- Migration: Add retry and fallback support to agent executions
-- Description: Adds columns to track retry attempts, fallback usage, and enhanced reliability metadata

-- Add retry and fallback metadata columns to agent_executions table
ALTER TABLE agent_builder.agent_executions
ADD COLUMN IF NOT EXISTS retry_attempts INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS fallback_used BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS failed_providers JSONB DEFAULT '[]',
ADD COLUMN IF NOT EXISTS total_retry_time_ms INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS provider_latency_ms INTEGER,
ADD COLUMN IF NOT EXISTS routing_reason JSONB DEFAULT '[]',
ADD COLUMN IF NOT EXISTS actual_cost_usd DECIMAL(10,6),
ADD COLUMN IF NOT EXISTS estimated_cost_usd DECIMAL(10,6);

-- Create indexes for new reliability tracking columns
CREATE INDEX IF NOT EXISTS idx_agent_executions_retry_attempts 
ON agent_builder.agent_executions(retry_attempts);

CREATE INDEX IF NOT EXISTS idx_agent_executions_fallback_used 
ON agent_builder.agent_executions(fallback_used);

CREATE INDEX IF NOT EXISTS idx_agent_executions_provider_latency 
ON agent_builder.agent_executions(provider_latency_ms);

-- Create a composite index for reliability analytics
CREATE INDEX IF NOT EXISTS idx_agent_executions_reliability 
ON agent_builder.agent_executions(agent_id, status, retry_attempts, fallback_used);

-- Add reliability statistics to agent_usage_stats table
ALTER TABLE agent_builder.agent_usage_stats
ADD COLUMN IF NOT EXISTS avg_retry_attempts DECIMAL(5,2) DEFAULT 0.00,
ADD COLUMN IF NOT EXISTS fallback_usage_rate DECIMAL(5,4) DEFAULT 0.0000,
ADD COLUMN IF NOT EXISTS reliability_score DECIMAL(5,4) DEFAULT 1.0000,
ADD COLUMN IF NOT EXISTS provider_failure_rate DECIMAL(5,4) DEFAULT 0.0000,
ADD COLUMN IF NOT EXISTS avg_provider_latency_ms INTEGER DEFAULT 0;

-- Create a view for agent reliability metrics
CREATE OR REPLACE VIEW agent_builder.agent_reliability_view AS
SELECT 
    a.id as agent_id,
    a.name as agent_name,
    a.llm_config->>'provider' as primary_provider,
    a.llm_config->>'model' as primary_model,
    
    -- Execution statistics
    COUNT(ae.id) as total_executions,
    COUNT(CASE WHEN ae.status = 'completed' THEN 1 END) as successful_executions,
    COUNT(CASE WHEN ae.status = 'failed' THEN 1 END) as failed_executions,
    
    -- Reliability metrics
    ROUND(
        COUNT(CASE WHEN ae.status = 'completed' THEN 1 END)::DECIMAL / 
        NULLIF(COUNT(ae.id), 0) * 100, 2
    ) as success_rate_percent,
    
    -- Retry metrics
    AVG(ae.retry_attempts) as avg_retry_attempts,
    COUNT(CASE WHEN ae.retry_attempts > 0 THEN 1 END) as executions_with_retries,
    ROUND(
        COUNT(CASE WHEN ae.retry_attempts > 0 THEN 1 END)::DECIMAL / 
        NULLIF(COUNT(ae.id), 0) * 100, 2
    ) as retry_rate_percent,
    
    -- Fallback metrics
    COUNT(CASE WHEN ae.fallback_used = true THEN 1 END) as executions_with_fallback,
    ROUND(
        COUNT(CASE WHEN ae.fallback_used = true THEN 1 END)::DECIMAL / 
        NULLIF(COUNT(ae.id), 0) * 100, 2
    ) as fallback_rate_percent,
    
    -- Performance metrics
    AVG(ae.total_duration_ms) as avg_response_time_ms,
    AVG(ae.provider_latency_ms) as avg_provider_latency_ms,
    AVG(ae.total_retry_time_ms) as avg_retry_time_ms,
    
    -- Cost metrics
    AVG(ae.cost_usd) as avg_cost_per_execution,
    SUM(ae.cost_usd) as total_cost_usd,
    
    -- Reliability score (composite metric)
    ROUND(
        (COUNT(CASE WHEN ae.status = 'completed' THEN 1 END)::DECIMAL / NULLIF(COUNT(ae.id), 0)) *
        (1 - (COUNT(CASE WHEN ae.retry_attempts > 0 THEN 1 END)::DECIMAL / NULLIF(COUNT(ae.id), 0)) * 0.1) *
        (1 - (COUNT(CASE WHEN ae.fallback_used = true THEN 1 END)::DECIMAL / NULLIF(COUNT(ae.id), 0)) * 0.05),
        4
    ) as reliability_score,
    
    -- Time range
    MIN(ae.created_at) as first_execution,
    MAX(ae.created_at) as last_execution

FROM agent_builder.agents a
LEFT JOIN agent_builder.agent_executions ae ON a.id = ae.agent_id
WHERE a.deleted_at IS NULL
GROUP BY a.id, a.name, a.llm_config->>'provider', a.llm_config->>'model'
HAVING COUNT(ae.id) > 0;

-- Create a function to update reliability statistics
CREATE OR REPLACE FUNCTION agent_builder.update_agent_reliability_stats(agent_uuid UUID)
RETURNS VOID AS $$
DECLARE
    stats_record RECORD;
BEGIN
    -- Get reliability statistics for the agent
    SELECT 
        total_executions,
        successful_executions,
        failed_executions,
        success_rate_percent,
        avg_retry_attempts,
        retry_rate_percent,
        fallback_rate_percent,
        avg_response_time_ms,
        avg_provider_latency_ms,
        avg_cost_per_execution,
        total_cost_usd,
        reliability_score
    INTO stats_record
    FROM agent_builder.agent_reliability_view
    WHERE agent_id = agent_uuid;
    
    -- Update the agent_usage_stats table
    INSERT INTO agent_builder.agent_usage_stats (
        agent_id,
        total_executions,
        successful_executions,
        failed_executions,
        success_rate,
        avg_retry_attempts,
        fallback_usage_rate,
        avg_response_time_ms,
        avg_provider_latency_ms,
        avg_cost_per_execution,
        total_cost_usd,
        reliability_score,
        stats_last_updated_at
    )
    VALUES (
        agent_uuid,
        COALESCE(stats_record.total_executions, 0),
        COALESCE(stats_record.successful_executions, 0),
        COALESCE(stats_record.failed_executions, 0),
        COALESCE(stats_record.success_rate_percent / 100.0, 0.0),
        COALESCE(stats_record.avg_retry_attempts, 0.0),
        COALESCE(stats_record.fallback_rate_percent / 100.0, 0.0),
        COALESCE(stats_record.avg_response_time_ms::INTEGER, 0),
        COALESCE(stats_record.avg_provider_latency_ms::INTEGER, 0),
        COALESCE(stats_record.avg_cost_per_execution, 0.0),
        COALESCE(stats_record.total_cost_usd, 0.0),
        COALESCE(stats_record.reliability_score, 1.0),
        NOW()
    )
    ON CONFLICT (agent_id) DO UPDATE SET
        total_executions = EXCLUDED.total_executions,
        successful_executions = EXCLUDED.successful_executions,
        failed_executions = EXCLUDED.failed_executions,
        success_rate = EXCLUDED.success_rate,
        avg_retry_attempts = EXCLUDED.avg_retry_attempts,
        fallback_usage_rate = EXCLUDED.fallback_usage_rate,
        avg_response_time_ms = EXCLUDED.avg_response_time_ms,
        avg_provider_latency_ms = EXCLUDED.avg_provider_latency_ms,
        avg_cost_per_execution = EXCLUDED.avg_cost_per_execution,
        total_cost_usd = EXCLUDED.total_cost_usd,
        reliability_score = EXCLUDED.reliability_score,
        stats_last_updated_at = EXCLUDED.stats_last_updated_at;
END;
$$ LANGUAGE plpgsql;

-- Create a trigger to automatically update reliability stats after each execution
CREATE OR REPLACE FUNCTION agent_builder.trigger_update_reliability_stats()
RETURNS TRIGGER AS $$
BEGIN
    -- Update stats for the affected agent (async to avoid blocking)
    PERFORM pg_notify('update_agent_stats', NEW.agent_id::text);
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create the trigger
DROP TRIGGER IF EXISTS trigger_agent_execution_stats ON agent_builder.agent_executions;
CREATE TRIGGER trigger_agent_execution_stats
    AFTER INSERT OR UPDATE ON agent_builder.agent_executions
    FOR EACH ROW
    EXECUTE FUNCTION agent_builder.trigger_update_reliability_stats();

-- Add comments to document the new columns
COMMENT ON COLUMN agent_builder.agent_executions.retry_attempts IS 'Number of retry attempts made for this execution';
COMMENT ON COLUMN agent_builder.agent_executions.fallback_used IS 'Whether fallback to another provider was used';
COMMENT ON COLUMN agent_builder.agent_executions.failed_providers IS 'JSON array of providers that failed before success/final failure';
COMMENT ON COLUMN agent_builder.agent_executions.total_retry_time_ms IS 'Total time spent on retries in milliseconds';
COMMENT ON COLUMN agent_builder.agent_executions.provider_latency_ms IS 'Provider-specific latency in milliseconds';
COMMENT ON COLUMN agent_builder.agent_executions.routing_reason IS 'JSON array of routing decision reasons';
COMMENT ON COLUMN agent_builder.agent_executions.actual_cost_usd IS 'Actual cost reported by router';
COMMENT ON COLUMN agent_builder.agent_executions.estimated_cost_usd IS 'Estimated cost before execution';

COMMENT ON VIEW agent_builder.agent_reliability_view IS 'Comprehensive view of agent reliability metrics including retry rates, fallback usage, and performance statistics';
COMMENT ON FUNCTION agent_builder.update_agent_reliability_stats(UUID) IS 'Updates reliability statistics for a specific agent based on execution history';