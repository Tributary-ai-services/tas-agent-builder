package models

import (
	"time"

	"github.com/google/uuid"
)

// MemoryType represents the type of memory storage
type MemoryType string

const (
	// MemoryTypeShortTerm is the conversation history buffer
	MemoryTypeShortTerm MemoryType = "short_term"
	// MemoryTypeWorking is the session document context
	MemoryTypeWorking MemoryType = "working"
	// MemoryTypeLongTerm is the vector store integration
	MemoryTypeLongTerm MemoryType = "long_term"
)

// MemoryEntry represents a single memory entry (conversation turn)
type MemoryEntry struct {
	ID        string     `json:"id"`
	SessionID string     `json:"session_id"`
	AgentID   uuid.UUID  `json:"agent_id"`
	TenantID  string     `json:"tenant_id"`
	UserID    string     `json:"user_id"`
	Type      MemoryType `json:"type"`
	Role      string     `json:"role"`    // "user", "assistant", "system"
	Content   string     `json:"content"`
	TokenCount int       `json:"token_count"`
	Timestamp time.Time  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ShortTermMemory holds the conversation buffer for a session
type ShortTermMemory struct {
	SessionID    string        `json:"session_id"`
	AgentID      uuid.UUID     `json:"agent_id"`
	TenantID     string        `json:"tenant_id"`
	UserID       string        `json:"user_id"`
	Entries      []MemoryEntry `json:"entries"`
	TotalTokens  int           `json:"total_tokens"`
	MaxTokens    int           `json:"max_tokens"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	ExpiresAt    time.Time     `json:"expires_at"`
}

// WorkingMemory holds session-specific document context
type WorkingMemory struct {
	SessionID        string           `json:"session_id"`
	AgentID          uuid.UUID        `json:"agent_id"`
	TenantID         string           `json:"tenant_id"`
	UserID           string           `json:"user_id"`
	LoadedDocuments  []LoadedDocument `json:"loaded_documents"`
	RetrievedChunks  []RetrievedChunk `json:"retrieved_chunks"`
	TotalTokens      int              `json:"total_tokens"`
	MaxTokens        int              `json:"max_tokens"`
	LastQuery        string           `json:"last_query,omitempty"`
	LastQueryTime    *time.Time       `json:"last_query_time,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	ExpiresAt        time.Time        `json:"expires_at"`
}

// LoadedDocument represents a document loaded into working memory
type LoadedDocument struct {
	DocumentID   uuid.UUID `json:"document_id"`
	DocumentName string    `json:"document_name"`
	NotebookID   uuid.UUID `json:"notebook_id,omitempty"`
	ChunkCount   int       `json:"chunk_count"`
	TokenCount   int       `json:"token_count"`
	LoadedAt     time.Time `json:"loaded_at"`
}

// LongTermMemoryEntry represents stored knowledge in the vector store
type LongTermMemoryEntry struct {
	ID           string                 `json:"id"`
	AgentID      uuid.UUID              `json:"agent_id"`
	TenantID     string                 `json:"tenant_id"`
	ContentType  string                 `json:"content_type"` // "summary", "fact", "insight", "conversation"
	Content      string                 `json:"content"`
	Embedding    []float32              `json:"embedding,omitempty"`
	TokenCount   int                    `json:"token_count"`
	Score        float64                `json:"score,omitempty"`
	SourceType   string                 `json:"source_type,omitempty"`   // "conversation", "document", "user_feedback"
	SourceID     string                 `json:"source_id,omitempty"`     // Reference to original source
	SessionID    string                 `json:"session_id,omitempty"`    // Original session if from conversation
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	AccessedAt   time.Time              `json:"accessed_at"`
	AccessCount  int                    `json:"access_count"`
}

// MemoryConfig defines configuration for the memory system
type MemoryConfig struct {
	// Short-term memory settings
	ShortTermMaxTokens   int           `json:"short_term_max_tokens"`   // Default: 4000
	ShortTermTTL         time.Duration `json:"short_term_ttl"`          // Default: 1 hour
	ShortTermMaxEntries  int           `json:"short_term_max_entries"`  // Default: 50

	// Working memory settings
	WorkingMemoryMaxTokens int           `json:"working_memory_max_tokens"` // Default: 8000
	WorkingMemoryTTL       time.Duration `json:"working_memory_ttl"`        // Default: 30 minutes
	MaxLoadedDocuments     int           `json:"max_loaded_documents"`      // Default: 10
	AutoRefreshThreshold   float64       `json:"auto_refresh_threshold"`    // Re-query if relevance drops below

	// Long-term memory settings
	LongTermEnabled       bool          `json:"long_term_enabled"`
	ConsolidationInterval time.Duration `json:"consolidation_interval"` // How often to consolidate memories
	MaxLongTermEntries    int           `json:"max_long_term_entries"`  // Per agent
	SummaryMinTokens      int           `json:"summary_min_tokens"`     // Min tokens before summarization
	SummaryMaxTokens      int           `json:"summary_max_tokens"`     // Max tokens for summary
}

// DefaultMemoryConfig returns sensible default memory configuration
func DefaultMemoryConfig() *MemoryConfig {
	return &MemoryConfig{
		ShortTermMaxTokens:     4000,
		ShortTermTTL:           time.Hour,
		ShortTermMaxEntries:    50,
		WorkingMemoryMaxTokens: 8000,
		WorkingMemoryTTL:       30 * time.Minute,
		MaxLoadedDocuments:     10,
		AutoRefreshThreshold:   0.5,
		LongTermEnabled:        true,
		ConsolidationInterval:  5 * time.Minute,
		MaxLongTermEntries:     1000,
		SummaryMinTokens:       500,
		SummaryMaxTokens:       500,
	}
}

// MemoryState represents the combined memory state for an execution
type MemoryState struct {
	ShortTerm    *ShortTermMemory      `json:"short_term,omitempty"`
	Working      *WorkingMemory        `json:"working,omitempty"`
	LongTerm     []LongTermMemoryEntry `json:"long_term,omitempty"`
	TotalTokens  int                   `json:"total_tokens"`
	TokenBudget  int                   `json:"token_budget"`
}

// MemoryContext is the formatted memory ready for injection into agent context
type MemoryContext struct {
	FormattedShortTerm string `json:"formatted_short_term"`
	FormattedWorking   string `json:"formatted_working"`
	FormattedLongTerm  string `json:"formatted_long_term"`
	TotalTokens        int    `json:"total_tokens"`
	Truncated          bool   `json:"truncated"`
	Strategy           string `json:"strategy"` // How memory was prioritized
}

// AddMemoryRequest represents a request to add a memory entry
type AddMemoryRequest struct {
	SessionID string                 `json:"session_id"`
	AgentID   uuid.UUID              `json:"agent_id"`
	TenantID  string                 `json:"tenant_id"`
	UserID    string                 `json:"user_id"`
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// GetMemoryRequest represents a request to retrieve memory
type GetMemoryRequest struct {
	SessionID    string     `json:"session_id,omitempty"`
	AgentID      uuid.UUID  `json:"agent_id"`
	TenantID     string     `json:"tenant_id"`
	UserID       string     `json:"user_id"`
	MaxTokens    int        `json:"max_tokens,omitempty"`
	IncludeTypes []MemoryType `json:"include_types,omitempty"` // Which memory tiers to include
	Query        string     `json:"query,omitempty"`           // For long-term memory search
}

// ConsolidationRequest represents a request to consolidate memories
type ConsolidationRequest struct {
	SessionID   string    `json:"session_id"`
	AgentID     uuid.UUID `json:"agent_id"`
	TenantID    string    `json:"tenant_id"`
	UserID      string    `json:"user_id"`
	Force       bool      `json:"force"`            // Force consolidation even if threshold not met
	MaxEntries  int       `json:"max_entries"`      // Max entries to consolidate at once
}

// ConsolidationResult represents the result of memory consolidation
type ConsolidationResult struct {
	EntriesProcessed   int       `json:"entries_processed"`
	SummariesCreated   int       `json:"summaries_created"`
	TokensConsolidated int       `json:"tokens_consolidated"`
	TokensSaved        int       `json:"tokens_saved"`
	Duration           int       `json:"duration_ms"`
	Timestamp          time.Time `json:"timestamp"`
}

// MemoryStats provides statistics about memory usage
type MemoryStats struct {
	SessionID           string    `json:"session_id"`
	AgentID             uuid.UUID `json:"agent_id"`
	ShortTermEntries    int       `json:"short_term_entries"`
	ShortTermTokens     int       `json:"short_term_tokens"`
	WorkingDocuments    int       `json:"working_documents"`
	WorkingChunks       int       `json:"working_chunks"`
	WorkingTokens       int       `json:"working_tokens"`
	LongTermEntries     int       `json:"long_term_entries"`
	TotalConsolidations int       `json:"total_consolidations"`
	LastConsolidation   *time.Time `json:"last_consolidation,omitempty"`
	LastAccess          time.Time `json:"last_access"`
}
