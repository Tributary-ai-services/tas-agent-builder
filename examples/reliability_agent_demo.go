package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
	"github.com/tas-agent-builder/services/impl"
)

func main() {
	fmt.Println("🔧 TAS Agent Builder - Reliability Features Demo")
	fmt.Println("=================================================")
	fmt.Println()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	dsn := cfg.GetDatabaseDSN()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test database connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("✅ Connected to database")

	// Create router service
	routerService := impl.NewRouterService(&cfg.Router)
	fmt.Println("✅ Router service initialized")

	// Test user and tenant
	userID := uuid.New()
	tenantID := "reliability-test-tenant"
	spaceID := uuid.New()

	// Demo 1: High Reliability Agent
	fmt.Println("\n🛡️  Demo 1: High Reliability Agent")
	fmt.Println("===================================")

	reliabilityAgent := createHighReliabilityAgent(userID, spaceID, tenantID)
	
	// Insert agent into database
	if err := insertAgent(db, reliabilityAgent); err != nil {
		log.Fatalf("Failed to create reliability agent: %v", err)
	}
	fmt.Printf("✅ High Reliability Agent created with ID: %s\n", reliabilityAgent.ID)

	// Test reliability features
	testReliabilityFeatures(routerService, reliabilityAgent, userID, db)

	// Demo 2: Cost Optimized Agent
	fmt.Println("\n💰 Demo 2: Cost Optimized Agent")
	fmt.Println("===============================")

	costAgent := createCostOptimizedAgent(userID, spaceID, tenantID)
	
	if err := insertAgent(db, costAgent); err != nil {
		log.Fatalf("Failed to create cost agent: %v", err)
	}
	fmt.Printf("✅ Cost Optimized Agent created with ID: %s\n", costAgent.ID)

	testCostOptimization(routerService, costAgent, userID, db)

	// Demo 3: Performance Agent
	fmt.Println("\n⚡ Demo 3: Performance Optimized Agent")
	fmt.Println("=====================================")

	perfAgent := createPerformanceAgent(userID, spaceID, tenantID)
	
	if err := insertAgent(db, perfAgent); err != nil {
		log.Fatalf("Failed to create performance agent: %v", err)
	}
	fmt.Printf("✅ Performance Agent created with ID: %s\n", perfAgent.ID)

	testPerformanceFeatures(routerService, perfAgent, userID, db)

	// Display comprehensive results
	fmt.Println("\n📊 Reliability Demo Summary")
	fmt.Println("===========================")
	displaySummary(db, []models.Agent{reliabilityAgent, costAgent, perfAgent})
}

func createHighReliabilityAgent(userID, spaceID uuid.UUID, tenantID string) models.Agent {
	retryConfig, fallbackConfig := models.HighReliabilityConfig()
	
	agent := models.Agent{
		ID:           uuid.New(),
		Name:         "High Reliability Agent",
		Description:  "Demonstrates maximum reliability with aggressive retry and fallback",
		SystemPrompt: `You are a highly reliable assistant focused on ensuring successful task completion. 
You should always provide helpful responses while the system ensures maximum uptime through 
advanced retry logic and provider fallback capabilities.`,
		LLMConfig: models.AgentLLMConfig{
			Provider:         "openai",
			Model:           "gpt-3.5-turbo",
			Temperature:     floatPtr(0.7),
			MaxTokens:       intPtr(200),
			OptimizeFor:     "reliability",
			RetryConfig:     retryConfig,
			FallbackConfig:  fallbackConfig,
			RequiredFeatures: []string{"chat_completions"},
			Metadata: map[string]any{
				"test_scenario": "high_reliability",
			},
		},
		OwnerID:     userID,
		SpaceID:     spaceID,
		TenantID:    tenantID,
		Status:      models.AgentStatusPublished,
		SpaceType:   models.SpaceTypePersonal,
		IsPublic:    true,
		IsTemplate:  false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	return agent
}

func createCostOptimizedAgent(userID, spaceID uuid.UUID, tenantID string) models.Agent {
	retryConfig, fallbackConfig := models.CostOptimizedConfig()
	
	agent := models.Agent{
		ID:           uuid.New(),
		Name:         "Cost Optimized Agent",
		Description:  "Demonstrates cost-efficient operation with conservative retry/fallback",
		SystemPrompt: `You are a cost-conscious assistant that provides quality responses while 
optimizing for cost efficiency. Keep responses concise and focused.`,
		LLMConfig: models.AgentLLMConfig{
			Provider:       "openai",
			Model:         "gpt-3.5-turbo",
			Temperature:   floatPtr(0.5),
			MaxTokens:     intPtr(150),
			OptimizeFor:   "cost",
			MaxCost:       floatPtr(0.01), // 1 cent per request
			RetryConfig:   retryConfig,
			FallbackConfig: fallbackConfig,
			Metadata: map[string]any{
				"test_scenario": "cost_optimized",
			},
		},
		OwnerID:     userID,
		SpaceID:     spaceID,
		TenantID:    tenantID,
		Status:      models.AgentStatusPublished,
		SpaceType:   models.SpaceTypePersonal,
		IsPublic:    true,
		IsTemplate:  false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	return agent
}

func createPerformanceAgent(userID, spaceID uuid.UUID, tenantID string) models.Agent {
	retryConfig, fallbackConfig := models.PerformanceOptimizedConfig()
	
	agent := models.Agent{
		ID:           uuid.New(),
		Name:         "Performance Agent",
		Description:  "Demonstrates speed-optimized operation with minimal latency",
		SystemPrompt: `You are a performance-focused assistant optimized for speed. 
Provide quick, accurate responses with minimal processing time.`,
		LLMConfig: models.AgentLLMConfig{
			Provider:       "openai",
			Model:         "gpt-3.5-turbo",
			Temperature:   floatPtr(0.3), // Lower temperature for faster responses
			MaxTokens:     intPtr(100),
			OptimizeFor:   "performance",
			RetryConfig:   retryConfig,
			FallbackConfig: fallbackConfig,
			RequiredFeatures: []string{"chat_completions"},
			Metadata: map[string]any{
				"test_scenario": "performance",
			},
		},
		OwnerID:     userID,
		SpaceID:     spaceID,
		TenantID:    tenantID,
		Status:      models.AgentStatusPublished,
		SpaceType:   models.SpaceTypePersonal,
		IsPublic:    true,
		IsTemplate:  false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	return agent
}

func testReliabilityFeatures(routerService services.RouterService, agent models.Agent, userID uuid.UUID, db *sql.DB) {
	fmt.Println("\n🔬 Testing Reliability Features:")
	
	testCases := []struct {
		name    string
		message string
	}{
		{"Basic Reliability", "Test the reliability features of this agent."},
		{"Complex Query", "Explain the benefits of retry logic and provider fallback in distributed systems."},
		{"Stress Test", "Handle this request even if providers are having issues."},
	}

	ctx := context.Background()
	
	for i, test := range testCases {
		fmt.Printf("\n   Test %d: %s\n", i+1, test.name)
		fmt.Printf("   👤 User: %s\n", test.message)
		
		messages := []services.Message{
			{
				Role:    "system",
				Content: agent.SystemPrompt,
			},
			{
				Role:    "user",
				Content: test.message,
			},
		}

		startTime := time.Now()
		response, err := routerService.SendRequest(ctx, agent.LLMConfig, messages, userID)
		responseTime := time.Since(startTime)

		if err != nil {
			fmt.Printf("   ❌ Error: %v\n", err)
			continue
		}

		fmt.Printf("   🤖 Agent: %s\n", response.Content)
		fmt.Printf("   📊 Reliability Stats:\n")
		fmt.Printf("      Provider: %s | Model: %s\n", response.Provider, response.Model)
		fmt.Printf("      Tokens: %d | Cost: $%.6f | Time: %dms\n", 
			response.TokenUsage, response.CostUSD, responseTime.Milliseconds())
		
		// Display enhanced reliability metadata if available
		if metadata := response.Metadata; metadata != nil {
			if retries, ok := metadata["retry_attempts"].(int); ok && retries > 0 {
				fmt.Printf("      🔄 Retry Attempts: %d\n", retries)
			}
			if fallback, ok := metadata["fallback_used"].(bool); ok && fallback {
				fmt.Printf("      🔀 Fallback Used: Yes\n")
			}
			if failed, ok := metadata["failed_providers"].([]string); ok && len(failed) > 0 {
				fmt.Printf("      ⚠️  Failed Providers: %v\n", failed)
			}
		}

		// Save execution with enhanced metadata
		saveExecution(db, agent.ID, userID, test.message, response, responseTime)
		
		time.Sleep(500 * time.Millisecond) // Brief pause between tests
	}
}

func testCostOptimization(routerService services.RouterService, agent models.Agent, userID uuid.UUID, db *sql.DB) {
	fmt.Println("\n🔬 Testing Cost Optimization:")
	
	messages := []services.Message{
		{
			Role:    "system",
			Content: agent.SystemPrompt,
		},
		{
			Role:    "user",
			Content: "Provide a cost-efficient response about cloud computing.",
		},
	}

	ctx := context.Background()
	startTime := time.Now()
	response, err := routerService.SendRequest(ctx, agent.LLMConfig, messages, userID)
	responseTime := time.Since(startTime)

	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
		return
	}

	fmt.Printf("   🤖 Agent: %s\n", response.Content)
	fmt.Printf("   💰 Cost Analysis:\n")
	fmt.Printf("      Cost: $%.6f | Target: $%.3f\n", response.CostUSD, *agent.LLMConfig.MaxCost)
	fmt.Printf("      Cost Efficiency: %.2f%%\n", (1.0 - response.CostUSD/(*agent.LLMConfig.MaxCost)) * 100)
	
	saveExecution(db, agent.ID, userID, "Cost optimization test", response, responseTime)
}

func testPerformanceFeatures(routerService services.RouterService, agent models.Agent, userID uuid.UUID, db *sql.DB) {
	fmt.Println("\n🔬 Testing Performance Features:")
	
	messages := []services.Message{
		{
			Role:    "system",
			Content: agent.SystemPrompt,
		},
		{
			Role:    "user",
			Content: "Quick response needed: What is 2+2?",
		},
	}

	ctx := context.Background()
	startTime := time.Now()
	response, err := routerService.SendRequest(ctx, agent.LLMConfig, messages, userID)
	responseTime := time.Since(startTime)

	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
		return
	}

	fmt.Printf("   🤖 Agent: %s\n", response.Content)
	fmt.Printf("   ⚡ Performance Metrics:\n")
	fmt.Printf("      Response Time: %dms\n", responseTime.Milliseconds())
	
	if metadata := response.Metadata; metadata != nil {
		if providerLatency, ok := metadata["provider_latency"].(int); ok {
			fmt.Printf("      Provider Latency: %dms\n", providerLatency)
		}
	}
	
	saveExecution(db, agent.ID, userID, "Performance test", response, responseTime)
}

func insertAgent(db *sql.DB, agent models.Agent) error {
	query := `
		INSERT INTO agent_builder.agents (
			id, name, description, system_prompt, llm_config,
			owner_id, space_id, tenant_id, status, space_type,
			is_public, is_template, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14
		) ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			system_prompt = EXCLUDED.system_prompt,
			llm_config = EXCLUDED.llm_config,
			updated_at = EXCLUDED.updated_at
	`

	_, err := db.Exec(query,
		agent.ID, agent.Name, agent.Description, agent.SystemPrompt, agent.LLMConfig,
		agent.OwnerID, agent.SpaceID, agent.TenantID, agent.Status, agent.SpaceType,
		agent.IsPublic, agent.IsTemplate, agent.CreatedAt, agent.UpdatedAt,
	)

	return err
}

func saveExecution(db *sql.DB, agentID, userID uuid.UUID, input string, response *services.RouterResponse, responseTime time.Duration) {
	executionID := uuid.New()
	
	inputData := map[string]interface{}{
		"message": input,
	}
	outputData := map[string]interface{}{
		"response": response.Content,
		"provider": response.Provider,
		"model":    response.Model,
		"tokens":   response.TokenUsage,
	}
	
	// Extract reliability metadata
	retryAttempts := 0
	fallbackUsed := false
	var failedProviders []string
	totalRetryTime := 0
	var providerLatency *int
	
	if metadata := response.Metadata; metadata != nil {
		if retries, ok := metadata["retry_attempts"].(int); ok {
			retryAttempts = retries
		}
		if fallback, ok := metadata["fallback_used"].(bool); ok {
			fallbackUsed = fallback
		}
		if failed, ok := metadata["failed_providers"].([]string); ok {
			failedProviders = failed
		}
		if retryTime, ok := metadata["total_retry_time"].(int); ok {
			totalRetryTime = retryTime
		}
		if latency, ok := metadata["provider_latency"].(int); ok {
			providerLatency = &latency
		}
	}
	
	inputJSON, _ := json.Marshal(inputData)
	outputJSON, _ := json.Marshal(outputData)
	failedProvidersJSON, _ := json.Marshal(failedProviders)
	
	executionQuery := `
		INSERT INTO agent_builder.agent_executions (
			id, agent_id, user_id, input_data, output_data,
			status, token_usage, cost_usd, total_duration_ms,
			retry_attempts, fallback_used, failed_providers,
			total_retry_time_ms, provider_latency_ms,
			started_at, completed_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14,
			$15, $16, $17, $18
		)
	`
	
	startedAt := time.Now().Add(-responseTime)
	completedAt := time.Now()
	
	_, err := db.Exec(executionQuery,
		executionID, agentID, userID, inputJSON, outputJSON,
		"completed", response.TokenUsage, response.CostUSD, responseTime.Milliseconds(),
		retryAttempts, fallbackUsed, failedProvidersJSON,
		totalRetryTime, providerLatency,
		startedAt, completedAt, time.Now(), time.Now(),
	)
	
	if err != nil {
		fmt.Printf("   ⚠️  Warning: Failed to save execution: %v\n", err)
	}
}

func displaySummary(db *sql.DB, agents []models.Agent) {
	for _, agent := range agents {
		fmt.Printf("\n📈 %s Summary:\n", agent.Name)
		
		var totalExecutions int
		var totalCost float64
		var avgResponseTime float64
		var avgRetryAttempts float64
		var fallbackUsageRate float64
		
		statsQuery := `
			SELECT 
				COUNT(*) as total_executions,
				COALESCE(SUM(cost_usd), 0) as total_cost,
				COALESCE(AVG(total_duration_ms), 0) as avg_response_time,
				COALESCE(AVG(retry_attempts), 0) as avg_retry_attempts,
				COALESCE(AVG(CASE WHEN fallback_used THEN 1.0 ELSE 0.0 END), 0) as fallback_rate
			FROM agent_builder.agent_executions
			WHERE agent_id = $1
		`
		
		row := db.QueryRow(statsQuery, agent.ID)
		err := row.Scan(&totalExecutions, &totalCost, &avgResponseTime, &avgRetryAttempts, &fallbackUsageRate)
		if err != nil {
			fmt.Printf("   ⚠️  Could not retrieve statistics: %v\n", err)
			continue
		}
		
		fmt.Printf("   Executions: %d\n", totalExecutions)
		fmt.Printf("   Total Cost: $%.6f\n", totalCost)
		fmt.Printf("   Avg Response Time: %.0fms\n", avgResponseTime)
		fmt.Printf("   Avg Retry Attempts: %.2f\n", avgRetryAttempts)
		fmt.Printf("   Fallback Usage Rate: %.2f%%\n", fallbackUsageRate * 100)
		
		// Calculate reliability score
		successRate := 1.0 // Assume 100% for demo
		reliabilityPenalty := avgRetryAttempts * 0.1 + fallbackUsageRate * 0.05
		reliabilityScore := successRate * (1.0 - reliabilityPenalty)
		fmt.Printf("   Reliability Score: %.4f\n", reliabilityScore)
	}
}

func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}