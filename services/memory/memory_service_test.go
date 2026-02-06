package memory

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
)

func setupMemoryServiceTestRedis(t *testing.T) (*redis.Client, func()) {
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

func TestMemoryService_GetMemoryState_Empty(t *testing.T) {
	client, cleanup := setupMemoryServiceTestRedis(t)
	defer cleanup()

	deeplakeConfig := &config.DeepLakeConfig{
		BaseURL: "http://localhost:8000",
	}
	routerConfig := &config.RouterConfig{
		BaseURL: "http://localhost:8081",
	}

	service := NewMemoryService(client, deeplakeConfig, routerConfig, nil)

	req := models.GetMemoryRequest{
		SessionID: "test-session",
		AgentID:   uuid.New(),
		TenantID:  "test-tenant",
		UserID:    "test-user",
	}

	state, err := service.GetMemoryState(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, state)
	assert.NotNil(t, state.ShortTerm)
	assert.NotNil(t, state.Working)
	assert.Empty(t, state.ShortTerm.Entries)
}

func TestMemoryService_AddMemory(t *testing.T) {
	client, cleanup := setupMemoryServiceTestRedis(t)
	defer cleanup()

	deeplakeConfig := &config.DeepLakeConfig{
		BaseURL: "http://localhost:8000",
	}
	routerConfig := &config.RouterConfig{
		BaseURL: "http://localhost:8081",
	}

	service := NewMemoryService(client, deeplakeConfig, routerConfig, nil)

	sessionID := "test-session"
	agentID := uuid.New()

	// Add user message
	req := models.AddMemoryRequest{
		SessionID: sessionID,
		AgentID:   agentID,
		TenantID:  "test-tenant",
		UserID:    "test-user",
		Role:      "user",
		Content:   "Hello, how are you?",
	}

	err := service.AddMemory(context.Background(), req)
	assert.NoError(t, err)

	// Add assistant message
	req2 := models.AddMemoryRequest{
		SessionID: sessionID,
		AgentID:   agentID,
		TenantID:  "test-tenant",
		UserID:    "test-user",
		Role:      "assistant",
		Content:   "I'm doing well, thank you!",
	}

	err = service.AddMemory(context.Background(), req2)
	assert.NoError(t, err)

	// Retrieve state
	getReq := models.GetMemoryRequest{
		SessionID: sessionID,
		AgentID:   agentID,
	}
	state, err := service.GetMemoryState(context.Background(), getReq)
	assert.NoError(t, err)
	assert.Len(t, state.ShortTerm.Entries, 2)
}

func TestMemoryService_GetFormattedMemory(t *testing.T) {
	client, cleanup := setupMemoryServiceTestRedis(t)
	defer cleanup()

	deeplakeConfig := &config.DeepLakeConfig{
		BaseURL: "http://localhost:8000",
	}
	routerConfig := &config.RouterConfig{
		BaseURL: "http://localhost:8081",
	}

	service := NewMemoryService(client, deeplakeConfig, routerConfig, nil)

	sessionID := "test-session"
	agentID := uuid.New()

	// Add some messages
	for i := 0; i < 3; i++ {
		req := models.AddMemoryRequest{
			SessionID: sessionID,
			AgentID:   agentID,
			Role:      "user",
			Content:   "Test message " + string(rune('A'+i)),
		}
		_ = service.AddMemory(context.Background(), req)
	}

	// Get formatted memory
	getReq := models.GetMemoryRequest{
		SessionID: sessionID,
		AgentID:   agentID,
	}
	memoryCtx, err := service.GetFormattedMemory(context.Background(), getReq, 4000)
	assert.NoError(t, err)
	assert.NotEmpty(t, memoryCtx.FormattedShortTerm)
}

func TestMemoryService_UpdateWorkingMemory(t *testing.T) {
	client, cleanup := setupMemoryServiceTestRedis(t)
	defer cleanup()

	deeplakeConfig := &config.DeepLakeConfig{
		BaseURL: "http://localhost:8000",
	}
	routerConfig := &config.RouterConfig{
		BaseURL: "http://localhost:8081",
	}

	service := NewMemoryService(client, deeplakeConfig, routerConfig, nil)

	sessionID := "test-session"
	agentID := uuid.New()

	chunks := []models.RetrievedChunk{
		{
			ID:           "chunk-1",
			DocumentName: "Test Doc",
			Content:      "Test content",
		},
	}

	err := service.UpdateWorkingMemory(context.Background(), sessionID, agentID, chunks)
	assert.NoError(t, err)

	// Verify
	state, _ := service.GetMemoryState(context.Background(), models.GetMemoryRequest{
		SessionID: sessionID,
		AgentID:   agentID,
	})
	assert.Len(t, state.Working.RetrievedChunks, 1)
}

func TestMemoryService_ClearSession(t *testing.T) {
	client, cleanup := setupMemoryServiceTestRedis(t)
	defer cleanup()

	deeplakeConfig := &config.DeepLakeConfig{
		BaseURL: "http://localhost:8000",
	}
	routerConfig := &config.RouterConfig{
		BaseURL: "http://localhost:8081",
	}

	service := NewMemoryService(client, deeplakeConfig, routerConfig, nil)

	sessionID := "test-session"
	agentID := uuid.New()

	// Add some data
	_ = service.AddMemory(context.Background(), models.AddMemoryRequest{
		SessionID: sessionID,
		AgentID:   agentID,
		Role:      "user",
		Content:   "Test",
	})

	_ = service.UpdateWorkingMemory(context.Background(), sessionID, agentID, []models.RetrievedChunk{
		{ID: "chunk-1", Content: "test"},
	})

	// Clear
	err := service.ClearSession(context.Background(), sessionID, agentID)
	assert.NoError(t, err)

	// Verify cleared
	state, _ := service.GetMemoryState(context.Background(), models.GetMemoryRequest{
		SessionID: sessionID,
		AgentID:   agentID,
	})
	assert.Empty(t, state.ShortTerm.Entries)
	assert.Empty(t, state.Working.RetrievedChunks)
}

func TestMemoryService_GetMemoryStats(t *testing.T) {
	client, cleanup := setupMemoryServiceTestRedis(t)
	defer cleanup()

	deeplakeConfig := &config.DeepLakeConfig{
		BaseURL: "http://localhost:8000",
	}
	routerConfig := &config.RouterConfig{
		BaseURL: "http://localhost:8081",
	}

	service := NewMemoryService(client, deeplakeConfig, routerConfig, nil)

	sessionID := "test-session"
	agentID := uuid.New()

	// Add some data
	_ = service.AddMemory(context.Background(), models.AddMemoryRequest{
		SessionID: sessionID,
		AgentID:   agentID,
		Role:      "user",
		Content:   "Test message with some content",
	})

	// Get stats
	stats, err := service.GetMemoryStats(context.Background(), sessionID, agentID)
	assert.NoError(t, err)
	assert.Equal(t, 1, stats.ShortTermEntries)
	assert.Greater(t, stats.ShortTermTokens, 0)
}

func TestMemoryService_NeedsDocumentRefresh(t *testing.T) {
	client, cleanup := setupMemoryServiceTestRedis(t)
	defer cleanup()

	deeplakeConfig := &config.DeepLakeConfig{
		BaseURL: "http://localhost:8000",
	}
	routerConfig := &config.RouterConfig{
		BaseURL: "http://localhost:8081",
	}

	service := NewMemoryService(client, deeplakeConfig, routerConfig, nil)

	sessionID := "test-session"
	agentID := uuid.New()

	// Initially needs refresh
	needsRefresh, err := service.NeedsDocumentRefresh(context.Background(), sessionID, agentID, "test query")
	assert.NoError(t, err)
	assert.True(t, needsRefresh)
}

func TestMemoryService_GetServices(t *testing.T) {
	client, cleanup := setupMemoryServiceTestRedis(t)
	defer cleanup()

	deeplakeConfig := &config.DeepLakeConfig{
		BaseURL: "http://localhost:8000",
	}
	routerConfig := &config.RouterConfig{
		BaseURL: "http://localhost:8081",
	}

	service := NewMemoryService(client, deeplakeConfig, routerConfig, nil)

	assert.NotNil(t, service.GetShortTermService())
	assert.NotNil(t, service.GetWorkingMemoryService())
	assert.NotNil(t, service.GetLongTermService())
	assert.NotNil(t, service.GetConsolidationService())
}
