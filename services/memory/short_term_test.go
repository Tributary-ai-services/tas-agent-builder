package memory

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tas-agent-builder/models"
)

func setupTestRedis(t *testing.T) (*redis.Client, func()) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cleanup := func() {
		client.Close()
		mr.Close()
	}

	return client, cleanup
}

func TestShortTermMemoryService_GetConversation_Empty(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewShortTermMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	memory, err := service.GetConversation(context.Background(), sessionID, agentID)

	assert.NoError(t, err)
	assert.NotNil(t, memory)
	assert.Equal(t, sessionID, memory.SessionID)
	assert.Equal(t, agentID, memory.AgentID)
	assert.Empty(t, memory.Entries)
	assert.Equal(t, 0, memory.TotalTokens)
}

func TestShortTermMemoryService_AddMessage(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewShortTermMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	// Add a message
	entry := models.MemoryEntry{
		SessionID:  sessionID,
		AgentID:    agentID,
		TenantID:   "test-tenant",
		UserID:     "test-user",
		Role:       "user",
		Content:    "Hello, how are you?",
		TokenCount: 10,
	}

	err := service.AddMessage(context.Background(), entry)
	assert.NoError(t, err)

	// Retrieve and verify
	memory, err := service.GetConversation(context.Background(), sessionID, agentID)
	assert.NoError(t, err)
	assert.Len(t, memory.Entries, 1)
	assert.Equal(t, "user", memory.Entries[0].Role)
	assert.Equal(t, "Hello, how are you?", memory.Entries[0].Content)
	assert.Equal(t, 10, memory.TotalTokens)
}

func TestShortTermMemoryService_AddMultipleMessages(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewShortTermMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	// Add user message
	userEntry := models.MemoryEntry{
		SessionID:  sessionID,
		AgentID:    agentID,
		Role:       "user",
		Content:    "What is the weather?",
		TokenCount: 10,
	}
	err := service.AddMessage(context.Background(), userEntry)
	assert.NoError(t, err)

	// Add assistant message
	assistantEntry := models.MemoryEntry{
		SessionID:  sessionID,
		AgentID:    agentID,
		Role:       "assistant",
		Content:    "I don't have access to weather data.",
		TokenCount: 15,
	}
	err = service.AddMessage(context.Background(), assistantEntry)
	assert.NoError(t, err)

	// Retrieve and verify
	memory, err := service.GetConversation(context.Background(), sessionID, agentID)
	assert.NoError(t, err)
	assert.Len(t, memory.Entries, 2)
	assert.Equal(t, 25, memory.TotalTokens)
}

func TestShortTermMemoryService_GetRecentMessages(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewShortTermMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	// Add multiple messages
	for i := 0; i < 5; i++ {
		entry := models.MemoryEntry{
			SessionID:  sessionID,
			AgentID:    agentID,
			Role:       "user",
			Content:    "Message " + string(rune('A'+i)),
			TokenCount: 5,
		}
		err := service.AddMessage(context.Background(), entry)
		assert.NoError(t, err)
	}

	// Get only 3 recent messages
	recent, err := service.GetRecentMessages(context.Background(), sessionID, agentID, 3)
	assert.NoError(t, err)
	assert.Len(t, recent, 3)
	assert.Equal(t, "Message C", recent[0].Content)
	assert.Equal(t, "Message E", recent[2].Content)
}

func TestShortTermMemoryService_TrimToTokenLimit(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewShortTermMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	// Add multiple messages
	for i := 0; i < 5; i++ {
		entry := models.MemoryEntry{
			SessionID:  sessionID,
			AgentID:    agentID,
			Role:       "user",
			Content:    "Message content here",
			TokenCount: 100,
		}
		err := service.AddMessage(context.Background(), entry)
		assert.NoError(t, err)
	}

	// Verify total is 500
	memory, _ := service.GetConversation(context.Background(), sessionID, agentID)
	assert.Equal(t, 500, memory.TotalTokens)

	// Trim to 250 tokens
	err := service.TrimToTokenLimit(context.Background(), sessionID, agentID, 250)
	assert.NoError(t, err)

	// Verify trimmed
	memory, _ = service.GetConversation(context.Background(), sessionID, agentID)
	assert.LessOrEqual(t, memory.TotalTokens, 250)
}

func TestShortTermMemoryService_ClearConversation(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewShortTermMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	// Add a message
	entry := models.MemoryEntry{
		SessionID:  sessionID,
		AgentID:    agentID,
		Role:       "user",
		Content:    "Test message",
		TokenCount: 5,
	}
	_ = service.AddMessage(context.Background(), entry)

	// Clear
	err := service.ClearConversation(context.Background(), sessionID, agentID)
	assert.NoError(t, err)

	// Verify cleared
	memory, _ := service.GetConversation(context.Background(), sessionID, agentID)
	assert.Empty(t, memory.Entries)
}

func TestShortTermMemoryService_FormatForContext(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewShortTermMemoryService(client, config)

	memory := &models.ShortTermMemory{
		Entries: []models.MemoryEntry{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
	}

	formatted := service.FormatForContext(memory)

	assert.Contains(t, formatted, "User: Hello")
	assert.Contains(t, formatted, "Assistant: Hi there!")
	assert.Contains(t, formatted, "User: How are you?")
}

func TestShortTermMemoryService_AutoTrim(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	config := &models.MemoryConfig{
		ShortTermMaxTokens:  100,
		ShortTermTTL:        time.Hour,
		ShortTermMaxEntries: 3,
	}
	service := NewShortTermMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	// Add 5 messages (should auto-trim to 3)
	for i := 0; i < 5; i++ {
		entry := models.MemoryEntry{
			SessionID:  sessionID,
			AgentID:    agentID,
			Role:       "user",
			Content:    "Message " + string(rune('A'+i)),
			TokenCount: 10,
		}
		_ = service.AddMessage(context.Background(), entry)
	}

	// Should only have 3 entries
	memory, _ := service.GetConversation(context.Background(), sessionID, agentID)
	assert.LessOrEqual(t, len(memory.Entries), 3)
}
