package impl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
)

type routerServiceImpl struct {
	config           *config.RouterConfig
	httpClient       *http.Client
	modelLimitsCache map[string]int // Cache for model max_output_tokens
}

func NewRouterService(cfg *config.RouterConfig) services.RouterService {
	return &routerServiceImpl{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
		modelLimitsCache: make(map[string]int),
	}
}

func (s *routerServiceImpl) SendRequest(ctx context.Context, agentConfig models.AgentLLMConfig, messages []services.Message, userID uuid.UUID) (*services.RouterResponse, error) {
	// Cap max_tokens to model-specific limits from router to prevent API errors
	maxTokens := s.capMaxTokensForModel(ctx, agentConfig.MaxTokens, agentConfig.Model)

	// Build router request
	request := RouterRequest{
		Model:            agentConfig.Model,
		Messages:         make([]RouterMessage, len(messages)),
		Temperature:      agentConfig.Temperature,
		MaxTokens:        maxTokens,
		TopP:             agentConfig.TopP,
		Stop:             agentConfig.Stop,
		OptimizeFor:      "cost", // Default optimization
		RequiredFeatures: agentConfig.RequiredFeatures,
		MaxCost:          agentConfig.MaxCost,
		RetryConfig:      buildRetryConfig(agentConfig.RetryConfig),
		FallbackConfig:   buildFallbackConfig(agentConfig.FallbackConfig),
	}

	// Convert messages
	for i, msg := range messages {
		request.Messages[i] = RouterMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Add metadata if present
	if agentConfig.Metadata != nil {
		if optimize, ok := agentConfig.Metadata["optimize_for"].(string); ok {
			request.OptimizeFor = optimize
		}
	}

	// Override with explicit OptimizeFor setting
	if agentConfig.OptimizeFor != "" {
		request.OptimizeFor = agentConfig.OptimizeFor
	}

	// Marshal request
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal router request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v1/chat/completions", s.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if s.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.APIKey))
	}

	// Add user context headers
	req.Header.Set("X-User-ID", userID.String())

	// Send request with retries
	var resp *http.Response
	var lastErr error
	
	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		startTime := time.Now()
		resp, err = s.httpClient.Do(req)
		responseTime := time.Since(startTime)

		if err != nil {
			lastErr = err
			if attempt < s.config.MaxRetries {
				time.Sleep(time.Duration(attempt+1) * time.Second) // Exponential backoff
				continue
			}
			break
		}

		defer resp.Body.Close()

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		// Handle non-200 status codes
		if resp.StatusCode != http.StatusOK {
			if attempt < s.config.MaxRetries && (resp.StatusCode == 429 || resp.StatusCode >= 500) {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return nil, fmt.Errorf("router returned status %d: %s", resp.StatusCode, string(body))
		}

		// Parse response
		var routerResp RouterAPIResponse
		if err := json.Unmarshal(body, &routerResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal router response: %w", err)
		}

		// Convert to service response
		if len(routerResp.Choices) == 0 {
			return nil, fmt.Errorf("no choices in router response")
		}

		// Extract provider from router_metadata if available
		provider := extractProvider(routerResp.Model)
		if routerResp.RouterMetadata != nil {
			if metaProvider, ok := routerResp.RouterMetadata["provider"].(string); ok {
				provider = metaProvider
			}
		}

		// Extract enhanced metadata from router response
		reliabilityMetadata := extractReliabilityMetadata(routerResp.RouterMetadata)

		response := &services.RouterResponse{
			Content:         routerResp.Choices[0].Message.Content,
			Provider:        provider,
			Model:           routerResp.Model,
			RoutingStrategy: request.OptimizeFor,
			TokenUsage:      routerResp.Usage.TotalTokens,
			CostUSD:         calculateCostUSD(routerResp.Usage, routerResp.Model),
			ResponseTimeMs:  int(responseTime.Milliseconds()),
			Metadata: map[string]interface{}{
				"request_id":         routerResp.ID,
				"finish_reason":      routerResp.Choices[0].FinishReason,
				"prompt_tokens":      routerResp.Usage.PromptTokens,
				"completion_tokens":  routerResp.Usage.CompletionTokens,
				"created":           routerResp.Created,
				"router_metadata":   routerResp.RouterMetadata,
				// Enhanced reliability metadata
				"retry_attempts":    reliabilityMetadata.RetryAttempts,
				"fallback_used":     reliabilityMetadata.FallbackUsed,
				"failed_providers":  reliabilityMetadata.FailedProviders,
				"total_retry_time":  reliabilityMetadata.TotalRetryTime,
				"provider_latency":  reliabilityMetadata.ProviderLatency,
				"routing_reason":    reliabilityMetadata.RoutingReason,
			},
		}

		return response, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed after %d retries: %w", s.config.MaxRetries, lastErr)
	}

	return nil, fmt.Errorf("unexpected error in router request")
}

func (s *routerServiceImpl) ValidateConfig(ctx context.Context, config models.AgentLLMConfig) error {
	if config.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if config.Model == "" {
		return fmt.Errorf("model is required")
	}

	// Check if provider/model is available
	providers, err := s.GetAvailableProviders(ctx)
	if err != nil {
		return fmt.Errorf("failed to get available providers: %w", err)
	}

	for _, provider := range providers {
		if provider.Name == config.Provider {
			for _, model := range provider.Models {
				if model == config.Model {
					return nil
				}
			}
			return fmt.Errorf("model %s not available for provider %s", config.Model, config.Provider)
		}
	}

	return fmt.Errorf("provider %s not available", config.Provider)
}

func (s *routerServiceImpl) GetAvailableProviders(ctx context.Context) ([]services.Provider, error) {
	url := fmt.Sprintf("%s/v1/providers", s.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if s.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.APIKey))
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("router returned status %d: %s", resp.StatusCode, string(body))
	}

	var providersResp ActualProvidersResponse
	if err := json.NewDecoder(resp.Body).Decode(&providersResp); err != nil {
		return nil, fmt.Errorf("failed to decode providers response: %w", err)
	}

	// Convert to service providers
	providers := make([]services.Provider, len(providersResp.Providers))
	for i, providerName := range providersResp.Providers {
		providers[i] = services.Provider{
			Name:        providerName,
			DisplayName: capitalizeFirst(providerName),
			Models:      []string{}, // We'll populate this with known models
			Features:    []string{"chat_completions"},
		}
		
		// Add known models for each provider
		switch providerName {
		case "openai":
			providers[i].Models = []string{"gpt-3.5-turbo", "gpt-4o", "gpt-4", "gpt-4-turbo"}
		case "anthropic":
			providers[i].Models = []string{"claude-sonnet-4-20250514", "claude-3-haiku-20240307", "claude-3-opus-20240229"}
		}
	}

	return providers, nil
}

func (s *routerServiceImpl) GetProviderModels(ctx context.Context, provider string) ([]services.Model, error) {
	url := fmt.Sprintf("%s/v1/providers/%s", s.config.BaseURL, provider)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if s.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.APIKey))
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("router returned status %d: %s", resp.StatusCode, string(body))
	}

	var providerResp ProviderResponse
	if err := json.NewDecoder(resp.Body).Decode(&providerResp); err != nil {
		return nil, fmt.Errorf("failed to decode provider response: %w", err)
	}

	// Convert to service models
	models := make([]services.Model, len(providerResp.Models))
	for i, m := range providerResp.Models {
		models[i] = services.Model{
			Name:         m.Name,
			DisplayName:  m.DisplayName,
			Provider:     provider,
			MaxTokens:    m.MaxTokens,
			CostPer1000:  m.CostPer1000,
			Features:     m.Features,
		}
	}

	return models, nil
}

// Helper types for router API
type RouterRequest struct {
	Model            string          `json:"model"`
	Messages         []RouterMessage `json:"messages"`
	Temperature      *float64        `json:"temperature,omitempty"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	TopK             *int            `json:"top_k,omitempty"`
	Stop             []string        `json:"stop,omitempty"`
	OptimizeFor      string          `json:"optimize_for,omitempty"`
	RequiredFeatures []string        `json:"required_features,omitempty"`
	MaxCost          *float64        `json:"max_cost,omitempty"`
	RetryConfig      *RetryConfig    `json:"retry_config,omitempty"`
	FallbackConfig   *FallbackConfig `json:"fallback_config,omitempty"`
}

// RetryConfig defines retry behavior for failed requests
// Note: BaseDelay and MaxDelay use time.Duration (int64 nanoseconds) to match LLM router expectations
type RetryConfig struct {
	MaxAttempts     int           `json:"max_attempts"`                   // Maximum retry attempts (1-5)
	BackoffType     string        `json:"backoff_type,omitempty"`         // "exponential" or "linear"
	BaseDelay       time.Duration `json:"base_delay,omitempty"`           // Base delay between retries
	MaxDelay        time.Duration `json:"max_delay,omitempty"`            // Maximum delay cap
	RetryableErrors []string      `json:"retryable_errors,omitempty"`     // Error patterns that trigger retries
}

// FallbackConfig defines automatic fallback to alternative providers
type FallbackConfig struct {
	Enabled            bool     `json:"enabled"`                               // Enable fallback to healthy providers
	PreferredChain     []string `json:"preferred_chain,omitempty"`            // Custom fallback order (provider names)
	MaxCostIncrease    *float64 `json:"max_cost_increase,omitempty"`          // Max cost increase allowed for fallback
	RequireSameFeatures bool    `json:"require_same_features,omitempty"`      // Whether fallback providers must support same features
}

type RouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RouterAPIResponse struct {
	ID             string                 `json:"id"`
	Object         string                 `json:"object"`
	Created        int64                  `json:"created"`
	Model          string                 `json:"model"`
	Choices        []RouterChoice         `json:"choices"`
	Usage          RouterUsage            `json:"usage"`
	RouterMetadata map[string]interface{} `json:"router_metadata"`
}

type RouterChoice struct {
	Index        int           `json:"index"`
	Message      RouterMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type RouterUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Actual response format from TAS-LLM-Router
type ActualProvidersResponse struct {
	Count     int      `json:"count"`
	Providers []string `json:"providers"`
}

// Legacy expected format (kept for compatibility)
type ProvidersResponse struct {
	Providers []ProviderInfo `json:"providers"`
}

type ProviderInfo struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Models      []string `json:"models"`
	Features    []string `json:"features"`
}

type ProviderResponse struct {
	Name    string      `json:"name"`
	Models  []ModelInfo `json:"models"`
}

type ModelInfo struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	MaxTokens   int      `json:"max_tokens"`
	CostPer1000 float64  `json:"cost_per_1000"`
	Features    []string `json:"features"`
}

// ActualProviderResponse matches the real LLM router /v1/providers/{provider} response
type ActualProviderResponse struct {
	Name         string               `json:"name"`
	Provider     string               `json:"provider"`
	Capabilities ProviderCapabilities `json:"capabilities"`
}

type ProviderCapabilities struct {
	ProviderName     string              `json:"provider_name"`
	SupportedModels  []SupportedModel    `json:"supported_models"`
	MaxContextWindow int                 `json:"max_context_window"`
}

type SupportedModel struct {
	Name            string  `json:"name"`
	DisplayName     string  `json:"display_name"`
	MaxContextWindow int    `json:"max_context_window"`
	MaxOutputTokens int     `json:"max_output_tokens"`
	InputCostPer1K  float64 `json:"input_cost_per_1k"`
	OutputCostPer1K float64 `json:"output_cost_per_1k"`
	ProviderModelID string  `json:"provider_model_id"`
}

// Helper functions
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func extractProvider(model string) string {
	// Simple logic to extract provider from model name
	if len(model) > 3 && model[:3] == "gpt" {
		return "openai"
	}
	if len(model) > 6 && model[:6] == "claude" {
		return "anthropic"
	}
	return "unknown"
}

// ReliabilityMetadata contains enhanced reliability information from router
type ReliabilityMetadata struct {
	RetryAttempts   int      `json:"retry_attempts"`
	FallbackUsed    bool     `json:"fallback_used"`
	FailedProviders []string `json:"failed_providers"`
	TotalRetryTime  int      `json:"total_retry_time"`  // milliseconds
	ProviderLatency int      `json:"provider_latency"` // milliseconds
	RoutingReason   []string `json:"routing_reason"`
}

// buildRetryConfig converts agent retry config to router format
// Parses string durations (e.g., "5s", "30s") to time.Duration for LLM router compatibility
func buildRetryConfig(agentRetry *models.RetryConfig) *RetryConfig {
	if agentRetry == nil {
		return nil
	}

	// Parse string durations to time.Duration
	var baseDelay, maxDelay time.Duration
	if agentRetry.BaseDelay != "" {
		if parsed, err := time.ParseDuration(agentRetry.BaseDelay); err == nil {
			baseDelay = parsed
		}
	}
	if agentRetry.MaxDelay != "" {
		if parsed, err := time.ParseDuration(agentRetry.MaxDelay); err == nil {
			maxDelay = parsed
		}
	}

	return &RetryConfig{
		MaxAttempts:     agentRetry.MaxAttempts,
		BackoffType:     agentRetry.BackoffType,
		BaseDelay:       baseDelay,
		MaxDelay:        maxDelay,
		RetryableErrors: agentRetry.RetryableErrors,
	}
}

// buildFallbackConfig converts agent fallback config to router format
func buildFallbackConfig(agentFallback *models.FallbackConfig) *FallbackConfig {
	if agentFallback == nil {
		return nil
	}

	return &FallbackConfig{
		Enabled:             agentFallback.Enabled,
		PreferredChain:      agentFallback.PreferredChain,
		MaxCostIncrease:     agentFallback.MaxCostIncrease,
		RequireSameFeatures: agentFallback.RequireSameFeatures,
	}
}

// extractReliabilityMetadata extracts reliability information from router metadata
func extractReliabilityMetadata(routerMeta map[string]interface{}) ReliabilityMetadata {
	metadata := ReliabilityMetadata{}

	if routerMeta == nil {
		return metadata
	}

	// Extract retry attempts
	if attempts, ok := routerMeta["attempt_count"].(float64); ok {
		metadata.RetryAttempts = int(attempts) - 1 // subtract 1 for initial attempt
	}

	// Extract fallback usage
	if fallback, ok := routerMeta["fallback_used"].(bool); ok {
		metadata.FallbackUsed = fallback
	}

	// Extract failed providers
	if failed, ok := routerMeta["failed_providers"].([]interface{}); ok {
		for _, provider := range failed {
			if providerStr, ok := provider.(string); ok {
				metadata.FailedProviders = append(metadata.FailedProviders, providerStr)
			}
		}
	}

	// Extract retry time
	if retryTime, ok := routerMeta["total_retry_time"].(float64); ok {
		metadata.TotalRetryTime = int(retryTime)
	}

	// Extract provider latency
	if latency, ok := routerMeta["provider_latency"].(string); ok {
		// Parse latency string like "180ms" to milliseconds
		if len(latency) > 2 && latency[len(latency)-2:] == "ms" {
			if ms, err := time.ParseDuration(latency); err == nil {
				metadata.ProviderLatency = int(ms.Milliseconds())
			}
		}
	}

	// Extract routing reason
	if reason, ok := routerMeta["routing_reason"].([]interface{}); ok {
		for _, r := range reason {
			if reasonStr, ok := r.(string); ok {
				metadata.RoutingReason = append(metadata.RoutingReason, reasonStr)
			}
		}
	}

	return metadata
}

func calculateCostUSD(usage RouterUsage, model string) float64 {
	// This is a simplified cost calculation
	// In production, this should use the router's cost calculation
	switch {
	case model == "gpt-3.5-turbo":
		return float64(usage.TotalTokens) * 0.001 / 1000 // $0.001 per 1K tokens
	case model == "gpt-4o":
		return float64(usage.TotalTokens) * 0.03 / 1000 // $0.03 per 1K tokens
	case len(model) > 6 && model[:6] == "claude":
		return float64(usage.TotalTokens) * 0.015 / 1000 // $0.015 per 1K tokens
	default:
		return 0.0
	}
}

// getModelMaxOutputTokens fetches the max output tokens for a model from the LLM router
// Uses a cache to avoid repeated API calls
func (s *routerServiceImpl) getModelMaxOutputTokens(ctx context.Context, model string) int {
	// Check cache first
	if limit, ok := s.modelLimitsCache[model]; ok {
		return limit
	}

	// Determine provider from model name
	provider := extractProvider(model)
	if provider == "unknown" {
		return 4096 // Safe default
	}

	// Fetch from router
	url := fmt.Sprintf("%s/v1/providers/%s", s.config.BaseURL, provider)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 4096 // Safe default on error
	}

	if s.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.APIKey))
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 4096 // Safe default on error
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 4096 // Safe default on error
	}

	var providerResp ActualProviderResponse
	if err := json.NewDecoder(resp.Body).Decode(&providerResp); err != nil {
		return 4096 // Safe default on error
	}

	// Find the model in supported models and cache all models from this provider
	for _, m := range providerResp.Capabilities.SupportedModels {
		if m.MaxOutputTokens > 0 {
			s.modelLimitsCache[m.Name] = m.MaxOutputTokens
		}
	}

	// Return the limit for the requested model
	if limit, ok := s.modelLimitsCache[model]; ok {
		return limit
	}

	// Model not found in provider response
	return 4096
}

// capMaxTokensForModel caps max_tokens to the model's maximum output token limit from the router
func (s *routerServiceImpl) capMaxTokensForModel(ctx context.Context, maxTokens *int, model string) *int {
	if maxTokens == nil {
		return nil
	}

	modelLimit := s.getModelMaxOutputTokens(ctx, model)
	if *maxTokens > modelLimit {
		capped := modelLimit
		return &capped
	}

	return maxTokens
}