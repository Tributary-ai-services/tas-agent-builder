package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services/impl"
)

// TestDocumentContextServiceInit tests DocumentContextService initialization
func TestDocumentContextServiceInit(t *testing.T) {
	t.Run("Creates service with valid config", func(t *testing.T) {
		deeplakeCfg := &config.DeepLakeConfig{
			BaseURL: "http://localhost:8000",
			Timeout: 30,
		}
		audimodalCfg := &config.AudiModalConfig{
			BaseURL: "http://localhost:8084",
			Timeout: 30,
		}
		aetherCfg := &config.AetherConfig{
			BaseURL: "http://localhost:8080",
			Timeout: 30,
		}

		// Create a mock cache service
		cacheSvc, err := impl.NewCacheService(nil)
		require.NoError(t, err)

		service := impl.NewDocumentContextService(deeplakeCfg, audimodalCfg, aetherCfg, cacheSvc)
		assert.NotNil(t, service, "Service should be created")
	})
}

// TestVectorSearchContextRetrieval tests vector search context retrieval
func TestVectorSearchContextRetrieval(t *testing.T) {
	// Mock DeepLake server
	mockDeepLake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/datasets/documents/search/text", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Decode request body
		var searchReq map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&searchReq)
		require.NoError(t, err)

		// Return mock response
		response := models.VectorSearchResponse{
			Results: []models.VectorSearchResult{
				{
					ID:         "chunk-1",
					DocumentID: "doc-1",
					Content:    "This is a test document chunk about AI.",
					Score:      0.95,
					Distance:   0.05,
					Rank:       1,
					Metadata:   map[string]interface{}{"source": "test"},
				},
				{
					ID:         "chunk-2",
					DocumentID: "doc-1",
					Content:    "Another chunk with relevant information.",
					Score:      0.85,
					Distance:   0.15,
					Rank:       2,
				},
			},
			TotalFound:  2,
			HasMore:     false,
			QueryTimeMs: 15.5,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockDeepLake.Close()

	deeplakeCfg := &config.DeepLakeConfig{
		BaseURL: mockDeepLake.URL,
		Timeout: 30,
	}
	audimodalCfg := &config.AudiModalConfig{
		BaseURL: "http://localhost:8084",
		Timeout: 30,
	}
	aetherCfg := &config.AetherConfig{
		BaseURL: "http://localhost:8080",
		Timeout: 30,
	}

	cacheSvc, _ := impl.NewCacheService(nil)
	service := impl.NewDocumentContextService(deeplakeCfg, audimodalCfg, aetherCfg, cacheSvc)

	t.Run("Retrieves vector context successfully", func(t *testing.T) {
		ctx := context.Background()
		req := models.VectorSearchRequest{
			QueryText: "What is AI?",
			TenantID:  "test-tenant",
			Options: models.SearchOptions{
				TopK:          10,
				MinScore:      0.7,
				IncludeChunks: true,
			},
		}

		result, err := service.RetrieveVectorContext(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Chunks, 2)
		assert.Equal(t, models.ContextStrategyVector, result.Strategy)
		assert.Equal(t, "This is a test document chunk about AI.", result.Chunks[0].Content)
	})
}

// TestTokenEstimation tests token count estimation
func TestTokenEstimation(t *testing.T) {
	deeplakeCfg := &config.DeepLakeConfig{
		BaseURL: "http://localhost:8000",
		Timeout: 30,
	}
	audimodalCfg := &config.AudiModalConfig{
		BaseURL: "http://localhost:8084",
		Timeout: 30,
	}
	aetherCfg := &config.AetherConfig{
		BaseURL: "http://localhost:8080",
		Timeout: 30,
	}

	cacheSvc, _ := impl.NewCacheService(nil)
	service := impl.NewDocumentContextService(deeplakeCfg, audimodalCfg, aetherCfg, cacheSvc)

	t.Run("Estimates token count for short text", func(t *testing.T) {
		text := "Hello, world!"
		tokens := service.EstimateTokenCount(text)
		assert.Greater(t, tokens, 0)
		assert.Less(t, tokens, 10)
	})

	t.Run("Estimates token count for longer text", func(t *testing.T) {
		text := "This is a longer text that should have more tokens. It contains multiple sentences and should give us a reasonable token estimate."
		tokens := service.EstimateTokenCount(text)
		assert.Greater(t, tokens, 20)
	})

	t.Run("Returns zero for empty text", func(t *testing.T) {
		tokens := service.EstimateTokenCount("")
		assert.Equal(t, 0, tokens)
	})
}

// TestContextFormatting tests context formatting for injection
func TestContextFormatting(t *testing.T) {
	deeplakeCfg := &config.DeepLakeConfig{
		BaseURL: "http://localhost:8000",
		Timeout: 30,
	}
	audimodalCfg := &config.AudiModalConfig{
		BaseURL: "http://localhost:8084",
		Timeout: 30,
	}
	aetherCfg := &config.AetherConfig{
		BaseURL: "http://localhost:8080",
		Timeout: 30,
	}

	cacheSvc, _ := impl.NewCacheService(nil)
	service := impl.NewDocumentContextService(deeplakeCfg, audimodalCfg, aetherCfg, cacheSvc)

	t.Run("Formats context for injection", func(t *testing.T) {
		result := &models.DocumentContextResult{
			Chunks: []models.RetrievedChunk{
				{
					ID:           "chunk-1",
					DocumentID:   "doc-1",
					DocumentName: "test-doc.pdf",
					Content:      "First chunk content about AI systems.",
					ChunkNumber:  1,
					Score:        0.95,
				},
				{
					ID:           "chunk-2",
					DocumentID:   "doc-1",
					DocumentName: "test-doc.pdf",
					Content:      "Second chunk about machine learning.",
					ChunkNumber:  2,
					Score:        0.88,
				},
			},
			TotalTokens: 100,
			Strategy:    models.ContextStrategyVector,
		}

		injection, err := service.FormatContextForInjection(result, 1000)
		require.NoError(t, err)
		assert.NotNil(t, injection)
		assert.Contains(t, injection.FormattedContext, "First chunk content")
		assert.Contains(t, injection.FormattedContext, "Second chunk")
		assert.Equal(t, 2, injection.ChunkCount)
		assert.Equal(t, 1, injection.DocumentCount)
	})

	t.Run("Truncates context when exceeding max tokens", func(t *testing.T) {
		// Create a result with lots of content
		chunks := make([]models.RetrievedChunk, 50)
		for i := 0; i < 50; i++ {
			chunks[i] = models.RetrievedChunk{
				ID:           uuid.New().String(),
				DocumentID:   "doc-1",
				DocumentName: "big-doc.pdf",
				Content:      "This is chunk number " + string(rune(i)) + " with some content that takes up tokens.",
				ChunkNumber:  i + 1,
				Score:        0.9 - float64(i)*0.01,
			}
		}

		result := &models.DocumentContextResult{
			Chunks:      chunks,
			TotalTokens: 5000,
			Strategy:    models.ContextStrategyFull,
		}

		// Request a small max tokens limit
		injection, err := service.FormatContextForInjection(result, 100)
		require.NoError(t, err)
		assert.NotNil(t, injection)
		assert.True(t, injection.Truncated)
		assert.Less(t, injection.TotalTokens, 5000)
	})
}

// TestCacheService tests the cache service functionality
func TestCacheService(t *testing.T) {
	t.Run("Creates cache service without Redis (disabled)", func(t *testing.T) {
		cacheSvc, err := impl.NewCacheService(nil)
		require.NoError(t, err)
		assert.NotNil(t, cacheSvc)
	})

	t.Run("Generates cache keys correctly", func(t *testing.T) {
		cacheSvc, err := impl.NewCacheService(nil)
		require.NoError(t, err)

		agentID := uuid.New()
		sessionID := "session-123"
		queryHash := "abc123hash"

		key := cacheSvc.GenerateCacheKey(agentID, &sessionID, queryHash)
		assert.Contains(t, key, agentID.String())
		assert.Contains(t, key, sessionID)
		assert.Contains(t, key, queryHash)
	})

	t.Run("Cache set and get work in memory mode", func(t *testing.T) {
		cacheSvc, err := impl.NewCacheService(nil)
		require.NoError(t, err)

		ctx := context.Background()
		testResult := &models.DocumentContextResult{
			Chunks: []models.RetrievedChunk{
				{Content: "Test chunk"},
			},
			TotalTokens: 10,
			Strategy:    models.ContextStrategyVector,
		}

		// Set cache
		err = cacheSvc.SetCachedContext(ctx, "test-key", testResult, 300)
		require.NoError(t, err)

		// Get cache
		retrieved, err := cacheSvc.GetCachedContext(ctx, "test-key")
		require.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, testResult.TotalTokens, retrieved.TotalTokens)
		assert.Equal(t, testResult.Strategy, retrieved.Strategy)
	})

	t.Run("Returns nil for non-existent cache key", func(t *testing.T) {
		cacheSvc, err := impl.NewCacheService(nil)
		require.NoError(t, err)

		ctx := context.Background()
		retrieved, err := cacheSvc.GetCachedContext(ctx, "non-existent-key")
		require.NoError(t, err)
		assert.Nil(t, retrieved)
	})
}

// TestNotebookDocumentRetrieval tests notebook document retrieval from Aether-BE
func TestNotebookDocumentRetrieval(t *testing.T) {
	// Mock Aether-BE server
	mockAether := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for internal notebook documents endpoint
		if r.URL.Path == "/api/v1/internal/notebooks/test-notebook-id/documents" {
			response := map[string]interface{}{
				"notebook_id": "test-notebook-id",
				"documents": []map[string]interface{}{
					{
						"id":           uuid.New().String(),
						"name":         "Document 1.pdf",
						"notebook_id":  "test-notebook-id",
						"file_id":      uuid.New().String(),
						"content_type": "application/pdf",
						"size_bytes":   1024,
						"chunk_count":  5,
					},
					{
						"id":           uuid.New().String(),
						"name":         "Document 2.txt",
						"notebook_id":  "test-notebook-id",
						"file_id":      uuid.New().String(),
						"content_type": "text/plain",
						"size_bytes":   512,
						"chunk_count":  2,
					},
				},
				"total": 2,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Check for recursive documents endpoint
		if r.URL.Path == "/api/v1/internal/notebooks/test-notebook-id/documents/recursive" {
			response := map[string]interface{}{
				"notebook_id": "test-notebook-id",
				"documents": []map[string]interface{}{
					{
						"id":            uuid.New().String(),
						"name":          "Root Doc.pdf",
						"notebook_id":   "test-notebook-id",
						"notebook_name": "Root Notebook",
						"file_id":       uuid.New().String(),
						"content_type":  "application/pdf",
					},
					{
						"id":            uuid.New().String(),
						"name":          "Child Doc.pdf",
						"notebook_id":   "child-notebook-id",
						"notebook_name": "Child Notebook",
						"file_id":       uuid.New().String(),
						"content_type":  "application/pdf",
					},
				},
				"total": 2,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		http.NotFound(w, r)
	}))
	defer mockAether.Close()

	deeplakeCfg := &config.DeepLakeConfig{
		BaseURL: "http://localhost:8000",
		Timeout: 30,
	}
	audimodalCfg := &config.AudiModalConfig{
		BaseURL: "http://localhost:8084",
		Timeout: 30,
	}
	aetherCfg := &config.AetherConfig{
		BaseURL: mockAether.URL,
		Timeout: 30,
	}

	cacheSvc, _ := impl.NewCacheService(nil)
	service := impl.NewDocumentContextService(deeplakeCfg, audimodalCfg, aetherCfg, cacheSvc)

	t.Run("Gets documents from single notebook", func(t *testing.T) {
		ctx := context.Background()
		notebookID, _ := uuid.Parse("test-notebook-id")

		docs, err := service.GetNotebookDocuments(ctx, []uuid.UUID{notebookID}, "test-tenant", false)
		require.NoError(t, err)
		assert.Len(t, docs, 2)
	})

	t.Run("Gets documents recursively from notebook hierarchy", func(t *testing.T) {
		ctx := context.Background()
		notebookID, _ := uuid.Parse("test-notebook-id")

		docs, err := service.GetNotebookDocuments(ctx, []uuid.UUID{notebookID}, "test-tenant", true)
		require.NoError(t, err)
		assert.Len(t, docs, 2)
	})
}

// TestHybridContextRetrieval tests hybrid context retrieval combining vector and full docs
func TestHybridContextRetrieval(t *testing.T) {
	// Mock servers
	mockDeepLake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := models.VectorSearchResponse{
			Results: []models.VectorSearchResult{
				{
					ID:         "chunk-1",
					DocumentID: "doc-1",
					Content:    "Vector search result 1",
					Score:      0.95,
					Rank:       1,
				},
			},
			TotalFound:  1,
			QueryTimeMs: 10,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockDeepLake.Close()

	mockAudiModal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"chunks": []map[string]interface{}{
				{
					"id":           uuid.New().String(),
					"content":      "Full document chunk 1",
					"chunk_number": 1,
					"chunk_type":   "text",
				},
				{
					"id":           uuid.New().String(),
					"content":      "Full document chunk 2",
					"chunk_number": 2,
					"chunk_type":   "text",
				},
			},
			"total":    2,
			"has_more": false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockAudiModal.Close()

	deeplakeCfg := &config.DeepLakeConfig{
		BaseURL: mockDeepLake.URL,
		Timeout: 30,
	}
	audimodalCfg := &config.AudiModalConfig{
		BaseURL: mockAudiModal.URL,
		Timeout: 30,
	}
	aetherCfg := &config.AetherConfig{
		BaseURL: "http://localhost:8080",
		Timeout: 30,
	}

	cacheSvc, _ := impl.NewCacheService(nil)
	service := impl.NewDocumentContextService(deeplakeCfg, audimodalCfg, aetherCfg, cacheSvc)

	t.Run("Retrieves hybrid context successfully", func(t *testing.T) {
		ctx := context.Background()
		docID := uuid.New()

		req := models.ChunkRetrievalRequest{
			TenantID: "test-tenant",
			FileIDs:  []uuid.UUID{docID},
		}

		result, err := service.RetrieveHybridContext(ctx, "test query", req, 0.5, 0.5)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, models.ContextStrategyHybrid, result.Strategy)
		// Should contain results from both vector search and full document retrieval
		assert.GreaterOrEqual(t, len(result.Chunks), 1)
	})
}

// TestDocumentContextModels tests the document context model structures
func TestDocumentContextModels(t *testing.T) {
	t.Run("VectorSearchRequest serializes correctly", func(t *testing.T) {
		req := models.VectorSearchRequest{
			QueryText:   "test query",
			TenantID:    "tenant-123",
			NotebookIDs: []uuid.UUID{uuid.New()},
			Options: models.SearchOptions{
				TopK:     10,
				MinScore: 0.75,
			},
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)
		assert.Contains(t, string(data), "test query")
		assert.Contains(t, string(data), "tenant-123")
	})

	t.Run("DocumentContextResult deserializes correctly", func(t *testing.T) {
		jsonData := `{
			"chunks": [{"id": "1", "content": "test", "score": 0.9}],
			"total_tokens": 100,
			"strategy": "vector"
		}`

		var result models.DocumentContextResult
		err := json.Unmarshal([]byte(jsonData), &result)
		require.NoError(t, err)
		assert.Len(t, result.Chunks, 1)
		assert.Equal(t, 100, result.TotalTokens)
		assert.Equal(t, models.ContextStrategyVector, result.Strategy)
	})

	t.Run("ContextStrategy enum values are correct", func(t *testing.T) {
		assert.Equal(t, models.ContextStrategy("vector"), models.ContextStrategyVector)
		assert.Equal(t, models.ContextStrategy("full"), models.ContextStrategyFull)
		assert.Equal(t, models.ContextStrategy("hybrid"), models.ContextStrategyHybrid)
		assert.Equal(t, models.ContextStrategy("mcp"), models.ContextStrategyMCP)
		assert.Equal(t, models.ContextStrategy("none"), models.ContextStrategyNone)
	})
}
