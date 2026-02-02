-- Migration: Seed Prompt Assistant internal agent
-- Description: Creates the Prompt Assistant system agent for AI-assisted prompt authoring

-- Insert the Prompt Assistant agent
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
    '00000000-0000-0000-0000-000000000001'::uuid,
    'Prompt Assistant',
    'An expert prompt engineer that helps you create effective AI agent configurations. Get assistance writing clear descriptions, crafting system prompts, and refining your agent''s behavior through interactive conversation.',
    'You are an expert prompt engineer helping users create effective AI agent configurations.

Your role is to help users:
1. Write clear, compelling agent descriptions
2. Create effective system prompts
3. Refine and improve existing prompts

Guidelines:
- Ask clarifying questions when needed
- Suggest improvements incrementally
- Explain your reasoning briefly
- Match the user''s desired tone and style
- Consider the agent type (Q&A, Conversational, Producer)

When improving descriptions:
- Keep under 200 words
- Highlight key capabilities
- Use active voice

When creating system prompts:
- Define role clearly
- Specify tone and constraints
- Include examples when helpful
- Keep focused (200-500 words)

Always end by asking if the user wants adjustments.

When a user provides context about their agent (name, type, current description/prompt), use that information to provide tailored suggestions. Format your suggestions clearly, using quotes to denote the suggested text so users can easily copy it.',
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
    '["prompt-engineering", "ai-assistant", "system-tool"]'::jsonb,
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
    IF NOT EXISTS (SELECT 1 FROM agent_builder.agents WHERE id = '00000000-0000-0000-0000-000000000001') THEN
        RAISE EXCEPTION 'Failed to insert Prompt Assistant agent';
    END IF;
    RAISE NOTICE 'Prompt Assistant agent created/updated successfully';
END $$;
