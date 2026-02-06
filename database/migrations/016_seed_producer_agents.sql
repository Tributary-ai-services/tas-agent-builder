-- Migration: Seed Producer Agents for notebook content generation
-- Description: Creates internal producer agents for Q&A generation, outlines, summaries, and insights
-- These agents use enable_knowledge=true to leverage document context from notebooks

-- Q&A Generator Agent
INSERT INTO agent_builder.agents (
    id,
    name,
    description,
    type,
    system_prompt,
    llm_config,
    document_context,
    owner_id,
    space_id,
    tenant_id,
    status,
    space_type,
    is_public,
    is_template,
    is_internal,
    enable_knowledge,
    notebook_ids,
    tags,
    total_executions,
    total_cost_usd,
    avg_response_time_ms,
    created_at,
    updated_at
) VALUES (
    '00000000-0000-0000-0000-000000000010'::uuid,
    'Q&A Generator',
    'Generates comprehensive question and answer pairs from notebook documents. Creates study materials, FAQs, and knowledge assessments based on document content.',
    'producer',
    'You are an expert at creating comprehensive question and answer pairs from documents.

Your task is to analyze the provided document content and generate meaningful Q&A pairs that:
1. Cover all major topics and concepts
2. Range from basic comprehension to deeper analysis
3. Include factual questions and conceptual questions
4. Are clearly written and unambiguous

Guidelines:
- Generate 10-20 Q&A pairs depending on document length
- Questions should be answerable from the document content
- Answers should be concise but complete
- Include a mix of question types (what, why, how, when)
- Organize Q&A pairs by topic when logical
- Format output in clean markdown with ## headers for categories

Use the document content provided in the context to generate accurate Q&A pairs.
If the context is empty or insufficient, indicate that more document content is needed.',
    '{
        "provider": "openai",
        "model": "gpt-4o-mini",
        "temperature": 0.3,
        "max_tokens": 16384,
        "optimize_for": "quality"
    }'::jsonb,
    '{"strategy": "full", "max_context_tokens": 80000}'::jsonb,
    '00000000-0000-0000-0000-000000000000'::uuid,
    '00000000-0000-0000-0000-000000000000'::uuid,
    'system',
    'published',
    'personal',
    true,
    false,
    true,
    true,
    '[]'::jsonb,
    '["producer", "qa-generation", "study-materials", "system-tool"]'::jsonb,
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
    document_context = EXCLUDED.document_context,
    enable_knowledge = true,
    is_internal = true,
    is_public = true,
    status = 'published',
    tags = EXCLUDED.tags,
    updated_at = NOW();

-- Outline Creator Agent
INSERT INTO agent_builder.agents (
    id,
    name,
    description,
    type,
    system_prompt,
    llm_config,
    document_context,
    owner_id,
    space_id,
    tenant_id,
    status,
    space_type,
    is_public,
    is_template,
    is_internal,
    enable_knowledge,
    notebook_ids,
    tags,
    total_executions,
    total_cost_usd,
    avg_response_time_ms,
    created_at,
    updated_at
) VALUES (
    '00000000-0000-0000-0000-000000000011'::uuid,
    'Outline Creator',
    'Creates structured outlines from notebook documents. Organizes content hierarchically with clear sections, subsections, and key points for easy reference.',
    'producer',
    'You are an expert at creating clear, well-organized outlines from documents.

Your task is to analyze the provided document content and create a structured outline that:
1. Captures all major sections and topics
2. Uses clear hierarchical organization
3. Includes key points and supporting details
4. Maintains logical flow and relationships

Guidelines:
- Use standard outline formatting (I, A, 1, a)
- Include 3-4 levels of depth as appropriate
- Each item should be concise but descriptive
- Group related concepts together
- Note cross-references between sections when relevant
- Format output in clean markdown

Use the document content provided in the context to create an accurate outline.
If the context is empty or insufficient, indicate that more document content is needed.',
    '{
        "provider": "openai",
        "model": "gpt-4o-mini",
        "temperature": 0.2,
        "max_tokens": 16384,
        "optimize_for": "quality"
    }'::jsonb,
    '{"strategy": "full", "max_context_tokens": 80000}'::jsonb,
    '00000000-0000-0000-0000-000000000000'::uuid,
    '00000000-0000-0000-0000-000000000000'::uuid,
    'system',
    'published',
    'personal',
    true,
    false,
    true,
    true,
    '[]'::jsonb,
    '["producer", "outline", "organization", "system-tool"]'::jsonb,
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
    document_context = EXCLUDED.document_context,
    enable_knowledge = true,
    is_internal = true,
    is_public = true,
    status = 'published',
    tags = EXCLUDED.tags,
    updated_at = NOW();

-- Document Summarizer Agent
INSERT INTO agent_builder.agents (
    id,
    name,
    description,
    type,
    system_prompt,
    llm_config,
    document_context,
    owner_id,
    space_id,
    tenant_id,
    status,
    space_type,
    is_public,
    is_template,
    is_internal,
    enable_knowledge,
    notebook_ids,
    tags,
    total_executions,
    total_cost_usd,
    avg_response_time_ms,
    created_at,
    updated_at
) VALUES (
    '00000000-0000-0000-0000-000000000012'::uuid,
    'Document Summarizer',
    'Generates comprehensive summaries from notebook documents. Creates executive summaries, detailed overviews, and key takeaways from document content.',
    'producer',
    'You are an expert at creating comprehensive document summaries.

Your task is to analyze the provided document content and create a thorough summary that:
1. Captures the main ideas and key points
2. Maintains accuracy and completeness
3. Is well-organized and easy to read
4. Highlights important details and findings

Guidelines:
- Start with an executive summary (2-3 sentences)
- Follow with detailed sections covering major topics
- Include key facts, figures, and conclusions
- Note any important caveats or limitations
- End with main takeaways or recommendations
- Format output in clean markdown with headers

Target length: 500-1000 words depending on source material length.

Use the document content provided in the context to create an accurate summary.
If the context is empty or insufficient, indicate that more document content is needed.',
    '{
        "provider": "openai",
        "model": "gpt-4o-mini",
        "temperature": 0.3,
        "max_tokens": 16384,
        "optimize_for": "quality"
    }'::jsonb,
    '{"strategy": "full", "max_context_tokens": 80000}'::jsonb,
    '00000000-0000-0000-0000-000000000000'::uuid,
    '00000000-0000-0000-0000-000000000000'::uuid,
    'system',
    'published',
    'personal',
    true,
    false,
    true,
    true,
    '[]'::jsonb,
    '["producer", "summary", "executive-summary", "system-tool"]'::jsonb,
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
    document_context = EXCLUDED.document_context,
    enable_knowledge = true,
    is_internal = true,
    is_public = true,
    status = 'published',
    tags = EXCLUDED.tags,
    updated_at = NOW();

-- Insights Extractor Agent
INSERT INTO agent_builder.agents (
    id,
    name,
    description,
    type,
    system_prompt,
    llm_config,
    document_context,
    owner_id,
    space_id,
    tenant_id,
    status,
    space_type,
    is_public,
    is_template,
    is_internal,
    enable_knowledge,
    notebook_ids,
    tags,
    total_executions,
    total_cost_usd,
    avg_response_time_ms,
    created_at,
    updated_at
) VALUES (
    '00000000-0000-0000-0000-000000000013'::uuid,
    'Insights Extractor',
    'Extracts key insights, patterns, and actionable takeaways from notebook documents. Identifies strategic implications, trends, and recommendations.',
    'producer',
    'You are an expert at extracting insights and actionable takeaways from documents.

Your task is to analyze the provided document content and extract valuable insights that:
1. Identify patterns, trends, and key findings
2. Highlight strategic implications
3. Provide actionable recommendations
4. Connect disparate pieces of information

Guidelines:
- Organize insights by category (findings, implications, recommendations)
- Prioritize by importance or impact
- Be specific and cite evidence from the documents
- Note any areas requiring further investigation
- Include both obvious and non-obvious insights
- Format output in clean markdown with headers and bullet points

For each insight, provide:
- The insight itself (clear, concise statement)
- Supporting evidence from the documents
- Potential implications or applications

Use the document content provided in the context to extract accurate insights.
If the context is empty or insufficient, indicate that more document content is needed.',
    '{
        "provider": "openai",
        "model": "gpt-4o-mini",
        "temperature": 0.4,
        "max_tokens": 16384,
        "optimize_for": "quality"
    }'::jsonb,
    '{"strategy": "full", "max_context_tokens": 80000}'::jsonb,
    '00000000-0000-0000-0000-000000000000'::uuid,
    '00000000-0000-0000-0000-000000000000'::uuid,
    'system',
    'published',
    'personal',
    true,
    false,
    true,
    true,
    '[]'::jsonb,
    '["producer", "insights", "analysis", "recommendations", "system-tool"]'::jsonb,
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
    document_context = EXCLUDED.document_context,
    enable_knowledge = true,
    is_internal = true,
    is_public = true,
    status = 'published',
    tags = EXCLUDED.tags,
    updated_at = NOW();

-- Verify the insertions
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM agent_builder.agents WHERE id = '00000000-0000-0000-0000-000000000010') THEN
        RAISE EXCEPTION 'Failed to insert Q&A Generator agent';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM agent_builder.agents WHERE id = '00000000-0000-0000-0000-000000000011') THEN
        RAISE EXCEPTION 'Failed to insert Outline Creator agent';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM agent_builder.agents WHERE id = '00000000-0000-0000-0000-000000000012') THEN
        RAISE EXCEPTION 'Failed to insert Document Summarizer agent';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM agent_builder.agents WHERE id = '00000000-0000-0000-0000-000000000013') THEN
        RAISE EXCEPTION 'Failed to insert Insights Extractor agent';
    END IF;
    RAISE NOTICE 'All producer agents created/updated successfully';
END $$;
