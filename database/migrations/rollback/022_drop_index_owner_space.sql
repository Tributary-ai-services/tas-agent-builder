-- Rollback: 022_drop_index_owner_space.sql
-- Description: Drop the composite owner/space index added in 022

BEGIN;

DROP INDEX IF EXISTS agent_builder.idx_agents_owner_space;

COMMIT;
