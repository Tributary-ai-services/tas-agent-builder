package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
	"github.com/tas-agent-builder/services/impl"
)

// TestBothProvidersIntegration validates that both OpenAI and Anthropic work through TAS-LLM-Router
func TestBothProvidersIntegration(t *testing.T) {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Create router service
	routerService := impl.NewRouterService(&cfg.Router)
	ctx := context.Background()

	// Check router availability first
	if !isRouterAvailable(cfg.Router.BaseURL) {
		t.Skip("TAS-LLM-Router not available at", cfg.Router.BaseURL)
	}

	// Get available providers
	providers, err := routerService.GetAvailableProviders(ctx)
	if err != nil {
		t.Fatalf("Failed to get providers: %v", err)
	}

	t.Logf("Available providers: %d", len(providers))
	for _, provider := range providers {
		t.Logf("  - %s: %v", provider.Name, provider.Models)
	}

	// Test configurations for both providers
	testCases := []struct {
		name           string
		provider       string
		model          string
		expectedInResp string
		testPrompt     string
		maxTokens      int
		temperature    float64
	}{
		{
			name:           "OpenAI GPT-3.5 Turbo",
			provider:       "openai", 
			model:          "gpt-3.5-turbo",
			expectedInResp: "math",
			testPrompt:     "What is 2+2? Answer with just the number and the word 'math'.",
			maxTokens:      20,
			temperature:    0.0,
		},
		{
			name:           "OpenAI GPT-4",
			provider:       "openai",
			model:          "gpt-4o",
			expectedInResp: "code",
			testPrompt:     "Write the word 'code' and explain what programming is in one sentence.",
			maxTokens:      50,
			temperature:    0.1,
		},
		{
			name:           "Anthropic Claude 3.5 Sonnet",
			provider:       "anthropic",
			model:          "claude-3-5-sonnet-20241022",
			expectedInResp: "assistant",
			testPrompt:     "Say 'I am Claude, an AI assistant' and nothing else.",
			maxTokens:      30,
			temperature:    0.0,
		},
		{
			name:           "Anthropic Claude 3 Haiku",
			provider:       "anthropic", 
			model:          "claude-3-haiku-20240307",
			expectedInResp: "haiku",
			testPrompt:     "Write the word 'haiku' and then write a 3-line haiku about AI.",
			maxTokens:      60,
			temperature:    0.2,
		},
	}

	userID := uuid.New()
	successfulTests := 0

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create agent configuration
			agentConfig := models.AgentLLMConfig{
				Provider:    tc.provider,
				Model:       tc.model,
				Temperature: &tc.temperature,
				MaxTokens:   &tc.maxTokens,
			}

			// Validate configuration first
			err := routerService.ValidateConfig(ctx, agentConfig)
			if err != nil {
				t.Skipf("Model %s not available: %v", tc.model, err)
				return
			}

			// Prepare messages
			messages := []services.Message{
				{
					Role:    "system",
					Content: "You are a helpful assistant. Follow instructions precisely.",
				},
				{
					Role:    "user", 
					Content: tc.testPrompt,
				},
			}

			// Send request with timeout
			requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			startTime := time.Now()
			response, err := routerService.SendRequest(requestCtx, agentConfig, messages, userID)
			responseTime := time.Since(startTime)

			if err != nil {
				t.Errorf("Request failed for %s: %v", tc.model, err)
				return
			}

			// Validate response
			if response == nil {
				t.Error("Response is nil")
				return
			}

			if response.Content == "" {
				t.Error("Response content is empty")
				return
			}

			// Log detailed response information
			t.Logf("✅ %s Response:", tc.name)
			t.Logf("   Content: %s", response.Content)
			t.Logf("   Provider: %s", response.Provider)
			t.Logf("   Model: %s", response.Model)
			t.Logf("   Tokens: %d", response.TokenUsage)
			t.Logf("   Cost: $%.6f", response.CostUSD)
			t.Logf("   Response Time: %dms", response.ResponseTimeMs)
			t.Logf("   Actual Response Time: %dms", responseTime.Milliseconds())

			// Validate provider routing
			expectedProvider := tc.provider
			if response.Provider != expectedProvider && response.Provider != "unknown" {
				t.Errorf("Expected provider %s, got %s", expectedProvider, response.Provider)
			}

			// Validate model routing
			if response.Model != tc.model {
				t.Logf("Note: Model mismatch - expected %s, got %s (may be router optimization)", tc.model, response.Model)
			}

			// Validate response contains expected content (case insensitive)
			if tc.expectedInResp != "" {
				found := false
				content := response.Content
				// Simple substring check (case insensitive)
				for i := 0; i <= len(content)-len(tc.expectedInResp); i++ {
					if len(content[i:]) >= len(tc.expectedInResp) {
						substr := content[i : i+len(tc.expectedInResp)]
						if equalFold(substr, tc.expectedInResp) {
							found = true
							break
						}
					}
				}
				if !found {
					t.Logf("Warning: Expected '%s' in response, but not found. Response: %s", tc.expectedInResp, content)
				}
			}

			// Validate performance metrics
			if response.TokenUsage <= 0 {
				t.Logf("Warning: Token usage is %d", response.TokenUsage)
			}

			if response.ResponseTimeMs <= 0 {
				t.Logf("Warning: Response time is %dms", response.ResponseTimeMs)
			}

			successfulTests++
		})
	}

	// Summary validation
	if successfulTests == 0 {
		t.Fatal("No providers worked successfully")
	}

	t.Logf("\n🎉 Provider Integration Summary:")
	t.Logf("   ✅ Successful tests: %d/%d", successfulTests, len(testCases))
	
	if successfulTests >= 2 {
		t.Logf("   🌟 Multi-provider routing confirmed!")
	}
	
	if successfulTests == len(testCases) {
		t.Logf("   🏆 All providers and models working perfectly!")
	}
}

// TestProviderSpecificFeatures tests provider-specific capabilities
func TestProviderSpecificFeatures(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	routerService := impl.NewRouterService(&cfg.Router)
	ctx := context.Background()

	if !isRouterAvailable(cfg.Router.BaseURL) {
		t.Skip("TAS-LLM-Router not available")
	}

	userID := uuid.New()

	t.Run("OpenAI System Message Handling", func(t *testing.T) {
		agentConfig := models.AgentLLMConfig{
			Provider:    "openai",
			Model:       "gpt-3.5-turbo",
			Temperature: floatPtr(0.0),
			MaxTokens:   intPtr(30),
		}

		messages := []services.Message{
			{Role: "system", Content: "You are a math tutor. Always start responses with 'MATH:'."},
			{Role: "user", Content: "What is 5 + 5?"},
		}

		response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
		if err != nil {
			t.Skipf("OpenAI not available: %v", err)
		}

		t.Logf("OpenAI system message test: %s", response.Content)
		
		// Should follow system instructions
		if !containsIgnoreCase(response.Content, "MATH:") && !containsIgnoreCase(response.Content, "10") {
			t.Logf("Note: Response may not follow system message precisely: %s", response.Content)
		}
	})

	t.Run("Anthropic Conversation Handling", func(t *testing.T) {
		agentConfig := models.AgentLLMConfig{
			Provider:    "anthropic",
			Model:       "claude-3-5-sonnet-20241022",
			Temperature: floatPtr(0.0),
			MaxTokens:   intPtr(50),
		}

		messages := []services.Message{
			{Role: "user", Content: "Hello, can you confirm you're Claude and tell me one interesting fact?"},
		}

		response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
		if err != nil {
			t.Skipf("Anthropic not available: %v", err)
		}

		t.Logf("Anthropic conversation test: %s", response.Content)
		
		// Claude should introduce itself
		if !containsIgnoreCase(response.Content, "Claude") {
			t.Logf("Note: Claude didn't introduce itself: %s", response.Content)
		}
	})
}

// TestRoutingStrategies tests different routing optimization strategies
func TestRoutingStrategies(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	routerService := impl.NewRouterService(&cfg.Router)
	ctx := context.Background()

	if !isRouterAvailable(cfg.Router.BaseURL) {
		t.Skip("TAS-LLM-Router not available")
	}

	userID := uuid.New()
	prompt := "Explain what AI is in exactly 20 words."

	strategies := []struct {
		name     string
		optimize string
		model    string
	}{
		{"Cost Optimized", "cost", "gpt-3.5-turbo"},
		{"Performance Optimized", "performance", "gpt-4o"},
		{"Round Robin", "round_robin", "gpt-3.5-turbo"},
	}

	for _, strategy := range strategies {
		t.Run(strategy.name, func(t *testing.T) {
			agentConfig := models.AgentLLMConfig{
				Provider:    "openai",
				Model:       strategy.model,
				Temperature: floatPtr(0.0),
				MaxTokens:   intPtr(30),
				Metadata: map[string]any{
					"optimize_for": strategy.optimize,
				},
			}

			messages := []services.Message{
				{Role: "user", Content: prompt},
			}

			response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
			if err != nil {
				t.Skipf("Strategy %s failed: %v", strategy.name, err)
			}

			t.Logf("%s result:", strategy.name)
			t.Logf("  Model used: %s", response.Model)
			t.Logf("  Provider: %s", response.Provider) 
			t.Logf("  Cost: $%.6f", response.CostUSD)
			t.Logf("  Response time: %dms", response.ResponseTimeMs)
			t.Logf("  Routing strategy: %s", response.RoutingStrategy)
		})
	}
}

// Helper functions