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
	fmt.Println("🤖 TAS Agent Builder - Hello Agent Demo")
	fmt.Println("=========================================")
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

	// Create a test user and tenant
	userID := uuid.New()
	tenantID := "test-tenant-001"
	spaceID := uuid.New()

	// Create Hello Agent configuration
	helloAgent := models.Agent{
		ID:           uuid.New(),
		Name:         "Hello Agent",
		Description:  "A friendly agent that greets users and demonstrates TAS-LLM-Router integration",
		SystemPrompt: `You are a friendly and helpful assistant called "Hello Agent". 
Your purpose is to:
1. Greet users warmly
2. Answer simple questions
3. Demonstrate that the TAS Agent Builder is working correctly
Always be positive and encouraging. Keep responses brief and friendly.`,
		LLMConfig: models.AgentLLMConfig{
			Provider:    "openai",
			Model:       "gpt-3.5-turbo",
			Temperature: floatPtr(0.7),
			MaxTokens:   intPtr(150),
			Metadata: map[string]any{
				"optimize_for": "cost",
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

	// Insert agent into database
	fmt.Println("\n📝 Creating Hello Agent in database...")
	
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

	_, err = db.Exec(query,
		helloAgent.ID,
		helloAgent.Name,
		helloAgent.Description,
		helloAgent.SystemPrompt,
		helloAgent.LLMConfig,
		helloAgent.OwnerID,
		helloAgent.SpaceID,
		helloAgent.TenantID,
		helloAgent.Status,
		helloAgent.SpaceType,
		helloAgent.IsPublic,
		helloAgent.IsTemplate,
		helloAgent.CreatedAt,
		helloAgent.UpdatedAt,
	)

	if err != nil {
		log.Fatalf("Failed to create agent in database: %v", err)
	}

	fmt.Printf("✅ Agent created with ID: %s\n", helloAgent.ID)
	fmt.Printf("   Name: %s\n", helloAgent.Name)
	fmt.Printf("   Provider: %s\n", helloAgent.LLMConfig.Provider)
	fmt.Printf("   Model: %s\n", helloAgent.LLMConfig.Model)

	// Test agent execution
	fmt.Println("\n🚀 Testing Hello Agent Execution...")
	fmt.Println("=====================================")

	// Simulate conversation
	conversations := []string{
		"Hello! Who are you?",
		"Can you help me test the TAS Agent Builder?",
		"What is 2 + 2?",
		"Tell me a joke about AI agents",
	}

	ctx := context.Background()
	
	for i, userMessage := range conversations {
		fmt.Printf("\n👤 User: %s\n", userMessage)
		
		// Prepare messages for the agent
		messages := []services.Message{
			{
				Role:    "system",
				Content: helloAgent.SystemPrompt,
			},
			{
				Role:    "user",
				Content: userMessage,
			},
		}

		// Send request to router
		startTime := time.Now()
		response, err := routerService.SendRequest(ctx, helloAgent.LLMConfig, messages, userID)
		responseTime := time.Since(startTime)

		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}

		fmt.Printf("🤖 Agent: %s\n", response.Content)
		fmt.Printf("   📊 Stats: Provider=%s, Model=%s, Tokens=%d, Cost=$%.6f, Time=%dms\n",
			response.Provider,
			response.Model,
			response.TokenUsage,
			response.CostUSD,
			responseTime.Milliseconds(),
		)

		// Create execution record
		executionID := uuid.New()
		executionQuery := `
			INSERT INTO agent_builder.agent_executions (
				id, agent_id, user_id, input_data, output_data,
				status, token_usage, cost_usd, total_duration_ms,
				started_at, completed_at, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8, $9,
				$10, $11, $12, $13
			)
		`

		inputData := map[string]interface{}{
			"message": userMessage,
		}
		outputData := map[string]interface{}{
			"response": response.Content,
			"provider": response.Provider,
			"model":    response.Model,
			"tokens":   response.TokenUsage,
		}
		
		inputJSON, _ := json.Marshal(inputData)
		outputJSON, _ := json.Marshal(outputData)
		
		_, err = db.Exec(executionQuery,
			executionID,
			helloAgent.ID,
			userID,
			inputJSON,
			outputJSON,
			"completed",
			response.TokenUsage,
			response.CostUSD,
			responseTime.Milliseconds(),
			startTime,
			time.Now(),
			time.Now(),
			time.Now(),
		)

		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to save execution record: %v\n", err)
		} else {
			fmt.Printf("   💾 Execution saved with ID: %s\n", executionID)
		}

		// Small delay between messages
		if i < len(conversations)-1 {
			time.Sleep(1 * time.Second)
		}
	}

	// Show agent statistics
	fmt.Println("\n📈 Agent Statistics")
	fmt.Println("===================")
	
	var totalExecutions int
	var totalCost float64
	var avgResponseTime float64

	statsQuery := `
		SELECT 
			COUNT(*) as total_executions,
			COALESCE(SUM(cost_usd), 0) as total_cost,
			COALESCE(AVG(total_duration_ms), 0) as avg_response_time
		FROM agent_builder.agent_executions
		WHERE agent_id = $1
	`

	row := db.QueryRow(statsQuery, helloAgent.ID)
	err = row.Scan(&totalExecutions, &totalCost, &avgResponseTime)
	if err != nil {
		fmt.Printf("⚠️  Could not retrieve statistics: %v\n", err)
	} else {
		fmt.Printf("   Total Executions: %d\n", totalExecutions)
		fmt.Printf("   Total Cost: $%.6f\n", totalCost)
		fmt.Printf("   Average Response Time: %.0fms\n", avgResponseTime)
	}

	fmt.Println("\n✨ Hello Agent Demo Complete!")
	fmt.Println("==============================")
	fmt.Printf("Agent ID: %s\n", helloAgent.ID)
	fmt.Println("\nThe Hello Agent has been created and tested successfully!")
	fmt.Println("You can now use this agent ID to test API endpoints and frontend integration.")
}

func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}