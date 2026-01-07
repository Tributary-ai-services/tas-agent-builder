package test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
	"github.com/tas-agent-builder/services/impl"
)

// TestExecutionEngineBasic tests basic execution functionality
func TestExecutionEngineBasic(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isRouterAvailable(cfg.Router.BaseURL) {
		t.Skip("TAS-LLM-Router not available")
	}

	routerService := impl.NewRouterService(&cfg.Router)
	ctx := context.Background()
	userID := uuid.New()

	t.Run("Single Execution Success", func(t *testing.T) {
		agentConfig := models.AgentLLMConfig{
			Provider:    "openai",
			Model:      "gpt-3.5-turbo",
			Temperature: floatPtr(0.0),
			MaxTokens:   intPtr(50),
		}

		messages := []services.Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant. Be concise.",
			},
			{
				Role:    "user",
				Content: "What is 2+2? Answer with just the number.",
			},
		}

		startTime := time.Now()
		response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
		executionTime := time.Since(startTime)

		require.NoError(t, err, "Basic execution should succeed")
		assert.NotNil(t, response, "Response should not be nil")
		assert.NotEmpty(t, response.Content, "Response content should not be empty")
		assert.Greater(t, response.TokenUsage, 0, "Token usage should be recorded")
		assert.Greater(t, response.CostUSD, 0.0, "Cost should be recorded")
		assert.Less(t, executionTime, 30*time.Second, "Execution should complete within 30 seconds")

		t.Logf("✅ Basic execution completed successfully")
		t.Logf("   Response: %s", response.Content)
		t.Logf("   Tokens: %d", response.TokenUsage)
		t.Logf("   Cost: $%.6f", response.CostUSD)
		t.Logf("   Time: %dms", executionTime.Milliseconds())
	})

	t.Run("Execution with Retry Configuration", func(t *testing.T) {
		agentConfig := models.AgentLLMConfig{
			Provider:    "openai",
			Model:      "gpt-3.5-turbo",
			Temperature: floatPtr(0.0),
			MaxTokens:   intPtr(30),
			RetryConfig: &models.RetryConfig{
				MaxAttempts:     3,
				BackoffType:     "exponential",
				BaseDelay:       "1s",
				MaxDelay:        "10s",
				RetryableErrors: []string{"timeout", "connection"},
			},
		}

		messages := []services.Message{
			{Role: "user", Content: "Count to 3."},
		}

		response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
		require.NoError(t, err, "Execution with retry config should succeed")

		// Check for retry metadata
		if metadata := response.Metadata; metadata != nil {
			if retryAttempts, ok := metadata["retry_attempts"]; ok {
				t.Logf("   Retry attempts: %v", retryAttempts)
			}
			if totalRetryTime, ok := metadata["total_retry_time"]; ok {
				t.Logf("   Total retry time: %v", totalRetryTime)
			}
		}

		t.Logf("✅ Execution with retry configuration completed")
		t.Logf("   Response: %s", response.Content)
	})

	t.Run("Execution with Fallback Configuration", func(t *testing.T) {
		agentConfig := models.AgentLLMConfig{
			Provider:    "openai",
			Model:      "gpt-3.5-turbo",
			OptimizeFor: "reliability",
			FallbackConfig: &models.FallbackConfig{
				Enabled:             true,
				PreferredChain:      []string{"anthropic", "openai"},
				MaxCostIncrease:     floatPtr(0.5),
				RequireSameFeatures: true,
			},
		}

		messages := []services.Message{
			{Role: "user", Content: "Hello, please introduce yourself briefly."},
		}

		response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
		require.NoError(t, err, "Execution with fallback config should succeed")

		// Check for fallback metadata
		if metadata := response.Metadata; metadata != nil {
			if fallbackUsed, ok := metadata["fallback_used"]; ok {
				t.Logf("   Fallback used: %v", fallbackUsed)
			}
			if failedProviders, ok := metadata["failed_providers"]; ok {
				t.Logf("   Failed providers: %v", failedProviders)
			}
		}

		t.Logf("✅ Execution with fallback configuration completed")
		t.Logf("   Provider: %s", response.Provider)
		t.Logf("   Model: %s", response.Model)
	})
}

// TestExecutionEngineMetadata tests comprehensive metadata collection
func TestExecutionEngineMetadata(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isRouterAvailable(cfg.Router.BaseURL) {
		t.Skip("TAS-LLM-Router not available")
	}

	routerService := impl.NewRouterService(&cfg.Router)
	ctx := context.Background()
	userID := uuid.New()

	t.Run("Comprehensive Metadata Collection", func(t *testing.T) {
		retryConfig, fallbackConfig := models.HighReliabilityConfig()
		
		agentConfig := models.AgentLLMConfig{
			Provider:       "openai",
			Model:         "gpt-3.5-turbo",
			OptimizeFor:   "reliability",
			RetryConfig:   retryConfig,
			FallbackConfig: fallbackConfig,
		}

		messages := []services.Message{
			{Role: "user", Content: "Write a haiku about testing."},
		}

		response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
		require.NoError(t, err, "Execution should succeed")

		// Validate all expected metadata fields
		expectedFields := []string{
			"request_id", "router_metadata", "retry_attempts", "fallback_used",
			"failed_providers", "total_retry_time", "provider_latency", "routing_reason",
		}

		metadata := response.Metadata
		require.NotNil(t, metadata, "Metadata should be present")

		for _, field := range expectedFields {
			assert.Contains(t, metadata, field, "Missing metadata field: %s", field)
		}

		// Validate metadata types and values
		if retryAttempts, ok := metadata["retry_attempts"]; ok {
			assert.IsType(t, 0, retryAttempts, "retry_attempts should be int")
			assert.GreaterOrEqual(t, retryAttempts.(int), 0, "retry_attempts should be non-negative")
		}

		if fallbackUsed, ok := metadata["fallback_used"]; ok {
			assert.IsType(t, false, fallbackUsed, "fallback_used should be bool")
		}

		if providerLatency, ok := metadata["provider_latency"]; ok {
			assert.IsType(t, 0, providerLatency, "provider_latency should be int (ms)")
			assert.Greater(t, providerLatency.(int), 0, "provider_latency should be positive")
		}

		t.Logf("✅ Comprehensive metadata collection validated")
		
		// Log all metadata for inspection
		metadataJSON, _ := json.MarshalIndent(metadata, "   ", "  ")
		t.Logf("   Metadata: %s", string(metadataJSON))
	})

	t.Run("Cost Tracking Validation", func(t *testing.T) {
		agentConfig := models.AgentLLMConfig{
			Provider:    "openai",
			Model:      "gpt-3.5-turbo",
			MaxTokens:   intPtr(100),
			OptimizeFor: "cost",
		}

		messages := []services.Message{
			{Role: "user", Content: "Explain machine learning in one sentence."},
		}

		response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
		require.NoError(t, err, "Cost tracking execution should succeed")

		// Validate cost tracking
		assert.Greater(t, response.CostUSD, 0.0, "Cost should be positive")
		assert.Less(t, response.CostUSD, 1.0, "Cost should be reasonable for small request")

		// Check for cost metadata
		if metadata := response.Metadata; metadata != nil {
			if routerMeta, ok := metadata["router_metadata"]; ok {
				if routerMap, ok := routerMeta.(map[string]interface{}); ok {
					if estimatedCost, ok := routerMap["estimated_cost"]; ok {
						t.Logf("   Estimated cost: %v", estimatedCost)
					}
					if actualCost, ok := routerMap["actual_cost"]; ok {
						t.Logf("   Actual cost: %v", actualCost)
					}
				}
			}
		}

		t.Logf("✅ Cost tracking validation passed")
		t.Logf("   Total cost: $%.6f", response.CostUSD)
		t.Logf("   Tokens used: %d", response.TokenUsage)
	})
}

// TestExecutionEngineConcurrency tests concurrent execution handling
func TestExecutionEngineConcurrency(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isRouterAvailable(cfg.Router.BaseURL) {
		t.Skip("TAS-LLM-Router not available")
	}

	routerService := impl.NewRouterService(&cfg.Router)
	ctx := context.Background()

	t.Run("Concurrent Executions - 5 simultaneous", func(t *testing.T) {
		concurrency := 5
		var wg sync.WaitGroup
		responses := make(chan *services.RouterResponse, concurrency)
		errors := make(chan error, concurrency)

		agentConfig := models.AgentLLMConfig{
			Provider:    "openai",
			Model:      "gpt-3.5-turbo",
			Temperature: floatPtr(0.1),
			MaxTokens:   intPtr(30),
		}

		// Launch concurrent executions
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				userID := uuid.New()
				messages := []services.Message{
					{Role: "user", Content: fmt.Sprintf("Count to %d", id+1)},
				}

				response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
				if err != nil {
					errors <- err
				} else {
					responses <- response
				}
			}(i)
		}

		wg.Wait()
		close(responses)
		close(errors)

		// Collect results
		successCount := 0
		errorCount := 0
		totalCost := 0.0
		totalTokens := 0

		for response := range responses {
			successCount++
			totalCost += response.CostUSD
			totalTokens += response.TokenUsage
		}

		for err := range errors {
			errorCount++
			t.Logf("   Concurrent execution error: %v", err)
		}

		// Validate results
		assert.Equal(t, concurrency, successCount+errorCount, "All executions should complete")
		assert.Greater(t, successCount, 0, "At least some executions should succeed")
		
		if successCount > 0 {
			avgCost := totalCost / float64(successCount)
			avgTokens := float64(totalTokens) / float64(successCount)
			
			t.Logf("✅ Concurrent execution completed")
			t.Logf("   Successful: %d/%d", successCount, concurrency)
			t.Logf("   Average cost: $%.6f", avgCost)
			t.Logf("   Average tokens: %.1f", avgTokens)
		}
	})

	t.Run("Concurrent Executions with Different Configurations", func(t *testing.T) {
		configs := []models.AgentLLMConfig{
			{
				Provider:    "openai",
				Model:      "gpt-3.5-turbo",
				OptimizeFor: "cost",
			},
			{
				Provider:    "openai",
				Model:      "gpt-3.5-turbo",
				OptimizeFor: "performance",
				RetryConfig: models.DefaultRetryConfig(),
			},
			{
				Provider:       "openai",
				Model:         "gpt-3.5-turbo",
				OptimizeFor:   "reliability",
				RetryConfig:   func() *models.RetryConfig { r, _ := models.HighReliabilityConfig(); return r }(),
				FallbackConfig: func() *models.FallbackConfig { _, f := models.HighReliabilityConfig(); return f }(),
			},
		}

		var wg sync.WaitGroup
		results := make(chan map[string]interface{}, len(configs))

		for i, config := range configs {
			wg.Add(1)
			go func(id int, cfg models.AgentLLMConfig) {
				defer wg.Done()
				
				userID := uuid.New()
				messages := []services.Message{
					{Role: "user", Content: "What is the capital of France?"},
				}

				startTime := time.Now()
				response, err := routerService.SendRequest(ctx, cfg, messages, userID)
				duration := time.Since(startTime)

				result := map[string]interface{}{
					"id":          id,
					"optimize_for": cfg.OptimizeFor,
					"success":     err == nil,
					"duration_ms": duration.Milliseconds(),
				}

				if err == nil {
					result["cost"] = response.CostUSD
					result["tokens"] = response.TokenUsage
					result["provider"] = response.Provider
				} else {
					result["error"] = err.Error()
				}

				results <- result
			}(i, config)
		}

		wg.Wait()
		close(results)

		// Analyze results
		successCount := 0
		for result := range results {
			if result["success"].(bool) {
				successCount++
				t.Logf("   Config %d (%s): success in %dms, cost $%.6f", 
					result["id"], result["optimize_for"], 
					result["duration_ms"], result["cost"])
			} else {
				t.Logf("   Config %d (%s): failed - %s", 
					result["id"], result["optimize_for"], result["error"])
			}
		}

		assert.Greater(t, successCount, 0, "At least some configurations should succeed")
		t.Logf("✅ Mixed configuration concurrent execution completed (%d/%d success)", 
			successCount, len(configs))
	})
}

// TestExecutionEngineErrorHandling tests error scenarios and recovery
func TestExecutionEngineErrorHandling(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isRouterAvailable(cfg.Router.BaseURL) {
		t.Skip("TAS-LLM-Router not available")
	}

	routerService := impl.NewRouterService(&cfg.Router)
	ctx := context.Background()
	userID := uuid.New()

	t.Run("Invalid Configuration Handling", func(t *testing.T) {
		invalidConfig := models.AgentLLMConfig{
			Provider: "invalid-provider",
			Model:   "invalid-model",
		}

		messages := []services.Message{
			{Role: "user", Content: "Test message"},
		}

		_, err := routerService.SendRequest(ctx, invalidConfig, messages, userID)
		assert.Error(t, err, "Invalid configuration should cause error")
		t.Logf("✅ Invalid configuration correctly rejected: %v", err)
	})

	t.Run("Timeout Handling", func(t *testing.T) {
		// Test with very short timeout context
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
		defer cancel()

		agentConfig := models.AgentLLMConfig{
			Provider: "openai",
			Model:   "gpt-3.5-turbo",
		}

		messages := []services.Message{
			{Role: "user", Content: "This should timeout"},
		}

		_, err := routerService.SendRequest(timeoutCtx, agentConfig, messages, userID)
		assert.Error(t, err, "Timeout should cause error")
		assert.Contains(t, err.Error(), "context", "Error should mention context timeout")
		t.Logf("✅ Timeout handling validated: %v", err)
	})

	t.Run("Empty Message Handling", func(t *testing.T) {
		agentConfig := models.AgentLLMConfig{
			Provider: "openai",
			Model:   "gpt-3.5-turbo",
		}

		// Test with empty messages
		emptyMessages := []services.Message{}

		_, err := routerService.SendRequest(ctx, agentConfig, emptyMessages, userID)
		// This might succeed or fail depending on router implementation
		// Log the result for analysis
		if err != nil {
			t.Logf("   Empty messages rejected: %v", err)
		} else {
			t.Logf("   Empty messages accepted (router handled gracefully)")
		}

		t.Logf("✅ Empty message handling tested")
	})

	t.Run("Large Input Handling", func(t *testing.T) {
		agentConfig := models.AgentLLMConfig{
			Provider:  "openai",
			Model:    "gpt-3.5-turbo",
			MaxTokens: intPtr(10), // Very small output limit
		}

		// Create a large input
		largeContent := ""
		for i := 0; i < 1000; i++ {
			largeContent += "This is a test sentence to create a large input. "
		}

		messages := []services.Message{
			{Role: "user", Content: largeContent},
		}

		response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
		if err != nil {
			t.Logf("   Large input rejected: %v", err)
		} else {
			assert.NotNil(t, response, "Response should not be nil")
			t.Logf("   Large input handled, tokens: %d, cost: $%.6f", 
				response.TokenUsage, response.CostUSD)
		}

		t.Logf("✅ Large input handling tested")
	})
}

// TestExecutionEnginePerformance tests performance characteristics
func TestExecutionEnginePerformance(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isRouterAvailable(cfg.Router.BaseURL) {
		t.Skip("TAS-LLM-Router not available")
	}

	routerService := impl.NewRouterService(&cfg.Router)
	ctx := context.Background()

	t.Run("Response Time Analysis", func(t *testing.T) {
		agentConfig := models.AgentLLMConfig{
			Provider:    "openai",
			Model:      "gpt-3.5-turbo",
			Temperature: floatPtr(0.0),
			MaxTokens:   intPtr(50),
		}

		messages := []services.Message{
			{Role: "user", Content: "What is the capital of Japan?"},
		}

		// Run multiple executions to analyze response times
		executionCount := 5
		durations := make([]time.Duration, executionCount)
		
		for i := 0; i < executionCount; i++ {
			userID := uuid.New()
			startTime := time.Now()
			
			response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
			durations[i] = time.Since(startTime)
			
			require.NoError(t, err, "Execution %d should succeed", i+1)
			assert.NotNil(t, response, "Response %d should not be nil", i+1)
		}

		// Calculate statistics
		var totalDuration time.Duration
		minDuration := durations[0]
		maxDuration := durations[0]

		for _, d := range durations {
			totalDuration += d
			if d < minDuration {
				minDuration = d
			}
			if d > maxDuration {
				maxDuration = d
			}
		}

		avgDuration := totalDuration / time.Duration(executionCount)

		t.Logf("✅ Response time analysis completed")
		t.Logf("   Executions: %d", executionCount)
		t.Logf("   Average: %dms", avgDuration.Milliseconds())
		t.Logf("   Min: %dms", minDuration.Milliseconds())
		t.Logf("   Max: %dms", maxDuration.Milliseconds())

		// Performance assertions
		assert.Less(t, avgDuration, 10*time.Second, "Average response time should be reasonable")
		assert.Less(t, maxDuration, 15*time.Second, "Max response time should be acceptable")
	})
}

