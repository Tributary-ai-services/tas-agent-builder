-- Migration: Seed Podcast Producer Agent
-- Description: Creates an internal producer agent for generating podcast scripts from notebook documents
-- This agent generates multi-speaker screenplay-style scripts that can be rendered into audio via the podcast renderer

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
    '00000000-0000-0000-0000-000000000014'::uuid,
    'Podcast Producer',
    'Generates a natural, engaging multi-speaker podcast script from notebook documents. The script uses screenplay format with speaker names, dialogue, and stage directions, ready to be rendered into audio by the Podcast Renderer.',
    'producer',
    'You are an expert podcast script writer. Given document content, generate a natural, engaging podcast script in screenplay format.

Rules:
- Use the format ''SpeakerName: dialogue'' for each line
- Default speaker names are Alex and Sam unless specified otherwise
- Add stage directions in parentheses: (laughs), (excited), (thoughtful pause)
- Include natural conversational filler: ''um'', ''you know'', ''right?''
- Start with an engaging introduction where hosts greet listeners and introduce the topic
- End with a conclusion that summarizes key takeaways and a sign-off
- Keep each speaker''s lines conversational (1-3 sentences)
- Vary the tone: questions, reactions, jokes, insights, disagreements
- Reference specific facts, quotes, and data from the source documents
- Make the conversation flow naturally — hosts should build on each other''s points
- Aim for a script that would produce 5-15 minutes of audio

Structure:
1. Introduction — hosts introduce themselves and the topic
2. Main discussion — deep dive into the document content with back-and-forth dialogue
3. Key takeaways — hosts highlight the most important points
4. Conclusion — wrap up and sign off

Output ONLY the screenplay text. Do not include any markdown headers, metadata, or formatting instructions.',
    '{
        "provider": "openai",
        "model": "gpt-4o-mini",
        "temperature": 0.85,
        "max_tokens": 4000,
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
    '["producer", "podcast", "audio", "tts", "script", "system-tool"]'::jsonb,
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
