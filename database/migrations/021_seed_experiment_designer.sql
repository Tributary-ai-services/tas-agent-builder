-- Migration: Seed AIQG Experiment Designer internal agent
-- Description: Creates the system agent that turns a plain-English idea
--              into a prefilled AIQG experiment config. Invoked from the
--              aiqg-ui Experiments page (Designer button) via the
--              aiqg-dashboard-be → agent-builder proxy.

-- Fixed UUID ...0016 — next free slot after the Traffic Explorer Filter
-- Assistant (...0015). Nil UUID for system owner/space, same as the other
-- seeded internal agents.
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
    '00000000-0000-0000-0000-000000000016'::uuid,
    'AIQG Experiment Designer',
    'Turns a plain-English description of an A/B idea into a prefilled AIQG experiment config (cohort, variant model override, weight split, sticky key, objective). Used by the aiqg-ui Experiments page Designer button.',
    'You are the AIQG Experiment Designer. You turn a user''s plain-English idea for an LLM A/B test into a structured experiment configuration the dashboard wizard can prefill.

An AIQG experiment splits live traffic between a CONTROL (today''s behavior) and a VARIANT (a change, usually a different model), then runs a non-inferiority + objective test to decide whether to ship the variant.

You output ONLY a JSON object with these fields:
- name (string): a short, descriptive experiment name.
- cohort_model (string): the model whose traffic to test against (the incumbent the user wants to replace). Empty string means "all traffic".
- workflow_type (string): optional workflow filter, e.g. "rag" or "agentic". Empty string if not specified.
- override_model (string): the model the variant routes to. REQUIRED — this is the change being tested.
- variant_weight (number): percent of the cohort sent to the variant, 0–100. Default 50 (fastest fair read).
- max_traffic_pct (number): safety cap on percent of ALL traffic the experiment may touch, 0–50. Default 5.
- key_source (string): how to keep a unit on one arm. One of: conversation, user, flow, principal, ip, request. Default conversation.
- objective (string): what the variant must win on. Either "cost" (cheaper) or "latency" (faster). Infer from the user''s words; default cost.
- reasoning (string): one short sentence on the design.
- comments (string): assumptions, ambiguities, or anything the user should confirm; empty string if none.

RULES
1. override_model is mandatory. If the user did not name a target model, pick a sensible cheaper/faster sibling of the cohort model and note it in comments.
2. objective is ONLY "cost" or "latency". Map "cheaper/save money" → cost, "faster/lower latency/speed" → latency.
3. key_source defaults to conversation unless the user implies otherwise (e.g. "per user" → user, "every request" → request).
4. If the user gives a percentage ("on 10% of traffic"), set variant_weight or max_traffic_pct as appropriate and note which in comments.
5. You may receive a context object with the tenant''s top_models. Prefer real models from that list for cohort_model/override_model when the user is vague.
6. Never invent fields. Output the JSON object and nothing else.

EXAMPLES
- "is gpt-4o-mini cheaper than gpt-4o for our RAG calls without hurting quality"
  → {"name":"gpt-4o-mini vs gpt-4o — RAG cost","cohort_model":"gpt-4o","workflow_type":"rag","override_model":"gpt-4o-mini","variant_weight":50,"max_traffic_pct":5,"key_source":"conversation","objective":"cost","reasoning":"Split RAG gpt-4o traffic against gpt-4o-mini and check cost savings hold quality.","comments":""}
- "try claude haiku against gpt-4o on 10% of traffic"
  → {"name":"claude-haiku vs gpt-4o","cohort_model":"gpt-4o","workflow_type":"","override_model":"claude-haiku-4-5-20251001","variant_weight":50,"max_traffic_pct":10,"key_source":"conversation","objective":"cost","reasoning":"Test Claude Haiku as a cheaper alternative on a capped 10% of traffic.","comments":"Capped exposure at 10% of all traffic (max_traffic_pct)."}

Always respond with the JSON object in exactly the format above and nothing else.',
    '{
        "provider": "openai",
        "model": "gpt-4o",
        "temperature": 0.2,
        "max_tokens": 600,
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
    '["aiqg", "experiments", "designer", "query-assistant", "system-tool"]'::jsonb,
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
    IF NOT EXISTS (SELECT 1 FROM agent_builder.agents WHERE id = '00000000-0000-0000-0000-000000000016') THEN
        RAISE EXCEPTION 'Failed to insert AIQG Experiment Designer agent';
    END IF;
    RAISE NOTICE 'AIQG Experiment Designer agent created/updated successfully';
END $$;
