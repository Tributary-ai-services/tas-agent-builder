package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
	"github.com/tas-agent-builder/services/memory"
)

type AgentHandlers struct {
	agentService           services.AgentService
	routerService          services.RouterService
	executionService       services.ExecutionService
	documentContextService services.DocumentContextService
	cacheService           services.CacheService
	memoryService          *memory.MemoryServiceImpl
}

func NewAgentHandlers(
	agentService services.AgentService,
	routerService services.RouterService,
	executionService services.ExecutionService,
	documentContextService services.DocumentContextService,
	cacheService services.CacheService,
	memoryService *memory.MemoryServiceImpl,
) *AgentHandlers {
	return &AgentHandlers{
		agentService:           agentService,
		routerService:          routerService,
		executionService:       executionService,
		documentContextService: documentContextService,
		cacheService:           cacheService,
		memoryService:          memoryService,
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

	// Parse execution request - support both simple map and structured request
	var rawReq map[string]interface{}
	if err := c.ShouldBindJSON(&rawReq); err != nil {
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
	input, ok := rawReq["input"].(string)
	if !ok || input == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input is required"})
		return
	}

	// Build ExecutionContextRequest for document context retrieval
	contextReq := models.ExecutionContextRequest{
		Input: input,
	}

	// Extract auth token from Authorization header for downstream API calls
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		contextReq.AuthToken = strings.TrimPrefix(authHeader, "Bearer ")
		log.Printf("[DEBUG] Extracted auth token from header (length: %d chars)", len(contextReq.AuthToken))
	} else {
		log.Printf("[DEBUG] No Bearer token in Authorization header: %q", authHeader)
	}

	// Parse notebook_ids from request if provided
	if notebookIDsRaw, exists := rawReq["notebook_ids"]; exists {
		if notebookIDsSlice, ok := notebookIDsRaw.([]interface{}); ok {
			for _, nbID := range notebookIDsSlice {
				if nbIDStr, ok := nbID.(string); ok {
					if nbUUID, err := uuid.Parse(nbIDStr); err == nil {
						contextReq.NotebookIDs = append(contextReq.NotebookIDs, nbUUID)
					}
				}
			}
		}
	}

	// Parse selected_documents from request if provided
	if selectedDocsRaw, exists := rawReq["selected_documents"]; exists {
		log.Printf("[DEBUG] selected_documents raw value: %v (type: %T)", selectedDocsRaw, selectedDocsRaw)
		if selectedDocsSlice, ok := selectedDocsRaw.([]interface{}); ok {
			log.Printf("[DEBUG] selected_documents slice length: %d", len(selectedDocsSlice))
			for i, docID := range selectedDocsSlice {
				if docIDStr, ok := docID.(string); ok {
					if docUUID, err := uuid.Parse(docIDStr); err == nil {
						contextReq.SelectedDocuments = append(contextReq.SelectedDocuments, docUUID)
						log.Printf("[DEBUG] Parsed selected_document[%d]: %s -> %s", i, docIDStr, docUUID.String())
					} else {
						log.Printf("[DEBUG] Failed to parse selected_document[%d] as UUID: %s, error: %v", i, docIDStr, err)
					}
				} else {
					log.Printf("[DEBUG] selected_document[%d] is not a string: %v (type: %T)", i, docID, docID)
				}
			}
		} else {
			log.Printf("[DEBUG] selected_documents is not a slice: %v (type: %T)", selectedDocsRaw, selectedDocsRaw)
		}
	} else {
		log.Printf("[DEBUG] No selected_documents in request")
	}

	// Parse tenant_id from request if provided (needed for document retrieval)
	if tenantID, exists := rawReq["tenant_id"]; exists {
		if tenantIDStr, ok := tenantID.(string); ok {
			contextReq.TenantID = tenantIDStr
		}
	}

	// Build system prompt with document context if notebook IDs are provided
	var systemPrompt string
	var contextMetadata map[string]interface{}

	if len(contextReq.NotebookIDs) > 0 && contextReq.TenantID != "" {
		// Use document context retrieval
		systemPrompt, contextMetadata = h.buildSystemPromptWithContext(c.Request.Context(), agent, contextReq)
		log.Printf("Built system prompt with context: %d notebooks, metadata: %v", len(contextReq.NotebookIDs), contextMetadata)
	} else {
		// Fall back to agent's static system prompt
		systemPrompt = agent.SystemPrompt
		contextMetadata = map[string]interface{}{
			"knowledge_enabled": false,
			"reason":            "no notebook_ids or tenant_id provided",
		}
	}

	// Build messages for router service
	messages := []services.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	// Add conversation history if provided
	if history, exists := rawReq["history"]; exists {
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

	// Log the messages being sent to the router (internal agent)
	log.Printf("[DEBUG] === INTERNAL AGENT - MESSAGES BEING SENT TO ROUTER ===")
	log.Printf("[DEBUG] Total messages: %d", len(messages))
	for i, msg := range messages {
		contentPreview := msg.Content
		if len(contentPreview) > 500 {
			contentPreview = contentPreview[:500] + fmt.Sprintf("... [truncated, total %d chars]", len(msg.Content))
		}
		log.Printf("[DEBUG] Message[%d] role=%s, length=%d chars, preview: %s", i, msg.Role, len(msg.Content), contentPreview)
	}
	log.Printf("[DEBUG] === END INTERNAL AGENT MESSAGES DEBUG ===")

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
		"context_metadata": contextMetadata,
	}

	// Add session/conversation ID if provided
	if sid, ok := rawReq["session_id"].(string); ok && sid != "" {
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
	var req models.ExecutionContextRequest
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

	// Validate input
	if req.Input == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input is required"})
		return
	}

	// Build system prompt with document context
	systemPrompt, contextMetadata := h.buildSystemPromptWithContext(c.Request.Context(), agent, req)

	// Get tenant ID for memory operations
	tenantID, _ := c.Get("tenant_id")
	tenantStr, _ := tenantID.(string)

	// Build messages for router service
	messages := []services.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	// Load memory context if memory is enabled and session ID is provided
	var memoryContextAdded bool
	if agent.EnableMemory && h.memoryService != nil && req.SessionID != nil && *req.SessionID != "" {
		memoryReq := models.GetMemoryRequest{
			SessionID: *req.SessionID,
			AgentID:   agentID,
			TenantID:  tenantStr,
			UserID:    userStr,
			Query:     req.Input,
		}

		// Get formatted memory for context injection
		memoryCtx, err := h.memoryService.GetFormattedMemory(c.Request.Context(), memoryReq, 4000) // 4000 token budget for memory
		if err != nil {
			// Log but don't fail - memory is supplementary
			fmt.Printf("Warning: Failed to get memory context: %v\n", err)
		} else if memoryCtx != nil {
			// Add long-term memory context first (relevant accumulated knowledge)
			if memoryCtx.FormattedLongTerm != "" {
				messages = append(messages, services.Message{
					Role:    "system",
					Content: memoryCtx.FormattedLongTerm,
				})
			}
			// Add short-term memory (recent conversation) as messages
			if memoryCtx.FormattedShortTerm != "" {
				messages = append(messages, services.Message{
					Role:    "system",
					Content: "Recent conversation context:\n" + memoryCtx.FormattedShortTerm,
				})
			}
			memoryContextAdded = true
			contextMetadata["memory_enabled"] = true
			contextMetadata["memory_tokens"] = memoryCtx.TotalTokens
		}
	}

	// Add conversation history if provided (fallback if no memory)
	if !memoryContextAdded {
		for _, msg := range req.History {
			messages = append(messages, services.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Add current user input
	messages = append(messages, services.Message{
		Role:    "user",
		Content: req.Input,
	})

	// Convert user ID to UUID for router call and execution record
	userUUID, err := uuid.Parse(userStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Create execution record (status: running)
	inputJSON, _ := json.Marshal(req)
	executionReq := models.StartExecutionRequest{
		AgentID:   agentID,
		SessionID: req.SessionID,
		InputData: map[string]any{
			"input":            req.Input,
			"messages":         messages,
			"context_metadata": contextMetadata,
		},
	}

	execution, err := h.executionService.StartExecution(c.Request.Context(), executionReq, userUUID)
	if err != nil {
		// Log but don't fail - execution tracking is non-critical
		fmt.Printf("Failed to create execution record: %v\n", err)
	}

	// Log the messages being sent to the router
	log.Printf("[DEBUG] === MESSAGES BEING SENT TO ROUTER ===")
	log.Printf("[DEBUG] Total messages: %d", len(messages))
	for i, msg := range messages {
		contentPreview := msg.Content
		if len(contentPreview) > 500 {
			contentPreview = contentPreview[:500] + fmt.Sprintf("... [truncated, total %d chars]", len(msg.Content))
		}
		log.Printf("[DEBUG] Message[%d] role=%s, length=%d chars, preview: %s", i, msg.Role, len(msg.Content), contentPreview)
	}
	log.Printf("[DEBUG] === END MESSAGES DEBUG ===")

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
		"context_metadata": contextMetadata,
	}

	if execution != nil {
		outputJSON, _ := json.Marshal(outputData)
		_ = inputJSON // suppress unused warning
		_ = outputJSON
		h.executionService.CompleteExecution(c.Request.Context(), execution.ID, models.ExecutionStatusCompleted, outputData, nil, totalDuration)
	}

	// Store interaction in memory if enabled
	if agent.EnableMemory && h.memoryService != nil && req.SessionID != nil && *req.SessionID != "" {
		// Store user input
		userMemoryReq := models.AddMemoryRequest{
			SessionID: *req.SessionID,
			AgentID:   agentID,
			TenantID:  tenantStr,
			UserID:    userStr,
			Role:      "user",
			Content:   req.Input,
		}
		if err := h.memoryService.AddMemory(c.Request.Context(), userMemoryReq); err != nil {
			fmt.Printf("Warning: Failed to store user input in memory: %v\n", err)
		}

		// Store assistant response
		assistantMemoryReq := models.AddMemoryRequest{
			SessionID: *req.SessionID,
			AgentID:   agentID,
			TenantID:  tenantStr,
			UserID:    userStr,
			Role:      "assistant",
			Content:   response.Content,
			Metadata: map[string]interface{}{
				"model":      response.Model,
				"provider":   response.Provider,
				"tokens":     response.TokenUsage,
				"cost_usd":   response.CostUSD,
			},
		}
		if err := h.memoryService.AddMemory(c.Request.Context(), assistantMemoryReq); err != nil {
			fmt.Printf("Warning: Failed to store assistant response in memory: %v\n", err)
		}
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
			"context_metadata": contextMetadata,
		},
	}

	c.JSON(http.StatusOK, executionResponse)
}

// buildSystemPrompt creates a system prompt based on agent configuration (without document context)
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

// buildSystemPromptWithContext creates a system prompt with document context injection
func (h *AgentHandlers) buildSystemPromptWithContext(ctx context.Context, agent *models.Agent, req models.ExecutionContextRequest) (string, map[string]interface{}) {
	metadata := make(map[string]interface{})

	// Start with base prompt
	basePrompt := h.buildSystemPrompt(agent)

	// Use custom system prompt if defined
	if agent.SystemPrompt != "" {
		basePrompt = agent.SystemPrompt
	}

	// Check if knowledge retrieval is enabled and not disabled for this execution
	if !agent.EnableKnowledge || req.DisableKnowledge {
		metadata["knowledge_enabled"] = false
		return basePrompt, metadata
	}

	// Get notebook IDs - prefer request notebook IDs over agent's static IDs
	var notebookIDs []uuid.UUID
	if len(req.NotebookIDs) > 0 {
		// Use notebook IDs from request (e.g., when executing producer on a specific notebook)
		notebookIDs = req.NotebookIDs
		metadata["notebook_source"] = "request"
	} else {
		// Fall back to agent's configured notebook IDs
		notebookIDs = h.parseNotebookIDs(agent)
		metadata["notebook_source"] = "agent"
	}

	if len(notebookIDs) == 0 {
		metadata["knowledge_enabled"] = true
		metadata["no_notebooks"] = true
		return basePrompt, metadata
	}

	// Determine context strategy
	strategy := h.getContextStrategy(agent)
	metadata["strategy"] = string(strategy)

	var contextResult *models.DocumentContextResult
	var err error

	switch strategy {
	case models.ContextStrategyVector:
		contextResult, err = h.retrieveVectorContext(ctx, agent, req, notebookIDs)
	case models.ContextStrategyFull:
		contextResult, err = h.retrieveFullContext(ctx, agent, req, notebookIDs)
	case models.ContextStrategyHybrid:
		contextResult, err = h.retrieveHybridContext(ctx, agent, req, notebookIDs)
	case models.ContextStrategyNone:
		metadata["knowledge_enabled"] = false
		return basePrompt, metadata
	default:
		// Default to vector search for Q&A and conversational agents
		contextResult, err = h.retrieveVectorContext(ctx, agent, req, notebookIDs)
	}

	if err != nil {
		log.Printf("Error retrieving document context: %v", err)
		metadata["context_error"] = err.Error()
		return basePrompt, metadata
	}

	if contextResult == nil || len(contextResult.Chunks) == 0 {
		metadata["context_empty"] = true
		return basePrompt, metadata
	}

	// Format context for injection
	maxTokens := h.getMaxContextTokens(agent)
	contextInjection, err := h.documentContextService.FormatContextForInjection(contextResult, maxTokens)
	if err != nil {
		log.Printf("Error formatting context for injection: %v", err)
		metadata["format_error"] = err.Error()
		return basePrompt, metadata
	}

	// Inject context into prompt
	enhancedPrompt := basePrompt + "\n\n" + contextInjection.FormattedContext

	// Log what's being sent to the LLM for debugging
	log.Printf("[DEBUG] === SYSTEM PROMPT BEING SENT TO LLM ===")
	log.Printf("[DEBUG] Base prompt length: %d chars", len(basePrompt))
	log.Printf("[DEBUG] Context injection length: %d chars", len(contextInjection.FormattedContext))
	log.Printf("[DEBUG] Total enhanced prompt length: %d chars", len(enhancedPrompt))

	// Show preview of base prompt
	basePreview := basePrompt
	if len(basePreview) > 500 {
		basePreview = basePreview[:500] + "..."
	}
	log.Printf("[DEBUG] Base prompt preview:\n%s", basePreview)

	// Show preview of context (first 2000 chars)
	contextPreview := contextInjection.FormattedContext
	if len(contextPreview) > 2000 {
		contextPreview = contextPreview[:2000] + "\n... [TRUNCATED - showing first 2000 of " + fmt.Sprintf("%d", len(contextInjection.FormattedContext)) + " chars]"
	}
	log.Printf("[DEBUG] Context injection preview:\n%s", contextPreview)
	log.Printf("[DEBUG] === END SYSTEM PROMPT DEBUG ===")

	// Add metadata about context
	metadata["knowledge_enabled"] = true
	metadata["chunk_count"] = contextInjection.ChunkCount
	metadata["document_count"] = contextInjection.DocumentCount
	metadata["total_tokens"] = contextInjection.TotalTokens
	metadata["truncated"] = contextInjection.Truncated
	metadata["retrieval_time_ms"] = contextResult.RetrievalTimeMs

	return enhancedPrompt, metadata
}

// parseNotebookIDs extracts notebook UUIDs from the agent's NotebookIDs JSON field
func (h *AgentHandlers) parseNotebookIDs(agent *models.Agent) []uuid.UUID {
	if agent.NotebookIDs == nil {
		return nil
	}

	var notebookIDs []uuid.UUID
	if err := json.Unmarshal(agent.NotebookIDs, &notebookIDs); err != nil {
		log.Printf("Error parsing notebook IDs: %v", err)
		return nil
	}

	return notebookIDs
}

// getContextStrategy determines the context strategy based on agent configuration
func (h *AgentHandlers) getContextStrategy(agent *models.Agent) models.ContextStrategy {
	// Check document context config first
	if agent.DocumentContext != nil && agent.DocumentContext.Strategy != "" {
		return agent.DocumentContext.Strategy
	}

	// Default strategy based on agent type
	switch agent.Type {
	case models.AgentTypeQA:
		return models.ContextStrategyVector
	case models.AgentTypeConversational:
		return models.ContextStrategyVector
	case models.AgentTypeProducer:
		return models.ContextStrategyFull
	default:
		return models.ContextStrategyVector
	}
}

// getMaxContextTokens returns the maximum tokens for context injection
func (h *AgentHandlers) getMaxContextTokens(agent *models.Agent) int {
	if agent.DocumentContext != nil && agent.DocumentContext.MaxContextTokens > 0 {
		return agent.DocumentContext.MaxContextTokens
	}
	return 8000 // Default
}

// retrieveVectorContext retrieves context using vector search
func (h *AgentHandlers) retrieveVectorContext(ctx context.Context, agent *models.Agent, req models.ExecutionContextRequest, notebookIDs []uuid.UUID) (*models.DocumentContextResult, error) {
	topK := 10
	minScore := 0.7

	if agent.DocumentContext != nil {
		if agent.DocumentContext.TopK > 0 {
			topK = agent.DocumentContext.TopK
		}
		if agent.DocumentContext.MinScore > 0 {
			minScore = agent.DocumentContext.MinScore
		}
	}

	// Use tenant ID from request (for internal agents) or fall back to agent's tenant ID
	tenantID := req.TenantID
	if tenantID == "" {
		tenantID = agent.TenantID
	}

	searchReq := models.VectorSearchRequest{
		QueryText:   req.Input,
		NotebookIDs: notebookIDs,
		TenantID:    tenantID,
		SpaceID:     agent.SpaceID,
		AuthToken:   req.AuthToken, // Pass auth token for downstream API calls
		Options: models.SearchOptions{
			TopK:          topK,
			MinScore:      minScore,
			IncludeChunks: true,
		},
	}

	// Use selected documents if provided
	if len(req.SelectedDocuments) > 0 {
		searchReq.DocumentIDs = req.SelectedDocuments
	}

	return h.documentContextService.RetrieveVectorContext(ctx, searchReq)
}

// retrieveFullContext retrieves full document content
func (h *AgentHandlers) retrieveFullContext(ctx context.Context, agent *models.Agent, req models.ExecutionContextRequest, notebookIDs []uuid.UUID) (*models.DocumentContextResult, error) {
	// Use tenant ID from request (for internal agents) or fall back to agent's tenant ID
	tenantID := req.TenantID
	if tenantID == "" {
		tenantID = agent.TenantID
	}

	log.Printf("[DEBUG] retrieveFullContext: tenant_id=%s, notebook_ids=%d, selected_documents=%d, auth_token_length=%d",
		tenantID, len(notebookIDs), len(req.SelectedDocuments), len(req.AuthToken))

	chunkReq := models.ChunkRetrievalRequest{
		TenantID:    tenantID,
		NotebookIDs: notebookIDs,
		AuthToken:   req.AuthToken, // Pass auth token for AudiModal API
	}

	// Use selected documents if provided
	if len(req.SelectedDocuments) > 0 {
		chunkReq.FileIDs = req.SelectedDocuments
		log.Printf("[DEBUG] retrieveFullContext: using %d selected documents as FileIDs", len(req.SelectedDocuments))
		for i, docID := range req.SelectedDocuments {
			log.Printf("[DEBUG] retrieveFullContext: FileID[%d]=%s", i, docID.String())
		}
	} else {
		log.Printf("[DEBUG] retrieveFullContext: no selected documents provided")
	}

	return h.documentContextService.RetrieveFullDocuments(ctx, chunkReq)
}

// retrieveHybridContext retrieves context using hybrid approach
func (h *AgentHandlers) retrieveHybridContext(ctx context.Context, agent *models.Agent, req models.ExecutionContextRequest, notebookIDs []uuid.UUID) (*models.DocumentContextResult, error) {
	vectorWeight := 0.5
	fullDocWeight := 0.5

	if agent.DocumentContext != nil {
		if agent.DocumentContext.VectorWeight > 0 {
			vectorWeight = agent.DocumentContext.VectorWeight
		}
		if agent.DocumentContext.FullDocWeight > 0 {
			fullDocWeight = agent.DocumentContext.FullDocWeight
		}
	}

	// Use tenant ID from request (for internal agents) or fall back to agent's tenant ID
	tenantID := req.TenantID
	if tenantID == "" {
		tenantID = agent.TenantID
	}

	chunkReq := models.ChunkRetrievalRequest{
		TenantID:    tenantID,
		NotebookIDs: notebookIDs,
		AuthToken:   req.AuthToken, // Pass auth token for AudiModal API
	}

	// Use selected documents if provided
	if len(req.SelectedDocuments) > 0 {
		chunkReq.FileIDs = req.SelectedDocuments
	}

	return h.documentContextService.RetrieveHybridContext(ctx, req.Input, chunkReq, vectorWeight, fullDocWeight)
}

