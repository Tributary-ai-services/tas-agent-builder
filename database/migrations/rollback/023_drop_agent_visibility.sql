-- Rollback: 023_drop_agent_visibility.sql
-- Description: Remove three-level visibility added in 023
--
-- is_public is still maintained, so dropping visibility loses only the
-- private/organization distinction: organization agents become private,
-- which is the narrower (safe) direction. Roll back the service image first,
-- or its INSERTs will name columns that no longer exist.

BEGIN;

DROP INDEX IF EXISTS agent_builder.idx_agents_visibility_org;
ALTER TABLE agent_builder.agents DROP CONSTRAINT IF EXISTS agents_is_public_matches;
ALTER TABLE agent_builder.agents DROP CONSTRAINT IF EXISTS agents_visibility_check;
ALTER TABLE agent_builder.agents DROP COLUMN IF EXISTS organization_id;
ALTER TABLE agent_builder.agents DROP COLUMN IF EXISTS visibility;

COMMIT;
