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

func setupWorkingMemoryTestRedis(t *testing.T) (*redis.Client, func()) {
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

func TestWorkingMemoryService_GetWorkingMemory_Empty(t *testing.T) {
	client, cleanup := setupWorkingMemoryTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewWorkingMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	memory, err := service.GetWorkingMemory(context.Background(), sessionID, agentID)

	assert.NoError(t, err)
	assert.NotNil(t, memory)
	assert.Equal(t, sessionID, memory.SessionID)
	assert.Equal(t, agentID, memory.AgentID)
	assert.Empty(t, memory.LoadedDocuments)
	assert.Empty(t, memory.RetrievedChunks)
}

func TestWorkingMemoryService_SetDocumentContext(t *testing.T) {
	client, cleanup := setupWorkingMemoryTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewWorkingMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	chunks := []models.RetrievedChunk{
		{
			ID:           "chunk-1",
			DocumentID:   "doc-1",
			DocumentName: "Test Document",
			Content:      "This is chunk 1 content",
			ChunkNumber:  1,
			Score:        0.95,
		},
		{
			ID:           "chunk-2",
			DocumentID:   "doc-1",
			DocumentName: "Test Document",
			Content:      "This is chunk 2 content",
			ChunkNumber:  2,
			Score:        0.90,
		},
	}

	err := service.SetDocumentContext(context.Background(), sessionID, agentID, chunks)
	assert.NoError(t, err)

	// Retrieve and verify
	memory, err := service.GetWorkingMemory(context.Background(), sessionID, agentID)
	assert.NoError(t, err)
	assert.Len(t, memory.RetrievedChunks, 2)
	assert.Greater(t, memory.TotalTokens, 0)
}

func TestWorkingMemoryService_LoadDocument(t *testing.T) {
	client, cleanup := setupWorkingMemoryTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewWorkingMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	doc := models.LoadedDocument{
		DocumentID:   uuid.New(),
		DocumentName: "Test Document.pdf",
		NotebookID:   uuid.New(),
		ChunkCount:   10,
		TokenCount:   500,
	}

	err := service.LoadDocument(context.Background(), sessionID, agentID, doc)
	assert.NoError(t, err)

	// Retrieve and verify
	memory, err := service.GetWorkingMemory(context.Background(), sessionID, agentID)
	assert.NoError(t, err)
	assert.Len(t, memory.LoadedDocuments, 1)
	assert.Equal(t, doc.DocumentName, memory.LoadedDocuments[0].DocumentName)
}

func TestWorkingMemoryService_UnloadDocument(t *testing.T) {
	client, cleanup := setupWorkingMemoryTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewWorkingMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()
	docID := uuid.New()

	// Load a document
	doc := models.LoadedDocument{
		DocumentID:   docID,
		DocumentName: "Test Document.pdf",
		ChunkCount:   10,
		TokenCount:   500,
	}
	_ = service.LoadDocument(context.Background(), sessionID, agentID, doc)

	// Also set some chunks
	chunks := []models.RetrievedChunk{
		{
			ID:         "chunk-1",
			DocumentID: docID.String(),
			Content:    "Chunk content",
		},
	}
	_ = service.SetDocumentContext(context.Background(), sessionID, agentID, chunks)

	// Unload document
	err := service.UnloadDocument(context.Background(), sessionID, agentID, docID)
	assert.NoError(t, err)

	// Verify unloaded
	memory, _ := service.GetWorkingMemory(context.Background(), sessionID, agentID)
	assert.Empty(t, memory.LoadedDocuments)
	assert.Empty(t, memory.RetrievedChunks)
}

func TestWorkingMemoryService_UpdateLastQuery(t *testing.T) {
	client, cleanup := setupWorkingMemoryTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewWorkingMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	query := "What is the capital of France?"
	err := service.UpdateLastQuery(context.Background(), sessionID, agentID, query)
	assert.NoError(t, err)

	memory, _ := service.GetWorkingMemory(context.Background(), sessionID, agentID)
	assert.Equal(t, query, memory.LastQuery)
	assert.NotNil(t, memory.LastQueryTime)
}

func TestWorkingMemoryService_IsContextStale(t *testing.T) {
	client, cleanup := setupWorkingMemoryTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewWorkingMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	// Initially stale (no context)
	isStale, err := service.IsContextStale(context.Background(), sessionID, agentID, "test query", 0.5)
	assert.NoError(t, err)
	assert.True(t, isStale)

	// Set context
	chunks := []models.RetrievedChunk{{ID: "chunk-1", Content: "test content"}}
	_ = service.SetDocumentContext(context.Background(), sessionID, agentID, chunks)
	_ = service.UpdateLastQuery(context.Background(), sessionID, agentID, "what is the weather")

	// Similar query - not stale
	isStale, _ = service.IsContextStale(context.Background(), sessionID, agentID, "what is the weather today", 0.3)
	assert.False(t, isStale)

	// Very different query - stale
	isStale, _ = service.IsContextStale(context.Background(), sessionID, agentID, "completely different topic", 0.9)
	assert.True(t, isStale)
}

func TestWorkingMemoryService_ClearWorkingMemory(t *testing.T) {
	client, cleanup := setupWorkingMemoryTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewWorkingMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	// Load document and set chunks
	doc := models.LoadedDocument{
		DocumentID:   uuid.New(),
		DocumentName: "Test.pdf",
	}
	_ = service.LoadDocument(context.Background(), sessionID, agentID, doc)

	chunks := []models.RetrievedChunk{{ID: "chunk-1", Content: "test"}}
	_ = service.SetDocumentContext(context.Background(), sessionID, agentID, chunks)

	// Clear
	err := service.ClearWorkingMemory(context.Background(), sessionID, agentID)
	assert.NoError(t, err)

	// Verify cleared (will get empty memory)
	memory, _ := service.GetWorkingMemory(context.Background(), sessionID, agentID)
	assert.Empty(t, memory.LoadedDocuments)
	assert.Empty(t, memory.RetrievedChunks)
}

func TestWorkingMemoryService_FormatForContext(t *testing.T) {
	client, cleanup := setupWorkingMemoryTestRedis(t)
	defer cleanup()

	config := models.DefaultMemoryConfig()
	service := NewWorkingMemoryService(client, config)

	memory := &models.WorkingMemory{
		RetrievedChunks: []models.RetrievedChunk{
			{
				DocumentName: "Document A",
				Content:      "Content from document A",
			},
			{
				DocumentName: "Document A",
				Content:      "More content from A",
			},
			{
				DocumentName: "Document B",
				Content:      "Content from document B",
			},
		},
	}

	formatted := service.FormatForContext(memory)

	assert.Contains(t, formatted, "Document: Document A")
	assert.Contains(t, formatted, "Content from document A")
	assert.Contains(t, formatted, "Document: Document B")
}

func TestWorkingMemoryService_MaxDocumentsLimit(t *testing.T) {
	client, cleanup := setupWorkingMemoryTestRedis(t)
	defer cleanup()

	config := &models.MemoryConfig{
		WorkingMemoryMaxTokens: 8000,
		WorkingMemoryTTL:       30 * time.Minute,
		MaxLoadedDocuments:     3,
	}
	service := NewWorkingMemoryService(client, config)

	sessionID := "test-session"
	agentID := uuid.New()

	// Load 5 documents (should only keep 3)
	for i := 0; i < 5; i++ {
		doc := models.LoadedDocument{
			DocumentID:   uuid.New(),
			DocumentName: "Document " + string(rune('A'+i)),
		}
		_ = service.LoadDocument(context.Background(), sessionID, agentID, doc)
	}

	memory, _ := service.GetWorkingMemory(context.Background(), sessionID, agentID)
	assert.LessOrEqual(t, len(memory.LoadedDocuments), 3)
}
