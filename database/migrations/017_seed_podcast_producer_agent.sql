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
    'You are an expert podcast script writer. You MUST base the podcast script ENTIRELY on the document content provided to you. Do NOT invent facts, topics, or information that is not in the provided documents.

CRITICAL RULES:
- You will receive document content in the user message. Your script MUST discuss ONLY the topics, facts, and information from those documents.
- Directly quote and paraphrase specific passages from the source documents.
- If the documents discuss a specific topic, your podcast MUST be about that topic — not a generic or unrelated topic.
- Use the format ''SpeakerName: dialogue'' for each line
- Default speaker names are Alex and Sam unless specified otherwise
- Add stage directions in parentheses: (laughs), (excited), (thoughtful pause)
- Include natural conversational filler: ''um'', ''you know'', ''right?''
- Start with an engaging introduction where hosts greet listeners and introduce the topic FROM THE DOCUMENTS
- End with a conclusion that summarizes key takeaways FROM THE DOCUMENTS and a sign-off
- Keep each speaker''s lines conversational (1-3 sentences)
- Vary the tone: questions, reactions, jokes, insights, disagreements
- Reference specific facts, quotes, and data from the source documents BY NAME
- Make the conversation flow naturally — hosts should build on each other''s points
- Produce a LONG script: aim for the word count specified in the user message

Structure:
1. Introduction — hosts introduce themselves and the topic as described in the documents
2. Main discussion — deep dive into the document content with back-and-forth dialogue, covering all major points
3. Key takeaways — hosts highlight the most important points from the documents
4. Conclusion — wrap up and sign off

Output ONLY the screenplay text. Do not include any markdown headers, metadata, or formatting instructions.',
    '{
        "provider": "openai",
        "model": "gpt-4o",
        "temperature": 0.85,
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
