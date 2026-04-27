-- Migration: 019_create_model_migrations_table.sql
-- Description: Audit table recording every automatic rewrite of an agent's
--              llm_config.model by the model-migrate tool or runtime
--              fallback path. Gives us a diff per change and a way to revert.
-- Created: 2026-04-21

BEGIN;

CREATE TABLE IF NOT EXISTS agent_builder.model_migrations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id    UUID NOT NULL,
    provider    TEXT NOT NULL,
    old_model   TEXT NOT NULL,
    new_model   TEXT NOT NULL,
    tier        TEXT,
    source      TEXT NOT NULL,
    migrated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_migrations_agent_id
    ON agent_builder.model_migrations(agent_id);
CREATE INDEX IF NOT EXISTS idx_model_migrations_migrated_at
    ON agent_builder.model_migrations(migrated_at DESC);

COMMENT ON TABLE agent_builder.model_migrations IS
    'Audit log of automatic llm_config.model rewrites. Source is ''cronjob'' for scheduled migrations or ''runtime-fallback'' for in-flight router fallbacks.';

COMMIT;
