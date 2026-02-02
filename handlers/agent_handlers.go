package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
)

type AgentHandlers struct {
	agentService     services.AgentService
	routerService    services.RouterService
	executionService services.ExecutionService
}

func NewAgentHandlers(agentService services.AgentService, routerService services.RouterService, executionService services.ExecutionService) *AgentHandlers {
	return &AgentHandlers{
		agentService:     agentService,
		routerService:    routerService,
		executionService: executionService,
	}
}

func (h *AgentHandlers) CreateAgent(c *gin.Context) {
	var req models.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Validate LLM configuration with router
	if err := h.validateLLMConfig(c.Request.Context(), req.LLMConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid LLM configuration", "details": err.Error()})
		return
	}

	ownerID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	tenantID, exists := c.Get("tenant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant ID not found in context"})
		return
	}

	ownerStr, ok := ownerID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid owner ID"})
		return
	}

	tenantStr, ok := tenantID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	agent, err := h.agentService.CreateAgent(c.Request.Context(), req, ownerStr, tenantStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create agent", "details": err.Error()})
		return
	}

	// Include configuration recommendations in response
	recommendations := h.generateConfigRecommendations(req.LLMConfig)
	response := gin.H{
		"agent":           agent,
		"recommendations": recommendations,
	}

	c.JSON(http.StatusCreated, response)
}

// Enhanced reliability handler functions

// validateLLMConfig validates the LLM configuration with the router
func (h *AgentHandlers) validateLLMConfig(ctx context.Context, config models.AgentLLMConfig) error {
	// Validate basic configuration
	if err := h.routerService.ValidateConfig(ctx, config); err != nil {
		return fmt.Errorf("router validation failed: %w", err)
	}

	// Validate retry configuration if present
	if config.RetryConfig != nil {
		if err := h.validateRetryConfig(*config.RetryConfig); err != nil {
			return fmt.Errorf("invalid retry config: %w", err)
		}
	}

	// Validate fallback configuration if present
	if config.FallbackConfig != nil {
		if err := h.validateFallbackConfig(ctx, *config.FallbackConfig); err != nil {
			return fmt.Errorf("invalid fallback config: %w", err)
		}
	}

	return nil
}

// validateRetryConfig validates retry configuration parameters
func (h *AgentHandlers) validateRetryConfig(config models.RetryConfig) error {
	if config.MaxAttempts < 1 || config.MaxAttempts > 5 {
		return fmt.Errorf("max_attempts must be between 1 and 5")
	}

	if config.BackoffType != "" && config.BackoffType != "exponential" && config.BackoffType != "linear" {
		return fmt.Errorf("backoff_type must be 'exponential' or 'linear'")
	}

	// Validate delay formats
	if config.BaseDelay != "" {
		if _, err := time.ParseDuration(config.BaseDelay); err != nil {
			return fmt.Errorf("invalid base_delay format: %w", err)
		}
	}

	if config.MaxDelay != "" {
		if _, err := time.ParseDuration(config.MaxDelay); err != nil {
			return fmt.Errorf("invalid max_delay format: %w", err)
		}
	}

	return nil
}

// validateFallbackConfig validates fallback configuration parameters
func (h *AgentHandlers) validateFallbackConfig(ctx context.Context, config models.FallbackConfig) error {
	if config.MaxCostIncrease != nil && (*config.MaxCostIncrease < 0 || *config.MaxCostIncrease > 2.0) {
		return fmt.Errorf("max_cost_increase must be between 0 and 2.0 (0-200%%)")
	}

	// Validate preferred chain providers are available
	if len(config.PreferredChain) > 0 {
		providers, err := h.routerService.GetAvailableProviders(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate providers: %w", err)
		}

		providerMap := make(map[string]bool)
		for _, provider := range providers {
			providerMap[provider.Name] = true
		}

		for _, providerName := range config.PreferredChain {
			if !providerMap[providerName] {
				return fmt.Errorf("provider '%s' in preferred_chain is not available", providerName)
			}
		}
	}

	return nil
}

// generateConfigRecommendations provides intelligent configuration recommendations
func (h *AgentHandlers) generateConfigRecommendations(config models.AgentLLMConfig) map[string]interface{} {
	recommendations := make(map[string]interface{})

	// Recommend reliability configuration based on optimize_for setting
	switch config.OptimizeFor {
	case "performance":
		retryConfig, fallbackConfig := models.PerformanceOptimizedConfig()
		recommendations["retry_config"] = retryConfig
		recommendations["fallback_config"] = fallbackConfig
		recommendations["reason"] = "Performance-optimized configuration with minimal retry delays"

	case "cost":
		retryConfig, fallbackConfig := models.CostOptimizedConfig()
		recommendations["retry_config"] = retryConfig
		recommendations["fallback_config"] = fallbackConfig
		recommendations["reason"] = "Cost-optimized configuration with conservative retry and fallback"

	default:
		retryConfig, fallbackConfig := models.HighReliabilityConfig()
		recommendations["retry_config"] = retryConfig
		recommendations["fallback_config"] = fallbackConfig
		recommendations["reason"] = "High-reliability configuration for maximum uptime"
	}

	// Add provider-specific recommendations
	providerRecommendations := make(map[string]string)
	switch config.Provider {
	case "openai":
		providerRecommendations["model_upgrade"] = "Consider upgrading to gpt-4o for better performance"
		providerRecommendations["features"] = "OpenAI supports function calling and vision capabilities"
	case "anthropic":
		providerRecommendations["model_latest"] = "Claude 3.5 Sonnet offers the best balance of speed and capability"
		providerRecommendations["context"] = "Claude models support very large context windows (200k tokens)"
	}
	recommendations["provider_tips"] = providerRecommendations

	return recommendations
}

// GetAgentReliabilityMetrics returns reliability metrics for an agent
func (h *AgentHandlers) GetAgentReliabilityMetrics(c *gin.Context) {
	idParam := c.Param("id")
	agentID, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	userStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Verify agent access
	_, err = h.agentService.GetAgent(c.Request.Context(), agentID, userStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// Return placeholder metrics for now
	metrics := gin.H{
		"agent_id":              agentID,
		"reliability_score":     0.95,
		"total_executions":      100,
		"successful_executions": 95,
		"failed_executions":     5,
		"avg_retry_attempts":    0.2,
		"fallback_usage_rate":   0.05,
		"avg_response_time_ms":  250,
		"last_30_days": gin.H{
			"executions":      75,
			"success_rate":    0.96,
			"avg_cost_usd":    0.002,
		},
	}

	c.JSON(http.StatusOK, metrics)
}

// ValidateAgentConfig validates an agent configuration without creating it
func (h *AgentHandlers) ValidateAgentConfig(c *gin.Context) {
	var req models.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Validate LLM configuration
	if err := h.validateLLMConfig(c.Request.Context(), req.LLMConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid":   false,
			"error":   err.Error(),
			"details": "Configuration validation failed",
		})
		return
	}

	// Get configuration recommendations
	recommendations := h.generateConfigRecommendations(req.LLMConfig)

	// Get provider availability
	providers, err := h.routerService.GetAvailableProviders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check provider availability"})
		return
	}

	response := gin.H{
		"valid":           true,
		"message":         "Configuration is valid",
		"recommendations": recommendations,
		"providers":       providers,
	}

	c.JSON(http.StatusOK, response)
}

// GetUserStats returns statistics for the current user
func (h *AgentHandlers) GetUserStats(c *gin.Context) {
	userID, _ := c.Get("user_id")
	
	// For now, return basic stats structure
	stats := gin.H{
		"total_agents":       0,
		"total_executions":   0,
		"total_cost_usd":     0.0,
		"avg_response_time_ms": 0,
		"executions_today":   0,
		"executions_week":    0,
		"executions_month":   0,
		"cost_today":         0.0,
		"cost_week":          0.0,
		"cost_month":         0.0,
		"active_sessions":    0,
		"user_id":            userID,
	}
	
	c.JSON(http.StatusOK, stats)
}

// GetAgentConfigTemplates returns pre-configured agent templates
func (h *AgentHandlers) GetAgentConfigTemplates(c *gin.Context) {
	templates := map[string]interface{}{
		"high_reliability": map[string]interface{}{
			"name":        "High Reliability Agent",
			"description": "Optimized for maximum uptime with aggressive retry and fallback",
			"llm_config": map[string]interface{}{
				"provider":      "openai",
				"model":         "gpt-3.5-turbo",
				"optimize_for":  "reliability",
			},
		},
		"cost_optimized": map[string]interface{}{
			"name":        "Cost Optimized Agent",
			"description": "Balanced performance with cost efficiency",
			"llm_config": map[string]interface{}{
				"provider":     "openai",
				"model":        "gpt-3.5-turbo",
				"optimize_for": "cost",
				"max_cost":     0.01,
			},
		},
		"performance": map[string]interface{}{
			"name":        "Performance Agent",
			"description": "Optimized for speed and low latency",
			"llm_config": map[string]interface{}{
				"provider":     "openai",
				"model":        "gpt-3.5-turbo",
				"optimize_for": "performance",
				"temperature":  0.3,
			},
		},
	}

	// Add reliability configurations
	retryHigh, fallbackHigh := models.HighReliabilityConfig()
	retryCost, fallbackCost := models.CostOptimizedConfig()
	retryPerf, fallbackPerf := models.PerformanceOptimizedConfig()

	templates["high_reliability"].(map[string]interface{})["llm_config"].(map[string]interface{})["retry_config"] = retryHigh
	templates["high_reliability"].(map[string]interface{})["llm_config"].(map[string]interface{})["fallback_config"] = fallbackHigh

	templates["cost_optimized"].(map[string]interface{})["llm_config"].(map[string]interface{})["retry_config"] = retryCost
	templates["cost_optimized"].(map[string]interface{})["llm_config"].(map[string]interface{})["fallback_config"] = fallbackCost

	templates["performance"].(map[string]interface{})["llm_config"].(map[string]interface{})["retry_config"] = retryPerf
	templates["performance"].(map[string]interface{})["llm_config"].(map[string]interface{})["fallback_config"] = fallbackPerf

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
	})
}

func (h *AgentHandlers) GetAgent(c *gin.Context) {
	idParam := c.Param("id")
	agentID, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	userStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	agent, err := h.agentService.GetAgent(c.Request.Context(), agentID, userStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, agent)
}

// GetInternalAgents returns all internal (system) agents
// These are available to all users regardless of ownership
func (h *AgentHandlers) GetInternalAgents(c *gin.Context) {
	agents, err := h.agentService.GetInternalAgents(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch internal agents", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"count":  len(agents),
	})
}

// GetInternalAgent returns a specific internal agent by ID
func (h *AgentHandlers) GetInternalAgent(c *gin.Context) {
	idParam := c.Param("id")
	agentID, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	agent, err := h.agentService.GetInternalAgent(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, agent)
}

// ExecuteInternalAgent executes an internal agent without requiring space context
func (h *AgentHandlers) ExecuteInternalAgent(c *gin.Context) {
	startTime := time.Now()

	idParam := c.Param("id")
	agentID, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	userStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Parse execution request
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Get the internal agent (no ownership check needed)
	agent, err := h.agentService.GetInternalAgent(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Internal agent not found"})
		return
	}

	// Extract input from request
	input, ok := req["input"].(string)
	if !ok || input == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input is required"})
		return
	}

	// Build messages for router service
	messages := []services.Message{
		{
			Role:    "system",
			Content: agent.SystemPrompt,
		},
	}

	// Add conversation history if provided
	if history, exists := req["history"]; exists {
		if historySlice, ok := history.([]interface{}); ok {
			for _, msg := range historySlice {
				if msgMap, ok := msg.(map[string]interface{}); ok {
					role, _ := msgMap["role"].(string)
					content, _ := msgMap["content"].(string)
					if role != "" && content != "" {
						messages = append(messages, services.Message{Role: role, Content: content})
					}
				}
			}
		}
	}

	// Add the current user message
	messages = append(messages, services.Message{
		Role:    "user",
		Content: input,
	})

	// Convert user ID to UUID for router call
	userUUID, err := uuid.Parse(userStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Call router service
	response, err := h.routerService.SendRequest(c.Request.Context(), agent.LLMConfig, messages, userUUID)

	// Calculate total duration
	totalDuration := int(time.Since(startTime).Milliseconds())

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Execution failed", "details": err.Error()})
		return
	}

	// Build execution response
	executionID := uuid.New()

	executionResponse := gin.H{
		"execution_id": executionID.String(),
		"output":       response.Content,
		"tokens_used":  response.TokenUsage,
		"cost_usd":     response.CostUSD,
		"metadata": gin.H{
			"model":            response.Model,
			"provider":         response.Provider,
			"routing_strategy": response.RoutingStrategy,
			"response_time_ms": response.ResponseTimeMs,
			"total_time_ms":    totalDuration,
		},
	}

	// Add session/conversation ID if provided
	if sid, ok := req["session_id"].(string); ok && sid != "" {
		executionResponse["conversation_id"] = sid
	} else {
		// Generate a new conversation ID
		executionResponse["conversation_id"] = uuid.New().String()
	}

	c.JSON(http.StatusOK, executionResponse)
}

func (h *AgentHandlers) UpdateAgent(c *gin.Context) {
	idParam := c.Param("id")
	agentID, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	var req models.UpdateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	ownerID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	ownerStr, ok := ownerID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid owner ID"})
		return
	}

	agent, err := h.agentService.UpdateAgent(c.Request.Context(), agentID, req, ownerStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agent", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, agent)
}

func (h *AgentHandlers) DeleteAgent(c *gin.Context) {
	idParam := c.Param("id")
	agentID, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	ownerID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	ownerStr, ok := ownerID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid owner ID"})
		return
	}

	err = h.agentService.DeleteAgent(c.Request.Context(), agentID, ownerStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete agent", "details": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *AgentHandlers) ListAgents(c *gin.Context) {
	var filter models.AgentListFilter

	if ownerIDStr := c.Query("owner_id"); ownerIDStr != "" {
		ownerID, err := uuid.Parse(ownerIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid owner_id"})
			return
		}
		filter.OwnerID = &ownerID
	}

	if spaceIDStr := c.Query("space_id"); spaceIDStr != "" {
		spaceID, err := uuid.Parse(spaceIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid space_id"})
			return
		}
		filter.SpaceID = &spaceID
	}

	if tenantIDStr := c.Query("tenant_id"); tenantIDStr != "" {
		tenantID, err := uuid.Parse(tenantIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant_id"})
			return
		}
		filter.TenantID = &tenantID
	}

	if statusStr := c.Query("status"); statusStr != "" {
		status := models.AgentStatus(statusStr)
		filter.Status = &status
	}

	if spaceTypeStr := c.Query("space_type"); spaceTypeStr != "" {
		spaceType := models.SpaceType(spaceTypeStr)
		filter.SpaceType = &spaceType
	}

	if isPublicStr := c.Query("is_public"); isPublicStr != "" {
		isPublic := isPublicStr == "true"
		filter.IsPublic = &isPublic
	}

	if isTemplateStr := c.Query("is_template"); isTemplateStr != "" {
		isTemplate := isTemplateStr == "true"
		filter.IsTemplate = &isTemplate
	}

	if tagsStr := c.Query("tags"); tagsStr != "" {
		var tags []string
		if err := json.Unmarshal([]byte(tagsStr), &tags); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tags format"})
			return
		}
		filter.Tags = tags
	}

	filter.Search = c.Query("search")

	if pageStr := c.Query("page"); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page parameter"})
			return
		}
		filter.Page = page
	} else {
		filter.Page = 1
	}

	if sizeStr := c.Query("size"); sizeStr != "" {
		size, err := strconv.Atoi(sizeStr)
		if err != nil || size < 1 || size > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid size parameter (must be 1-100)"})
			return
		}
		filter.Size = size
	} else {
		filter.Size = 20
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	userStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	response, err := h.agentService.ListAgents(c.Request.Context(), filter, userStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list agents", "details": err.Error()})
		return
	}

	// DEBUG: Log agent types before returning
	for i, agent := range response.Agents {
		log.Printf("DEBUG: Agent[%d] id=%s name=%s type=%s status=%s", i, agent.ID.String(), agent.Name, agent.Type, agent.Status)
	}

	// DEBUG: Also log the raw JSON that will be sent
	jsonBytes, _ := json.Marshal(response)
	truncLen := len(jsonBytes)
	if truncLen > 1000 {
		truncLen = 1000
	}
	log.Printf("DEBUG: Raw response JSON (first %d chars): %s", truncLen, string(jsonBytes)[:truncLen])

	c.JSON(http.StatusOK, response)
}

func (h *AgentHandlers) PublishAgent(c *gin.Context) {
	idParam := c.Param("id")
	agentID, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	ownerID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	ownerStr, ok := ownerID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid owner ID"})
		return
	}

	err = h.agentService.PublishAgent(c.Request.Context(), agentID, ownerStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to publish agent", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Agent published successfully"})
}

func (h *AgentHandlers) UnpublishAgent(c *gin.Context) {
	idParam := c.Param("id")
	agentID, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	ownerID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	ownerStr, ok := ownerID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid owner ID"})
		return
	}

	err = h.agentService.UnpublishAgent(c.Request.Context(), agentID, ownerStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unpublish agent", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Agent unpublished successfully"})
}

func (h *AgentHandlers) DuplicateAgent(c *gin.Context) {
	idParam := c.Param("id")
	sourceID, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid source agent ID"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	tenantID, exists := c.Get("tenant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant ID not found in context"})
		return
	}

	userStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	tenantStr, ok := tenantID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	agent, err := h.agentService.DuplicateAgent(c.Request.Context(), sourceID, req.Name, userStr, tenantStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to duplicate agent", "details": err.Error()})
		return
	}

	// Include configuration recommendations for duplicated agent
	recommendations := h.generateConfigRecommendations(agent.LLMConfig)
	response := gin.H{
		"agent":           agent,
		"recommendations": recommendations,
	}

	c.JSON(http.StatusCreated, response)
}

func (h *AgentHandlers) ExecuteAgent(c *gin.Context) {
	startTime := time.Now()

	idParam := c.Param("id")
	agentID, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	userStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Parse execution request
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Verify agent access
	agent, err := h.agentService.GetAgent(c.Request.Context(), agentID, userStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// Extract input from request
	input, ok := req["input"].(string)
	if !ok || input == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input is required"})
		return
	}

	// Build messages for router service
	messages := []services.Message{
		{
			Role:    "system",
			Content: h.buildSystemPrompt(agent),
		},
		{
			Role:    "user",
			Content: input,
		},
	}

	// Add conversation history if provided
	if history, exists := req["history"]; exists {
		if historySlice, ok := history.([]interface{}); ok {
			for _, msg := range historySlice {
				if msgMap, ok := msg.(map[string]interface{}); ok {
					if role, ok := msgMap["role"].(string); ok {
						if content, ok := msgMap["content"].(string); ok {
							messages = append([]services.Message{{Role: role, Content: content}}, messages[len(messages)-1])
						}
					}
				}
			}
		}
	}

	// Convert user ID to UUID for router call and execution record
	userUUID, err := uuid.Parse(userStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Extract session ID if provided
	var sessionID *string
	if sid, ok := req["session_id"].(string); ok && sid != "" {
		sessionID = &sid
	}

	// Create execution record (status: running)
	inputJSON, _ := json.Marshal(req)
	executionReq := models.StartExecutionRequest{
		AgentID:   agentID,
		SessionID: sessionID,
		InputData: map[string]any{
			"input":    input,
			"messages": messages,
		},
	}

	execution, err := h.executionService.StartExecution(c.Request.Context(), executionReq, userUUID)
	if err != nil {
		// Log but don't fail - execution tracking is non-critical
		fmt.Printf("Failed to create execution record: %v\n", err)
	}

	// Call router service
	response, err := h.routerService.SendRequest(c.Request.Context(), agent.LLMConfig, messages, userUUID)

	// Calculate total duration
	totalDuration := int(time.Since(startTime).Milliseconds())

	if err != nil {
		// Update execution with failure
		if execution != nil {
			errorMsg := err.Error()
			h.executionService.CompleteExecution(c.Request.Context(), execution.ID, models.ExecutionStatusFailed, nil, &errorMsg, totalDuration)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Execution failed", "details": err.Error()})
		return
	}

	// Update execution with success
	outputData := map[string]any{
		"content":          response.Content,
		"tokens_used":      response.TokenUsage,
		"cost_usd":         response.CostUSD,
		"model":            response.Model,
		"provider":         response.Provider,
		"routing_strategy": response.RoutingStrategy,
		"response_time_ms": response.ResponseTimeMs,
	}

	if execution != nil {
		outputJSON, _ := json.Marshal(outputData)
		_ = inputJSON // suppress unused warning
		_ = outputJSON
		h.executionService.CompleteExecution(c.Request.Context(), execution.ID, models.ExecutionStatusCompleted, outputData, nil, totalDuration)
	}

	// Build execution response
	executionID := uuid.New()
	if execution != nil {
		executionID = execution.ID
	}

	executionResponse := gin.H{
		"execution_id": executionID.String(),
		"output":       response.Content,
		"tokens_used":  response.TokenUsage,
		"cost_usd":     response.CostUSD,
		"metadata": gin.H{
			"model":            response.Model,
			"provider":         response.Provider,
			"routing_strategy": response.RoutingStrategy,
			"response_time_ms": response.ResponseTimeMs,
		},
	}

	c.JSON(http.StatusOK, executionResponse)
}

// buildSystemPrompt creates a system prompt based on agent configuration
func (h *AgentHandlers) buildSystemPrompt(agent *models.Agent) string {
	basePrompt := "You are a helpful AI assistant."
	
	// Add agent-specific configuration
	if agent.Name != "" {
		basePrompt = fmt.Sprintf("You are %s, an AI assistant.", agent.Name)
	}
	
	if agent.Description != "" {
		basePrompt += fmt.Sprintf(" %s", agent.Description)
	}
	
	// Add default behavior guidance
	basePrompt += " Provide helpful, accurate, and well-structured responses."
	
	return basePrompt
}

