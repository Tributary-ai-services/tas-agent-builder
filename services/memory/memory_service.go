package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
)

// MemoryServiceImpl implements the unified MemoryService interface
type MemoryServiceImpl struct {
	shortTerm     *ShortTermMemoryServiceImpl
	working       *WorkingMemoryServiceImpl
	longTerm      *LongTermMemoryServiceImpl
	consolidation *MemoryConsolidationServiceImpl
	config        *models.MemoryConfig
}

// NewMemoryService creates a new unified memory service
func NewMemoryService(
	redisClient *redis.Client,
	deeplakeConfig *config.DeepLakeConfig,
	routerConfig *config.RouterConfig,
	memoryConfig *models.MemoryConfig,
) *MemoryServiceImpl {
	if memoryConfig == nil {
		memoryConfig = models.DefaultMemoryConfig()
	}

	shortTerm := NewShortTermMemoryService(redisClient, memoryConfig)
	working := NewWorkingMemoryService(redisClient, memoryConfig)
	longTerm := NewLongTermMemoryService(deeplakeConfig, memoryConfig)
	consolidation := NewMemoryConsolidationService(shortTerm, longTerm, redisClient, routerConfig, memoryConfig)

	return &MemoryServiceImpl{
		shortTerm:     shortTerm,
		working:       working,
		longTerm:      longTerm,
		consolidation: consolidation,
		config:        memoryConfig,
	}
}

// GetMemoryState retrieves the full memory state for an execution
func (s *MemoryServiceImpl) GetMemoryState(ctx context.Context, req models.GetMemoryRequest) (*models.MemoryState, error) {
	state := &models.MemoryState{
		TokenBudget: req.MaxTokens,
	}

	// Determine which memory types to include
	includeShortTerm := true
	includeWorking := true
	includeLongTerm := s.config.LongTermEnabled

	if len(req.IncludeTypes) > 0 {
		includeShortTerm = false
		includeWorking = false
		includeLongTerm = false
		for _, t := range req.IncludeTypes {
			switch t {
			case models.MemoryTypeShortTerm:
				includeShortTerm = true
			case models.MemoryTypeWorking:
				includeWorking = true
			case models.MemoryTypeLongTerm:
				includeLongTerm = s.config.LongTermEnabled
			}
		}
	}

	// Get short-term memory
	if includeShortTerm && req.SessionID != "" {
		shortTerm, err := s.shortTerm.GetConversation(ctx, req.SessionID, req.AgentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get short-term memory: %w", err)
		}
		state.ShortTerm = shortTerm
		state.TotalTokens += shortTerm.TotalTokens
	}

	// Get working memory
	if includeWorking && req.SessionID != "" {
		working, err := s.working.GetWorkingMemory(ctx, req.SessionID, req.AgentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get working memory: %w", err)
		}
		state.Working = working
		state.TotalTokens += working.TotalTokens
	}

	// Get long-term memory (if query provided)
	if includeLongTerm && req.Query != "" {
		longTerm, err := s.longTerm.SearchMemory(ctx, req.AgentID, req.Query, 5) // Top 5 relevant memories
		if err != nil {
			// Log but don't fail - long-term is optional
			fmt.Printf("Warning: failed to get long-term memory: %v\n", err)
		} else {
			state.LongTerm = longTerm
			for _, entry := range longTerm {
				state.TotalTokens += entry.TokenCount
			}
		}
	}

	return state, nil
}

// GetFormattedMemory retrieves and formats memory for context injection
func (s *MemoryServiceImpl) GetFormattedMemory(ctx context.Context, req models.GetMemoryRequest, tokenBudget int) (*models.MemoryContext, error) {
	memoryCtx := &models.MemoryContext{
		Strategy: "priority", // Prioritize recent conversation, then working, then long-term
	}

	// Budget allocation strategy:
	// - 50% for short-term (conversation history)
	// - 35% for working memory (document context)
	// - 15% for long-term (accumulated knowledge)
	shortTermBudget := tokenBudget / 2
	workingBudget := tokenBudget * 35 / 100
	longTermBudget := tokenBudget * 15 / 100

	// Get and format short-term memory
	if req.SessionID != "" {
		shortTerm, err := s.shortTerm.GetConversation(ctx, req.SessionID, req.AgentID)
		if err == nil && shortTerm != nil {
			// Trim to budget if needed
			if shortTerm.TotalTokens > shortTermBudget {
				if err := s.shortTerm.TrimToTokenLimit(ctx, req.SessionID, req.AgentID, shortTermBudget); err == nil {
					shortTerm, _ = s.shortTerm.GetConversation(ctx, req.SessionID, req.AgentID)
				}
			}
			memoryCtx.FormattedShortTerm = s.shortTerm.FormatForContext(shortTerm)
			memoryCtx.TotalTokens += shortTerm.TotalTokens
		}
	}

	// Get and format working memory
	if req.SessionID != "" {
		working, err := s.working.GetWorkingMemory(ctx, req.SessionID, req.AgentID)
		if err == nil && working != nil && working.TotalTokens <= workingBudget {
			memoryCtx.FormattedWorking = s.working.FormatForContext(working)
			memoryCtx.TotalTokens += working.TotalTokens
		}
	}

	// Get and format long-term memory
	if s.config.LongTermEnabled && req.Query != "" {
		longTerm, err := s.longTerm.SearchMemory(ctx, req.AgentID, req.Query, 3)
		if err == nil && len(longTerm) > 0 {
			// Trim to budget
			totalLongTermTokens := 0
			var includedEntries []models.LongTermMemoryEntry
			for _, entry := range longTerm {
				if totalLongTermTokens+entry.TokenCount > longTermBudget {
					break
				}
				includedEntries = append(includedEntries, entry)
				totalLongTermTokens += entry.TokenCount
			}
			memoryCtx.FormattedLongTerm = s.longTerm.FormatForContext(includedEntries)
			memoryCtx.TotalTokens += totalLongTermTokens
		}
	}

	memoryCtx.Truncated = memoryCtx.TotalTokens > tokenBudget
	return memoryCtx, nil
}

// AddMemory adds a new entry to short-term memory
func (s *MemoryServiceImpl) AddMemory(ctx context.Context, req models.AddMemoryRequest) error {
	entry := models.MemoryEntry{
		ID:        uuid.New().String(),
		SessionID: req.SessionID,
		AgentID:   req.AgentID,
		TenantID:  req.TenantID,
		UserID:    req.UserID,
		Type:      models.MemoryTypeShortTerm,
		Role:      req.Role,
		Content:   req.Content,
		TokenCount: estimateTokenCount(req.Content),
		Timestamp: time.Now(),
		Metadata:  req.Metadata,
	}

	if err := s.shortTerm.AddMessage(ctx, entry); err != nil {
		return fmt.Errorf("failed to add to short-term memory: %w", err)
	}

	// Check if consolidation is needed (async in production)
	shouldConsolidate, _ := s.consolidation.ShouldConsolidate(ctx, req.SessionID, req.AgentID)
	if shouldConsolidate {
		// In production, this would be done asynchronously
		go func() {
			consolidationReq := models.ConsolidationRequest{
				SessionID: req.SessionID,
				AgentID:   req.AgentID,
				TenantID:  req.TenantID,
				UserID:    req.UserID,
			}
			_, _ = s.consolidation.ConsolidateSession(context.Background(), consolidationReq)
		}()
	}

	return nil
}

// UpdateWorkingMemory updates the working memory with new document context
func (s *MemoryServiceImpl) UpdateWorkingMemory(ctx context.Context, sessionID string, agentID uuid.UUID, chunks []models.RetrievedChunk) error {
	return s.working.SetDocumentContext(ctx, sessionID, agentID, chunks)
}

// ConsolidateMemory consolidates short-term to long-term memory
func (s *MemoryServiceImpl) ConsolidateMemory(ctx context.Context, req models.ConsolidationRequest) (*models.ConsolidationResult, error) {
	return s.consolidation.ConsolidateSession(ctx, req)
}

// GetMemoryStats retrieves memory usage statistics
func (s *MemoryServiceImpl) GetMemoryStats(ctx context.Context, sessionID string, agentID uuid.UUID) (*models.MemoryStats, error) {
	stats := &models.MemoryStats{
		SessionID:  sessionID,
		AgentID:    agentID,
		LastAccess: time.Now(),
	}

	// Get short-term stats
	shortTerm, err := s.shortTerm.GetConversation(ctx, sessionID, agentID)
	if err == nil && shortTerm != nil {
		stats.ShortTermEntries = len(shortTerm.Entries)
		stats.ShortTermTokens = shortTerm.TotalTokens
	}

	// Get working memory stats
	working, err := s.working.GetWorkingMemory(ctx, sessionID, agentID)
	if err == nil && working != nil {
		stats.WorkingDocuments = len(working.LoadedDocuments)
		stats.WorkingChunks = len(working.RetrievedChunks)
		stats.WorkingTokens = working.TotalTokens
	}

	// Get long-term stats
	if s.config.LongTermEnabled {
		count, err := s.longTerm.GetMemoryCount(ctx, agentID)
		if err == nil {
			stats.LongTermEntries = count
		}
	}

	return stats, nil
}

// ClearSession removes all memory for a session
func (s *MemoryServiceImpl) ClearSession(ctx context.Context, sessionID string, agentID uuid.UUID) error {
	// Clear short-term
	if err := s.shortTerm.ClearConversation(ctx, sessionID, agentID); err != nil {
		return fmt.Errorf("failed to clear short-term memory: %w", err)
	}

	// Clear working memory
	if err := s.working.ClearWorkingMemory(ctx, sessionID, agentID); err != nil {
		return fmt.Errorf("failed to clear working memory: %w", err)
	}

	return nil
}

// NeedsDocumentRefresh checks if working memory needs refresh for a new query
func (s *MemoryServiceImpl) NeedsDocumentRefresh(ctx context.Context, sessionID string, agentID uuid.UUID, newQuery string) (bool, error) {
	return s.working.IsContextStale(ctx, sessionID, agentID, newQuery, s.config.AutoRefreshThreshold)
}

// GetShortTermService returns the short-term memory service
func (s *MemoryServiceImpl) GetShortTermService() *ShortTermMemoryServiceImpl {
	return s.shortTerm
}

// GetWorkingMemoryService returns the working memory service
func (s *MemoryServiceImpl) GetWorkingMemoryService() *WorkingMemoryServiceImpl {
	return s.working
}

// GetLongTermService returns the long-term memory service
func (s *MemoryServiceImpl) GetLongTermService() *LongTermMemoryServiceImpl {
	return s.longTerm
}

// GetConsolidationService returns the consolidation service
func (s *MemoryServiceImpl) GetConsolidationService() *MemoryConsolidationServiceImpl {
	return s.consolidation
}
