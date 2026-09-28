-- Migration: 022_index_owner_space.sql
-- Description: Composite index for the space-scoped agent predicate (AB-5)
-- Author: TAS Agent Builder Team
-- Created: 2026-09-28

BEGIN;

-- Every agent read and write now filters on owner_id together with space_id.
-- The existing indexes pair each of those with status instead, so the new
-- predicate could use only the leading column of either. This index matches
-- it exactly.
CREATE INDEX IF NOT EXISTS idx_agents_owner_space
    ON agent_builder.agents(owner_id, space_id);

COMMIT;
