package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type AgentStatus string
type SpaceType string
type AgentType string

const (
	AgentStatusDraft      AgentStatus = "draft"
	AgentStatusPublished  AgentStatus = "published"
	AgentStatusDisabled   AgentStatus = "disabled"

	SpaceTypePersonal      SpaceType = "personal"
	SpaceTypeOrganization  SpaceType = "organization"

	AgentTypeConversational AgentType = "conversational"
	AgentTypeQA             AgentType = "qa"
	AgentTypeProducer       AgentType = "producer"
)

type AgentLLMConfig struct {
	Provider         string            `json:"provider" gorm:"not null"`
	Model            string            `json:"model" gorm:"not null"`
	Temperature      *float64          `json:"temperature,omitempty"`
	MaxTokens        *int              `json:"max_tokens,omitempty"`
	TopP             *float64          `json:"top_p,omitempty"`
	TopK             *int              `json:"top_k,omitempty"`
	Stop             []string          `json:"stop,omitempty"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
	// Enhanced reliability configuration
	OptimizeFor      string            `json:"optimize_for,omitempty"`      // "cost", "performance", "quality"
	RequiredFeatures []string          `json:"required_features,omitempty"` // e.g., ["functions", "vision"]
	MaxCost          *float64          `json:"max_cost,omitempty"`          // Maximum cost threshold
	RetryConfig      *RetryConfig      `json:"retry_config,omitempty"`      // Retry configuration
	FallbackConfig   *FallbackConfig   `json:"fallback_config,omitempty"`   // Fallback configuration
}

// RetryConfig defines retry behavior for failed requests
type RetryConfig struct {
	MaxAttempts     int      `json:"max_attempts"`               // Maximum retry attempts (1-5)
	BackoffType     string   `json:"backoff_type,omitempty"`     // "exponential" or "linear"
	BaseDelay       string   `json:"base_delay,omitempty"`       // Base delay between retries (e.g., "1s", "500ms")
	MaxDelay        string   `json:"max_delay,omitempty"`        // Maximum delay cap (e.g., "30s")
	RetryableErrors []string `json:"retryable_errors,omitempty"` // Error patterns that trigger retries
}

// FallbackConfig defines automatic fallback to alternative providers
type FallbackConfig struct {
	Enabled             bool     `json:"enabled"`                          // Enable fallback to healthy providers
	PreferredChain      []string `json:"preferred_chain,omitempty"`       // Custom fallback order (provider names)
	MaxCostIncrease     *float64 `json:"max_cost_increase,omitempty"`     // Max cost increase allowed for fallback
	RequireSameFeatures bool     `json:"require_same_features,omitempty"` // Whether fallback providers must support same features
}

func (c AgentLLMConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *AgentLLMConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), c)
	}
	
	return json.Unmarshal(bytes, c)
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:     3,
		BackoffType:     "exponential",
		BaseDelay:       "1s",
		MaxDelay:        "30s",
		RetryableErrors: []string{"timeout", "connection", "unavailable", "rate_limit"},
	}
}

// DefaultFallbackConfig returns a sensible default fallback configuration
func DefaultFallbackConfig() *FallbackConfig {
	return &FallbackConfig{
		Enabled:             true,
		MaxCostIncrease:     floatPtr(0.5), // Allow up to 50% cost increase
		RequireSameFeatures: true,
	}
}

// HighReliabilityConfig returns a configuration optimized for maximum reliability
func HighReliabilityConfig() (*RetryConfig, *FallbackConfig) {
	retryConfig := &RetryConfig{
		MaxAttempts:     5,
		BackoffType:     "exponential",
		BaseDelay:       "500ms",
		MaxDelay:        "60s",
		RetryableErrors: []string{"timeout", "connection", "unavailable", "rate_limit", "server_error"},
	}

	fallbackConfig := &FallbackConfig{
		Enabled:             true,
		MaxCostIncrease:     floatPtr(1.0), // Allow up to 100% cost increase for reliability
		RequireSameFeatures: false,          // Allow fallback to different feature sets
	}

	return retryConfig, fallbackConfig
}

// CostOptimizedConfig returns a configuration optimized for cost efficiency
func CostOptimizedConfig() (*RetryConfig, *FallbackConfig) {
	retryConfig := &RetryConfig{
		MaxAttempts:     2,
		BackoffType:     "linear",
		BaseDelay:       "2s",
		MaxDelay:        "10s",
		RetryableErrors: []string{"timeout", "connection"},
	}

	fallbackConfig := &FallbackConfig{
		Enabled:             true,
		PreferredChain:      []string{"openai", "anthropic"}, // Prefer cost-effective providers
		MaxCostIncrease:     floatPtr(0.2),                   // Allow only 20% cost increase
		RequireSameFeatures: true,
	}

	return retryConfig, fallbackConfig
}

// PerformanceOptimizedConfig returns a configuration optimized for speed
func PerformanceOptimizedConfig() (*RetryConfig, *FallbackConfig) {
	retryConfig := &RetryConfig{
		MaxAttempts:     2,
		BackoffType:     "linear",
		BaseDelay:       "100ms",
		MaxDelay:        "2s",
		RetryableErrors: []string{"timeout", "connection"},
	}

	fallbackConfig := &FallbackConfig{
		Enabled:             true,
		PreferredChain:      []string{"openai", "anthropic"}, // Prefer fast providers
		MaxCostIncrease:     floatPtr(0.3),                   // Allow moderate cost increase for speed
		RequireSameFeatures: true,
	}

	return retryConfig, fallbackConfig
}

func floatPtr(f float64) *float64 {
	return &f
}

// ContextStrategy defines how documents are retrieved and injected into agent context
type ContextStrategy string

const (
	ContextStrategyVector ContextStrategy = "vector" // Vector search RAG (default for Q&A/Conversational)
	ContextStrategyFull   ContextStrategy = "full"   // Full document injection (for Producer agents)
	ContextStrategyHybrid ContextStrategy = "hybrid" // Combined vector + full document sections
	ContextStrategyMCP    ContextStrategy = "mcp"    // MCP server integration for autonomous retrieval
	ContextStrategyNone   ContextStrategy = "none"   // No document context
)

// DocumentScope defines which documents to include in context
type DocumentScope string

const (
	DocumentScopeAll      DocumentScope = "all"      // All documents in notebook(s)
	DocumentScopeSelected DocumentScope = "selected" // Only selected documents
	DocumentScopeNone     DocumentScope = "none"     // No documents
)

// MultiPassConfig defines configuration for multi-pass document processing
type MultiPassConfig struct {
	Enabled           bool   `json:"enabled" gorm:"default:false"`
	SegmentSize       int    `json:"segment_size" gorm:"default:8000"`         // Tokens per segment
	OverlapTokens     int    `json:"overlap_tokens" gorm:"default:500"`        // Overlap between segments
	MaxPasses         int    `json:"max_passes" gorm:"default:10"`             // Maximum segments to process
	AggregationPrompt string `json:"aggregation_prompt,omitempty"`             // Custom prompt for aggregation
}

// DocumentContextConfig holds all document context settings for an agent
type DocumentContextConfig struct {
	Strategy            ContextStrategy  `json:"strategy" gorm:"default:'vector'"`
	Scope               DocumentScope    `json:"scope" gorm:"default:'all'"`
	DefaultDocuments    []uuid.UUID      `json:"default_documents,omitempty"`
	IncludeSubNotebooks bool             `json:"include_sub_notebooks" gorm:"default:false"`
	MaxContextTokens    int              `json:"max_context_tokens" gorm:"default:8000"`
	TopK                int              `json:"top_k" gorm:"default:10"`           // For vector search
	MinScore            float64          `json:"min_score" gorm:"default:0.7"`      // Minimum similarity score
	VectorWeight        float64          `json:"vector_weight" gorm:"default:0.5"`  // For hybrid search
	FullDocWeight       float64          `json:"full_doc_weight" gorm:"default:0.5"` // For hybrid search
	MultiPass           *MultiPassConfig `json:"multi_pass,omitempty"`
}

func (c DocumentContextConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *DocumentContextConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), c)
	}

	return json.Unmarshal(bytes, c)
}

type Agent struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name        string     `json:"name" gorm:"not null"`
	Description string     `json:"description"`

	SystemPrompt string `json:"system_prompt" gorm:"not null"`

	LLMConfig AgentLLMConfig `json:"llm_config" gorm:"type:jsonb;not null"`

	OwnerID  string `json:"owner_id" gorm:"type:varchar(255);not null"`
	SpaceID  string `json:"space_id" gorm:"type:varchar(255);not null"`
	TenantID string `json:"tenant_id" gorm:"type:varchar(255);not null"`

	Status    AgentStatus `json:"status" gorm:"type:varchar(50);not null;default:'draft'"`
	SpaceType SpaceType   `json:"space_type" gorm:"type:varchar(50);not null"`
	Type      AgentType   `json:"type" gorm:"type:varchar(50);not null;default:'conversational'"`

	IsPublic   bool `json:"is_public" gorm:"default:false"`
	IsTemplate bool `json:"is_template" gorm:"default:false"`
	IsInternal bool `json:"is_internal" gorm:"default:false"` // System agents available to all users

	NotebookIDs datatypes.JSON `json:"notebook_ids" gorm:"type:jsonb;default:'[]'"`

	// Document Context Configuration
	EnableKnowledge     bool                   `json:"enable_knowledge" gorm:"default:true"`
	EnableMemory        bool                   `json:"enable_memory" gorm:"default:true"`
	DocumentContext     *DocumentContextConfig `json:"document_context,omitempty" gorm:"type:jsonb"`

	Tags   datatypes.JSON `json:"tags" gorm:"type:jsonb;default:'[]'"`
	Skills datatypes.JSON `json:"skills" gorm:"type:jsonb;default:'[]'"` // Skill name strings

	TotalExecutions     int     `json:"total_executions" gorm:"default:0"`
	TotalCostUSD        float64 `json:"total_cost_usd" gorm:"type:decimal(10,6);default:0"`
	AvgResponseTimeMs   int     `json:"avg_response_time_ms" gorm:"default:0"`
	LastExecutedAt      *time.Time `json:"last_executed_at"`

	CreatedAt time.Time  `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"not null;default:now()"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

func (Agent) TableName() string {
	return "agent_builder.agents"
}

type CreateAgentRequest struct {
	Name         string         `json:"name" validate:"required,min=1,max=255"`
	Description  string         `json:"description" validate:"max=1000"`
	SystemPrompt string         `json:"system_prompt" validate:"required,min=1"`
	LLMConfig    AgentLLMConfig `json:"llm_config" validate:"required"`
	SpaceID      string         `json:"space_id" validate:"required"`
	SpaceType    SpaceType      `json:"space_type"` // "personal" or "organization" - defaults to "personal" if not specified
	Type         AgentType      `json:"type"`       // "conversational", "qa", or "producer" - defaults to "conversational"
	IsPublic     bool           `json:"is_public"`
	IsTemplate   bool           `json:"is_template"`
	IsInternal   bool           `json:"is_internal"` // System agents - only settable by system
	NotebookIDs  []uuid.UUID    `json:"notebook_ids"`
	Tags         []string       `json:"tags"`
	Skills       []string       `json:"skills"`

	// Document Context Configuration
	EnableKnowledge bool                   `json:"enable_knowledge"`
	EnableMemory    bool                   `json:"enable_memory"`
	DocumentContext *DocumentContextConfig `json:"document_context,omitempty"`
}

type UpdateAgentRequest struct {
	Name         *string         `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description  *string         `json:"description,omitempty" validate:"omitempty,max=1000"`
	SystemPrompt *string         `json:"system_prompt,omitempty" validate:"omitempty,min=1"`
	LLMConfig    *AgentLLMConfig `json:"llm_config,omitempty"`
	Status       *AgentStatus    `json:"status,omitempty"`
	Type         *AgentType      `json:"type,omitempty"`
	IsPublic     *bool           `json:"is_public,omitempty"`
	IsTemplate   *bool           `json:"is_template,omitempty"`
	IsInternal   *bool           `json:"is_internal,omitempty"` // System agents - only settable by system
	NotebookIDs  []uuid.UUID     `json:"notebook_ids,omitempty"`
	Tags         []string        `json:"tags,omitempty"`
	Skills       []string        `json:"skills,omitempty"`

	// Document Context Configuration
	EnableKnowledge *bool                  `json:"enable_knowledge,omitempty"`
	EnableMemory    *bool                  `json:"enable_memory,omitempty"`
	DocumentContext *DocumentContextConfig `json:"document_context,omitempty"`
}

type AgentListResponse struct {
	Agents []Agent `json:"agents"`
	Total  int64   `json:"total"`
	Page   int     `json:"page"`
	Size   int     `json:"size"`
}

type AgentListFilter struct {
	OwnerID    *uuid.UUID   `json:"owner_id"`
	SpaceID    *uuid.UUID   `json:"space_id"`
	TenantID   *uuid.UUID   `json:"tenant_id"`
	Status     *AgentStatus `json:"status"`
	SpaceType  *SpaceType   `json:"space_type"`
	IsPublic   *bool        `json:"is_public"`
	IsTemplate *bool        `json:"is_template"`
	IsInternal *bool        `json:"is_internal"` // Filter for system agents
	Tags       []string     `json:"tags"`
	Search     string       `json:"search"`
	Page       int          `json:"page"`
	Size       int          `json:"size"`
}