-- Migration: 015_add_document_context.sql
-- Description: Add document context configuration and agent type columns
-- Author: TAS Agent Builder Team
-- Created: 2025-02-03

BEGIN;

-- Add type column for agent categorization (conversational, qa, producer)
ALTER TABLE agent_builder.agents
ADD COLUMN IF NOT EXISTS type VARCHAR(50) NOT NULL DEFAULT 'conversational'
CHECK (type IN ('conversational', 'qa', 'producer'));

-- Add document_context JSONB column for document context configuration
-- This stores the DocumentContextConfig struct with strategy, scope, weights, etc.
ALTER TABLE agent_builder.agents
ADD COLUMN IF NOT EXISTS document_context JSONB DEFAULT NULL;

-- Create index for agent type queries
CREATE INDEX IF NOT EXISTS idx_agents_type ON agent_builder.agents(type);

-- Create composite index for type + status queries
CREATE INDEX IF NOT EXISTS idx_agents_type_status ON agent_builder.agents(type, status)
WHERE deleted_at IS NULL;

-- Create GIN index for document_context JSONB queries
CREATE INDEX IF NOT EXISTS idx_agents_document_context ON agent_builder.agents USING GIN (document_context)
WHERE document_context IS NOT NULL;

-- Add comments for documentation
COMMENT ON COLUMN agent_builder.agents.type IS 'Agent type: conversational (multi-turn dialogue), qa (stateless Q&A), producer (artifact generation)';
COMMENT ON COLUMN agent_builder.agents.document_context IS 'Document context configuration including strategy (vector/full/hybrid/mcp/none), scope, weights, and multi-pass settings';

-- Update existing agents to have a default document_context based on enable_knowledge
UPDATE agent_builder.agents
SET document_context = jsonb_build_object(
    'strategy', 'vector',
    'scope', 'all',
    'include_sub_notebooks', false,
    'max_context_tokens', 8000,
    'top_k', 10,
    'min_score', 0.7,
    'vector_weight', 0.5,
    'full_doc_weight', 0.5
)
WHERE enable_knowledge = true AND document_context IS NULL;

COMMIT;
