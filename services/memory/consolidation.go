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
	"github.com/redis/go-redis/v9"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
)

// MemoryConsolidationServiceImpl implements MemoryConsolidationService
type MemoryConsolidationServiceImpl struct {
	shortTerm    *ShortTermMemoryServiceImpl
	longTerm     *LongTermMemoryServiceImpl
	redis        *redis.Client
	routerConfig *config.RouterConfig
	httpClient   *http.Client
	config       *models.MemoryConfig
	keyPrefix    string
}

// NewMemoryConsolidationService creates a new memory consolidation service
func NewMemoryConsolidationService(
	shortTerm *ShortTermMemoryServiceImpl,
	longTerm *LongTermMemoryServiceImpl,
	redisClient *redis.Client,
	routerConfig *config.RouterConfig,
	memoryConfig *models.MemoryConfig,
) *MemoryConsolidationServiceImpl {
	if memoryConfig == nil {
		memoryConfig = models.DefaultMemoryConfig()
	}
	return &MemoryConsolidationServiceImpl{
		shortTerm:    shortTerm,
		longTerm:     longTerm,
		redis:        redisClient,
		routerConfig: routerConfig,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		config:    memoryConfig,
		keyPrefix: "memory:consolidation",
	}
}

// consolidationKey generates the Redis key for consolidation tracking
func (s *MemoryConsolidationServiceImpl) consolidationKey(sessionID string, agentID uuid.UUID) string {
	return fmt.Sprintf("%s:%s:%s", s.keyPrefix, agentID.String(), sessionID)
}

// ShouldConsolidate checks if consolidation is needed
func (s *MemoryConsolidationServiceImpl) ShouldConsolidate(ctx context.Context, sessionID string, agentID uuid.UUID) (bool, error) {
	// Check last consolidation time
	key := s.consolidationKey(sessionID, agentID)
	lastConsolidation, err := s.redis.Get(ctx, key).Time()
	if err != nil && err != redis.Nil {
		return false, fmt.Errorf("failed to get last consolidation time: %w", err)
	}

	// If never consolidated or interval exceeded, should consolidate
	if err == redis.Nil {
		return true, nil
	}

	return time.Since(lastConsolidation) > s.config.ConsolidationInterval, nil
}

// ConsolidateSession consolidates a session's memories
func (s *MemoryConsolidationServiceImpl) ConsolidateSession(ctx context.Context, req models.ConsolidationRequest) (*models.ConsolidationResult, error) {
	startTime := time.Now()

	// Get current short-term memory
	memory, err := s.shortTerm.GetConversation(ctx, req.SessionID, req.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get short-term memory: %w", err)
	}

	// Check if we have enough entries to consolidate
	if len(memory.Entries) < 2 && !req.Force {
		return &models.ConsolidationResult{
			EntriesProcessed: 0,
			Timestamp:        time.Now(),
		}, nil
	}

	// Check token threshold
	if memory.TotalTokens < s.config.SummaryMinTokens && !req.Force {
		return &models.ConsolidationResult{
			EntriesProcessed: 0,
			Timestamp:        time.Now(),
		}, nil
	}

	// Determine how many entries to consolidate
	entriesToProcess := memory.Entries
	if req.MaxEntries > 0 && len(entriesToProcess) > req.MaxEntries {
		entriesToProcess = entriesToProcess[:req.MaxEntries]
	}

	// Generate summary
	summary, err := s.GenerateSummary(ctx, entriesToProcess, s.config.SummaryMaxTokens)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	// Extract source entry IDs
	sourceIDs := make([]string, len(entriesToProcess))
	for i, entry := range entriesToProcess {
		sourceIDs[i] = entry.ID
	}

	// Store summary in long-term memory
	if err := s.longTerm.StoreSummary(ctx, req.AgentID, req.SessionID, summary, sourceIDs); err != nil {
		return nil, fmt.Errorf("failed to store summary: %w", err)
	}

	// Calculate tokens saved
	originalTokens := 0
	for _, entry := range entriesToProcess {
		originalTokens += entry.TokenCount
	}
	summaryTokens := estimateTokenCount(summary)
	tokensSaved := originalTokens - summaryTokens

	// Update last consolidation time
	key := s.consolidationKey(req.SessionID, req.AgentID)
	if err := s.redis.Set(ctx, key, time.Now(), 24*time.Hour).Err(); err != nil {
		// Log but don't fail - consolidation was successful
		fmt.Printf("Warning: failed to update consolidation timestamp: %v\n", err)
	}

	return &models.ConsolidationResult{
		EntriesProcessed:   len(entriesToProcess),
		SummariesCreated:   1,
		TokensConsolidated: originalTokens,
		TokensSaved:        tokensSaved,
		Duration:           int(time.Since(startTime).Milliseconds()),
		Timestamp:          time.Now(),
	}, nil
}

// GenerateSummary generates a summary of conversation entries using the LLM
func (s *MemoryConsolidationServiceImpl) GenerateSummary(ctx context.Context, entries []models.MemoryEntry, maxTokens int) (string, error) {
	if len(entries) == 0 {
		return "", nil
	}

	// Format entries for summarization
	var conversationText string
	for _, entry := range entries {
		conversationText += fmt.Sprintf("%s: %s\n", entry.Role, entry.Content)
	}

	// Build summarization prompt
	systemPrompt := `You are a memory consolidation assistant. Your task is to create concise summaries of conversations that preserve the key information, decisions, and context.

Requirements:
- Preserve important facts, decisions, and action items
- Maintain the essence of the discussion
- Be concise but comprehensive
- Write in third person narrative
- Focus on what the user was trying to accomplish and what was learned`

	userPrompt := fmt.Sprintf(`Please summarize the following conversation, preserving the key information and context:

%s

Summary (maximum %d tokens):`, conversationText, maxTokens)

	// Call the LLM router
	reqBody := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  maxTokens,
		"temperature": 0.3, // Lower temperature for factual summarization
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chat/completions", s.routerConfig.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if s.routerConfig.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.routerConfig.APIKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call LLM router: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM router returned status %d: %s", resp.StatusCode, string(body))
	}

	var routerResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&routerResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(routerResp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return routerResp.Choices[0].Message.Content, nil
}

// ExtractFacts extracts factual information from conversation
func (s *MemoryConsolidationServiceImpl) ExtractFacts(ctx context.Context, entries []models.MemoryEntry) ([]models.LongTermMemoryEntry, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	// Format entries for fact extraction
	var conversationText string
	for _, entry := range entries {
		conversationText += fmt.Sprintf("%s: %s\n", entry.Role, entry.Content)
	}

	// Build fact extraction prompt
	systemPrompt := `You are a fact extraction assistant. Your task is to identify and extract distinct factual statements from conversations.

Requirements:
- Extract concrete facts, preferences, and decisions
- Each fact should be a standalone statement
- Format output as JSON array of fact strings
- Focus on information that would be useful to remember for future conversations
- Exclude opinions, pleasantries, and meta-commentary`

	userPrompt := fmt.Sprintf(`Extract the key facts from this conversation as a JSON array:

%s

Output format: ["fact 1", "fact 2", ...]`, conversationText)

	// Call the LLM router
	reqBody := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  500,
		"temperature": 0.1, // Low temperature for factual extraction
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chat/completions", s.routerConfig.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if s.routerConfig.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.routerConfig.APIKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM router: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LLM router returned status %d: %s", resp.StatusCode, string(body))
	}

	var routerResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&routerResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(routerResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	// Parse JSON array of facts
	var facts []string
	if err := json.Unmarshal([]byte(routerResp.Choices[0].Message.Content), &facts); err != nil {
		// If JSON parsing fails, return empty - the LLM might not have returned valid JSON
		return []models.LongTermMemoryEntry{}, nil
	}

	// Convert facts to long-term memory entries
	var memoryEntries []models.LongTermMemoryEntry
	for _, fact := range facts {
		if fact == "" {
			continue
		}

		// Get agent ID and tenant ID from the first entry
		var agentID uuid.UUID
		var tenantID string
		var sessionID string
		if len(entries) > 0 {
			agentID = entries[0].AgentID
			tenantID = entries[0].TenantID
			sessionID = entries[0].SessionID
		}

		memoryEntries = append(memoryEntries, models.LongTermMemoryEntry{
			ID:          uuid.New().String(),
			AgentID:     agentID,
			TenantID:    tenantID,
			ContentType: "fact",
			Content:     fact,
			SourceType:  "conversation",
			SessionID:   sessionID,
			TokenCount:  estimateTokenCount(fact),
			CreatedAt:   time.Now(),
			AccessedAt:  time.Now(),
			AccessCount: 0,
		})
	}

	return memoryEntries, nil
}

// ScheduleConsolidation schedules periodic consolidation
func (s *MemoryConsolidationServiceImpl) ScheduleConsolidation(ctx context.Context, intervalSeconds int) error {
	// This would typically be implemented with a background goroutine or job scheduler
	// For now, we just validate the parameters
	if intervalSeconds < 60 {
		return fmt.Errorf("consolidation interval must be at least 60 seconds")
	}
	return nil
}
