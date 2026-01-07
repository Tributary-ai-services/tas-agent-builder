package test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tas-agent-builder/models"
)

// TestRetryConfigValidation tests the retry configuration validation
func TestRetryConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      models.RetryConfig
		expectValid bool
		errorMsg    string
	}{
		{
			name: "Valid exponential retry config",
			config: models.RetryConfig{
				MaxAttempts:     3,
				BackoffType:     "exponential",
				BaseDelay:       "1s",
				MaxDelay:        "30s",
				RetryableErrors: []string{"timeout", "connection"},
			},
			expectValid: true,
		},
		{
			name: "Valid linear retry config",
			config: models.RetryConfig{
				MaxAttempts:     2,
				BackoffType:     "linear",
				BaseDelay:       "500ms",
				MaxDelay:        "10s",
				RetryableErrors: []string{"unavailable"},
			},
			expectValid: true,
		},
		{
			name: "Invalid max attempts - too low",
			config: models.RetryConfig{
				MaxAttempts: 0,
				BackoffType: "exponential",
			},
			expectValid: false,
			errorMsg:    "max_attempts must be between 1 and 5",
		},
		{
			name: "Invalid max attempts - too high",
			config: models.RetryConfig{
				MaxAttempts: 6,
				BackoffType: "exponential",
			},
			expectValid: false,
			errorMsg:    "max_attempts must be between 1 and 5",
		},
		{
			name: "Invalid backoff type",
			config: models.RetryConfig{
				MaxAttempts: 3,
				BackoffType: "invalid",
			},
			expectValid: false,
			errorMsg:    "backoff_type must be 'exponential' or 'linear'",
		},
		{
			name: "Invalid base delay format",
			config: models.RetryConfig{
				MaxAttempts: 3,
				BackoffType: "exponential",
				BaseDelay:   "invalid",
			},
			expectValid: false,
			errorMsg:    "invalid base_delay format",
		},
		{
			name: "Valid with milliseconds",
			config: models.RetryConfig{
				MaxAttempts: 2,
				BackoffType: "linear",
				BaseDelay:   "100ms",
				MaxDelay:    "2s",
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRetryConfig(tt.config)
			if tt.expectValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			}
		})
	}
}

// TestFallbackConfigValidation tests the fallback configuration validation
func TestFallbackConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      models.FallbackConfig
		expectValid bool
		errorMsg    string
	}{
		{
			name: "Valid fallback config with all fields",
			config: models.FallbackConfig{
				Enabled:             true,
				PreferredChain:      []string{"openai", "anthropic"},
				MaxCostIncrease:     floatPtr(0.5),
				RequireSameFeatures: true,
			},
			expectValid: true,
		},
		{
			name: "Valid with no cost increase",
			config: models.FallbackConfig{
				Enabled:             true,
				MaxCostIncrease:     floatPtr(0),
				RequireSameFeatures: false,
			},
			expectValid: true,
		},
		{
			name: "Invalid max cost increase - negative",
			config: models.FallbackConfig{
				Enabled:         true,
				MaxCostIncrease: floatPtr(-0.1),
			},
			expectValid: false,
			errorMsg:    "max_cost_increase must be between 0 and 2.0",
		},
		{
			name: "Invalid max cost increase - too high",
			config: models.FallbackConfig{
				Enabled:         true,
				MaxCostIncrease: floatPtr(2.1),
			},
			expectValid: false,
			errorMsg:    "max_cost_increase must be between 0 and 2.0",
		},
		{
			name: "Valid with 200% cost increase",
			config: models.FallbackConfig{
				Enabled:         true,
				MaxCostIncrease: floatPtr(2.0),
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFallbackConfig(tt.config)
			if tt.expectValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			}
		})
	}
}

// TestConfigurationPresets tests the pre-configured reliability templates
func TestConfigurationPresets(t *testing.T) {
	t.Run("High Reliability Config", func(t *testing.T) {
		retry, fallback := models.HighReliabilityConfig()
		
		assert.NotNil(t, retry)
		assert.NotNil(t, fallback)
		
		// High reliability should have aggressive retry
		assert.Equal(t, 5, retry.MaxAttempts)
		assert.Equal(t, "exponential", retry.BackoffType)
		assert.Equal(t, "500ms", retry.BaseDelay)
		assert.Equal(t, "60s", retry.MaxDelay)
		assert.Contains(t, retry.RetryableErrors, "timeout")
		assert.Contains(t, retry.RetryableErrors, "server_error")
		
		// Should allow high cost increase for reliability
		assert.True(t, fallback.Enabled)
		assert.Equal(t, 1.0, *fallback.MaxCostIncrease)
		assert.False(t, fallback.RequireSameFeatures)
	})

	t.Run("Cost Optimized Config", func(t *testing.T) {
		retry, fallback := models.CostOptimizedConfig()
		
		assert.NotNil(t, retry)
		assert.NotNil(t, fallback)
		
		// Cost optimized should have conservative retry
		assert.Equal(t, 2, retry.MaxAttempts)
		assert.Equal(t, "linear", retry.BackoffType)
		assert.Equal(t, "2s", retry.BaseDelay)
		
		// Should limit cost increase
		assert.True(t, fallback.Enabled)
		assert.Equal(t, 0.2, *fallback.MaxCostIncrease)
		assert.True(t, fallback.RequireSameFeatures)
		assert.Equal(t, []string{"openai", "anthropic"}, fallback.PreferredChain)
	})

	t.Run("Performance Optimized Config", func(t *testing.T) {
		retry, fallback := models.PerformanceOptimizedConfig()
		
		assert.NotNil(t, retry)
		assert.NotNil(t, fallback)
		
		// Performance should have minimal delays
		assert.Equal(t, 2, retry.MaxAttempts)
		assert.Equal(t, "linear", retry.BackoffType)
		assert.Equal(t, "100ms", retry.BaseDelay)
		assert.Equal(t, "2s", retry.MaxDelay)
		
		// Moderate cost increase for speed
		assert.True(t, fallback.Enabled)
		assert.Equal(t, 0.3, *fallback.MaxCostIncrease)
	})

	t.Run("Default Configs", func(t *testing.T) {
		retry := models.DefaultRetryConfig()
		fallback := models.DefaultFallbackConfig()
		
		assert.NotNil(t, retry)
		assert.NotNil(t, fallback)
		
		// Defaults should be balanced
		assert.Equal(t, 3, retry.MaxAttempts)
		assert.Equal(t, "exponential", retry.BackoffType)
		
		assert.True(t, fallback.Enabled)
		assert.Equal(t, 0.5, *fallback.MaxCostIncrease)
		assert.True(t, fallback.RequireSameFeatures)
	})
}

// TestEnhancedAgentLLMConfig tests the enhanced LLM configuration
func TestEnhancedAgentLLMConfig(t *testing.T) {
	t.Run("Full configuration with all fields", func(t *testing.T) {
		config := models.AgentLLMConfig{
			Provider:         "openai",
			Model:           "gpt-4",
			Temperature:     floatPtr(0.7),
			MaxTokens:       intPtr(200),
			OptimizeFor:     "reliability",
			RequiredFeatures: []string{"chat_completions", "functions"},
			MaxCost:         floatPtr(0.05),
			RetryConfig:     models.DefaultRetryConfig(),
			FallbackConfig:  models.DefaultFallbackConfig(),
			Metadata: map[string]any{
				"test": "value",
			},
		}

		// Test JSON marshaling
		jsonData, err := json.Marshal(config)
		require.NoError(t, err)
		
		// Test JSON unmarshaling
		var unmarshaledConfig models.AgentLLMConfig
		err = json.Unmarshal(jsonData, &unmarshaledConfig)
		require.NoError(t, err)
		
		assert.Equal(t, config.Provider, unmarshaledConfig.Provider)
		assert.Equal(t, config.Model, unmarshaledConfig.Model)
		assert.Equal(t, config.OptimizeFor, unmarshaledConfig.OptimizeFor)
		assert.Equal(t, config.RequiredFeatures, unmarshaledConfig.RequiredFeatures)
		assert.Equal(t, *config.MaxCost, *unmarshaledConfig.MaxCost)
		assert.NotNil(t, unmarshaledConfig.RetryConfig)
		assert.NotNil(t, unmarshaledConfig.FallbackConfig)
	})

	t.Run("Minimal configuration", func(t *testing.T) {
		config := models.AgentLLMConfig{
			Provider: "anthropic",
			Model:    "claude-3-sonnet-20240229",
		}

		jsonData, err := json.Marshal(config)
		require.NoError(t, err)
		
		var unmarshaledConfig models.AgentLLMConfig
		err = json.Unmarshal(jsonData, &unmarshaledConfig)
		require.NoError(t, err)
		
		assert.Equal(t, "anthropic", unmarshaledConfig.Provider)
		assert.Equal(t, "claude-3-sonnet-20240229", unmarshaledConfig.Model)
		assert.Nil(t, unmarshaledConfig.RetryConfig)
		assert.Nil(t, unmarshaledConfig.FallbackConfig)
		assert.Empty(t, unmarshaledConfig.OptimizeFor)
	})

	t.Run("Database Value/Scan methods", func(t *testing.T) {
		config := models.AgentLLMConfig{
			Provider:    "openai",
			Model:       "gpt-3.5-turbo",
			OptimizeFor: "cost",
			RetryConfig: &models.RetryConfig{
				MaxAttempts: 2,
				BackoffType: "linear",
			},
		}

		// Test Value method
		value, err := config.Value()
		require.NoError(t, err)
		assert.NotNil(t, value)

		// Test Scan method
		var scannedConfig models.AgentLLMConfig
		err = scannedConfig.Scan(value)
		require.NoError(t, err)
		
		assert.Equal(t, config.Provider, scannedConfig.Provider)
		assert.Equal(t, config.Model, scannedConfig.Model)
		assert.Equal(t, config.OptimizeFor, scannedConfig.OptimizeFor)
		assert.NotNil(t, scannedConfig.RetryConfig)
		assert.Equal(t, 2, scannedConfig.RetryConfig.MaxAttempts)
	})
}

// TestAgentExecutionReliabilityFields tests the enhanced execution model
func TestAgentExecutionReliabilityFields(t *testing.T) {
	execution := models.AgentExecution{
		ID:                uuid.New(),
		AgentID:           uuid.New(),
		UserID:            uuid.New(),
		Status:            models.ExecutionStatusCompleted,
		RetryAttempts:     2,
		FallbackUsed:      true,
		TotalRetryTimeMs:  1500,
		ProviderLatencyMs: intPtr(180),
		ActualCostUSD:     floatPtr(0.003),
		EstimatedCostUSD:  floatPtr(0.002),
	}

	t.Run("Reliability fields are set correctly", func(t *testing.T) {
		assert.Equal(t, 2, execution.RetryAttempts)
		assert.True(t, execution.FallbackUsed)
		assert.Equal(t, 1500, execution.TotalRetryTimeMs)
		assert.Equal(t, 180, *execution.ProviderLatencyMs)
		assert.Equal(t, 0.003, *execution.ActualCostUSD)
		assert.Equal(t, 0.002, *execution.EstimatedCostUSD)
	})

	t.Run("Failed providers JSON handling", func(t *testing.T) {
		failedProviders := []string{"openai", "anthropic"}
		jsonData, err := json.Marshal(failedProviders)
		require.NoError(t, err)
		
		execution.FailedProviders = jsonData
		
		// Unmarshal to verify
		var providers []string
		err = json.Unmarshal(execution.FailedProviders, &providers)
		require.NoError(t, err)
		assert.Equal(t, failedProviders, providers)
	})

	t.Run("Routing reason JSON handling", func(t *testing.T) {
		routingReason := []string{"Primary provider unavailable", "Fallback to anthropic"}
		jsonData, err := json.Marshal(routingReason)
		require.NoError(t, err)
		
		execution.RoutingReason = jsonData
		
		// Unmarshal to verify
		var reasons []string
		err = json.Unmarshal(execution.RoutingReason, &reasons)
		require.NoError(t, err)
		assert.Equal(t, routingReason, reasons)
	})
}

// TestReliabilityMetrics tests the reliability metrics structure
func TestReliabilityMetrics(t *testing.T) {
	metrics := models.ReliabilityMetrics{
		RetryAttempts:     3,
		FallbackUsed:      true,
		FailedProviders:   []string{"openai"},
		TotalRetryTimeMs:  2500,
		ProviderLatencyMs: intPtr(200),
		RoutingReason:     []string{"Rate limit exceeded", "Fallback to anthropic"},
		ActualCostUSD:     floatPtr(0.004),
		EstimatedCostUSD:  floatPtr(0.003),
		ReliabilityScore:  floatPtr(0.95),
	}

	t.Run("All fields are accessible", func(t *testing.T) {
		assert.Equal(t, 3, metrics.RetryAttempts)
		assert.True(t, metrics.FallbackUsed)
		assert.Contains(t, metrics.FailedProviders, "openai")
		assert.Equal(t, 2500, metrics.TotalRetryTimeMs)
		assert.Equal(t, 200, *metrics.ProviderLatencyMs)
		assert.Len(t, metrics.RoutingReason, 2)
		assert.Equal(t, 0.004, *metrics.ActualCostUSD)
		assert.Equal(t, 0.003, *metrics.EstimatedCostUSD)
		assert.Equal(t, 0.95, *metrics.ReliabilityScore)
	})

	t.Run("JSON marshaling", func(t *testing.T) {
		jsonData, err := json.Marshal(metrics)
		require.NoError(t, err)
		
		var unmarshaledMetrics models.ReliabilityMetrics
		err = json.Unmarshal(jsonData, &unmarshaledMetrics)
		require.NoError(t, err)
		
		assert.Equal(t, metrics.RetryAttempts, unmarshaledMetrics.RetryAttempts)
		assert.Equal(t, metrics.FallbackUsed, unmarshaledMetrics.FallbackUsed)
		assert.Equal(t, metrics.FailedProviders, unmarshaledMetrics.FailedProviders)
		assert.Equal(t, *metrics.ReliabilityScore, *unmarshaledMetrics.ReliabilityScore)
	})
}

// TestExecutionListFilterEnhancements tests the enhanced execution filter
func TestExecutionListFilterEnhancements(t *testing.T) {
	filter := models.ExecutionListFilter{
		AgentID:        uuidPtr(uuid.New()),
		Status:         statusPtr(models.ExecutionStatusCompleted),
		WithRetries:    boolPtr(true),
		WithFallback:   boolPtr(false),
		MinReliability: floatPtr(0.9),
		Page:           1,
		Size:           20,
	}

	t.Run("Enhanced filter fields", func(t *testing.T) {
		assert.True(t, *filter.WithRetries)
		assert.False(t, *filter.WithFallback)
		assert.Equal(t, 0.9, *filter.MinReliability)
	})

	t.Run("Filter serialization", func(t *testing.T) {
		jsonData, err := json.Marshal(filter)
		require.NoError(t, err)
		
		var unmarshaledFilter models.ExecutionListFilter
		err = json.Unmarshal(jsonData, &unmarshaledFilter)
		require.NoError(t, err)
		
		assert.Equal(t, *filter.WithRetries, *unmarshaledFilter.WithRetries)
		assert.Equal(t, *filter.WithFallback, *unmarshaledFilter.WithFallback)
		assert.Equal(t, *filter.MinReliability, *unmarshaledFilter.MinReliability)
	})
}

// Helper functions for tests