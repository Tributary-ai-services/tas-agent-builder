-- Rollback Migration: 000_drop_schema.sql
-- Description: Drop agent_builder schema from shared database
-- Author: Agent Builder Team
-- Created: 2024-01-15

BEGIN;

-- Drop all objects in schema (CASCADE removes dependent objects)
DROP SCHEMA IF EXISTS agent_builder CASCADE;

COMMIT;