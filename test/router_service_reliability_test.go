package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// TestRouterServiceWithReliabilityFeatures tests the enhanced router service
func TestRouterServiceWithReliabilityFeatures(t *testing.T) {
	// Mock router server that simulates retry and fallback scenarios
	mockServer := createMockRouterServer(t)
	defer mockServer.Close()

	cfg := &config.RouterConfig{
		BaseURL:    mockServer.URL,
		APIKey:     "test-key",
		Timeout:    30,
		MaxRetries: 3,
	}

	routerService := impl.NewRouterService(cfg)
	ctx := context.Background()
	userID := uuid.New()

	t.Run("Request with retry configuration", func(t *testing.T) {
		agentConfig := models.AgentLLMConfig{
			Provider:    "openai",
			Model:      "gpt-3.5-turbo",
			Temperature: floatPtr(0.7),
			MaxTokens:   intPtr(150),
			OptimizeFor: "reliability",
			RetryConfig: &models.RetryConfig{
				MaxAttempts:     3,
				BackoffType:     "exponential",
				BaseDelay:       "1s",
				MaxDelay:        "30s",
				RetryableErrors: []string{"timeout", "connection"},
			},
		}

		messages := []services.Message{
			{Role: "user", Content: "Test message with retry config"},
		}

		response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.NotEmpty(t, response.Content)
		assert.Equal(t, "openai", response.Provider)
		
		// Check for enhanced metadata
		assert.NotNil(t, response.Metadata)
		if retries, ok := response.Metadata["retry_attempts"]; ok {
			assert.GreaterOrEqual(t, retries.(int), 0)
		}
	})

	t.Run("Request with fallback configuration", func(t *testing.T) {
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
			{Role: "user", Content: "Test message with fallback config"},
		}

		response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
		require.NoError(t, err)
		assert.NotNil(t, response)
		
		// Verify fallback metadata is captured
		if fallback, ok := response.Metadata["fallback_used"]; ok {
			assert.IsType(t, false, fallback)
		}
	})

	t.Run("Request with both retry and fallback", func(t *testing.T) {
		retryConfig, fallbackConfig := models.HighReliabilityConfig()
		
		agentConfig := models.AgentLLMConfig{
			Provider:         "openai",
			Model:           "gpt-4",
			OptimizeFor:     "reliability",
			RequiredFeatures: []string{"chat_completions"},
			MaxCost:         floatPtr(0.05),
			RetryConfig:     retryConfig,
			FallbackConfig:  fallbackConfig,
		}

		messages := []services.Message{
			{Role: "user", Content: "Test message with full reliability config"},
		}

		response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
		require.NoError(t, err)
		assert.NotNil(t, response)
		
		// Verify all reliability metadata fields are present
		metadata := response.Metadata
		assert.NotNil(t, metadata)
		
		// Check for standard fields
		assert.Contains(t, metadata, "request_id")
		assert.Contains(t, metadata, "router_metadata")
		
		// Check for enhanced reliability fields
		expectedFields := []string{
			"retry_attempts", "fallback_used", "failed_providers",
			"total_retry_time", "provider_latency", "routing_reason",
		}
		
		for _, field := range expectedFields {
			assert.Contains(t, metadata, field, "Missing reliability field: %s", field)
		}
	})

	t.Run("Configuration validation with router", func(t *testing.T) {
		validConfig := models.AgentLLMConfig{
			Provider: "openai",
			Model:    "gpt-3.5-turbo",
		}

		err := routerService.ValidateConfig(ctx, validConfig)
		assert.NoError(t, err)

		invalidConfig := models.AgentLLMConfig{
			Provider: "invalid-provider",
			Model:    "invalid-model",
		}

		err = routerService.ValidateConfig(ctx, invalidConfig)
		assert.Error(t, err)
	})
}

// TestReliabilityMetadataExtraction tests the metadata extraction functionality
func TestReliabilityMetadataExtraction(t *testing.T) {
	tests := []struct {
		name             string
		routerMetadata   map[string]interface{}
		expectedMetadata map[string]interface{}
	}{
		{
			name: "Complete reliability metadata",
			routerMetadata: map[string]interface{}{
				"attempt_count":     float64(3),
				"fallback_used":     true,
				"failed_providers":  []interface{}{"openai", "anthropic"},
				"total_retry_time":  float64(2500),
				"provider_latency":  "180ms",
				"routing_reason":    []interface{}{"Rate limit", "Fallback to claude"},
			},
			expectedMetadata: map[string]interface{}{
				"retry_attempts":   2, // attempt_count - 1
				"fallback_used":    true,
				"failed_providers": []string{"openai", "anthropic"},
				"total_retry_time": 2500,
				"provider_latency": 180,
				"routing_reason":   []string{"Rate limit", "Fallback to claude"},
			},
		},
		{
			name: "Minimal metadata",
			routerMetadata: map[string]interface{}{
				"attempt_count": float64(1),
			},
			expectedMetadata: map[string]interface{}{
				"retry_attempts":   0,
				"fallback_used":    false,
				"failed_providers": []string{},
				"total_retry_time": 0,
				"provider_latency": 0,
				"routing_reason":   []string{},
			},
		},
		{
			name:             "Empty metadata",
			routerMetadata:   nil,
			expectedMetadata: map[string]interface{}{
				"retry_attempts":   0,
				"fallback_used":    false,
				"failed_providers": []string{},
				"total_retry_time": 0,
				"provider_latency": 0,
				"routing_reason":   []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would test the extractReliabilityMetadata function
			// Since it's internal, we test it through the service response
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response := map[string]interface{}{
					"id":      "test-id",
					"object":  "chat.completion",
					"created": time.Now().Unix(),
					"model":   "gpt-3.5-turbo",
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"message": map[string]interface{}{
								"role":    "assistant",
								"content": "Test response",
							},
							"finish_reason": "stop",
						},
					},
					"usage": map[string]interface{}{
						"prompt_tokens":     10,
						"completion_tokens": 20,
						"total_tokens":     30,
					},
					"router_metadata": tt.routerMetadata,
				}
				
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer mockServer.Close()

			cfg := &config.RouterConfig{
				BaseURL:    mockServer.URL,
				APIKey:     "test-key",
				Timeout:    30,
				MaxRetries: 0,
			}

			routerService := impl.NewRouterService(cfg)
			
			agentConfig := models.AgentLLMConfig{
				Provider: "openai",
				Model:    "gpt-3.5-turbo",
			}

			messages := []services.Message{
				{Role: "user", Content: "Test"},
			}

			response, err := routerService.SendRequest(context.Background(), agentConfig, messages, uuid.New())
			require.NoError(t, err)
			
			// Verify extracted metadata matches expectations
			for key, expectedValue := range tt.expectedMetadata {
				actualValue := response.Metadata[key]
				
				switch key {
				case "failed_providers", "routing_reason":
					// Compare slices
					expectedSlice, ok1 := expectedValue.([]string)
					actualSlice, ok2 := actualValue.([]string)
					if ok1 && ok2 {
						assert.Equal(t, expectedSlice, actualSlice, "Mismatch in %s", key)
					} else if len(expectedSlice) == 0 && actualValue == nil {
						// Empty slice equivalent to nil
						continue
					} else {
						t.Errorf("Type mismatch for %s: expected %T, got %T", key, expectedValue, actualValue)
					}
				default:
					assert.Equal(t, expectedValue, actualValue, "Mismatch in %s", key)
				}
			}
		})
	}
}

// TestProviderAvailability tests provider availability checking
func TestProviderAvailability(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/providers" {
			response := map[string]interface{}{
				"count":     2,
				"providers": []string{"openai", "anthropic"},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(404)
	}))
	defer mockServer.Close()

	cfg := &config.RouterConfig{
		BaseURL: mockServer.URL,
		APIKey:  "test-key",
		Timeout: 30,
	}

	routerService := impl.NewRouterService(cfg)
	ctx := context.Background()

	t.Run("Get available providers", func(t *testing.T) {
		providers, err := routerService.GetAvailableProviders(ctx)
		require.NoError(t, err)
		assert.Len(t, providers, 2)
		
		providerNames := make([]string, len(providers))
		for i, p := range providers {
			providerNames[i] = p.Name
		}
		
		assert.Contains(t, providerNames, "openai")
		assert.Contains(t, providerNames, "anthropic")
		
		// Check that models are populated for known providers
		for _, provider := range providers {
			if provider.Name == "openai" || provider.Name == "anthropic" {
				assert.NotEmpty(t, provider.Models, "Provider %s should have models", provider.Name)
			}
		}
	})
}

// TestConfigurationTemplatesWithRouter tests router integration with config templates
func TestConfigurationTemplatesWithRouter(t *testing.T) {
	mockServer := createMockRouterServer(t)
	defer mockServer.Close()

	cfg := &config.RouterConfig{
		BaseURL: mockServer.URL,
		APIKey:  "test-key",
		Timeout: 30,
	}

	routerService := impl.NewRouterService(cfg)
	ctx := context.Background()

	testConfigs := map[string]models.AgentLLMConfig{
		"high_reliability": {
			Provider:       "openai",
			Model:         "gpt-3.5-turbo",
			OptimizeFor:   "reliability",
			RetryConfig:   func() *models.RetryConfig { r, _ := models.HighReliabilityConfig(); return r }(),
			FallbackConfig: func() *models.FallbackConfig { _, f := models.HighReliabilityConfig(); return f }(),
		},
		"cost_optimized": {
			Provider:       "openai",
			Model:         "gpt-3.5-turbo",
			OptimizeFor:   "cost",
			MaxCost:       floatPtr(0.01),
			RetryConfig:   func() *models.RetryConfig { r, _ := models.CostOptimizedConfig(); return r }(),
			FallbackConfig: func() *models.FallbackConfig { _, f := models.CostOptimizedConfig(); return f }(),
		},
		"performance": {
			Provider:       "openai",
			Model:         "gpt-3.5-turbo",
			OptimizeFor:   "performance",
			RetryConfig:   func() *models.RetryConfig { r, _ := models.PerformanceOptimizedConfig(); return r }(),
			FallbackConfig: func() *models.FallbackConfig { _, f := models.PerformanceOptimizedConfig(); return f }(),
		},
	}

	for name, config := range testConfigs {
		t.Run(name+" configuration validation", func(t *testing.T) {
			err := routerService.ValidateConfig(ctx, config)
			assert.NoError(t, err, "Configuration %s should be valid", name)
		})

		t.Run(name+" request execution", func(t *testing.T) {
			messages := []services.Message{
				{Role: "user", Content: "Test " + name + " configuration"},
			}

			response, err := routerService.SendRequest(ctx, config, messages, uuid.New())
			require.NoError(t, err)
			assert.NotNil(t, response)
			assert.NotEmpty(t, response.Content)
			
			// Verify optimization strategy is reflected
			assert.Equal(t, config.OptimizeFor, response.RoutingStrategy)
		})
	}
}

// createMockRouterServer creates a mock HTTP server that simulates the TAS-LLM-Router
func createMockRouterServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			handleChatCompletion(w, r, t)
		case "/v1/providers":
			handleProviders(w, r)
		default:
			w.WriteHeader(404)
		}
	}))
}

func handleChatCompletion(w http.ResponseWriter, r *http.Request, t *testing.T) {
	var request map[string]interface{}
	err := json.NewDecoder(r.Body).Decode(&request)
	require.NoError(t, err)

	// Simulate different reliability scenarios based on request
	retryAttempts := 0
	fallbackUsed := false
	var failedProviders []string
	totalRetryTime := 0
	providerLatency := "150ms"
	routingReason := []string{"Direct routing to primary provider"}

	// Check if retry config is present
	if retryConfig, ok := request["retry_config"]; ok && retryConfig != nil {
		retryAttempts = 1 // Simulate one retry
		totalRetryTime = 1000
		routingReason = []string{"Retry after timeout", "Succeeded on retry"}
	}

	// Check if fallback config is present
	if fallbackConfig, ok := request["fallback_config"]; ok && fallbackConfig != nil {
		if fc, ok := fallbackConfig.(map[string]interface{}); ok {
			if enabled, ok := fc["enabled"].(bool); ok && enabled {
				fallbackUsed = true
				failedProviders = []string{"openai"}
				routingReason = []string{"Primary provider failed", "Fallback to anthropic"}
			}
		}
	}

	response := map[string]interface{}{
		"id":      "chatcmpl-test-" + uuid.New().String(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   request["model"],
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "Mock response for reliability testing",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     15,
			"completion_tokens": 10,
			"total_tokens":     25,
		},
		"router_metadata": map[string]interface{}{
			"provider":          "openai",
			"routing_reason":    routingReason,
			"estimated_cost":    0.000025,
			"actual_cost":       0.000025,
			"processing_time":   "200ms",
			"provider_latency":  providerLatency,
			"attempt_count":     float64(retryAttempts + 1),
			"failed_providers":  failedProviders,
			"fallback_used":     fallbackUsed,
			"total_retry_time":  float64(totalRetryTime),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleProviders(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"count":     2,
		"providers": []string{"openai", "anthropic"},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

