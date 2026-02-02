package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ProviderStats map[string]int
type ModelStats map[string]int

func (p ProviderStats) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *ProviderStats) Scan(value interface{}) error {
	if value == nil {
		*p = make(ProviderStats)
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), p)
	}
	
	return json.Unmarshal(bytes, p)
}

func (m ModelStats) Value() (driver.Value, error) {
	return json.Marshal(m)
}

func (m *ModelStats) Scan(value interface{}) error {
	if value == nil {
		*m = make(ModelStats)
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), m)
	}
	
	return json.Unmarshal(bytes, m)
}

type AgentUsageStats struct {
	AgentID uuid.UUID `json:"agent_id" gorm:"type:uuid;primary_key"`
	
	TotalExecutions     int `json:"total_executions" gorm:"default:0"`
	SuccessfulExecutions int `json:"successful_executions" gorm:"default:0"`
	FailedExecutions    int `json:"failed_executions" gorm:"default:0"`
	
	TotalCostUSD        float64 `json:"total_cost_usd" gorm:"type:decimal(10,6);default:0"`
	TotalTokensUsed     int64   `json:"total_tokens_used" gorm:"default:0"`
	AvgCostPerExecution float64 `json:"avg_cost_per_execution" gorm:"type:decimal(10,6);default:0"`
	
	AvgResponseTimeMs int `json:"avg_response_time_ms" gorm:"default:0"`
	MinResponseTimeMs *int `json:"min_response_time_ms,omitempty"`
	MaxResponseTimeMs *int `json:"max_response_time_ms,omitempty"`
	P95ResponseTimeMs *int `json:"p95_response_time_ms,omitempty"`
	
	ExecutionsToday     int     `json:"executions_today" gorm:"default:0"`
	ExecutionsThisWeek  int     `json:"executions_this_week" gorm:"default:0"`
	ExecutionsThisMonth int     `json:"executions_this_month" gorm:"default:0"`
	
	CostToday     float64 `json:"cost_today" gorm:"type:decimal(8,6);default:0"`
	CostThisWeek  float64 `json:"cost_this_week" gorm:"type:decimal(8,6);default:0"`
	CostThisMonth float64 `json:"cost_this_month" gorm:"type:decimal(8,6);default:0"`
	
	SuccessRate float64 `json:"success_rate" gorm:"type:decimal(5,4);default:0"`
	ErrorRate   float64 `json:"error_rate" gorm:"type:decimal(5,4);default:0"`
	
	AvgKnowledgeItemsUsed float64 `json:"avg_knowledge_items_used" gorm:"type:decimal(8,2);default:0"`
	AvgMemoryItemsUsed    float64 `json:"avg_memory_items_used" gorm:"type:decimal(8,2);default:0"`
	
	ProviderUsageStats ProviderStats `json:"provider_usage_stats" gorm:"type:jsonb;default:'{}'"`
	ModelUsageStats    ModelStats    `json:"model_usage_stats" gorm:"type:jsonb;default:'{}'"`
	
	LastExecutionAt       *time.Time `json:"last_execution_at,omitempty"`
	StatsLastUpdatedAt    time.Time  `json:"stats_last_updated_at" gorm:"not null;default:now()"`
	DailyStatsResetAt     time.Time  `json:"daily_stats_reset_at" gorm:"type:date;default:current_date"`
	WeeklyStatsResetAt    time.Time  `json:"weekly_stats_reset_at" gorm:"type:date;default:current_date"`
	MonthlyStatsResetAt   time.Time  `json:"monthly_stats_reset_at" gorm:"type:date;default:current_date"`
	
	Agent *Agent `json:"agent,omitempty" gorm:"foreignKey:AgentID"`
}

func (AgentUsageStats) TableName() string {
	return "ab_agent_usage_stats"
}

type StatsResponse struct {
	AgentID uuid.UUID `json:"agent_id"`
	
	ExecutionStats struct {
		Total      int     `json:"total"`
		Successful int     `json:"successful"`
		Failed     int     `json:"failed"`
		SuccessRate float64 `json:"success_rate"`
		ErrorRate   float64 `json:"error_rate"`
	} `json:"execution_stats"`
	
	CostStats struct {
		TotalUSD        float64 `json:"total_usd"`
		AvgPerExecution float64 `json:"avg_per_execution"`
		TodayUSD        float64 `json:"today_usd"`
		ThisWeekUSD     float64 `json:"this_week_usd"`
		ThisMonthUSD    float64 `json:"this_month_usd"`
	} `json:"cost_stats"`
	
	PerformanceStats struct {
		AvgResponseMs int  `json:"avg_response_ms"`
		MinResponseMs *int `json:"min_response_ms"`
		MaxResponseMs *int `json:"max_response_ms"`
		P95ResponseMs *int `json:"p95_response_ms"`
	} `json:"performance_stats"`
	
	UsageStats struct {
		TotalTokens     int64 `json:"total_tokens"`
		ExecutionsToday int   `json:"executions_today"`
		ExecutionsWeek  int   `json:"executions_week"`
		ExecutionsMonth int   `json:"executions_month"`
	} `json:"usage_stats"`
	
	ProviderBreakdown ProviderStats `json:"provider_breakdown"`
	ModelBreakdown    ModelStats    `json:"model_breakdown"`
	
	LastExecutionAt    *time.Time `json:"last_execution_at"`
	StatsLastUpdatedAt time.Time  `json:"stats_last_updated_at"`
}