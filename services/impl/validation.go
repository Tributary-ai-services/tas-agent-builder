package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
)

// ValidationError represents a validation error with field context
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	return fmt.Sprintf("%d validation errors", len(e))
}

// AgentValidator provides validation for agent configurations
type AgentValidator struct {
	aetherConfig *config.AetherConfig
	httpClient   *http.Client
}

// NewAgentValidator creates a new agent validator
func NewAgentValidator(aetherCfg *config.AetherConfig) *AgentValidator {
	return &AgentValidator{
		aetherConfig: aetherCfg,
		httpClient: &http.Client{
			Timeout: time.Duration(aetherCfg.Timeout) * time.Second,
		},
	}
}

// ValidateCreateRequest validates a CreateAgentRequest
func (v *AgentValidator) ValidateCreateRequest(ctx context.Context, req models.CreateAgentRequest, tenantID string) ValidationErrors {
	var errors ValidationErrors

	// Validate basic fields
	if req.Name == "" {
		errors = append(errors, ValidationError{Field: "name", Message: "name is required"})
	}
	if len(req.Name) > 255 {
		errors = append(errors, ValidationError{Field: "name", Message: "name must be 255 characters or less"})
	}

	if req.SystemPrompt == "" {
		errors = append(errors, ValidationError{Field: "system_prompt", Message: "system prompt is required"})
	}

	if req.SpaceID == "" {
		errors = append(errors, ValidationError{Field: "space_id", Message: "space ID is required"})
	}

	// Validate LLM config
	if err := v.validateLLMConfig(req.LLMConfig); err != nil {
		errors = append(errors, ValidationError{Field: "llm_config", Message: err.Error()})
	}

	// Validate document context config if provided
	if req.DocumentContext != nil {
		docErrors := v.ValidateDocumentContextConfig(*req.DocumentContext)
		errors = append(errors, docErrors...)
	}

	// Validate notebook IDs exist (if knowledge is enabled and notebooks are provided)
	if req.EnableKnowledge && len(req.NotebookIDs) > 0 {
		notebookErrors := v.ValidateNotebookIDs(ctx, req.NotebookIDs, tenantID)
		errors = append(errors, notebookErrors...)
	}

	return errors
}

// ValidateUpdateRequest validates an UpdateAgentRequest
func (v *AgentValidator) ValidateUpdateRequest(ctx context.Context, req models.UpdateAgentRequest, tenantID string) ValidationErrors {
	var errors ValidationErrors

	// Validate name if provided
	if req.Name != nil {
		if *req.Name == "" {
			errors = append(errors, ValidationError{Field: "name", Message: "name cannot be empty"})
		}
		if len(*req.Name) > 255 {
			errors = append(errors, ValidationError{Field: "name", Message: "name must be 255 characters or less"})
		}
	}

	// Validate system prompt if provided
	if req.SystemPrompt != nil && *req.SystemPrompt == "" {
		errors = append(errors, ValidationError{Field: "system_prompt", Message: "system prompt cannot be empty"})
	}

	// Validate LLM config if provided
	if req.LLMConfig != nil {
		if err := v.validateLLMConfig(*req.LLMConfig); err != nil {
			errors = append(errors, ValidationError{Field: "llm_config", Message: err.Error()})
		}
	}

	// Validate document context config if provided
	if req.DocumentContext != nil {
		docErrors := v.ValidateDocumentContextConfig(*req.DocumentContext)
		errors = append(errors, docErrors...)
	}

	// Validate notebook IDs if provided
	if len(req.NotebookIDs) > 0 {
		notebookErrors := v.ValidateNotebookIDs(ctx, req.NotebookIDs, tenantID)
		errors = append(errors, notebookErrors...)
	}

	return errors
}

// ValidateDocumentContextConfig validates document context configuration
func (v *AgentValidator) ValidateDocumentContextConfig(cfg models.DocumentContextConfig) ValidationErrors {
	var errors ValidationErrors

	// Validate strategy
	validStrategies := map[models.ContextStrategy]bool{
		models.ContextStrategyVector: true,
		models.ContextStrategyFull:   true,
		models.ContextStrategyHybrid: true,
		models.ContextStrategyMCP:    true,
		models.ContextStrategyNone:   true,
	}
	if cfg.Strategy != "" && !validStrategies[cfg.Strategy] {
		errors = append(errors, ValidationError{
			Field:   "document_context.strategy",
			Message: fmt.Sprintf("invalid strategy '%s', must be one of: vector, full, hybrid, mcp, none", cfg.Strategy),
		})
	}

	// Validate scope
	validScopes := map[models.DocumentScope]bool{
		models.DocumentScopeAll:      true,
		models.DocumentScopeSelected: true,
		models.DocumentScopeNone:     true,
	}
	if cfg.Scope != "" && !validScopes[cfg.Scope] {
		errors = append(errors, ValidationError{
			Field:   "document_context.scope",
			Message: fmt.Sprintf("invalid scope '%s', must be one of: all, selected, none", cfg.Scope),
		})
	}

	// Validate numeric ranges
	if cfg.MaxContextTokens < 0 {
		errors = append(errors, ValidationError{
			Field:   "document_context.max_context_tokens",
			Message: "max_context_tokens must be non-negative",
		})
	}
	if cfg.MaxContextTokens > 128000 {
		errors = append(errors, ValidationError{
			Field:   "document_context.max_context_tokens",
			Message: "max_context_tokens must be 128000 or less",
		})
	}

	if cfg.TopK < 0 {
		errors = append(errors, ValidationError{
			Field:   "document_context.top_k",
			Message: "top_k must be non-negative",
		})
	}
	if cfg.TopK > 100 {
		errors = append(errors, ValidationError{
			Field:   "document_context.top_k",
			Message: "top_k must be 100 or less",
		})
	}

	if cfg.MinScore < 0.0 || cfg.MinScore > 1.0 {
		errors = append(errors, ValidationError{
			Field:   "document_context.min_score",
			Message: "min_score must be between 0.0 and 1.0",
		})
	}

	if cfg.VectorWeight < 0.0 || cfg.VectorWeight > 1.0 {
		errors = append(errors, ValidationError{
			Field:   "document_context.vector_weight",
			Message: "vector_weight must be between 0.0 and 1.0",
		})
	}

	if cfg.FullDocWeight < 0.0 || cfg.FullDocWeight > 1.0 {
		errors = append(errors, ValidationError{
			Field:   "document_context.full_doc_weight",
			Message: "full_doc_weight must be between 0.0 and 1.0",
		})
	}

	// Validate multi-pass config if present
	if cfg.MultiPass != nil {
		mpErrors := v.validateMultiPassConfig(*cfg.MultiPass)
		errors = append(errors, mpErrors...)
	}

	return errors
}

// validateMultiPassConfig validates multi-pass processing configuration
func (v *AgentValidator) validateMultiPassConfig(cfg models.MultiPassConfig) ValidationErrors {
	var errors ValidationErrors

	if cfg.SegmentSize < 100 {
		errors = append(errors, ValidationError{
			Field:   "document_context.multi_pass.segment_size",
			Message: "segment_size must be at least 100 tokens",
		})
	}
	if cfg.SegmentSize > 128000 {
		errors = append(errors, ValidationError{
			Field:   "document_context.multi_pass.segment_size",
			Message: "segment_size must be 128000 or less",
		})
	}

	if cfg.OverlapTokens < 0 {
		errors = append(errors, ValidationError{
			Field:   "document_context.multi_pass.overlap_tokens",
			Message: "overlap_tokens must be non-negative",
		})
	}
	if cfg.OverlapTokens >= cfg.SegmentSize {
		errors = append(errors, ValidationError{
			Field:   "document_context.multi_pass.overlap_tokens",
			Message: "overlap_tokens must be less than segment_size",
		})
	}

	if cfg.MaxPasses < 1 {
		errors = append(errors, ValidationError{
			Field:   "document_context.multi_pass.max_passes",
			Message: "max_passes must be at least 1",
		})
	}
	if cfg.MaxPasses > 50 {
		errors = append(errors, ValidationError{
			Field:   "document_context.multi_pass.max_passes",
			Message: "max_passes must be 50 or less",
		})
	}

	return errors
}

// ValidateNotebookIDs validates that notebook IDs exist in Aether-BE
func (v *AgentValidator) ValidateNotebookIDs(ctx context.Context, notebookIDs []uuid.UUID, tenantID string) ValidationErrors {
	var errors ValidationErrors

	if len(notebookIDs) == 0 {
		return errors
	}

	// Skip validation if Aether config is not available
	if v.aetherConfig == nil || v.aetherConfig.BaseURL == "" {
		return errors
	}

	// Check each notebook exists via Aether-BE
	for i, notebookID := range notebookIDs {
		exists, err := v.checkNotebookExists(ctx, notebookID, tenantID)
		if err != nil {
			// Log warning but don't fail validation if Aether is unavailable
			continue
		}
		if !exists {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("notebook_ids[%d]", i),
				Message: fmt.Sprintf("notebook %s not found", notebookID.String()),
			})
		}
	}

	return errors
}

// checkNotebookExists verifies a notebook exists in Aether-BE
func (v *AgentValidator) checkNotebookExists(ctx context.Context, notebookID uuid.UUID, tenantID string) (bool, error) {
	reqURL, err := url.JoinPath(v.aetherConfig.BaseURL, "/api/v1/notebooks", notebookID.String())
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, err
	}

	// Add authentication headers
	if v.aetherConfig.APIKey != "" {
		req.Header.Set("X-API-Key", v.aetherConfig.APIKey)
	}
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

// validateLLMConfig validates the LLM configuration
func (v *AgentValidator) validateLLMConfig(cfg models.AgentLLMConfig) error {
	if cfg.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if cfg.Model == "" {
		return fmt.Errorf("model is required")
	}

	// Validate temperature if provided
	if cfg.Temperature != nil {
		if *cfg.Temperature < 0.0 || *cfg.Temperature > 2.0 {
			return fmt.Errorf("temperature must be between 0.0 and 2.0")
		}
	}

	// Validate max tokens if provided
	if cfg.MaxTokens != nil {
		if *cfg.MaxTokens < 1 || *cfg.MaxTokens > 128000 {
			return fmt.Errorf("max_tokens must be between 1 and 128000")
		}
	}

	// Validate top_p if provided
	if cfg.TopP != nil {
		if *cfg.TopP < 0.0 || *cfg.TopP > 1.0 {
			return fmt.Errorf("top_p must be between 0.0 and 1.0")
		}
	}

	// Validate top_k if provided
	if cfg.TopK != nil {
		if *cfg.TopK < 0 {
			return fmt.Errorf("top_k must be non-negative")
		}
	}

	// Validate retry config if provided
	if cfg.RetryConfig != nil {
		if cfg.RetryConfig.MaxAttempts < 1 || cfg.RetryConfig.MaxAttempts > 10 {
			return fmt.Errorf("retry_config.max_attempts must be between 1 and 10")
		}
	}

	// Validate fallback config if provided
	if cfg.FallbackConfig != nil && cfg.FallbackConfig.MaxCostIncrease != nil {
		if *cfg.FallbackConfig.MaxCostIncrease < 0.0 {
			return fmt.Errorf("fallback_config.max_cost_increase must be non-negative")
		}
	}

	return nil
}

// NotebookExistsResponse represents the response from Aether-BE notebook check
type NotebookExistsResponse struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	TenantID string    `json:"tenant_id"`
}

// ValidateNotebookIDsWithDetails validates notebooks and returns their details
func (v *AgentValidator) ValidateNotebookIDsWithDetails(ctx context.Context, notebookIDs []uuid.UUID, tenantID string) ([]NotebookExistsResponse, ValidationErrors) {
	var errors ValidationErrors
	var notebooks []NotebookExistsResponse

	if len(notebookIDs) == 0 {
		return notebooks, errors
	}

	// Skip validation if Aether config is not available
	if v.aetherConfig == nil || v.aetherConfig.BaseURL == "" {
		return notebooks, errors
	}

	// Check each notebook exists via Aether-BE
	for i, notebookID := range notebookIDs {
		notebook, err := v.getNotebookDetails(ctx, notebookID, tenantID)
		if err != nil {
			// Log warning but don't fail validation if Aether is unavailable
			continue
		}
		if notebook == nil {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("notebook_ids[%d]", i),
				Message: fmt.Sprintf("notebook %s not found", notebookID.String()),
			})
		} else {
			notebooks = append(notebooks, *notebook)
		}
	}

	return notebooks, errors
}

// getNotebookDetails retrieves notebook details from Aether-BE
func (v *AgentValidator) getNotebookDetails(ctx context.Context, notebookID uuid.UUID, tenantID string) (*NotebookExistsResponse, error) {
	reqURL, err := url.JoinPath(v.aetherConfig.BaseURL, "/api/v1/notebooks", notebookID.String())
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	// Add authentication headers
	if v.aetherConfig.APIKey != "" {
		req.Header.Set("X-API-Key", v.aetherConfig.APIKey)
	}
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var notebook NotebookExistsResponse
		if err := json.NewDecoder(resp.Body).Decode(&notebook); err != nil {
			return nil, err
		}
		return &notebook, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}
