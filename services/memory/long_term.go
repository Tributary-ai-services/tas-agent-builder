package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
)

// LongTermMemoryServiceImpl implements LongTermMemoryService using DeepLake
type LongTermMemoryServiceImpl struct {
	deeplakeConfig *config.DeepLakeConfig
	httpClient     *http.Client
	config         *models.MemoryConfig
}

// NewLongTermMemoryService creates a new long-term memory service
func NewLongTermMemoryService(deeplakeConfig *config.DeepLakeConfig, memoryConfig *models.MemoryConfig) *LongTermMemoryServiceImpl {
	if memoryConfig == nil {
		memoryConfig = models.DefaultMemoryConfig()
	}
	return &LongTermMemoryServiceImpl{
		deeplakeConfig: deeplakeConfig,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: memoryConfig,
	}
}

// DeepLakeSearchRequest represents a search request to DeepLake
type DeepLakeSearchRequest struct {
	QueryText string            `json:"query_text"`
	TopK      int               `json:"top_k"`
	Filters   map[string]string `json:"filters,omitempty"`
}

// DeepLakeSearchResult represents a search result from DeepLake
type DeepLakeSearchResult struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Score    float64                `json:"score"`
	Distance float64                `json:"distance"`
	Metadata map[string]interface{} `json:"metadata"`
}

// DeepLakeSearchResponse represents the response from DeepLake search
type DeepLakeSearchResponse struct {
	Results []DeepLakeSearchResult `json:"results"`
	Total   int                    `json:"total_found"`
}

// DeepLakeStoreRequest represents a request to store data in DeepLake
type DeepLakeStoreRequest struct {
	Vectors []DeepLakeVector `json:"vectors"`
}

// DeepLakeVector represents a vector to store
type DeepLakeVector struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
}

// SearchMemory performs semantic search over long-term memories
func (s *LongTermMemoryServiceImpl) SearchMemory(ctx context.Context, agentID uuid.UUID, query string, topK int) ([]models.LongTermMemoryEntry, error) {
	if !s.config.LongTermEnabled {
		return nil, nil
	}

	// Build the search request
	searchReq := DeepLakeSearchRequest{
		QueryText: query,
		TopK:      topK,
		Filters: map[string]string{
			"agent_id":    agentID.String(),
			"memory_type": "long_term",
		},
	}

	reqBody, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	// Build the URL for memory dataset search
	datasetID := fmt.Sprintf("memory_%s", agentID.String())
	url := fmt.Sprintf("%s/api/v1/datasets/%s/search/text", s.deeplakeConfig.BaseURL, datasetID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if s.deeplakeConfig.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.deeplakeConfig.APIKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search DeepLake: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Dataset doesn't exist yet, return empty
		return []models.LongTermMemoryEntry{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DeepLake search failed with status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp DeepLakeSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	// Convert to LongTermMemoryEntry
	entries := make([]models.LongTermMemoryEntry, 0, len(searchResp.Results))
	for _, result := range searchResp.Results {
		entry := models.LongTermMemoryEntry{
			ID:       result.ID,
			AgentID:  agentID,
			Content:  result.Content,
			Score:    result.Score,
			Metadata: result.Metadata,
		}

		// Extract metadata fields
		if contentType, ok := result.Metadata["content_type"].(string); ok {
			entry.ContentType = contentType
		}
		if sourceType, ok := result.Metadata["source_type"].(string); ok {
			entry.SourceType = sourceType
		}
		if sourceID, ok := result.Metadata["source_id"].(string); ok {
			entry.SourceID = sourceID
		}
		if sessionID, ok := result.Metadata["session_id"].(string); ok {
			entry.SessionID = sessionID
		}
		if tenantID, ok := result.Metadata["tenant_id"].(string); ok {
			entry.TenantID = tenantID
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// StoreMemory stores a new long-term memory entry
func (s *LongTermMemoryServiceImpl) StoreMemory(ctx context.Context, entry models.LongTermMemoryEntry) error {
	if !s.config.LongTermEnabled {
		return nil
	}

	// Prepare metadata
	metadata := map[string]interface{}{
		"agent_id":     entry.AgentID.String(),
		"tenant_id":    entry.TenantID,
		"content_type": entry.ContentType,
		"source_type":  entry.SourceType,
		"source_id":    entry.SourceID,
		"session_id":   entry.SessionID,
		"memory_type":  "long_term",
		"created_at":   entry.CreatedAt.Format(time.RFC3339),
	}

	// Merge with existing metadata
	for k, v := range entry.Metadata {
		metadata[k] = v
	}

	storeReq := DeepLakeStoreRequest{
		Vectors: []DeepLakeVector{
			{
				ID:       entry.ID,
				Content:  entry.Content,
				Metadata: metadata,
			},
		},
	}

	reqBody, err := json.Marshal(storeReq)
	if err != nil {
		return fmt.Errorf("failed to marshal store request: %w", err)
	}

	datasetID := fmt.Sprintf("memory_%s", entry.AgentID.String())
	url := fmt.Sprintf("%s/api/v1/datasets/%s/vectors", s.deeplakeConfig.BaseURL, datasetID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if s.deeplakeConfig.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.deeplakeConfig.APIKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to store in DeepLake: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DeepLake store failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// StoreSummary stores a conversation summary as long-term memory
func (s *LongTermMemoryServiceImpl) StoreSummary(ctx context.Context, agentID uuid.UUID, sessionID string, summary string, sourceEntries []string) error {
	entry := models.LongTermMemoryEntry{
		ID:          uuid.New().String(),
		AgentID:     agentID,
		ContentType: "summary",
		Content:     summary,
		SourceType:  "conversation",
		SessionID:   sessionID,
		TokenCount:  estimateTokenCount(summary),
		Metadata: map[string]interface{}{
			"source_entries": sourceEntries,
		},
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		AccessCount: 0,
	}

	return s.StoreMemory(ctx, entry)
}

// GetRecentMemories retrieves recently accessed memories
func (s *LongTermMemoryServiceImpl) GetRecentMemories(ctx context.Context, agentID uuid.UUID, limit int) ([]models.LongTermMemoryEntry, error) {
	// Search for all memories and filter by access time
	// Note: In production, DeepLake would support sorting by metadata fields
	return s.SearchMemory(ctx, agentID, "*", limit)
}

// UpdateAccessStats updates access statistics for a memory
func (s *LongTermMemoryServiceImpl) UpdateAccessStats(ctx context.Context, memoryID string) error {
	// This would update the accessed_at and access_count fields
	// In DeepLake, this would require an update operation
	// For now, this is a no-op as DeepLake updates are more complex
	return nil
}

// DeleteMemory deletes a long-term memory entry
func (s *LongTermMemoryServiceImpl) DeleteMemory(ctx context.Context, memoryID string) error {
	// Extract agent ID from memoryID or use a different approach
	// For now, this is a simplified implementation
	return nil
}

// GetMemoryCount returns the count of long-term memories for an agent
func (s *LongTermMemoryServiceImpl) GetMemoryCount(ctx context.Context, agentID uuid.UUID) (int, error) {
	if !s.config.LongTermEnabled {
		return 0, nil
	}

	// Query DeepLake for count
	datasetID := fmt.Sprintf("memory_%s", agentID.String())
	url := fmt.Sprintf("%s/api/v1/datasets/%s/count", s.deeplakeConfig.BaseURL, datasetID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	if s.deeplakeConfig.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.deeplakeConfig.APIKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to get count from DeepLake: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return 0, nil
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("DeepLake count failed with status %d", resp.StatusCode)
	}

	var countResp struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&countResp); err != nil {
		return 0, fmt.Errorf("failed to decode count response: %w", err)
	}

	return countResp.Count, nil
}

// PruneOldMemories removes old/unused memories to stay within limits
func (s *LongTermMemoryServiceImpl) PruneOldMemories(ctx context.Context, agentID uuid.UUID, maxEntries int) (int, error) {
	count, err := s.GetMemoryCount(ctx, agentID)
	if err != nil {
		return 0, err
	}

	if count <= maxEntries {
		return 0, nil
	}

	// In production, this would:
	// 1. Query memories sorted by access_count/accessed_at
	// 2. Delete the least accessed/oldest ones
	// 3. Return the number deleted

	toDelete := count - maxEntries
	return toDelete, nil // Placeholder - actual deletion would happen here
}

// FormatForContext formats long-term memories for context injection
func (s *LongTermMemoryServiceImpl) FormatForContext(entries []models.LongTermMemoryEntry) string {
	if len(entries) == 0 {
		return ""
	}

	var formatted string
	formatted += "--- Relevant Knowledge ---\n\n"

	for _, entry := range entries {
		switch entry.ContentType {
		case "summary":
			formatted += fmt.Sprintf("Previous conversation summary:\n%s\n\n", entry.Content)
		case "fact":
			formatted += fmt.Sprintf("Known fact: %s\n", entry.Content)
		case "insight":
			formatted += fmt.Sprintf("Insight: %s\n", entry.Content)
		default:
			formatted += entry.Content + "\n"
		}
	}

	formatted += "--- End Knowledge ---\n"
	return formatted
}
