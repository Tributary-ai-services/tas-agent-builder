package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/models"
)

type AgentService interface {
	CreateAgent(ctx context.Context, req models.CreateAgentRequest, ownerID string, tenantID string) (*models.Agent, error)
	GetAgent(ctx context.Context, id uuid.UUID, userID string) (*models.Agent, error)
	GetAgentByOwner(ctx context.Context, id uuid.UUID, ownerID string) (*models.Agent, error)
	UpdateAgent(ctx context.Context, id uuid.UUID, req models.UpdateAgentRequest, ownerID string) (*models.Agent, error)
	DeleteAgent(ctx context.Context, id uuid.UUID, ownerID string) error
	ListAgents(ctx context.Context, filter models.AgentListFilter, userID string) (*models.AgentListResponse, error)
	
	PublishAgent(ctx context.Context, id uuid.UUID, ownerID string) error
	UnpublishAgent(ctx context.Context, id uuid.UUID, ownerID string) error
	
	DuplicateAgent(ctx context.Context, sourceID uuid.UUID, newName string, userID string, tenantID string) (*models.Agent, error)
	
	GetAgentsBySpace(ctx context.Context, spaceID uuid.UUID, userID string) ([]models.Agent, error)
	GetPublicAgents(ctx context.Context, filter models.AgentListFilter) (*models.AgentListResponse, error)
	GetAgentTemplates(ctx context.Context, filter models.AgentListFilter) (*models.AgentListResponse, error)

	// Internal agents (system agents available to all users)
	GetInternalAgents(ctx context.Context) ([]models.Agent, error)
	GetInternalAgent(ctx context.Context, id uuid.UUID) (*models.Agent, error)
}

type ExecutionService interface {
	StartExecution(ctx context.Context, req models.StartExecutionRequest, userID uuid.UUID) (*models.AgentExecution, error)
	CompleteExecution(ctx context.Context, executionID uuid.UUID, status models.ExecutionStatus, outputData map[string]any, errorMsg *string, durationMs int) error
	GetExecution(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.AgentExecution, error)
	ListExecutions(ctx context.Context, filter models.ExecutionListFilter, userID uuid.UUID) (*models.ExecutionListResponse, error)

	CancelExecution(ctx context.Context, id uuid.UUID, userID uuid.UUID) error

	GetExecutionsByAgent(ctx context.Context, agentID uuid.UUID, userID uuid.UUID, limit int) ([]models.AgentExecution, error)
	GetExecutionsBySession(ctx context.Context, sessionID string, userID uuid.UUID) ([]models.AgentExecution, error)
}

type StatsService interface {
	GetAgentStats(ctx context.Context, agentID uuid.UUID, userID uuid.UUID) (*models.StatsResponse, error)
	UpdateAgentStats(ctx context.Context, agentID uuid.UUID) error
	
	GetUserAgentStats(ctx context.Context, userID uuid.UUID) ([]models.StatsResponse, error)
	GetSpaceAgentStats(ctx context.Context, spaceID uuid.UUID, userID uuid.UUID) ([]models.StatsResponse, error)
	
	RefreshAllStats(ctx context.Context) error
	ResetDailyStats(ctx context.Context) error
	ResetWeeklyStats(ctx context.Context) error
	ResetMonthlyStats(ctx context.Context) error
}

type RouterService interface {
	SendRequest(ctx context.Context, agentConfig models.AgentLLMConfig, messages []Message, userID uuid.UUID) (*RouterResponse, error)
	SendRequestWithTools(ctx context.Context, agentConfig models.AgentLLMConfig, messages []Message, tools []ToolDefinition, toolChoice string, userID uuid.UUID) (*RouterResponse, error)
	ValidateConfig(ctx context.Context, config models.AgentLLMConfig) error
	GetAvailableProviders(ctx context.Context) ([]Provider, error)
	GetProviderModels(ctx context.Context, provider string) ([]Model, error)
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall represents a tool call requested by the LLM
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction contains the function name and arguments for a tool call
type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolDefinition defines a tool available for LLM function calling
type ToolDefinition struct {
	Type     string          `json:"type"` // "function"
	Function ToolFunctionDef `json:"function"`
}

// ToolFunctionDef defines a function that can be called by the LLM
type ToolFunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"` // JSON Schema
}

type RouterResponse struct {
	Content         string                 `json:"content"`
	Provider        string                 `json:"provider"`
	Model           string                 `json:"model"`
	RoutingStrategy string                 `json:"routing_strategy"`
	TokenUsage      int                    `json:"token_usage"`
	CostUSD         float64                `json:"cost_usd"`
	ResponseTimeMs  int                    `json:"response_time_ms"`
	Metadata        map[string]interface{} `json:"metadata"`
	ToolCalls       []ToolCall             `json:"tool_calls,omitempty"`
	FinishReason    string                 `json:"finish_reason,omitempty"`
}

type Provider struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Models      []string `json:"models"`
	Features    []string `json:"features"`
}

type Model struct {
	Name         string  `json:"name"`
	DisplayName  string  `json:"display_name"`
	Provider     string  `json:"provider"`
	MaxTokens    int     `json:"max_tokens"`
	CostPer1000  float64 `json:"cost_per_1000"`
	Features     []string `json:"features"`
}

// DocumentContextService provides document retrieval and context injection for agents
type DocumentContextService interface {
	// RetrieveVectorContext performs vector search to retrieve relevant document chunks
	RetrieveVectorContext(ctx context.Context, req models.VectorSearchRequest) (*models.DocumentContextResult, error)

	// RetrieveFullDocuments retrieves complete document content for injection
	RetrieveFullDocuments(ctx context.Context, req models.ChunkRetrievalRequest) (*models.DocumentContextResult, error)

	// RetrieveHybridContext combines vector search with full document sections
	RetrieveHybridContext(ctx context.Context, query string, req models.ChunkRetrievalRequest, vectorWeight, fullDocWeight float64) (*models.DocumentContextResult, error)

	// RetrieveHybridContextWithConfig combines vector search with full document sections using advanced configuration
	RetrieveHybridContextWithConfig(ctx context.Context, query string, req models.ChunkRetrievalRequest, config *models.HybridContextConfig) (*models.HybridContextResult, error)

	// GetNotebookDocuments retrieves document list for a notebook (including sub-notebooks if specified)
	GetNotebookDocuments(ctx context.Context, notebookIDs []uuid.UUID, tenantID string, includeSubNotebooks bool) ([]models.NotebookDocument, error)

	// FormatContextForInjection formats retrieved chunks into a string ready for prompt injection
	FormatContextForInjection(result *models.DocumentContextResult, maxTokens int) (*models.ContextInjectionResult, error)

	// EstimateTokenCount estimates the number of tokens in a string
	EstimateTokenCount(text string) int
}

// ChunkRetrievalService provides chunk retrieval from AudiModal
type ChunkRetrievalService interface {
	// GetChunksByDocuments retrieves all chunks for specific document IDs
	GetChunksByDocuments(ctx context.Context, documentIDs []uuid.UUID, tenantID string) ([]models.StoredChunk, error)

	// GetChunksByNotebook retrieves all chunks for documents in a notebook
	GetChunksByNotebook(ctx context.Context, notebookID uuid.UUID, tenantID string) ([]models.StoredChunk, error)

	// SearchChunks searches chunks with filters
	SearchChunks(ctx context.Context, req models.ChunkRetrievalRequest) (*models.ChunkRetrievalResponse, error)
}

// NotebookService provides notebook hierarchy and document relationship queries
type NotebookService interface {
	// GetNotebookHierarchy retrieves notebook with sub-notebooks recursively
	GetNotebookHierarchy(ctx context.Context, notebookID uuid.UUID, tenantID string) (*models.NotebookHierarchy, error)

	// GetDocumentsRecursive retrieves all documents from a notebook and its sub-notebooks
	GetDocumentsRecursive(ctx context.Context, notebookID uuid.UUID, tenantID string) ([]models.NotebookDocument, error)

	// GetSubNotebookIDs retrieves IDs of all sub-notebooks for a parent notebook
	GetSubNotebookIDs(ctx context.Context, parentNotebookID uuid.UUID, tenantID string) ([]uuid.UUID, error)
}

// CacheService provides caching for document context retrieval
type CacheService interface {
	// GetCachedContext retrieves cached context if available
	GetCachedContext(ctx context.Context, cacheKey string) (*models.DocumentContextResult, error)

	// SetCachedContext stores context in cache with TTL
	SetCachedContext(ctx context.Context, cacheKey string, result *models.DocumentContextResult, ttlSeconds int) error

	// InvalidateCache invalidates cached context for specific patterns
	InvalidateCache(ctx context.Context, pattern string) error

	// GenerateCacheKey generates a cache key for context retrieval
	GenerateCacheKey(agentID uuid.UUID, sessionID *string, queryHash string) string
}

// MemoryService provides the unified 3-tier memory system for agents
type MemoryService interface {
	// GetMemoryState retrieves the full memory state for an execution
	GetMemoryState(ctx context.Context, req models.GetMemoryRequest) (*models.MemoryState, error)

	// GetFormattedMemory retrieves and formats memory for context injection
	GetFormattedMemory(ctx context.Context, req models.GetMemoryRequest, tokenBudget int) (*models.MemoryContext, error)

	// AddMemory adds a new entry to short-term memory
	AddMemory(ctx context.Context, req models.AddMemoryRequest) error

	// UpdateWorkingMemory updates the working memory with new document context
	UpdateWorkingMemory(ctx context.Context, sessionID string, agentID uuid.UUID, chunks []models.RetrievedChunk) error

	// ConsolidateMemory consolidates short-term to long-term memory
	ConsolidateMemory(ctx context.Context, req models.ConsolidationRequest) (*models.ConsolidationResult, error)

	// GetMemoryStats retrieves memory usage statistics
	GetMemoryStats(ctx context.Context, sessionID string, agentID uuid.UUID) (*models.MemoryStats, error)

	// ClearSession removes all memory for a session
	ClearSession(ctx context.Context, sessionID string, agentID uuid.UUID) error
}

// ShortTermMemoryService manages conversation buffer (Tier 1)
type ShortTermMemoryService interface {
	// GetConversation retrieves the conversation history for a session
	GetConversation(ctx context.Context, sessionID string, agentID uuid.UUID) (*models.ShortTermMemory, error)

	// AddMessage adds a message to the conversation buffer
	AddMessage(ctx context.Context, entry models.MemoryEntry) error

	// GetRecentMessages retrieves the N most recent messages
	GetRecentMessages(ctx context.Context, sessionID string, agentID uuid.UUID, limit int) ([]models.MemoryEntry, error)

	// TrimToTokenLimit trims conversation to fit within token limit
	TrimToTokenLimit(ctx context.Context, sessionID string, agentID uuid.UUID, maxTokens int) error

	// ClearConversation clears the conversation buffer for a session
	ClearConversation(ctx context.Context, sessionID string, agentID uuid.UUID) error

	// SetExpiration updates the TTL for a session's memory
	SetExpiration(ctx context.Context, sessionID string, agentID uuid.UUID, ttl int) error
}

// WorkingMemoryService manages session document context (Tier 2)
type WorkingMemoryService interface {
	// GetWorkingMemory retrieves the working memory for a session
	GetWorkingMemory(ctx context.Context, sessionID string, agentID uuid.UUID) (*models.WorkingMemory, error)

	// SetDocumentContext sets the document chunks in working memory
	SetDocumentContext(ctx context.Context, sessionID string, agentID uuid.UUID, chunks []models.RetrievedChunk) error

	// LoadDocument loads a document into working memory
	LoadDocument(ctx context.Context, sessionID string, agentID uuid.UUID, doc models.LoadedDocument) error

	// UnloadDocument removes a document from working memory
	UnloadDocument(ctx context.Context, sessionID string, agentID uuid.UUID, documentID uuid.UUID) error

	// UpdateLastQuery updates the last query that retrieved this context
	UpdateLastQuery(ctx context.Context, sessionID string, agentID uuid.UUID, query string) error

	// IsContextStale checks if the working memory needs refresh based on new query
	IsContextStale(ctx context.Context, sessionID string, agentID uuid.UUID, newQuery string, threshold float64) (bool, error)

	// ClearWorkingMemory clears all working memory for a session
	ClearWorkingMemory(ctx context.Context, sessionID string, agentID uuid.UUID) error
}

// LongTermMemoryService manages persistent knowledge (Tier 3)
type LongTermMemoryService interface {
	// SearchMemory performs semantic search over long-term memories
	SearchMemory(ctx context.Context, agentID uuid.UUID, query string, topK int) ([]models.LongTermMemoryEntry, error)

	// StoreMemory stores a new long-term memory entry
	StoreMemory(ctx context.Context, entry models.LongTermMemoryEntry) error

	// StoreSummary stores a conversation summary as long-term memory
	StoreSummary(ctx context.Context, agentID uuid.UUID, sessionID string, summary string, sourceEntries []string) error

	// GetRecentMemories retrieves recently accessed memories
	GetRecentMemories(ctx context.Context, agentID uuid.UUID, limit int) ([]models.LongTermMemoryEntry, error)

	// UpdateAccessStats updates access statistics for a memory
	UpdateAccessStats(ctx context.Context, memoryID string) error

	// DeleteMemory deletes a long-term memory entry
	DeleteMemory(ctx context.Context, memoryID string) error

	// GetMemoryCount returns the count of long-term memories for an agent
	GetMemoryCount(ctx context.Context, agentID uuid.UUID) (int, error)

	// PruneOldMemories removes old/unused memories to stay within limits
	PruneOldMemories(ctx context.Context, agentID uuid.UUID, maxEntries int) (int, error)
}

// MemoryConsolidationService handles memory consolidation logic
type MemoryConsolidationService interface {
	// ShouldConsolidate checks if consolidation is needed
	ShouldConsolidate(ctx context.Context, sessionID string, agentID uuid.UUID) (bool, error)

	// ConsolidateSession consolidates a session's memories
	ConsolidateSession(ctx context.Context, req models.ConsolidationRequest) (*models.ConsolidationResult, error)

	// GenerateSummary generates a summary of conversation entries
	GenerateSummary(ctx context.Context, entries []models.MemoryEntry, maxTokens int) (string, error)

	// ExtractFacts extracts factual information from conversation
	ExtractFacts(ctx context.Context, entries []models.MemoryEntry) ([]models.LongTermMemoryEntry, error)

	// ScheduleConsolidation schedules periodic consolidation
	ScheduleConsolidation(ctx context.Context, interval int) error
}

// MCPContextService provides MCP-based document retrieval for autonomous agents
type MCPContextService interface {
	// InvokeTool invokes an MCP tool and returns the result
	InvokeTool(ctx context.Context, req models.MCPToolRequest) (*models.MCPToolResponse, error)

	// ListAvailableTools lists all available MCP tools for document retrieval
	ListAvailableTools(ctx context.Context) ([]models.MCPToolDefinition, error)

	// ListToolsForLLM returns tools in OpenAI function-calling format for LLM requests
	ListToolsForLLM(ctx context.Context) ([]ToolDefinition, error)

	// SearchDocuments searches documents using MCP search tool
	SearchDocuments(ctx context.Context, req models.MCPSearchRequest) (*models.DocumentContextResult, error)

	// GetDocumentContent retrieves full document content via MCP
	GetDocumentContent(ctx context.Context, documentID string, tenantID string) (*models.DocumentContextResult, error)

	// GetDocumentSummary retrieves cached document summary via MCP
	GetDocumentSummary(ctx context.Context, documentID string, tenantID string) (string, error)

	// RetrieveMCPContext performs autonomous context retrieval using MCP tools
	// The agent decides which tools to use based on the query
	RetrieveMCPContext(ctx context.Context, query string, notebookIDs []uuid.UUID, tenantID string, maxTokens int) (*models.MCPContextResult, error)
}