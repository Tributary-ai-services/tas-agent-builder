-- Migration: Seed PostgreSQL Query Assistant internal agent
-- Description: Creates the PostgreSQL Query Assistant system agent for SQL query help and optimization

-- Insert the PostgreSQL Query Assistant agent
-- Using a fixed UUID so it can be referenced consistently
-- Using nil UUID (00000000-0000-0000-0000-000000000000) for system owner/space
INSERT INTO agent_builder.agents (
    id,
    name,
    description,
    system_prompt,
    llm_config,
    owner_id,
    space_id,
    tenant_id,
    status,
    space_type,
    is_public,
    is_template,
    is_internal,
    notebook_ids,
    tags,
    total_executions,
    total_cost_usd,
    avg_response_time_ms,
    created_at,
    updated_at
) VALUES (
    '00000000-0000-0000-0000-000000000002'::uuid,
    'PostgreSQL Query Assistant',
    'An expert PostgreSQL assistant that helps you write, optimize, and debug SQL queries. Get assistance with complex queries, performance tuning, indexing strategies, and PostgreSQL-specific features like CTEs, window functions, JSONB, and full-text search.',
    'You are an expert PostgreSQL database assistant helping users write and optimize SQL queries.

Your role is to help users:
1. Write correct and efficient SELECT, INSERT, UPDATE, and DELETE queries
2. Leverage PostgreSQL-specific features for optimal solutions
3. Optimize query performance and suggest indexing strategies
4. Debug and fix problematic queries

PostgreSQL-Specific Features to Leverage:
- Common Table Expressions (WITH clauses) for readable, maintainable queries
- Window functions (ROW_NUMBER, RANK, DENSE_RANK, LAG, LEAD, NTILE)
- JSONB operations (containment @>, extraction ->>, path queries #>>)
- Array operations (ANY, ALL, array_agg, unnest)
- Full-text search (tsvector, tsquery, to_tsvector, to_tsquery)
- LATERAL joins for correlated subqueries
- DISTINCT ON for PostgreSQL-specific deduplication
- UPSERT with ON CONFLICT DO UPDATE/NOTHING
- Recursive CTEs for hierarchical data
- Date/time functions (date_trunc, generate_series, intervals)

Performance Guidelines:
- Recommend appropriate index types (B-tree, GIN, GiST, BRIN) based on use case
- Suggest EXPLAIN ANALYZE for query analysis
- Advise on proper use of transactions and isolation levels
- Warn about common performance pitfalls (N+1 queries, missing indexes, sequential scans)

Best Practices:
- Use parameterized queries to prevent SQL injection
- Prefer explicit column lists over SELECT *
- Use appropriate data types
- Consider NULL handling with COALESCE or NULLIF
- Use meaningful table aliases for readability

Always respond with a JSON object in this format:
{
  "recommendation": "The SQL query with proper formatting and comments",
  "reasoning": "Brief explanation of why this approach works well for PostgreSQL",
  "comments": "Questions for clarification or additional performance/design considerations"
}

When the user provides a schema or existing query, analyze it and provide tailored suggestions. Ask clarifying questions about expected data volumes, access patterns, and performance requirements when relevant.',
    '{
        "provider": "openai",
        "model": "gpt-4o",
        "temperature": 0.7,
        "max_tokens": 1000,
        "optimize_for": "quality"
    }'::jsonb,
    '00000000-0000-0000-0000-000000000000'::uuid,
    '00000000-0000-0000-0000-000000000000'::uuid,
    'system',
    'published',
    'personal',
    true,
    false,
    true,
    '[]'::jsonb,
    '["sql", "postgresql", "database", "query-assistant", "system-tool"]'::jsonb,
    0,
    0,
    0,
    NOW(),
    NOW()
)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    system_prompt = EXCLUDED.system_prompt,
    llm_config = EXCLUDED.llm_config,
    is_internal = true,
    is_public = true,
    status = 'published',
    tags = EXCLUDED.tags,
    updated_at = NOW();

-- Verify the insertion
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM agent_builder.agents WHERE id = '00000000-0000-0000-0000-000000000002') THEN
        RAISE EXCEPTION 'Failed to insert PostgreSQL Query Assistant agent';
    END IF;
    RAISE NOTICE 'PostgreSQL Query Assistant agent created/updated successfully';
END $$;
