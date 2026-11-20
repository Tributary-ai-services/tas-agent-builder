-- Migration: 023_agent_visibility.sql
-- Description: Three-level agent visibility (AB-13, Plan #22 W0.3)
-- Created: 2026-11-20
--
-- is_public meant "every user of every organization", with no tenant
-- condition, so one click on "make public" shared an agent's system prompt
-- and notebook bindings platform-wide. It becomes one of three levels:
--
--   private       the owner, in the space it was created in
--   organization  anyone working in a space of the same Aether Organization
--   public        everyone (admin-set: system agents, templates)
--
-- is_public stays for one release so existing readers keep working, held equal
-- to visibility = 'public' by a CHECK. It is a plain column, not a generated
-- one, because the service runs GORM AutoMigrate on boot and would fight a
-- generated column. Drop it, and the CHECK, in a later migration.
--
-- Idempotent: safe to run again, and safe if AutoMigrate already added the
-- two columns with their defaults.
--
-- Live before (2026-11-20): 18 agents; the 16 system agents (tenant 'system')
-- are is_public = true, the other 2 are private. So the backfill moves 16 rows
-- to 'public' and no customer agent changes who can read it.

BEGIN;

ALTER TABLE agent_builder.agents
    ADD COLUMN IF NOT EXISTS visibility VARCHAR(20) NOT NULL DEFAULT 'private';

ALTER TABLE agent_builder.agents
    ADD COLUMN IF NOT EXISTS organization_id VARCHAR(255) NOT NULL DEFAULT '';

UPDATE agent_builder.agents
   SET visibility = CASE WHEN is_public THEN 'public' ELSE 'private' END
 WHERE visibility IS DISTINCT FROM CASE WHEN is_public THEN 'public' ELSE 'private' END;

ALTER TABLE agent_builder.agents DROP CONSTRAINT IF EXISTS agents_visibility_check;
ALTER TABLE agent_builder.agents
    ADD CONSTRAINT agents_visibility_check
    CHECK (visibility IN ('private', 'organization', 'public'));

ALTER TABLE agent_builder.agents DROP CONSTRAINT IF EXISTS agents_is_public_matches;
ALTER TABLE agent_builder.agents
    ADD CONSTRAINT agents_is_public_matches
    CHECK (is_public = (visibility = 'public'));

-- The read rule's new arm filters on these two together.
CREATE INDEX IF NOT EXISTS idx_agents_visibility_org
    ON agent_builder.agents(visibility, organization_id);

COMMIT;
