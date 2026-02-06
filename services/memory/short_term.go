package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tas-agent-builder/models"
)

// ShortTermMemoryServiceImpl implements ShortTermMemoryService using Redis
type ShortTermMemoryServiceImpl struct {
	redis     *redis.Client
	config    *models.MemoryConfig
	keyPrefix string
}

// NewShortTermMemoryService creates a new short-term memory service
func NewShortTermMemoryService(redisClient *redis.Client, config *models.MemoryConfig) *ShortTermMemoryServiceImpl {
	if config == nil {
		config = models.DefaultMemoryConfig()
	}
	return &ShortTermMemoryServiceImpl{
		redis:     redisClient,
		config:    config,
		keyPrefix: "memory:short_term",
	}
}

// memoryKey generates the Redis key for a session's short-term memory
func (s *ShortTermMemoryServiceImpl) memoryKey(sessionID string, agentID uuid.UUID) string {
	return fmt.Sprintf("%s:%s:%s", s.keyPrefix, agentID.String(), sessionID)
}

// GetConversation retrieves the conversation history for a session
func (s *ShortTermMemoryServiceImpl) GetConversation(ctx context.Context, sessionID string, agentID uuid.UUID) (*models.ShortTermMemory, error) {
	key := s.memoryKey(sessionID, agentID)

	data, err := s.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			// Return empty memory if not found
			return &models.ShortTermMemory{
				SessionID:   sessionID,
				AgentID:     agentID,
				Entries:     []models.MemoryEntry{},
				TotalTokens: 0,
				MaxTokens:   s.config.ShortTermMaxTokens,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				ExpiresAt:   time.Now().Add(s.config.ShortTermTTL),
			}, nil
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	var memory models.ShortTermMemory
	if err := json.Unmarshal(data, &memory); err != nil {
		return nil, fmt.Errorf("failed to unmarshal conversation: %w", err)
	}

	return &memory, nil
}

// AddMessage adds a message to the conversation buffer
func (s *ShortTermMemoryServiceImpl) AddMessage(ctx context.Context, entry models.MemoryEntry) error {
	key := s.memoryKey(entry.SessionID, entry.AgentID)

	// Get existing memory or create new
	memory, err := s.GetConversation(ctx, entry.SessionID, entry.AgentID)
	if err != nil {
		return fmt.Errorf("failed to get existing conversation: %w", err)
	}

	// Set entry timestamp if not set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Generate ID if not set
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	entry.Type = models.MemoryTypeShortTerm

	// Add entry
	memory.Entries = append(memory.Entries, entry)
	memory.TotalTokens += entry.TokenCount
	memory.UpdatedAt = time.Now()

	// Trim if exceeds limits
	if memory.TotalTokens > s.config.ShortTermMaxTokens || len(memory.Entries) > s.config.ShortTermMaxEntries {
		s.trimMemory(memory)
	}

	// Serialize and store
	data, err := json.Marshal(memory)
	if err != nil {
		return fmt.Errorf("failed to marshal conversation: %w", err)
	}

	// Store with TTL
	if err := s.redis.Set(ctx, key, data, s.config.ShortTermTTL).Err(); err != nil {
		return fmt.Errorf("failed to store conversation: %w", err)
	}

	return nil
}

// GetRecentMessages retrieves the N most recent messages
func (s *ShortTermMemoryServiceImpl) GetRecentMessages(ctx context.Context, sessionID string, agentID uuid.UUID, limit int) ([]models.MemoryEntry, error) {
	memory, err := s.GetConversation(ctx, sessionID, agentID)
	if err != nil {
		return nil, err
	}

	if len(memory.Entries) <= limit {
		return memory.Entries, nil
	}

	// Return the most recent entries
	start := len(memory.Entries) - limit
	return memory.Entries[start:], nil
}

// TrimToTokenLimit trims conversation to fit within token limit
func (s *ShortTermMemoryServiceImpl) TrimToTokenLimit(ctx context.Context, sessionID string, agentID uuid.UUID, maxTokens int) error {
	memory, err := s.GetConversation(ctx, sessionID, agentID)
	if err != nil {
		return err
	}

	if memory.TotalTokens <= maxTokens {
		return nil // No trimming needed
	}

	// Trim from the beginning (oldest messages first)
	for memory.TotalTokens > maxTokens && len(memory.Entries) > 0 {
		removed := memory.Entries[0]
		memory.Entries = memory.Entries[1:]
		memory.TotalTokens -= removed.TokenCount
	}

	memory.UpdatedAt = time.Now()

	// Store updated memory
	key := s.memoryKey(sessionID, agentID)
	data, err := json.Marshal(memory)
	if err != nil {
		return fmt.Errorf("failed to marshal conversation: %w", err)
	}

	ttl := s.redis.TTL(ctx, key).Val()
	if ttl <= 0 {
		ttl = s.config.ShortTermTTL
	}

	return s.redis.Set(ctx, key, data, ttl).Err()
}

// ClearConversation clears the conversation buffer for a session
func (s *ShortTermMemoryServiceImpl) ClearConversation(ctx context.Context, sessionID string, agentID uuid.UUID) error {
	key := s.memoryKey(sessionID, agentID)
	return s.redis.Del(ctx, key).Err()
}

// SetExpiration updates the TTL for a session's memory
func (s *ShortTermMemoryServiceImpl) SetExpiration(ctx context.Context, sessionID string, agentID uuid.UUID, ttlSeconds int) error {
	key := s.memoryKey(sessionID, agentID)
	return s.redis.Expire(ctx, key, time.Duration(ttlSeconds)*time.Second).Err()
}

// trimMemory trims memory to fit within configured limits
func (s *ShortTermMemoryServiceImpl) trimMemory(memory *models.ShortTermMemory) {
	// First, trim by entry count
	for len(memory.Entries) > s.config.ShortTermMaxEntries {
		removed := memory.Entries[0]
		memory.Entries = memory.Entries[1:]
		memory.TotalTokens -= removed.TokenCount
	}

	// Then, trim by token count
	for memory.TotalTokens > s.config.ShortTermMaxTokens && len(memory.Entries) > 0 {
		removed := memory.Entries[0]
		memory.Entries = memory.Entries[1:]
		memory.TotalTokens -= removed.TokenCount
	}
}

// FormatForContext formats the short-term memory for context injection
func (s *ShortTermMemoryServiceImpl) FormatForContext(memory *models.ShortTermMemory) string {
	if memory == nil || len(memory.Entries) == 0 {
		return ""
	}

	var formatted string
	for _, entry := range memory.Entries {
		switch entry.Role {
		case "user":
			formatted += fmt.Sprintf("User: %s\n", entry.Content)
		case "assistant":
			formatted += fmt.Sprintf("Assistant: %s\n", entry.Content)
		case "system":
			formatted += fmt.Sprintf("System: %s\n", entry.Content)
		}
	}

	return formatted
}
