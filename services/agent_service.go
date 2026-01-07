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
}

type ExecutionService interface {
	StartExecution(ctx context.Context, req models.StartExecutionRequest, userID uuid.UUID) (*models.AgentExecution, error)
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
	ValidateConfig(ctx context.Context, config models.AgentLLMConfig) error
	GetAvailableProviders(ctx context.Context) ([]Provider, error)
	GetProviderModels(ctx context.Context, provider string) ([]Model, error)
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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