package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
	"github.com/tas-agent-builder/services/impl"
)

func main() {
	fmt.Println("TAS Agent Builder - Router Integration Example")
	fmt.Println("==============================================")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create router service
	routerService := impl.NewRouterService(&cfg.Router)

	// Test basic connectivity
	fmt.Println("\n1. Testing Router Connectivity...")
	ctx := context.Background()
	
	providers, err := routerService.GetAvailableProviders(ctx)
	if err != nil {
		fmt.Printf("❌ Failed to connect to router: %v\n", err)
		fmt.Println("\nMake sure TAS-LLM-Router is running on:", cfg.Router.BaseURL)
		os.Exit(1)
	}

	fmt.Printf("✅ Connected to router at %s\n", cfg.Router.BaseURL)
	fmt.Printf("📋 Available providers: %d\n", len(providers))
	
	for _, provider := range providers {
		fmt.Printf("   - %s (%s) - %d models\n", provider.DisplayName, provider.Name, len(provider.Models))
	}

	// Test agent configuration validation
	fmt.Println("\n2. Testing Agent Configuration...")
	
	agentConfig := models.AgentLLMConfig{
		Provider:    "openai",
		Model:       "gpt-3.5-turbo",
		Temperature: float64Ptr(0.7),
		MaxTokens:   intPtr(150),
	}

	err = routerService.ValidateConfig(ctx, agentConfig)
	if err != nil {
		fmt.Printf("❌ Agent config validation failed: %v\n", err)
	} else {
		fmt.Printf("✅ Agent configuration is valid\n")
		fmt.Printf("   Provider: %s, Model: %s\n", agentConfig.Provider, agentConfig.Model)
	}

	// Test simple query
	fmt.Println("\n3. Testing Simple Query...")
	
	messages := []services.Message{
		{
			Role:    "system",
			Content: "You are a helpful assistant for TAS Agent Builder testing.",
		},
		{
			Role:    "user",
			Content: "Say hello and confirm you're working with TAS Agent Builder. Keep it brief.",
		},
	}

	userID := uuid.New()
	response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
	if err != nil {
		fmt.Printf("❌ Query failed: %v\n", err)
	} else {
		fmt.Printf("✅ Query successful!\n")
		fmt.Printf("   Response: %s\n", response.Content)
		fmt.Printf("   Provider: %s\n", response.Provider)
		fmt.Printf("   Model: %s\n", response.Model)
		fmt.Printf("   Tokens: %d\n", response.TokenUsage)
		fmt.Printf("   Cost: $%.6f\n", response.CostUSD)
		fmt.Printf("   Response Time: %dms\n", response.ResponseTimeMs)
	}

	// Test agent-like interaction
	fmt.Println("\n4. Testing Agent-like Interaction...")
	
	codeReviewConfig := models.AgentLLMConfig{
		Provider:    "openai",
		Model:       "gpt-4o",
		Temperature: float64Ptr(0.2),
		MaxTokens:   intPtr(300),
		Metadata: map[string]any{
			"optimize_for": "performance",
		},
	}

	agentMessages := []services.Message{
		{
			Role: "system",
			Content: `You are a code review assistant. Analyze code for:
1. Code quality and best practices
2. Security vulnerabilities
3. Performance optimizations
Provide concise, actionable feedback.`,
		},
		{
			Role: "user",
			Content: `Review this Go function:

func ProcessUser(db *sql.DB, userID string) error {
    query := "SELECT * FROM users WHERE id = " + userID
    rows, err := db.Query(query)
    if err != nil {
        return err
    }
    defer rows.Close()
    return nil
}`,
		},
	}

	agentResponse, err := routerService.SendRequest(ctx, codeReviewConfig, agentMessages, userID)
	if err != nil {
		fmt.Printf("❌ Agent query failed: %v\n", err)
	} else {
		fmt.Printf("✅ Agent query successful!\n")
		fmt.Printf("   Response: %s\n", agentResponse.Content)
		fmt.Printf("   Provider: %s\n", agentResponse.Provider)
		fmt.Printf("   Routing: %s\n", agentResponse.RoutingStrategy)
		fmt.Printf("   Tokens: %d\n", agentResponse.TokenUsage)
		fmt.Printf("   Cost: $%.6f\n", agentResponse.CostUSD)
	}

	fmt.Println("\n✅ Router integration test completed successfully!")
	fmt.Println("\nNext Steps:")
	fmt.Println("- Integrate this router service into agent execution")
	fmt.Println("- Add conversation memory and context")
	fmt.Println("- Implement knowledge retrieval integration")
	fmt.Println("- Add streaming support for real-time responses")
}

func float64Ptr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}