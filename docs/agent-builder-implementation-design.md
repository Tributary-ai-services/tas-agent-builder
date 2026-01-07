# Agent Builder Implementation Design (Revised)

## Executive Summary

This document outlines the implementation design for the Agent Builder component of the TAS Agent Platform. The Agent Builder extends the existing Aether-BE backend, leverages Audimodal's processing capabilities, and integrates with TAS-LLM-Router for intelligent LLM execution to create an AI-assisted agent creation and management system.

## Current Architecture Integration

### Existing Foundation
- **Aether-BE**: User management, notebook system, document processing
- **Audimodal**: Event-driven processing, classification, workflow engine
- **TAS-LLM-Router**: Production-ready multi-provider LLM routing with cost optimization, health monitoring, streaming, authentication, and intelligent provider selection
- **Neo4j**: Graph database for relationships and organizational intelligence
- **Space-based multi-tenancy**: Personal and organizational spaces

### Integration Strategy
The Agent Builder will extend the existing models and services while leveraging TAS-LLM-Router's production-ready capabilities for LLM provider management, cost optimization, and intelligent routing. This approach eliminates the need to rebuild LLM infrastructure and focuses the Agent Builder on agent-specific logic: memory management, knowledge retrieval, context assembly, and response processing.

## Core Data Models

### Agent Model Extension

```go
// Agent represents an AI agent in the system
type Agent struct {
    // Base identification
    ID          string `json:"id" validate:"required,uuid"`
    Name        string `json:"name" validate:"required,min=1,max=255"`
    Description string `json:"description,omitempty" validate:"max=1000"`
    Version     string `json:"version" validate:"required"`
    
    // Agent configuration
    Type        AgentType          `json:"type" validate:"required"`
    Status      AgentStatus        `json:"status" validate:"required"`
    Config      AgentConfiguration `json:"config" validate:"required"`
    
    // Relationships (extending existing patterns)
    OwnerID  string    `json:"owner_id" validate:"required,uuid"`
    SpaceID  string    `json:"space_id" validate:"required,uuid"`
    TenantID string    `json:"tenant_id" validate:"required"`
    TeamID   string    `json:"team_id,omitempty" validate:"omitempty,uuid"`
    
    // Knowledge base integration
    NotebookIDs      []string              `json:"notebook_ids,omitempty"`
    DocumentFilters  []DocumentFilter      `json:"document_filters,omitempty"`
    KnowledgeConfig  KnowledgeConfiguration `json:"knowledge_config"`
    
    // LLM routing configuration (for TAS-LLM-Router)
    LLMConfig    AgentLLMConfig    `json:"llm_config" validate:"required"`
    ToolsConfig  []ToolConfiguration `json:"tools_config,omitempty"`     // Agent-specific tools
    MCPServices  []MCPServiceConfig  `json:"mcp_services,omitempty"`      // MCP services (via router when available)
    
    // Memory and state management
    MemoryConfig MemoryConfiguration `json:"memory_config"`
    StateConfig  StateConfiguration  `json:"state_config"`
    
    // Performance and analytics
    PerformanceMetrics AgentMetrics        `json:"performance_metrics,omitempty"`
    UsageStats        AgentUsageStats     `json:"usage_stats,omitempty"`
    
    // Collaboration features
    Visibility    string   `json:"visibility" validate:"required,oneof=private shared public"`
    Tags          []string `json:"tags,omitempty"`
    Collaborators []AgentCollaborator `json:"collaborators,omitempty"`
    
    // Search optimization
    SearchText string `json:"search_text,omitempty"`
    
    // Standard timestamps
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// AgentType defines the different types of agents
type AgentType string

const (
    AgentTypeQA            AgentType = "qa"              // Q&A agents
    AgentTypeConversational AgentType = "conversational" // Conversational agents with memory
    AgentTypeProducer      AgentType = "producer"        // Content/artifact generation
    AgentTypeWorkflow      AgentType = "workflow"        // Multi-step workflow agents
    AgentTypeAPI          AgentType = "api"             // External API integration
    AgentTypeHybrid       AgentType = "hybrid"          // Combination of built-in + custom
)

// AgentStatus defines agent lifecycle states
type AgentStatus string

const (
    AgentStatusDraft     AgentStatus = "draft"     // Being configured
    AgentStatusActive    AgentStatus = "active"    // Ready for use
    AgentStatusTesting   AgentStatus = "testing"   // In testing phase
    AgentStatusInactive  AgentStatus = "inactive"  // Temporarily disabled
    AgentStatusArchived  AgentStatus = "archived"  // Archived but preserved
    AgentStatusFailed    AgentStatus = "failed"    // Configuration or runtime failure
)

// AgentConfiguration holds the core agent behavior configuration
type AgentConfiguration struct {
    // Intent and behavior
    SystemPrompt     string                 `json:"system_prompt" validate:"required"`
    UserPrompt       string                 `json:"user_prompt,omitempty"`
    Instructions     []string               `json:"instructions,omitempty"`
    Constraints      []string               `json:"constraints,omitempty"`
    
    // Response configuration
    ResponseFormat   ResponseFormat         `json:"response_format"`
    MaxTokens        int                   `json:"max_tokens,omitempty"`
    Temperature      float64               `json:"temperature,omitempty"`
    TopP            float64                `json:"top_p,omitempty"`
    
    // Execution parameters
    TimeoutSeconds   int                   `json:"timeout_seconds,omitempty"`
    MaxRetries       int                   `json:"max_retries,omitempty"`
    ErrorHandling    ErrorHandlingStrategy  `json:"error_handling"`
    
    // Advanced features
    StreamingEnabled bool                  `json:"streaming_enabled"`
    CachingEnabled   bool                  `json:"caching_enabled"`
    LoggingLevel     string                `json:"logging_level"`
    
    // Custom metadata
    CustomProperties map[string]interface{} `json:"custom_properties,omitempty"`
}

// KnowledgeConfiguration defines how agents access knowledge
type KnowledgeConfiguration struct {
    // Vector search configuration
    EnableVectorSearch   bool    `json:"enable_vector_search"`
    VectorSearchK        int     `json:"vector_search_k,omitempty"`
    SimilarityThreshold  float64 `json:"similarity_threshold,omitempty"`
    
    // Document filtering
    DocumentTypes       []string `json:"document_types,omitempty"`
    DateRangeFilter     *DateRange `json:"date_range_filter,omitempty"`
    TagFilters          []string `json:"tag_filters,omitempty"`
    
    // Knowledge integration
    UsePersonalNotebooks bool     `json:"use_personal_notebooks"`
    UseTeamNotebooks     bool     `json:"use_team_notebooks"`
    UsePublicNotebooks   bool     `json:"use_public_notebooks"`
    
    // Processing configuration
    MaxDocuments        int      `json:"max_documents,omitempty"`
    RerankResults       bool     `json:"rerank_results"`
    IncludeMetadata     bool     `json:"include_metadata"`
}

// AgentLLMConfig defines routing preferences for TAS-LLM-Router integration
// All provider management, authentication, cost optimization, and failover
// are handled by TAS-LLM-Router, not the agent layer
type AgentLLMConfig struct {
    // TAS-LLM-Router integration
    RouterURL       string `json:"router_url" validate:"required,url"`
    RouterAPIKey    string `json:"router_api_key,omitempty"` // For router authentication
    
    // Model and routing preferences
    PreferredModel  string  `json:"preferred_model" validate:"required"`  // e.g., "gpt-4o", "claude-3-5-sonnet" 
    RoutingStrategy string  `json:"routing_strategy"`                      // "cost", "performance", "round_robin"
    
    // Agent-specific LLM behavior
    SystemPrompt    string  `json:"system_prompt" validate:"required"`
    Temperature     float64 `json:"temperature,omitempty"`                 // 0.0-2.0
    MaxTokens       int     `json:"max_tokens,omitempty"`                  // Max response tokens
    TopP           float64 `json:"top_p,omitempty"`                       // 0.0-1.0
    
    // Response configuration
    ResponseFormat  *ResponseFormat `json:"response_format,omitempty"`      // JSON schema for structured output
    StreamingEnabled bool           `json:"streaming_enabled"`                // Default: true
    
    // Agent execution constraints
    MaxCostPerRequest float64 `json:"max_cost_per_request,omitempty"`       // Cost limit per request
    TimeoutSeconds    int     `json:"timeout_seconds,omitempty"`            // Request timeout
    
    // Error handling
    MaxRetries     int    `json:"max_retries,omitempty"`                   // Retry on failures
    RetryDelay     string `json:"retry_delay,omitempty"`                   // Delay between retries
}

// MemoryConfiguration defines agent memory behavior
type MemoryConfiguration struct {
    // Memory types
    EnableShortTermMemory bool `json:"enable_short_term_memory"`
    EnableLongTermMemory  bool `json:"enable_long_term_memory"`
    EnableWorkingMemory   bool `json:"enable_working_memory"`
    
    // Retention policies
    ShortTermRetentionHours int `json:"short_term_retention_hours,omitempty"`
    LongTermRetentionDays   int `json:"long_term_retention_days,omitempty"`
    MaxMemoryEntries        int `json:"max_memory_entries,omitempty"`
    
    // Memory processing
    MemoryRetrievalK       int     `json:"memory_retrieval_k,omitempty"`
    MemorySimilarityThreshold float64 `json:"memory_similarity_threshold,omitempty"`
    AutoSummarization      bool    `json:"auto_summarization"`
    
    // Privacy and security
    EncryptMemory          bool     `json:"encrypt_memory"`
    PurgeOnDeactivation    bool     `json:"purge_on_deactivation"`
    DataLocalityRules      []string `json:"data_locality_rules,omitempty"`
}
```

### Agent Execution and Runtime Models

```go
// AgentExecution represents a single agent execution
type AgentExecution struct {
    ID              string    `json:"id" validate:"required,uuid"`
    AgentID         string    `json:"agent_id" validate:"required,uuid"`
    UserID          string    `json:"user_id" validate:"required,uuid"`
    SessionID       string    `json:"session_id,omitempty"`
    
    // Request and response
    Input           AgentInput           `json:"input" validate:"required"`
    Output          *AgentOutput         `json:"output,omitempty"`
    Status          ExecutionStatus      `json:"status" validate:"required"`
    
    // Execution chain tracking
    ExecutionChain  []ExecutionStep      `json:"execution_chain,omitempty"`
    TotalDuration   time.Duration        `json:"total_duration"`
    
    // Performance tracking
    TokenUsage      TokenUsage           `json:"token_usage,omitempty"`
    CostUSD         float64              `json:"cost_usd,omitempty"`
    
    // Error handling
    Error           *ExecutionError      `json:"error,omitempty"`
    Warnings        []string             `json:"warnings,omitempty"`
    
    // Timestamps
    StartedAt       time.Time            `json:"started_at"`
    CompletedAt     *time.Time           `json:"completed_at,omitempty"`
}

// AgentInput represents input to an agent
type AgentInput struct {
    Message         string                 `json:"message" validate:"required"`
    Context         map[string]interface{} `json:"context,omitempty"`
    Documents       []string               `json:"document_ids,omitempty"`
    Parameters      map[string]interface{} `json:"parameters,omitempty"`
    
    // Execution options
    StreamResponse  bool                   `json:"stream_response"`
    IncludeChain    bool                   `json:"include_execution_chain"`
    MaxTokens       int                    `json:"max_tokens,omitempty"`
    Temperature     float64                `json:"temperature,omitempty"`
}

// ExecutionStep tracks individual steps in agent processing
type ExecutionStep struct {
    StepType        string                 `json:"step_type"`        // memory_retrieval, vector_search, llm_invocation, etc.
    StartTime       time.Time              `json:"start_time"`
    Duration        time.Duration          `json:"duration"`
    Success         bool                   `json:"success"`
    
    // Step-specific details
    Details         map[string]interface{} `json:"details,omitempty"`
    TokensUsed      int                    `json:"tokens_used,omitempty"`
    CostUSD         float64                `json:"cost_usd,omitempty"`
    
    // Tracing information
    TraceID         string                 `json:"trace_id,omitempty"`
    SpanID          string                 `json:"span_id,omitempty"`
    
    // Error information
    Error           string                 `json:"error,omitempty"`
    Warnings        []string               `json:"warnings,omitempty"`
}
```

## Service Architecture

### Agent Service Layer

```go
// AgentService defines the main agent management interface
type AgentService interface {
    // Agent CRUD operations
    CreateAgent(ctx context.Context, req *AgentCreateRequest) (*Agent, error)
    GetAgent(ctx context.Context, agentID string) (*Agent, error)
    UpdateAgent(ctx context.Context, agentID string, req *AgentUpdateRequest) (*Agent, error)
    DeleteAgent(ctx context.Context, agentID string) error
    ListAgents(ctx context.Context, req *AgentListRequest) (*AgentListResponse, error)
    
    // Agent execution
    ExecuteAgent(ctx context.Context, agentID string, input *AgentInput) (*AgentExecution, error)
    ExecuteAgentStreaming(ctx context.Context, agentID string, input *AgentInput) (<-chan AgentStreamChunk, error)
    
    // Agent testing and validation
    TestAgent(ctx context.Context, agentID string, testCases []TestCase) (*TestResults, error)
    ValidateAgentConfig(ctx context.Context, config *AgentConfiguration) (*ValidationResult, error)
    
    // Agent analytics and optimization
    GetAgentMetrics(ctx context.Context, agentID string, timeRange TimeRange) (*AgentMetrics, error)
    GetOptimizationSuggestions(ctx context.Context, agentID string) (*OptimizationSuggestions, error)
    
    // Collaboration features
    ShareAgent(ctx context.Context, agentID string, shareReq *ShareRequest) error
    CloneAgent(ctx context.Context, agentID string, cloneReq *CloneRequest) (*Agent, error)
    
    // Agent marketplace
    PublishAgent(ctx context.Context, agentID string, publishReq *PublishRequest) error
    SearchMarketplace(ctx context.Context, searchReq *MarketplaceSearchRequest) (*MarketplaceSearchResponse, error)
}

// AgentExecutionEngine handles agent-specific execution logic
// LLM provider management is delegated to TAS-LLM-Router
type AgentExecutionEngine interface {
    // Core execution using TAS-LLM-Router
    Execute(ctx context.Context, agent *Agent, input *AgentInput) (*AgentOutput, error)
    
    // Agent-specific operations (not LLM provider management)
    RetrieveKnowledge(ctx context.Context, agent *Agent, query string) ([]KnowledgeItem, error)
    RetrieveMemory(ctx context.Context, agentID string, query string) ([]MemoryItem, error)
    StoreMemory(ctx context.Context, agentID string, item MemoryItem) error
    
    // Context assembly and response processing
    AssembleContext(ctx context.Context, agent *Agent, input *AgentInput) (*AgentContext, error)
    ProcessResponse(ctx context.Context, response *RouterResponse, context *AgentContext) (*AgentOutput, error)
    
    // TAS-LLM-Router integration
    CallLLMRouter(ctx context.Context, request *RouterRequest) (*RouterResponse, error)
}

// RouterRequest represents a request to TAS-LLM-Router
type RouterRequest struct {
    Model       string          `json:"model"`                          // Model preference
    Messages    []RouterMessage `json:"messages"`                       // Conversation messages
    Temperature float64         `json:"temperature,omitempty"`          // Sampling temperature
    MaxTokens   int             `json:"max_tokens,omitempty"`           // Max response tokens
    Stream      bool            `json:"stream"`                         // Enable streaming
    OptimizeFor string          `json:"optimize_for,omitempty"`         // "cost", "performance", "round_robin"
    Tools       []RouterTool    `json:"tools,omitempty"`                // Available tools (when router supports)
}

// RouterResponse represents a response from TAS-LLM-Router
type RouterResponse struct {
    ID            string            `json:"id"`
    Provider      string            `json:"provider"`                     // Which provider was used
    Model         string            `json:"model"`                        // Actual model used
    Choices       []RouterChoice    `json:"choices"`
    Usage         *RouterUsage      `json:"usage,omitempty"`
    CostUSD       float64           `json:"cost_usd,omitempty"`           // Cost from router
    RoutingReason string            `json:"routing_reason,omitempty"`     // Why this provider was chosen
    Created       time.Time         `json:"created"`
}

// AgentContext holds assembled context for LLM execution
type AgentContext struct {
    Agent          *Agent
    Input          *AgentInput
    KnowledgeItems []KnowledgeItem
    MemoryItems    []MemoryItem
    SessionID      string
}
```

### AI Assistant Integration

Building on the high-level design's AI-assisted agent building concept:

```go
// AgentAssistantService provides AI-powered agent building assistance
type AgentAssistantService interface {
    // Intent analysis and suggestions
    AnalyzeIntent(ctx context.Context, userIntent string) (*IntentAnalysis, error)
    SuggestAgentConfig(ctx context.Context, analysis *IntentAnalysis, userID string) (*AgentSuggestion, error)
    
    // Knowledge base recommendations
    SuggestNotebooks(ctx context.Context, intent string, userID string) ([]NotebookSuggestion, error)
    SuggestTools(ctx context.Context, intent string) ([]ToolSuggestion, error)
    
    // Configuration optimization
    OptimizePrompt(ctx context.Context, currentPrompt string, performance *AgentMetrics) (*PromptOptimization, error)
    SuggestImprovements(ctx context.Context, agentID string) (*ImprovementSuggestions, error)
    
    // Collaborative intelligence
    FindSimilarAgents(ctx context.Context, agentID string, userID string) ([]SimilarAgentSuggestion, error)
    GetOrganizationalPatterns(ctx context.Context, userID string) (*OrganizationalPatterns, error)
}

// IntentAnalysis represents the analysis of user intent
type IntentAnalysis struct {
    PrimaryIntent    string              `json:"primary_intent"`
    SecondaryIntents []string            `json:"secondary_intents"`
    DomainCategory   string              `json:"domain_category"`
    Complexity       IntentComplexity    `json:"complexity"`
    
    // Extracted requirements
    RequiredCapabilities []string         `json:"required_capabilities"`
    SuggestedAgentType   AgentType        `json:"suggested_agent_type"`
    EstimatedParameters  map[string]interface{} `json:"estimated_parameters"`
    
    // Confidence and alternatives
    Confidence       float64             `json:"confidence"`
    Alternatives     []IntentAlternative `json:"alternatives,omitempty"`
}

// AgentSuggestion provides comprehensive agent configuration suggestions
type AgentSuggestion struct {
    RecommendedConfig   *AgentConfiguration      `json:"recommended_config"`
    LLMRecommendations  []LLMRecommendation      `json:"llm_recommendations"`
    ToolSuggestions     []ToolSuggestion         `json:"tool_suggestions"`
    KnowledgeSuggestions []KnowledgeSuggestion   `json:"knowledge_suggestions"`
    
    // Justification and confidence
    Reasoning          string                    `json:"reasoning"`
    Confidence         float64                   `json:"confidence"`
    EstimatedCost      *CostEstimate            `json:"estimated_cost,omitempty"`
}
```

## Neo4j Graph Extensions

Extending the existing Neo4j schema to support agent relationships and organizational intelligence:

```cypher
// Agent nodes and relationships
(:Agent {id, name, type, owner_id, space_id, tenant_id, created_at, updated_at, version, status})
(:AgentExecution {id, agent_id, user_id, status, duration, cost_usd, created_at})
(:AgentVersion {id, agent_id, version, config_hash, created_at})

// Agent relationships
(user:User)-[:CREATED {timestamp}]->(agent:Agent)
(user:User)-[:USES {frequency, last_used, satisfaction_rating}]->(agent:Agent)
(user:User)-[:COLLABORATES_ON {role, permissions}]->(agent:Agent)

// Knowledge relationships
(agent:Agent)-[:SEARCHES_NOTEBOOK {relevance_score, usage_count}]->(notebook:Notebook)
(agent:Agent)-[:USES_DOCUMENT {relevance_score, last_used}]->(document:Document)

// Tool and service relationships
(agent:Agent)-[:USES_TOOL {configuration, success_rate}]->(tool:Tool)
(agent:Agent)-[:USES_MCP_SERVICE {configuration, reliability_score}]->(mcp:MCPService)

// LLM relationships
(agent:Agent)-[:CONFIGURED_FOR {primary, fallback_order}]->(llm:LLMProvider)

// Success patterns
(agent:Agent)-[:SUCCEEDED_FOR {use_case, success_rate, evidence}]->(pattern:SuccessPattern)
(pattern:SuccessPattern)-[:INVOLVES_TOOL]->(tool:Tool)
(pattern:SuccessPattern)-[:USES_KNOWLEDGE]->(notebook:Notebook)

// Organizational learning
(team:Team)-[:HAS_PATTERN {success_rate, usage_frequency}]->(pattern:SuccessPattern)
(dept:Department)-[:SHARES_KNOWLEDGE {access_level}]->(agent:Agent)
```

## Event-Driven Integration

Leveraging Audimodal's existing event system for agent operations:

```go
// Agent-specific event types
const (
    EventAgentCreated           = "agent.created"
    EventAgentUpdated           = "agent.updated"
    EventAgentExecutionStarted  = "agent.execution.started"
    EventAgentExecutionCompleted = "agent.execution.completed"
    EventAgentExecutionFailed   = "agent.execution.failed"
    EventAgentShared            = "agent.shared"
    EventAgentCloned            = "agent.cloned"
    EventAgentPublished         = "agent.published"
    EventAgentOptimized         = "agent.optimized"
)

// AgentExecutionEvent extends the base event system
type AgentExecutionEvent struct {
    events.BaseEvent
    Data AgentExecutionEventData `json:"data"`
}

type AgentExecutionEventData struct {
    AgentID         string                 `json:"agent_id"`
    ExecutionID     string                 `json:"execution_id"`
    UserID          string                 `json:"user_id"`
    Duration        time.Duration          `json:"duration"`
    TokenUsage      TokenUsage             `json:"token_usage"`
    CostUSD         float64                `json:"cost_usd"`
    Success         bool                   `json:"success"`
    ExecutionChain  []ExecutionStep        `json:"execution_chain"`
    Performance     PerformanceMetrics     `json:"performance"`
}
```

## Implementation Phases

### Phase 1: Core Agent Management (Month 1-2)
- Extend existing models with Agent entities
- Implement basic CRUD operations for agents
- Create agent configuration management
- Basic execution engine without advanced features

### Phase 2: Knowledge Integration (Month 2-3)  
- Integrate with existing notebook/document system
- Implement vector search for knowledge retrieval
- Memory management system
- Basic LLM integration

### Phase 3: AI Assistant Features (Month 3-4)
- Intent analysis and configuration suggestions
- Tool and notebook recommendations
- Performance optimization suggestions
- Basic collaborative features

### Phase 4: Advanced Features (Month 4-5)
- Multi-LLM testing framework
- Agent marketplace and sharing
- Advanced analytics and insights
- Organizational intelligence features

## TAS-LLM-Router Integration Details

### HTTP Client Integration

The Agent Builder integrates with TAS-LLM-Router via HTTP API calls, leveraging all existing router capabilities:

```go
// AgentLLMClient handles communication with TAS-LLM-Router
type AgentLLMClient struct {
    httpClient   *http.Client
    routerURL    string
    routerAPIKey string
}

func (c *AgentLLMClient) ExecuteAgent(ctx context.Context, agent *Agent, input *AgentInput) (*RouterResponse, error) {
    // Assemble context from knowledge and memory
    context, err := c.assembleContext(ctx, agent, input)
    if err != nil {
        return nil, err
    }
    
    // Build messages for TAS-LLM-Router
    messages := []RouterMessage{
        {
            Role:    "system",
            Content: c.buildSystemPrompt(agent, context),
        },
    }
    
    // Add conversation history from memory
    for _, memory := range context.MemoryItems {
        if memory.Type == MemoryTypeConversation {
            messages = append(messages, RouterMessage{Role: "user", Content: memory.Content})
            messages = append(messages, RouterMessage{Role: "assistant", Content: memory.Response})
        }
    }
    
    // Add current user input
    messages = append(messages, RouterMessage{
        Role:    "user",
        Content: input.Message,
    })
    
    // Prepare request for TAS-LLM-Router  
    routerReq := RouterRequest{
        Model:       agent.LLMConfig.PreferredModel,
        Messages:    messages,
        Temperature: agent.LLMConfig.Temperature,
        MaxTokens:   agent.LLMConfig.MaxTokens,
        Stream:      agent.LLMConfig.StreamingEnabled,
        OptimizeFor: agent.LLMConfig.RoutingStrategy, // Let router handle cost optimization
    }
    
    // Call TAS-LLM-Router
    return c.callRouter(ctx, routerReq)
}

func (c *AgentLLMClient) callRouter(ctx context.Context, req RouterRequest) (*RouterResponse, error) {
    reqBody, _ := json.Marshal(req)
    httpReq, err := http.NewRequestWithContext(ctx, "POST", 
        c.routerURL+"/v1/chat/completions", bytes.NewReader(reqBody))
    if err != nil {
        return nil, err
    }
    
    // Use TAS-LLM-Router's authentication
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("X-API-Key", c.routerAPIKey)
    
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("router returned status %d", resp.StatusCode)
    }
    
    var routerResponse RouterResponse
    if err := json.NewDecoder(resp.Body).Decode(&routerResponse); err != nil {
        return nil, err
    }
    
    return &routerResponse, nil
}
```

### Benefits of Router Integration

1. **Zero Provider Management**: Agents don't need to handle OpenAI, Anthropic, or other provider APIs directly
2. **Automatic Cost Optimization**: Router's `optimize_for: "cost"` handles provider selection for cost efficiency  
3. **Built-in Failover**: Router's health monitoring provides automatic failover between providers
4. **Enterprise Security**: Leverage router's existing JWT/API key authentication and rate limiting
5. **Performance Monitoring**: Router provides response times, error rates, and provider health metrics
6. **Streaming Support**: Router's native streaming capabilities work seamlessly with agent responses
7. **Future Tool Support**: When router implements tool calling, agents can leverage it immediately

### Configuration Management

Agent LLM configuration focuses only on routing preferences, not provider management:

```yaml
# Agent configuration example
agent:
  llm_config:
    router_url: "http://llm-router:8080"
    router_api_key: "${LLM_ROUTER_API_KEY}"
    preferred_model: "gpt-4o"
    routing_strategy: "cost"  # Let router optimize for cost
    system_prompt: "You are a helpful AI assistant..."
    temperature: 0.7
    max_tokens: 1000
    streaming_enabled: true
    max_cost_per_request: 0.10  # Agent-level cost control
```

### Error Handling and Retries

Router handles provider-level errors and retries. Agents handle application-level errors:

```go
func (e *agentExecutionEngine) executeWithRetry(ctx context.Context, agent *Agent, input *AgentInput) (*AgentOutput, error) {
    maxRetries := agent.LLMConfig.MaxRetries
    if maxRetries == 0 {
        maxRetries = 3
    }
    
    for attempt := 1; attempt <= maxRetries; attempt++ {
        response, err := e.llmClient.ExecuteAgent(ctx, agent, input)
        
        if err == nil {
            return e.ProcessResponse(ctx, response, context)
        }
        
        // Router handles provider errors; agents handle application errors
        if isNonRetryableError(err) {
            return nil, err
        }
        
        if attempt < maxRetries {
            delay := time.Duration(attempt) * time.Second
            time.Sleep(delay)
        }
    }
    
    return nil, fmt.Errorf("agent execution failed after %d attempts", maxRetries)
}
```

## API Specifications

### Agent Management Endpoints

```http
POST /api/v1/agents
GET /api/v1/agents/{agentId}
PUT /api/v1/agents/{agentId}
DELETE /api/v1/agents/{agentId}
GET /api/v1/agents

POST /api/v1/agents/{agentId}/execute
POST /api/v1/agents/{agentId}/execute/stream
POST /api/v1/agents/{agentId}/test

GET /api/v1/agents/{agentId}/metrics
GET /api/v1/agents/{agentId}/suggestions
POST /api/v1/agents/{agentId}/share
POST /api/v1/agents/{agentId}/clone
```

### Agent Assistant Endpoints

```http
POST /api/v1/agent-assistant/analyze-intent
POST /api/v1/agent-assistant/suggest-config
GET /api/v1/agent-assistant/notebook-suggestions
GET /api/v1/agent-assistant/tool-suggestions
GET /api/v1/agent-assistant/similar-agents
```

## Security and Compliance

### Access Control
- Extend existing space-based access control to agents
- Role-based permissions for agent creation, execution, and sharing
- API key management for external tool integrations

### Data Privacy
- Leverage existing DLP and classification systems
- Encrypted storage for sensitive agent configurations
- Audit trails for all agent operations

### Compliance Integration
- Use Audimodal's existing compliance frameworks
- Automated compliance checking for agent configurations
- Data locality and retention policy enforcement

## Performance and Scalability

### Caching Strategy
- Agent configuration caching
- LLM response caching for repeated queries
- Memory retrieval optimization

### Resource Management
- Execution quotas and rate limiting
- Cost tracking and budget controls
- Auto-scaling for high-demand agents

### Monitoring and Observability
- Extend existing metrics collection
- Distributed tracing for agent executions
- Performance bottleneck identification

## Success Metrics

### Technical Metrics
- Agent execution latency < 3 seconds (95th percentile)
- Agent creation time < 2 minutes (average)
- System availability > 99.9%
- Cost optimization > 20% through intelligent routing

### User Experience Metrics
- Time to first working agent < 10 minutes
- Agent builder adoption rate > 80%
- User satisfaction score > 4.5/5.0
- Collaboration frequency > 30%

This implementation design builds upon the existing solid foundation while introducing the advanced AI agent capabilities outlined in the high-level design document.