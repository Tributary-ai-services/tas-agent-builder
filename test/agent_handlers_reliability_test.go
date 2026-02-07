package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tas-agent-builder/handlers"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
)

// MockAgentService is a mock implementation of the AgentService interface
type MockAgentService struct {
	mock.Mock
}

func (m *MockAgentService) CreateAgent(ctx context.Context, req models.CreateAgentRequest, ownerID uuid.UUID, tenantID string) (*models.Agent, error) {
	args := m.Called(ctx, req, ownerID, tenantID)
	return args.Get(0).(*models.Agent), args.Error(1)
}

func (m *MockAgentService) GetAgent(ctx context.Context, agentID, userID uuid.UUID) (*models.Agent, error) {
	args := m.Called(ctx, agentID, userID)
	return args.Get(0).(*models.Agent), args.Error(1)
}

func (m *MockAgentService) UpdateAgent(ctx context.Context, agentID uuid.UUID, req models.UpdateAgentRequest, ownerID uuid.UUID) (*models.Agent, error) {
	args := m.Called(ctx, agentID, req, ownerID)
	return args.Get(0).(*models.Agent), args.Error(1)
}

func (m *MockAgentService) DeleteAgent(ctx context.Context, agentID, ownerID uuid.UUID) error {
	args := m.Called(ctx, agentID, ownerID)
	return args.Error(0)
}

func (m *MockAgentService) ListAgents(ctx context.Context, filter models.AgentListFilter, userID uuid.UUID) (*models.AgentListResponse, error) {
	args := m.Called(ctx, filter, userID)
	return args.Get(0).(*models.AgentListResponse), args.Error(1)
}

func (m *MockAgentService) PublishAgent(ctx context.Context, agentID, ownerID uuid.UUID) error {
	args := m.Called(ctx, agentID, ownerID)
	return args.Error(0)
}

func (m *MockAgentService) UnpublishAgent(ctx context.Context, agentID, ownerID uuid.UUID) error {
	args := m.Called(ctx, agentID, ownerID)
	return args.Error(0)
}

func (m *MockAgentService) DuplicateAgent(ctx context.Context, sourceID uuid.UUID, newName string, userID uuid.UUID, tenantID string) (*models.Agent, error) {
	args := m.Called(ctx, sourceID, newName, userID, tenantID)
	return args.Get(0).(*models.Agent), args.Error(1)
}

func (m *MockAgentService) GetReliabilityMetrics(ctx context.Context, agentID uuid.UUID) (interface{}, error) {
	args := m.Called(ctx, agentID)
	return args.Get(0), args.Error(1)
}

// MockRouterService is a mock implementation of the RouterService interface
type MockRouterService struct {
	mock.Mock
}

func (m *MockRouterService) SendRequest(ctx context.Context, agentConfig models.AgentLLMConfig, messages []services.Message, userID uuid.UUID) (*services.RouterResponse, error) {
	args := m.Called(ctx, agentConfig, messages, userID)
	return args.Get(0).(*services.RouterResponse), args.Error(1)
}

func (m *MockRouterService) ValidateConfig(ctx context.Context, config models.AgentLLMConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockRouterService) GetAvailableProviders(ctx context.Context) ([]services.Provider, error) {
	args := m.Called(ctx)
	return args.Get(0).([]services.Provider), args.Error(1)
}

func (m *MockRouterService) GetProviderModels(ctx context.Context, provider string) ([]services.Model, error) {
	args := m.Called(ctx, provider)
	return args.Get(0).([]services.Model), args.Error(1)
}

// TestAgentHandlersReliabilityFeatures tests the enhanced agent handlers
func TestAgentHandlersReliabilityFeatures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockAgentService := new(MockAgentService)
	mockRouterService := new(MockRouterService)
	
	h := handlers.NewAgentHandlers(mockAgentService, mockRouterService, nil, nil, nil, nil, nil, nil, false, 10)

	t.Run("CreateAgent with reliability configuration validation", func(t *testing.T) {
		// Setup mocks
		mockRouterService.On("ValidateConfig", mock.Anything, mock.AnythingOfType("models.AgentLLMConfig")).Return(nil)
		mockAgentService.On("CreateAgent", mock.Anything, mock.AnythingOfType("models.CreateAgentRequest"), mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).Return(&models.Agent{
			ID:   uuid.New(),
			Name: "Test Agent",
			LLMConfig: models.AgentLLMConfig{
				Provider:    "openai",
				Model:      "gpt-3.5-turbo",
				OptimizeFor: "reliability",
				RetryConfig: models.DefaultRetryConfig(),
			},
		}, nil)

		// Prepare request
		requestBody := models.CreateAgentRequest{
			Name:         "Reliability Test Agent",
			Description:  "Agent with reliability features",
			SystemPrompt: "You are a reliable assistant",
			LLMConfig: models.AgentLLMConfig{
				Provider:    "openai",
				Model:      "gpt-3.5-turbo",
				OptimizeFor: "reliability",
				RetryConfig: &models.RetryConfig{
					MaxAttempts: 3,
					BackoffType: "exponential",
					BaseDelay:   "1s",
					MaxDelay:    "30s",
				},
				FallbackConfig: &models.FallbackConfig{
					Enabled:         true,
					MaxCostIncrease: floatPtr(0.5),
				},
			},
			SpaceID: uuid.New(),
		}

		jsonBody, _ := json.Marshal(requestBody)
		
		// Create request
		req, _ := http.NewRequest("POST", "/agents", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		
		// Create response recorder
		w := httptest.NewRecorder()
		
		// Create gin context
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", uuid.New().String())
		c.Set("tenant_id", "test-tenant")

		// Call handler
		h.CreateAgent(c)

		// Assertions
		assert.Equal(t, http.StatusCreated, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		// Verify response structure
		assert.Contains(t, response, "agent")
		assert.Contains(t, response, "recommendations")
		
		// Verify recommendations are provided
		recommendations := response["recommendations"].(map[string]interface{})
		assert.Contains(t, recommendations, "retry_config")
		assert.Contains(t, recommendations, "fallback_config")
		assert.Contains(t, recommendations, "reason")

		mockAgentService.AssertExpectations(t)
		mockRouterService.AssertExpectations(t)
	})

	t.Run("ValidateAgentConfig endpoint", func(t *testing.T) {
		// Setup mocks
		mockRouterService.On("ValidateConfig", mock.Anything, mock.AnythingOfType("models.AgentLLMConfig")).Return(nil)
		mockRouterService.On("GetAvailableProviders", mock.Anything).Return([]services.Provider{
			{Name: "openai", Models: []string{"gpt-3.5-turbo", "gpt-4"}},
			{Name: "anthropic", Models: []string{"claude-3-sonnet-20240229"}},
		}, nil)

		// Prepare request
		requestBody := models.CreateAgentRequest{
			Name:         "Validation Test",
			SystemPrompt: "Test prompt",
			LLMConfig: models.AgentLLMConfig{
				Provider:    "openai",
				Model:      "gpt-3.5-turbo",
				OptimizeFor: "performance",
				RetryConfig: &models.RetryConfig{
					MaxAttempts: 2,
					BackoffType: "linear",
				},
			},
			SpaceID: uuid.New(),
		}

		jsonBody, _ := json.Marshal(requestBody)
		
		req, _ := http.NewRequest("POST", "/agents/validate", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		// Call handler
		h.ValidateAgentConfig(c)

		// Assertions
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.True(t, response["valid"].(bool))
		assert.Contains(t, response, "recommendations")
		assert.Contains(t, response, "providers")

		mockRouterService.AssertExpectations(t)
	})

	t.Run("ValidateAgentConfig with invalid configuration", func(t *testing.T) {
		// Setup mocks - router validation should fail
		mockRouterService.On("ValidateConfig", mock.Anything, mock.AnythingOfType("models.AgentLLMConfig")).Return(assert.AnError)

		// Invalid configuration
		requestBody := models.CreateAgentRequest{
			Name:         "Invalid Test",
			SystemPrompt: "Test prompt",
			LLMConfig: models.AgentLLMConfig{
				Provider: "invalid-provider",
				Model:    "invalid-model",
				RetryConfig: &models.RetryConfig{
					MaxAttempts: 10, // Invalid - too high
				},
			},
			SpaceID: uuid.New(),
		}

		jsonBody, _ := json.Marshal(requestBody)
		
		req, _ := http.NewRequest("POST", "/agents/validate", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		h.ValidateAgentConfig(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.False(t, response["valid"].(bool))
		assert.Contains(t, response, "error")

		mockRouterService.AssertExpectations(t)
	})

	t.Run("GetAgentReliabilityMetrics endpoint", func(t *testing.T) {
		agentID := uuid.New()
		userID := uuid.New()
		
		// Setup mocks
		mockAgentService.On("GetAgent", mock.Anything, agentID, userID).Return(&models.Agent{
			ID:      agentID,
			Name:    "Test Agent",
			OwnerID: userID,
		}, nil)

		// Mock reliability metrics response
		expectedMetrics := map[string]interface{}{
			"agent_id":              agentID.String(),
			"reliability_score":     0.95,
			"total_executions":      100,
			"successful_executions": 95,
			"failed_executions":     5,
			"avg_retry_attempts":    0.2,
			"fallback_usage_rate":   0.05,
			"avg_response_time_ms":  250,
		}
		
		mockAgentService.On("GetReliabilityMetrics", mock.Anything, agentID).Return(expectedMetrics, nil)

		req, _ := http.NewRequest("GET", "/agents/"+agentID.String()+"/reliability", nil)
		
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: agentID.String()}}
		c.Set("user_id", userID.String())

		h.GetAgentReliabilityMetrics(c)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, agentID.String(), response["agent_id"])
		assert.Equal(t, 0.95, response["reliability_score"])
		assert.Contains(t, response, "total_executions")
		assert.Contains(t, response, "avg_retry_attempts")

		mockAgentService.AssertExpectations(t)
	})

	t.Run("GetAgentConfigTemplates endpoint", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/agents/templates", nil)
		
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		h.GetAgentConfigTemplates(c)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Contains(t, response, "templates")
		templates := response["templates"].(map[string]interface{})
		
		// Verify all expected templates are present
		expectedTemplates := []string{"high_reliability", "cost_optimized", "performance"}
		for _, templateName := range expectedTemplates {
			assert.Contains(t, templates, templateName)
			
			template := templates[templateName].(map[string]interface{})
			assert.Contains(t, template, "name")
			assert.Contains(t, template, "description")
			assert.Contains(t, template, "llm_config")
			
			llmConfig := template["llm_config"].(map[string]interface{})
			assert.Contains(t, llmConfig, "provider")
			assert.Contains(t, llmConfig, "model")
			assert.Contains(t, llmConfig, "retry_config")
			assert.Contains(t, llmConfig, "fallback_config")
		}
	})
}

// TestRetryConfigValidationInHandler tests retry config validation in handlers
func TestRetryConfigValidationInHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		retryConfig    *models.RetryConfig
		expectValid    bool
		expectedStatus int
	}{
		{
			name: "Valid retry config",
			retryConfig: &models.RetryConfig{
				MaxAttempts: 3,
				BackoffType: "exponential",
				BaseDelay:   "1s",
				MaxDelay:    "30s",
			},
			expectValid:    true,
			expectedStatus: http.StatusOK,
		},
		{
			name: "Invalid max attempts",
			retryConfig: &models.RetryConfig{
				MaxAttempts: 6, // Too high
				BackoffType: "exponential",
			},
			expectValid:    false,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Invalid backoff type",
			retryConfig: &models.RetryConfig{
				MaxAttempts: 2,
				BackoffType: "invalid",
			},
			expectValid:    false,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Invalid delay format",
			retryConfig: &models.RetryConfig{
				MaxAttempts: 2,
				BackoffType: "linear",
				BaseDelay:   "invalid-delay",
			},
			expectValid:    false,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAgentService := new(MockAgentService)
			mockRouterService := new(MockRouterService)
			
			h := handlers.NewAgentHandlers(mockAgentService, mockRouterService, nil, nil, nil, nil, nil, nil, false, 10)

			// Only setup router validation mock if config should be valid
			if tt.expectValid {
				mockRouterService.On("ValidateConfig", mock.Anything, mock.AnythingOfType("models.AgentLLMConfig")).Return(nil)
				mockRouterService.On("GetAvailableProviders", mock.Anything).Return([]services.Provider{}, nil)
			}

			requestBody := models.CreateAgentRequest{
				Name:         "Test Agent",
				SystemPrompt: "Test prompt",
				LLMConfig: models.AgentLLMConfig{
					Provider:    "openai",
					Model:      "gpt-3.5-turbo",
					RetryConfig: tt.retryConfig,
				},
				SpaceID: uuid.New(),
			}

			jsonBody, _ := json.Marshal(requestBody)
			
			req, _ := http.NewRequest("POST", "/agents/validate", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			h.ValidateAgentConfig(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			
			assert.Equal(t, tt.expectValid, response["valid"])

			if tt.expectValid {
				mockRouterService.AssertExpectations(t)
			}
		})
	}
}

// TestFallbackConfigValidationInHandler tests fallback config validation in handlers
func TestFallbackConfigValidationInHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		fallbackConfig *models.FallbackConfig
		expectValid    bool
		expectedStatus int
	}{
		{
			name: "Valid fallback config",
			fallbackConfig: &models.FallbackConfig{
				Enabled:         true,
				MaxCostIncrease: floatPtr(0.5),
				PreferredChain:  []string{"openai", "anthropic"},
			},
			expectValid:    true,
			expectedStatus: http.StatusOK,
		},
		{
			name: "Invalid max cost increase - negative",
			fallbackConfig: &models.FallbackConfig{
				Enabled:         true,
				MaxCostIncrease: floatPtr(-0.1),
			},
			expectValid:    false,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Invalid max cost increase - too high",
			fallbackConfig: &models.FallbackConfig{
				Enabled:         true,
				MaxCostIncrease: floatPtr(2.5),
			},
			expectValid:    false,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAgentService := new(MockAgentService)
			mockRouterService := new(MockRouterService)
			
			h := handlers.NewAgentHandlers(mockAgentService, mockRouterService, nil, nil, nil, nil, nil, nil, false, 10)

			if tt.expectValid {
				mockRouterService.On("ValidateConfig", mock.Anything, mock.AnythingOfType("models.AgentLLMConfig")).Return(nil)
				mockRouterService.On("GetAvailableProviders", mock.Anything).Return([]services.Provider{
					{Name: "openai"}, {Name: "anthropic"},
				}, nil)
			}

			requestBody := models.CreateAgentRequest{
				Name:         "Test Agent",
				SystemPrompt: "Test prompt",
				LLMConfig: models.AgentLLMConfig{
					Provider:       "openai",
					Model:         "gpt-3.5-turbo",
					FallbackConfig: tt.fallbackConfig,
				},
				SpaceID: uuid.New(),
			}

			jsonBody, _ := json.Marshal(requestBody)
			
			req, _ := http.NewRequest("POST", "/agents/validate", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			h.ValidateAgentConfig(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			
			assert.Equal(t, tt.expectValid, response["valid"])

			if tt.expectValid {
				mockRouterService.AssertExpectations(t)
			}
		})
	}
}

