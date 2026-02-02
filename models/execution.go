package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type ExecutionStatus string

const (
	ExecutionStatusQueued     ExecutionStatus = "queued"
	ExecutionStatusRunning    ExecutionStatus = "running"
	ExecutionStatusCompleted  ExecutionStatus = "completed"
	ExecutionStatusFailed     ExecutionStatus = "failed"
	ExecutionStatusTimeout    ExecutionStatus = "timeout"
	ExecutionStatusCancelled  ExecutionStatus = "cancelled"
)

type ExecutionStep struct {
	Step        string            `json:"step"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Status      ExecutionStatus   `json:"status"`
	Output      string            `json:"output,omitempty"`
	Error       string            `json:"error,omitempty"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

func (s ExecutionStep) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *ExecutionStep) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), s)
	}

	return json.Unmarshal(bytes, s)
}

// ExecutionStepList is a custom type for GORM to properly handle JSONB array of ExecutionStep
type ExecutionStepList []ExecutionStep

func (e ExecutionStepList) Value() (driver.Value, error) {
	if e == nil {
		return json.Marshal([]ExecutionStep{})
	}
	return json.Marshal(e)
}

func (e *ExecutionStepList) Scan(value interface{}) error {
	if value == nil {
		*e = ExecutionStepList{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		strVal, ok := value.(string)
		if !ok {
			*e = ExecutionStepList{}
			return nil
		}
		bytes = []byte(strVal)
	}

	var steps []ExecutionStep
	if err := json.Unmarshal(bytes, &steps); err != nil {
		*e = ExecutionStepList{}
		return nil // Don't fail on unmarshal errors, just use empty list
	}
	*e = steps
	return nil
}

type RouterResponse struct {
	Provider        string         `json:"provider"`
	Model           string         `json:"model"`
	RoutingStrategy string         `json:"routing_strategy"`
	TokenUsage      int            `json:"token_usage"`
	CostUSD         float64        `json:"cost_usd"`
	ResponseTimeMs  int            `json:"response_time_ms"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

func (r RouterResponse) Value() (driver.Value, error) {
	return json.Marshal(r)
}

func (r *RouterResponse) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), r)
	}
	
	return json.Unmarshal(bytes, r)
}

type AgentExecution struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	AgentID  uuid.UUID `json:"agent_id" gorm:"type:uuid;not null;index"`
	UserID   uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
	
	SessionID *string `json:"session_id,omitempty" gorm:"index"`
	
	InputData  datatypes.JSON `json:"input_data" gorm:"type:jsonb;not null"`
	OutputData datatypes.JSON `json:"output_data,omitempty" gorm:"type:jsonb"`
	
	Status ExecutionStatus `json:"status" gorm:"type:varchar(50);not null;default:'queued';index"`
	
	RouterResponse *RouterResponse `json:"router_response,omitempty" gorm:"type:jsonb"`
	
	ExecutionSteps ExecutionStepList `json:"execution_steps,omitempty" gorm:"type:jsonb;default:'[]'"`
	
	TokenUsage      *int     `json:"token_usage,omitempty"`
	CostUSD         *float64 `json:"cost_usd,omitempty" gorm:"type:decimal(10,6)"`
	TotalDurationMs *int     `json:"total_duration_ms,omitempty"`
	
	// Enhanced reliability metadata
	RetryAttempts       int             `json:"retry_attempts" gorm:"default:0"`
	FallbackUsed        bool            `json:"fallback_used" gorm:"default:false"`
	FailedProviders     datatypes.JSON  `json:"failed_providers,omitempty" gorm:"type:jsonb;default:'[]'"`
	TotalRetryTimeMs    int             `json:"total_retry_time_ms" gorm:"default:0"`
	ProviderLatencyMs   *int            `json:"provider_latency_ms,omitempty"`
	RoutingReason       datatypes.JSON  `json:"routing_reason,omitempty" gorm:"type:jsonb;default:'[]'"`
	ActualCostUSD       *float64        `json:"actual_cost_usd,omitempty" gorm:"type:decimal(10,6)"`
	EstimatedCostUSD    *float64        `json:"estimated_cost_usd,omitempty" gorm:"type:decimal(10,6)"`
	
	ErrorMessage   *string `json:"error_message,omitempty"`
	ErrorDetails   datatypes.JSON `json:"error_details,omitempty" gorm:"type:jsonb"`
	
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	
	CreatedAt time.Time  `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"not null;default:now()"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
	
	Agent *Agent `json:"agent,omitempty" gorm:"foreignKey:AgentID"`
}

func (AgentExecution) TableName() string {
	return "ab_agent_executions"
}

type StartExecutionRequest struct {
	AgentID   uuid.UUID      `json:"agent_id" validate:"required"`
	SessionID *string        `json:"session_id,omitempty"`
	InputData map[string]any `json:"input_data" validate:"required"`
}

type ExecutionResponse struct {
	ID     uuid.UUID       `json:"id"`
	Status ExecutionStatus `json:"status"`
	Output map[string]any  `json:"output,omitempty"`
	Error  *string         `json:"error,omitempty"`
	// Enhanced reliability information
	ReliabilityMetrics *ReliabilityMetrics `json:"reliability_metrics,omitempty"`
}

// ReliabilityMetrics provides detailed reliability information for an execution
type ReliabilityMetrics struct {
	RetryAttempts       int      `json:"retry_attempts"`
	FallbackUsed        bool     `json:"fallback_used"`
	FailedProviders     []string `json:"failed_providers,omitempty"`
	TotalRetryTimeMs    int      `json:"total_retry_time_ms"`
	ProviderLatencyMs   *int     `json:"provider_latency_ms,omitempty"`
	RoutingReason       []string `json:"routing_reason,omitempty"`
	ActualCostUSD       *float64 `json:"actual_cost_usd,omitempty"`
	EstimatedCostUSD    *float64 `json:"estimated_cost_usd,omitempty"`
	ReliabilityScore    *float64 `json:"reliability_score,omitempty"`
}

type ExecutionListResponse struct {
	Executions []AgentExecution `json:"executions"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	Size       int              `json:"size"`
}

type ExecutionListFilter struct {
	AgentID   *uuid.UUID       `json:"agent_id"`
	UserID    *uuid.UUID       `json:"user_id"`
	SessionID *string          `json:"session_id"`
	Status    *ExecutionStatus `json:"status"`
	StartDate *time.Time       `json:"start_date"`
	EndDate   *time.Time       `json:"end_date"`
	// Enhanced filtering options
	WithRetries    *bool `json:"with_retries,omitempty"`    // Filter executions that had retries
	WithFallback   *bool `json:"with_fallback,omitempty"`   // Filter executions that used fallback
	MinReliability *float64 `json:"min_reliability,omitempty"` // Filter by minimum reliability score
	Page           int   `json:"page"`
	Size           int   `json:"size"`
}