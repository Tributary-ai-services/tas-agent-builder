package test

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
	_ "github.com/lib/pq"
)

// ReliabilityIntegrationTestSuite is a comprehensive test suite for reliability features
type ReliabilityIntegrationTestSuite struct {
	suite.Suite
	db     *sql.DB
	config *config.Config
}

func (suite *ReliabilityIntegrationTestSuite) SetupSuite() {
	// Load test configuration
	cfg, err := config.LoadConfig()
	require.NoError(suite.T(), err)
	suite.config = cfg

	// Connect to test database
	dsn := cfg.GetDatabaseDSN()
	db, err := sql.Open("postgres", dsn)
	require.NoError(suite.T(), err)
	suite.db = db

	// Verify database connection
	err = db.Ping()
	require.NoError(suite.T(), err)
}

func (suite *ReliabilityIntegrationTestSuite) TearDownSuite() {
	if suite.db != nil {
		suite.db.Close()
	}
}

func (suite *ReliabilityIntegrationTestSuite) TestDatabaseSchemaEnhancements() {
	t := suite.T()

	t.Run("Enhanced agent_executions columns exist", func(t *testing.T) {
		expectedColumns := []string{
			"retry_attempts",
			"fallback_used",
			"failed_providers",
			"total_retry_time_ms",
			"provider_latency_ms",
			"routing_reason",
			"actual_cost_usd",
			"estimated_cost_usd",
		}

		for _, column := range expectedColumns {
			var exists bool
			query := `
				SELECT EXISTS (
					SELECT 1 FROM information_schema.columns 
					WHERE table_schema = 'agent_builder' 
					AND table_name = 'agent_executions' 
					AND column_name = $1
				)
			`
			err := suite.db.QueryRow(query, column).Scan(&exists)
			require.NoError(t, err)
			assert.True(t, exists, "Column %s should exist", column)
		}
	})

	t.Run("Reliability view exists and functions", func(t *testing.T) {
		// Check if view exists
		var viewExists bool
		viewQuery := `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.views 
				WHERE table_schema = 'agent_builder' 
				AND table_name = 'agent_reliability_view'
			)
		`
		err := suite.db.QueryRow(viewQuery).Scan(&viewExists)
		require.NoError(t, err)
		assert.True(t, viewExists, "agent_reliability_view should exist")

		// Test view query
		testQuery := `
			SELECT COUNT(*) FROM agent_builder.agent_reliability_view
		`
		var count int
		err = suite.db.QueryRow(testQuery).Scan(&count)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})

	t.Run("Reliability function exists and can be called", func(t *testing.T) {
		// Check if function exists
		var functionExists bool
		functionQuery := `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.routines 
				WHERE routine_schema = 'agent_builder' 
				AND routine_name = 'update_agent_reliability_stats'
				AND routine_type = 'FUNCTION'
			)
		`
		err := suite.db.QueryRow(functionQuery).Scan(&functionExists)
		require.NoError(t, err)
		assert.True(t, functionExists, "update_agent_reliability_stats function should exist")

		// Test function call with dummy UUID
		testUUID := uuid.New()
		_, err = suite.db.Exec("SELECT agent_builder.update_agent_reliability_stats($1)", testUUID)
		require.NoError(t, err)
	})
}

func (suite *ReliabilityIntegrationTestSuite) TestReliabilityConfigurationValidation() {
	t := suite.T()

	t.Run("Retry configuration validation", func(t *testing.T) {
		validConfigs := []models.RetryConfig{
			{MaxAttempts: 1, BackoffType: "exponential"},
			{MaxAttempts: 5, BackoffType: "linear"},
			{MaxAttempts: 3, BackoffType: "exponential", BaseDelay: "500ms", MaxDelay: "30s"},
		}

		for _, config := range validConfigs {
			err := validateRetryConfig(config)
			assert.NoError(t, err, "Config should be valid: %+v", config)
		}

		invalidConfigs := []models.RetryConfig{
			{MaxAttempts: 0}, // Too low
			{MaxAttempts: 6}, // Too high
			{MaxAttempts: 3, BackoffType: "invalid"},
			{MaxAttempts: 3, BaseDelay: "invalid"},
		}

		for _, config := range invalidConfigs {
			err := validateRetryConfig(config)
			assert.Error(t, err, "Config should be invalid: %+v", config)
		}
	})

	t.Run("Fallback configuration validation", func(t *testing.T) {
		validConfigs := []models.FallbackConfig{
			{Enabled: true},
			{Enabled: true, MaxCostIncrease: floatPtr(0.0)},
			{Enabled: true, MaxCostIncrease: floatPtr(2.0)},
			{Enabled: false, MaxCostIncrease: floatPtr(1.0)},
		}

		for _, config := range validConfigs {
			err := validateFallbackConfig(config)
			assert.NoError(t, err, "Config should be valid: %+v", config)
		}

		invalidConfigs := []models.FallbackConfig{
			{Enabled: true, MaxCostIncrease: floatPtr(-0.1)}, // Negative
			{Enabled: true, MaxCostIncrease: floatPtr(2.1)},  // Too high
		}

		for _, config := range invalidConfigs {
			err := validateFallbackConfig(config)
			assert.Error(t, err, "Config should be invalid: %+v", config)
		}
	})
}

func (suite *ReliabilityIntegrationTestSuite) TestConfigurationPresets() {
	t := suite.T()

	t.Run("All preset configurations are valid", func(t *testing.T) {
		presets := []struct {
			name string
			fn   func() (*models.RetryConfig, *models.FallbackConfig)
		}{
			{"HighReliability", models.HighReliabilityConfig},
			{"CostOptimized", models.CostOptimizedConfig},
			{"PerformanceOptimized", models.PerformanceOptimizedConfig},
		}

		for _, preset := range presets {
			retry, fallback := preset.fn()
			
			// Validate retry config
			err := validateRetryConfig(*retry)
			assert.NoError(t, err, "%s retry config should be valid", preset.name)
			
			// Validate fallback config
			err = validateFallbackConfig(*fallback)
			assert.NoError(t, err, "%s fallback config should be valid", preset.name)
			
			// Verify meaningful values
			assert.Greater(t, retry.MaxAttempts, 0)
			assert.NotEmpty(t, retry.BackoffType)
			assert.True(t, fallback.Enabled)
			assert.NotNil(t, fallback.MaxCostIncrease)
		}
	})

	t.Run("Default configurations are valid", func(t *testing.T) {
		defaultRetry := models.DefaultRetryConfig()
		defaultFallback := models.DefaultFallbackConfig()

		err := validateRetryConfig(*defaultRetry)
		assert.NoError(t, err)

		err = validateFallbackConfig(*defaultFallback)
		assert.NoError(t, err)

		// Verify default values are reasonable
		assert.Equal(t, 3, defaultRetry.MaxAttempts)
		assert.Equal(t, "exponential", defaultRetry.BackoffType)
		assert.True(t, defaultFallback.Enabled)
		assert.Equal(t, 0.5, *defaultFallback.MaxCostIncrease)
	})
}

func (suite *ReliabilityIntegrationTestSuite) TestEnhancedExecutionTracking() {
	t := suite.T()

	// Create a test agent
	testAgent := suite.createTestAgent()
	defer suite.cleanupTestAgent(testAgent.ID)

	t.Run("Execute and track reliability metadata", func(t *testing.T) {
		executionData := []struct {
			retryAttempts       int
			fallbackUsed        bool
			failedProviders     []string
			totalRetryTimeMs    int
			providerLatencyMs   int
			actualCostUSD       float64
			estimatedCostUSD    float64
		}{
			{0, false, []string{}, 0, 150, 0.002, 0.002},                    // Success first try
			{2, false, []string{"anthropic"}, 1500, 180, 0.002, 0.002},     // Success after retries
			{3, true, []string{"openai", "anthropic"}, 4000, 220, 0.003, 0.002}, // Fallback used
		}

		for i, data := range executionData {
			executionID := uuid.New()
			
			// Insert execution with reliability metadata
			query := `
				INSERT INTO agent_builder.agent_executions (
					id, agent_id, user_id, input_data, output_data, status,
					token_usage, cost_usd, total_duration_ms,
					retry_attempts, fallback_used, failed_providers,
					total_retry_time_ms, provider_latency_ms,
					actual_cost_usd, estimated_cost_usd,
					created_at, updated_at
				) VALUES (
					$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
				)
			`
			
			failedProvidersJSON, _ := json.Marshal(data.failedProviders)
			inputData := `{"message": "test"}`
			outputData := `{"response": "test response"}`
			
			_, err := suite.db.Exec(query,
				executionID, testAgent.ID, testAgent.OwnerID, inputData, outputData, "completed",
				100, data.actualCostUSD, 1000,
				data.retryAttempts, data.fallbackUsed, failedProvidersJSON,
				data.totalRetryTimeMs, data.providerLatencyMs,
				data.actualCostUSD, data.estimatedCostUSD,
				time.Now(), time.Now(),
			)
			require.NoError(t, err, "Failed to insert execution %d", i+1)
		}

		// Verify data was inserted correctly
		var count int
		err := suite.db.QueryRow("SELECT COUNT(*) FROM agent_builder.agent_executions WHERE agent_id = $1", testAgent.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, len(executionData), count)
	})

	t.Run("Reliability analytics calculation", func(t *testing.T) {
		// Query reliability view for our test agent
		reliabilityQuery := `
			SELECT 
				total_executions, success_rate_percent, avg_retry_attempts,
				retry_rate_percent, fallback_rate_percent, reliability_score
			FROM agent_builder.agent_reliability_view
			WHERE agent_id = $1
		`
		
		var totalExec int
		var successRate, avgRetry, retryRate, fallbackRate, reliabilityScore float64
		
		err := suite.db.QueryRow(reliabilityQuery, testAgent.ID).Scan(
			&totalExec, &successRate, &avgRetry, &retryRate, &fallbackRate, &reliabilityScore,
		)
		require.NoError(t, err)

		// Verify calculated metrics
		assert.Equal(t, 3, totalExec)
		assert.Equal(t, 100.0, successRate) // All completed successfully
		assert.Greater(t, avgRetry, 0.0)    // Should have average retry attempts
		assert.Greater(t, retryRate, 0.0)   // Some executions had retries
		assert.Greater(t, fallbackRate, 0.0) // Some executions used fallback
		assert.Greater(t, reliabilityScore, 0.0)
		assert.LessOrEqual(t, reliabilityScore, 1.0)
	})

	t.Run("Reliability stats function updates correctly", func(t *testing.T) {
		// Call the reliability stats update function
		_, err := suite.db.Exec("SELECT agent_builder.update_agent_reliability_stats($1)", testAgent.ID)
		require.NoError(t, err)

		// Verify stats were updated in agent_usage_stats table
		statsQuery := `
			SELECT 
				total_executions, successful_executions, avg_retry_attempts,
				fallback_usage_rate, reliability_score
			FROM agent_builder.agent_usage_stats
			WHERE agent_id = $1
		`
		
		var totalExec, successfulExec int
		var avgRetry, fallbackRate, reliabilityScore float64
		
		err = suite.db.QueryRow(statsQuery, testAgent.ID).Scan(
			&totalExec, &successfulExec, &avgRetry, &fallbackRate, &reliabilityScore,
		)
		require.NoError(t, err)

		assert.Equal(t, 3, totalExec)
		assert.Equal(t, 3, successfulExec)
		assert.Greater(t, avgRetry, 0.0)
		assert.Greater(t, fallbackRate, 0.0)
		assert.Greater(t, reliabilityScore, 0.0)
	})
}

func (suite *ReliabilityIntegrationTestSuite) TestJSONSerializationOfNewFields() {
	t := suite.T()

	t.Run("AgentLLMConfig serialization with reliability fields", func(t *testing.T) {
		config := models.AgentLLMConfig{
			Provider:         "openai",
			Model:           "gpt-4",
			OptimizeFor:     "reliability",
			RequiredFeatures: []string{"chat_completions", "functions"},
			MaxCost:         floatPtr(0.05),
			RetryConfig: &models.RetryConfig{
				MaxAttempts:     3,
				BackoffType:     "exponential",
				BaseDelay:       "1s",
				MaxDelay:        "30s",
				RetryableErrors: []string{"timeout", "connection"},
			},
			FallbackConfig: &models.FallbackConfig{
				Enabled:             true,
				PreferredChain:      []string{"anthropic", "openai"},
				MaxCostIncrease:     floatPtr(0.5),
				RequireSameFeatures: true,
			},
		}

		// Test JSON marshaling
		jsonData, err := json.Marshal(config)
		require.NoError(t, err)

		// Test JSON unmarshaling
		var unmarshaledConfig models.AgentLLMConfig
		err = json.Unmarshal(jsonData, &unmarshaledConfig)
		require.NoError(t, err)

		// Verify all fields are preserved
		assert.Equal(t, config.Provider, unmarshaledConfig.Provider)
		assert.Equal(t, config.OptimizeFor, unmarshaledConfig.OptimizeFor)
		assert.Equal(t, config.RequiredFeatures, unmarshaledConfig.RequiredFeatures)
		assert.Equal(t, *config.MaxCost, *unmarshaledConfig.MaxCost)
		
		// Verify retry config
		assert.Equal(t, config.RetryConfig.MaxAttempts, unmarshaledConfig.RetryConfig.MaxAttempts)
		assert.Equal(t, config.RetryConfig.BackoffType, unmarshaledConfig.RetryConfig.BackoffType)
		
		// Verify fallback config
		assert.Equal(t, config.FallbackConfig.Enabled, unmarshaledConfig.FallbackConfig.Enabled)
		assert.Equal(t, config.FallbackConfig.PreferredChain, unmarshaledConfig.FallbackConfig.PreferredChain)
	})

	t.Run("Database Value/Scan methods", func(t *testing.T) {
		config := models.AgentLLMConfig{
			Provider:    "anthropic",
			Model:      "claude-3-sonnet-20240229",
			OptimizeFor: "cost",
			RetryConfig: models.DefaultRetryConfig(),
			FallbackConfig: models.DefaultFallbackConfig(),
		}

		// Test Value method (for database storage)
		value, err := config.Value()
		require.NoError(t, err)
		assert.NotNil(t, value)

		// Test Scan method (for database retrieval)
		var scannedConfig models.AgentLLMConfig
		err = scannedConfig.Scan(value)
		require.NoError(t, err)

		assert.Equal(t, config.Provider, scannedConfig.Provider)
		assert.Equal(t, config.Model, scannedConfig.Model)
		assert.Equal(t, config.OptimizeFor, scannedConfig.OptimizeFor)
		assert.NotNil(t, scannedConfig.RetryConfig)
		assert.NotNil(t, scannedConfig.FallbackConfig)
	})
}

// Helper methods
func (suite *ReliabilityIntegrationTestSuite) createTestAgent() *models.Agent {
	agent := &models.Agent{
		ID:           uuid.New(),
		Name:         "Test Reliability Agent",
		Description:  "Agent for testing reliability features",
		SystemPrompt: "You are a test agent.",
		LLMConfig: models.AgentLLMConfig{
			Provider:       "openai",
			Model:         "gpt-3.5-turbo",
			OptimizeFor:   "reliability",
			RetryConfig:   models.DefaultRetryConfig(),
			FallbackConfig: models.DefaultFallbackConfig(),
		},
		OwnerID:   uuid.New(),
		SpaceID:   uuid.New(),
		TenantID:  "test-tenant",
		Status:    models.AgentStatusPublished,
		SpaceType: models.SpaceTypePersonal,
		IsPublic:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	insertQuery := `
		INSERT INTO agent_builder.agents (
			id, name, description, system_prompt, llm_config,
			owner_id, space_id, tenant_id, status, space_type,
			is_public, is_template, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
	`
	
	_, err := suite.db.Exec(insertQuery,
		agent.ID, agent.Name, agent.Description, agent.SystemPrompt,
		agent.LLMConfig, agent.OwnerID, agent.SpaceID, agent.TenantID,
		agent.Status, agent.SpaceType, agent.IsPublic, false,
		agent.CreatedAt, agent.UpdatedAt,
	)
	require.NoError(suite.T(), err)

	return agent
}

func (suite *ReliabilityIntegrationTestSuite) cleanupTestAgent(agentID uuid.UUID) {
	// Clean up executions first (foreign key constraint)
	_, err := suite.db.Exec("DELETE FROM agent_builder.agent_executions WHERE agent_id = $1", agentID)
	if err != nil {
		suite.T().Logf("Warning: Failed to clean up executions: %v", err)
	}
	
	// Clean up usage stats
	_, err = suite.db.Exec("DELETE FROM agent_builder.agent_usage_stats WHERE agent_id = $1", agentID)
	if err != nil {
		suite.T().Logf("Warning: Failed to clean up usage stats: %v", err)
	}
	
	// Clean up agent
	_, err = suite.db.Exec("DELETE FROM agent_builder.agents WHERE id = $1", agentID)
	if err != nil {
		suite.T().Logf("Warning: Failed to clean up agent: %v", err)
	}
}

// Helper functions


// TestReliabilityIntegrationSuite runs the full integration test suite
func TestReliabilityIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ReliabilityIntegrationTestSuite))
}