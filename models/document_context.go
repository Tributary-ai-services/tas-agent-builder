package models

import (
	"time"

	"github.com/google/uuid"
)

// RetrievedChunk represents a document chunk retrieved for context injection
type RetrievedChunk struct {
	ID           string                 `json:"id"`
	DocumentID   string                 `json:"document_id"`
	DocumentName string                 `json:"document_name,omitempty"`
	NotebookID   string                 `json:"notebook_id,omitempty"`
	Content      string                 `json:"content"`
	ChunkNumber  int                    `json:"chunk_number"`
	TotalChunks  int                    `json:"total_chunks,omitempty"`
	Score        float64                `json:"score,omitempty"`       // Similarity score for vector search
	Distance     float64                `json:"distance,omitempty"`    // Distance metric
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ContentType  string                 `json:"content_type,omitempty"` // text, table, image, etc.
	Language     string                 `json:"language,omitempty"`
	PageNumber   *int                   `json:"page_number,omitempty"`
}

// DocumentContextResult contains the retrieved context for agent execution
type DocumentContextResult struct {
	Chunks          []RetrievedChunk       `json:"chunks"`
	TotalTokens     int                    `json:"total_tokens"`
	TruncatedChunks int                    `json:"truncated_chunks,omitempty"` // Number of chunks truncated due to token limit
	Strategy        ContextStrategy        `json:"strategy"`
	RetrievalTimeMs int                    `json:"retrieval_time_ms"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// SearchOptions configures how document search is performed
type SearchOptions struct {
	TopK          int                    `json:"top_k"`
	MinScore      float64                `json:"min_score,omitempty"`
	MaxTokens     int                    `json:"max_tokens,omitempty"`
	IncludeChunks bool                   `json:"include_chunks"`
	Filters       map[string]interface{} `json:"filters,omitempty"`
}

// VectorSearchRequest represents a request to search vectors via DeepLake
type VectorSearchRequest struct {
	QueryText   string        `json:"query_text"`
	DatasetID   string        `json:"dataset_id,omitempty"`
	NotebookIDs []uuid.UUID   `json:"notebook_ids,omitempty"`
	DocumentIDs []uuid.UUID   `json:"document_ids,omitempty"`
	TenantID    string        `json:"tenant_id"`
	SpaceID     string        `json:"space_id,omitempty"`
	Options     SearchOptions `json:"options"`
	AuthToken   string        `json:"-"` // Auth token for downstream API calls (not serialized)
}

// VectorSearchResponse represents a response from DeepLake vector search
type VectorSearchResponse struct {
	Results       []VectorSearchResult `json:"results"`
	TotalFound    int                  `json:"total_found"`
	HasMore       bool                 `json:"has_more"`
	QueryTimeMs   float64              `json:"query_time_ms"`
	EmbeddingTimeMs float64            `json:"embedding_time_ms,omitempty"`
}

// VectorSearchResult represents a single search result from DeepLake
type VectorSearchResult struct {
	ID          string                 `json:"id"`
	DocumentID  string                 `json:"document_id"`
	ChunkID     string                 `json:"chunk_id,omitempty"`
	Content     string                 `json:"content"`
	ContentHash string                 `json:"content_hash,omitempty"`
	Score       float64                `json:"score"`
	Distance    float64                `json:"distance"`
	Rank        int                    `json:"rank"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	ChunkIndex  *int                   `json:"chunk_index,omitempty"`
	ChunkCount  *int                   `json:"chunk_count,omitempty"`
	TenantID    string                 `json:"tenant_id,omitempty"`
}

// ChunkRetrievalRequest represents a request to retrieve document chunks from AudiModal
type ChunkRetrievalRequest struct {
	TenantID    string      `json:"tenant_id"`
	FileIDs     []uuid.UUID `json:"file_ids,omitempty"`
	NotebookIDs []uuid.UUID `json:"notebook_ids,omitempty"`
	ChunkTypes  []string    `json:"chunk_types,omitempty"` // text, table, image, etc.
	Limit       int         `json:"limit,omitempty"`
	Offset      int         `json:"offset,omitempty"`
	OrderBy     string      `json:"order_by,omitempty"` // chunk_number, created_at
	AuthToken   string      `json:"-"`                  // Auth token for AudiModal API (not serialized)
}

// ChunkRetrievalResponse represents chunks retrieved from AudiModal
type ChunkRetrievalResponse struct {
	Chunks     []StoredChunk `json:"chunks"`
	Total      int           `json:"total"`
	HasMore    bool          `json:"has_more"`
	RetrievalTimeMs int      `json:"retrieval_time_ms"`
}

// StoredChunk represents a chunk from AudiModal database
type StoredChunk struct {
	ID              uuid.UUID              `json:"id"`
	TenantID        uuid.UUID              `json:"tenant_id"`
	FileID          uuid.UUID              `json:"file_id"`
	ChunkID         string                 `json:"chunk_id"`
	ChunkType       string                 `json:"chunk_type"`
	ChunkNumber     int                    `json:"chunk_number"`
	Content         string                 `json:"content"`
	ContentHash     string                 `json:"content_hash"`
	SizeBytes       int64                  `json:"size_bytes"`
	StartPosition   *int64                 `json:"start_position,omitempty"`
	EndPosition     *int64                 `json:"end_position,omitempty"`
	PageNumber      *int                   `json:"page_number,omitempty"`
	LineNumber      *int                   `json:"line_number,omitempty"`
	EmbeddingStatus string                 `json:"embedding_status"`
	Language        string                 `json:"language,omitempty"`
	ContentCategory string                 `json:"content_category,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// ExecutionContextRequest extends the execution request with document selection
type ExecutionContextRequest struct {
	Input               string      `json:"input"`
	History             []Message   `json:"history,omitempty"`
	SessionID           *string     `json:"session_id,omitempty"`
	NotebookIDs         []uuid.UUID `json:"notebook_ids,omitempty"`          // Override agent's notebook IDs for context retrieval
	SelectedDocuments   []uuid.UUID `json:"selected_documents,omitempty"`    // Per-execution document selection
	IncludeSubNotebooks bool        `json:"include_sub_notebooks,omitempty"` // Include docs from sub-notebooks
	DisableKnowledge    bool        `json:"disable_knowledge,omitempty"`     // Temporarily disable knowledge retrieval
	TenantID            string      `json:"tenant_id,omitempty"`             // Tenant ID for document retrieval
	AuthToken           string      `json:"-"`                               // Auth token for downstream API calls (not serialized)
}

// Message represents a conversation message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// MultiPassResult represents the result of multi-pass document processing
type MultiPassResult struct {
	Segments        []SegmentResult `json:"segments"`
	AggregatedResult string         `json:"aggregated_result"`
	TotalPasses     int             `json:"total_passes"`
	TotalTokens     int             `json:"total_tokens"`
	ProcessingTimeMs int            `json:"processing_time_ms"`
}

// SegmentResult represents the result of processing a single document segment
type SegmentResult struct {
	SegmentNumber  int    `json:"segment_number"`
	Content        string `json:"content"`
	PartialResult  string `json:"partial_result"`
	TokensUsed     int    `json:"tokens_used"`
	ProcessingTimeMs int  `json:"processing_time_ms"`
}

// NotebookDocument represents a document in a notebook (from Neo4j via Aether-BE)
type NotebookDocument struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	NotebookID   uuid.UUID `json:"notebook_id"`
	NotebookName string    `json:"notebook_name,omitempty"`
	FileID       uuid.UUID `json:"file_id,omitempty"` // AudiModal file ID
	ContentType  string    `json:"content_type,omitempty"`
	SizeBytes    int64     `json:"size_bytes,omitempty"`
	ChunkCount   int       `json:"chunk_count,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

// NotebookHierarchy represents a notebook with potential sub-notebooks
type NotebookHierarchy struct {
	ID           uuid.UUID           `json:"id"`
	Name         string              `json:"name"`
	ParentID     *uuid.UUID          `json:"parent_id,omitempty"`
	Documents    []NotebookDocument  `json:"documents,omitempty"`
	SubNotebooks []NotebookHierarchy `json:"sub_notebooks,omitempty"`
}

// ContextInjectionResult contains the formatted context ready for injection
type ContextInjectionResult struct {
	FormattedContext string                 `json:"formatted_context"` // Ready to inject into prompt
	ChunkCount       int                    `json:"chunk_count"`
	DocumentCount    int                    `json:"document_count"`
	TotalTokens      int                    `json:"total_tokens"`
	Strategy         ContextStrategy        `json:"strategy"`
	Truncated        bool                   `json:"truncated"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// HybridContextConfig defines configuration for hybrid context retrieval strategy
type HybridContextConfig struct {
	// Weight for vector search results (0.0 - 1.0)
	VectorWeight float64 `json:"vector_weight"`
	// Weight for full document content (0.0 - 1.0)
	FullDocWeight float64 `json:"full_doc_weight"`
	// Weight for document position (earlier chunks score higher)
	PositionWeight float64 `json:"position_weight"`
	// Boost multiplier for document summaries
	SummaryBoost float64 `json:"summary_boost"`
	// Number of top results to retrieve from vector search
	VectorTopK int `json:"vector_top_k"`
	// Minimum similarity score for vector results
	VectorMinScore float64 `json:"vector_min_score"`
	// Maximum number of chunks from full documents
	FullDocMaxChunks int `json:"full_doc_max_chunks"`
	// Token budget for the merged result
	TokenBudget int `json:"token_budget"`
	// Include document summaries if available
	IncludeSummaries bool `json:"include_summaries"`
	// Deduplicate by content hash
	DeduplicateByContent bool `json:"deduplicate_by_content"`
	// Priority tiers for token budget allocation
	PriorityTiers []HybridPriorityTier `json:"priority_tiers,omitempty"`
}

// HybridPriorityTier defines a priority tier for token budget allocation
type HybridPriorityTier struct {
	Name       string  `json:"name"`        // e.g., "high_relevance", "summaries", "context"
	MinScore   float64 `json:"min_score"`   // Minimum score to qualify for this tier
	MaxTokens  int     `json:"max_tokens"`  // Maximum tokens for this tier
	Percentage float64 `json:"percentage"`  // Percentage of budget for this tier
}

// DefaultHybridContextConfig returns sensible defaults for hybrid context
func DefaultHybridContextConfig() *HybridContextConfig {
	return &HybridContextConfig{
		VectorWeight:         0.6,
		FullDocWeight:        0.3,
		PositionWeight:       0.1,
		SummaryBoost:         1.5,
		VectorTopK:           20,
		VectorMinScore:       0.5,
		FullDocMaxChunks:     50,
		TokenBudget:          8000,
		IncludeSummaries:     true,
		DeduplicateByContent: true,
		PriorityTiers: []HybridPriorityTier{
			{Name: "high_relevance", MinScore: 0.8, Percentage: 0.5},
			{Name: "medium_relevance", MinScore: 0.6, Percentage: 0.3},
			{Name: "context", MinScore: 0.0, Percentage: 0.2},
		},
	}
}

// ScoredChunk represents a chunk with a computed hybrid score
type ScoredChunk struct {
	Chunk           RetrievedChunk `json:"chunk"`
	VectorScore     float64        `json:"vector_score"`
	PositionScore   float64        `json:"position_score"`
	FullDocScore    float64        `json:"full_doc_score"`
	SummaryBoost    float64        `json:"summary_boost"`
	CombinedScore   float64        `json:"combined_score"`
	Source          string         `json:"source"` // "vector", "full_doc", "both"
	PriorityTier    string         `json:"priority_tier"`
	EstimatedTokens int            `json:"estimated_tokens"`
}

// HybridContextResult extends DocumentContextResult with hybrid-specific metadata
type HybridContextResult struct {
	*DocumentContextResult
	ScoredChunks      []ScoredChunk        `json:"scored_chunks"`
	VectorChunkCount  int                  `json:"vector_chunk_count"`
	FullDocChunkCount int                  `json:"full_doc_chunk_count"`
	DuplicatesRemoved int                  `json:"duplicates_removed"`
	TierBreakdown     map[string]int       `json:"tier_breakdown"`
	Config            *HybridContextConfig `json:"config"`
}

// ============================================
// MCP Context Strategy Types
// ============================================

// MCPToolDefinition defines an MCP tool available for document retrieval
type MCPToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Server      string                 `json:"server"` // MCP server that provides this tool
}

// MCPToolRequest represents a request to invoke an MCP tool
type MCPToolRequest struct {
	ToolName   string                 `json:"tool_name"`
	Parameters map[string]interface{} `json:"parameters"`
	TenantID   string                 `json:"tenant_id"`
	Timeout    int                    `json:"timeout_ms,omitempty"` // Timeout in milliseconds
}

// MCPToolResponse represents the response from an MCP tool invocation
type MCPToolResponse struct {
	ToolName     string                 `json:"tool_name"`
	Success      bool                   `json:"success"`
	Result       interface{}            `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
	ExecutionMs  int                    `json:"execution_ms"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// MCPSearchRequest represents a request to search documents via MCP
type MCPSearchRequest struct {
	Query       string      `json:"query"`
	NotebookIDs []uuid.UUID `json:"notebook_ids,omitempty"`
	TenantID    string      `json:"tenant_id"`
	TopK        int         `json:"top_k,omitempty"`
	MinScore    float64     `json:"min_score,omitempty"`
	Filters     map[string]interface{} `json:"filters,omitempty"`
}

// MCPContextResult contains the result of MCP-based context retrieval
type MCPContextResult struct {
	*DocumentContextResult
	ToolsUsed       []MCPToolInvocation `json:"tools_used"`
	TotalToolCalls  int                 `json:"total_tool_calls"`
	AutonomousSteps []MCPAutonomousStep `json:"autonomous_steps,omitempty"`
}

// MCPToolInvocation records a single MCP tool invocation
type MCPToolInvocation struct {
	ToolName    string      `json:"tool_name"`
	Parameters  interface{} `json:"parameters"`
	Success     bool        `json:"success"`
	ExecutionMs int         `json:"execution_ms"`
	ChunksFound int         `json:"chunks_found,omitempty"`
}

// MCPAutonomousStep represents a step in autonomous MCP retrieval
type MCPAutonomousStep struct {
	StepNumber  int    `json:"step_number"`
	Action      string `json:"action"`      // "search", "get_content", "get_summary", "refine_query"
	Reasoning   string `json:"reasoning"`   // Agent's reasoning for this step
	ToolUsed    string `json:"tool_used"`
	Success     bool   `json:"success"`
	ChunksAdded int    `json:"chunks_added"`
}

// MCPConfig holds configuration for MCP-based context retrieval
type MCPConfig struct {
	// Base MCP server URL
	ServerURL string `json:"server_url"`
	// Timeout for MCP tool invocations
	TimeoutMs int `json:"timeout_ms"`
	// Maximum number of autonomous steps
	MaxAutonomousSteps int `json:"max_autonomous_steps"`
	// Whether to allow query refinement
	AllowQueryRefinement bool `json:"allow_query_refinement"`
	// Available tools for document retrieval
	EnabledTools []string `json:"enabled_tools"`
}

// DefaultMCPConfig returns sensible defaults for MCP context
func DefaultMCPConfig() *MCPConfig {
	return &MCPConfig{
		ServerURL:            "http://tas-mcp:8082",
		TimeoutMs:            30000,
		MaxAutonomousSteps:   5,
		AllowQueryRefinement: true,
		EnabledTools: []string{
			"search_documents",
			"get_document_content",
			"get_document_summary",
			"list_notebook_documents",
		},
	}
}
