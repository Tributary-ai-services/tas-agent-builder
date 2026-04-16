-- Migration: Seed Notebook Chat Assistant internal agent
-- Description: Creates the Notebook Chat Assistant system agent (ID 09)
-- This agent is used by aether-be for notebook chat functionality

INSERT INTO agent_builder.agents (
    id,
    name,
    description,
    type,
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
    enable_knowledge,
    document_context,
    notebook_ids,
    tags,
    total_executions,
    total_cost_usd,
    avg_response_time_ms,
    created_at,
    updated_at
) VALUES (
    '00000000-0000-0000-0000-000000000009'::uuid,
    'Notebook Chat Assistant',
    'An intelligent chat assistant that answers questions about documents in your notebook. Uses vector search and document context to provide accurate, grounded responses based on your uploaded content.',
    'conversational',
    'You are a knowledgeable assistant that helps users understand and analyze documents in their notebook.

Your role is to:
1. Answer questions about the content of documents in the current notebook
2. Summarize key information from documents when asked
3. Compare and contrast information across multiple documents
4. Help users find specific details within their document collection
5. Provide context-aware responses grounded in the actual document content

Guidelines:
- Always base your answers on the provided document context
- If the document context does not contain enough information to answer a question, say so clearly
- Quote or reference specific parts of documents when relevant
- Be concise but thorough in your responses
- When multiple documents are relevant, synthesize information across them
- Maintain conversation context for follow-up questions
- If asked about something outside the document scope, politely redirect to the available content

When document context is provided, use it as your primary knowledge source. Always prefer information from the documents over general knowledge.',
    '{
        "provider": "anthropic",
        "model": "claude-sonnet-4-6",
        "temperature": 0.3,
        "max_tokens": 4000,
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
    true,
    '{
        "strategy": "hybrid",
        "max_context_tokens": 8000,
        "include_sub_notebooks": false,
        "top_k": 10,
        "min_score": 0.5,
        "hybrid_config": {
            "vector_weight": 0.6,
            "full_doc_weight": 0.3,
            "position_weight": 0.1,
            "summary_boost": 1.5,
            "vector_top_k": 20,
            "vector_min_score": 0.5,
            "full_doc_max_chunks": 50,
            "token_budget": 8000,
            "include_summaries": true,
            "deduplicate_by_content": true
        }
    }'::jsonb,
    '[]'::jsonb,
    '["notebook-chat", "document-qa", "rag", "system-tool"]'::jsonb,
    0,
    0,
    0,
    NOW(),
    NOW()
)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    type = EXCLUDED.type,
    system_prompt = EXCLUDED.system_prompt,
    llm_config = EXCLUDED.llm_config,
    is_internal = true,
    is_public = true,
    enable_knowledge = true,
    document_context = EXCLUDED.document_context,
    skills = EXCLUDED.skills,
    status = 'published',
    tags = EXCLUDED.tags,
    updated_at = NOW();

-- Verify the insertion
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM agent_builder.agents WHERE id = '00000000-0000-0000-0000-000000000009') THEN
        RAISE EXCEPTION 'Failed to insert Notebook Chat Assistant agent';
    END IF;
    RAISE NOTICE 'Notebook Chat Assistant agent created/updated successfully';
END $$;
