package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/models"
	"gorm.io/datatypes"
)

// isRouterAvailable checks if the TAS-LLM-Router is available at the given URL
func isRouterAvailable(routerURL string) bool {
	// Try the providers endpoint instead of health since health might return 503
	providersURL := fmt.Sprintf("%s/v1/providers", routerURL)
	
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	
	resp, err := client.Get(providersURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	// Accept 200 or any 2xx status
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// createTagsJSON converts a slice of strings to datatypes.JSON for Tags field
func createTagsJSON(tags []string) datatypes.JSON {
	if tags == nil {
		tags = []string{}
	}
	jsonData, err := models.ConvertToJSON(tags)
	if err != nil {
		return datatypes.JSON("[]")
	}
	return jsonData
}

// createNotebookIDsJSON converts a slice of UUIDs to datatypes.JSON for NotebookIDs field
func createNotebookIDsJSON(ids []uuid.UUID) datatypes.JSON {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	jsonData, err := models.ConvertToJSON(ids)
	if err != nil {
		return datatypes.JSON("[]")
	}
	return jsonData
}

// appendTagsJSON adds a new tag to existing tags JSON and returns new JSON
func appendTagsJSON(existingTagsJSON datatypes.JSON, newTag string) datatypes.JSON {
	var existingTags []string
	
	// Parse existing tags
	if existingTagsJSON != nil {
		err := json.Unmarshal(existingTagsJSON, &existingTags)
		if err != nil {
			existingTags = []string{}
		}
	}
	
	// Append new tag
	updatedTags := append(existingTags, newTag)
	return createTagsJSON(updatedTags)
}

// extractTagsFromJSON converts datatypes.JSON tags back to []string for testing
func extractTagsFromJSON(tagsJSON datatypes.JSON) []string {
	var tags []string
	if tagsJSON != nil {
		json.Unmarshal(tagsJSON, &tags)
	}
	return tags
}

// Helper functions for tests
func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func uuidPtr(u uuid.UUID) *uuid.UUID {
	return &u
}

func statusPtr(s models.ExecutionStatus) *models.ExecutionStatus {
	return &s
}

// extractOptimizationFromTemplate extracts optimization strategy from template configuration
func extractOptimizationFromTemplate(templateName string) string {
	switch templateName {
	case "high_reliability":
		return "reliability"
	case "cost_optimized":
		return "cost"
	case "performance_optimized":
		return "performance"
	default:
		return "balanced"
	}
}

// containsIgnoreCase checks if a string contains a substring case-insensitively
func containsIgnoreCase(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

// equalFold compares two strings case-insensitively
func equalFold(s1, s2 string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		c1, c2 := s1[i], s2[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

// validateRetryConfig validates retry configuration parameters
func validateRetryConfig(config models.RetryConfig) error {
	if config.MaxAttempts < 1 || config.MaxAttempts > 5 {
		return fmt.Errorf("max_attempts must be between 1 and 5")
	}

	if config.BackoffType != "" && config.BackoffType != "exponential" && config.BackoffType != "linear" {
		return fmt.Errorf("backoff_type must be 'exponential' or 'linear'")
	}

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
func validateFallbackConfig(config models.FallbackConfig) error {
	if config.MaxCostIncrease != nil && (*config.MaxCostIncrease < 0 || *config.MaxCostIncrease > 2.0) {
		return fmt.Errorf("max_cost_increase must be between 0 and 2.0")
	}
	return nil
}