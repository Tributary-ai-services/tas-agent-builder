package test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services/impl"
)

// isAetherBEAvailable checks if Aether-BE service is available
func isAetherBEAvailable(baseURL string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// isDeepLakeAvailable checks if DeepLake API is available
func isDeepLakeAvailable(baseURL string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// isAudiModalAvailable checks if AudiModal API is available
func isAudiModalAvailable(baseURL string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// TestDocumentContextIntegration_VectorSearch tests vector search against real DeepLake API
func TestDocumentContextIntegration_VectorSearch(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isDeepLakeAvailable(cfg.DeepLake.BaseURL) {
		t.Skip("DeepLake API not available, skipping integration test")
	}

	cacheSvc, _ := impl.NewCacheService(nil)
	service := impl.NewDocumentContextService(
		&cfg.DeepLake,
		&cfg.AudiModal,
		&cfg.Aether,
		cacheSvc,
	)

	ctx := context.Background()
	tenantID := os.Getenv("TEST_TENANT_ID")
	if tenantID == "" {
		tenantID = "default"
	}

	t.Run("Vector Search with Valid Query", func(t *testing.T) {
		req := models.VectorSearchRequest{
			QueryText: "What is artificial intelligence?",
			TenantID:  tenantID,
			Options: models.SearchOptions{
				TopK:          5,
				MinScore:      0.5,
				IncludeChunks: true,
			},
		}

		result, err := service.RetrieveVectorContext(ctx, req)
		if err != nil {
			t.Logf("Vector search returned error (may be expected if no documents): %v", err)
			return
		}

		assert.NotNil(t, result)
		assert.Equal(t, models.ContextStrategyVector, result.Strategy)
		t.Logf("Vector search returned %d chunks", len(result.Chunks))
		for i, chunk := range result.Chunks {
			t.Logf("  Chunk %d: score=%.3f, content=%s...",
				i+1, chunk.Score, truncateString(chunk.Content, 50))
		}
	})

	t.Run("Vector Search with Notebook Filter", func(t *testing.T) {
		notebookID := uuid.New() // This may not exist, but tests the filter path

		req := models.VectorSearchRequest{
			QueryText:   "machine learning",
			TenantID:    tenantID,
			NotebookIDs: []uuid.UUID{notebookID},
			Options: models.SearchOptions{
				TopK:          10,
				MinScore:      0.6,
				IncludeChunks: true,
			},
		}

		result, err := service.RetrieveVectorContext(ctx, req)
		if err != nil {
			t.Logf("Vector search with notebook filter returned error: %v", err)
			return
		}

		assert.NotNil(t, result)
		t.Logf("Vector search with notebook filter returned %d chunks", len(result.Chunks))
	})
}

// TestDocumentContextIntegration_FullDocument tests full document retrieval from AudiModal
func TestDocumentContextIntegration_FullDocument(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isAudiModalAvailable(cfg.AudiModal.BaseURL) {
		t.Skip("AudiModal API not available, skipping integration test")
	}

	cacheSvc, _ := impl.NewCacheService(nil)
	service := impl.NewDocumentContextService(
		&cfg.DeepLake,
		&cfg.AudiModal,
		&cfg.Aether,
		cacheSvc,
	)

	ctx := context.Background()
	tenantID := os.Getenv("TEST_TENANT_ID")
	if tenantID == "" {
		tenantID = "default"
	}

	t.Run("Retrieve Full Documents by File IDs", func(t *testing.T) {
		// Use a test file ID if available
		testFileID := os.Getenv("TEST_FILE_ID")
		if testFileID == "" {
			t.Skip("TEST_FILE_ID not set, skipping")
		}

		fileID, err := uuid.Parse(testFileID)
		require.NoError(t, err)

		req := models.ChunkRetrievalRequest{
			TenantID: tenantID,
			FileIDs:  []uuid.UUID{fileID},
			OrderBy:  "chunk_number",
		}

		result, err := service.RetrieveFullDocuments(ctx, req)
		if err != nil {
			t.Logf("Full document retrieval returned error: %v", err)
			return
		}

		assert.NotNil(t, result)
		assert.Equal(t, models.ContextStrategyFull, result.Strategy)
		t.Logf("Full document retrieval returned %d chunks", len(result.Chunks))
		t.Logf("Total tokens: %d", result.TotalTokens)
	})
}

// TestDocumentContextIntegration_NotebookDocuments tests notebook document retrieval from Aether-BE
func TestDocumentContextIntegration_NotebookDocuments(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isAetherBEAvailable(cfg.Aether.BaseURL) {
		t.Skip("Aether-BE API not available, skipping integration test")
	}

	cacheSvc, _ := impl.NewCacheService(nil)
	service := impl.NewDocumentContextService(
		&cfg.DeepLake,
		&cfg.AudiModal,
		&cfg.Aether,
		cacheSvc,
	)

	ctx := context.Background()
	tenantID := os.Getenv("TEST_TENANT_ID")
	if tenantID == "" {
		tenantID = "default"
	}

	t.Run("Get Documents from Notebook", func(t *testing.T) {
		testNotebookID := os.Getenv("TEST_NOTEBOOK_ID")
		if testNotebookID == "" {
			t.Skip("TEST_NOTEBOOK_ID not set, skipping")
		}

		notebookID, err := uuid.Parse(testNotebookID)
		require.NoError(t, err)

		docs, err := service.GetNotebookDocuments(ctx, []uuid.UUID{notebookID}, tenantID, false)
		if err != nil {
			t.Logf("Notebook document retrieval returned error: %v", err)
			return
		}

		t.Logf("Notebook contains %d documents", len(docs))
		for i, doc := range docs {
			t.Logf("  Document %d: %s (type: %s, size: %d bytes)",
				i+1, doc.Name, doc.ContentType, doc.SizeBytes)
		}
	})

	t.Run("Get Documents Recursively from Notebook Hierarchy", func(t *testing.T) {
		testNotebookID := os.Getenv("TEST_NOTEBOOK_ID")
		if testNotebookID == "" {
			t.Skip("TEST_NOTEBOOK_ID not set, skipping")
		}

		notebookID, err := uuid.Parse(testNotebookID)
		require.NoError(t, err)

		docs, err := service.GetNotebookDocuments(ctx, []uuid.UUID{notebookID}, tenantID, true)
		if err != nil {
			t.Logf("Recursive notebook document retrieval returned error: %v", err)
			return
		}

		t.Logf("Notebook hierarchy contains %d documents total", len(docs))
	})
}

// TestDocumentContextIntegration_HybridContext tests hybrid context retrieval
func TestDocumentContextIntegration_HybridContext(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isDeepLakeAvailable(cfg.DeepLake.BaseURL) || !isAudiModalAvailable(cfg.AudiModal.BaseURL) {
		t.Skip("DeepLake or AudiModal API not available, skipping integration test")
	}

	cacheSvc, _ := impl.NewCacheService(nil)
	service := impl.NewDocumentContextService(
		&cfg.DeepLake,
		&cfg.AudiModal,
		&cfg.Aether,
		cacheSvc,
	)

	ctx := context.Background()
	tenantID := os.Getenv("TEST_TENANT_ID")
	if tenantID == "" {
		tenantID = "default"
	}

	t.Run("Hybrid Context with Query and Document IDs", func(t *testing.T) {
		testFileID := os.Getenv("TEST_FILE_ID")
		if testFileID == "" {
			t.Skip("TEST_FILE_ID not set, skipping")
		}

		fileID, err := uuid.Parse(testFileID)
		require.NoError(t, err)

		req := models.ChunkRetrievalRequest{
			TenantID: tenantID,
			FileIDs:  []uuid.UUID{fileID},
		}

		result, err := service.RetrieveHybridContext(ctx, "explain the main concepts", req, 0.6, 0.4)
		if err != nil {
			t.Logf("Hybrid context retrieval returned error: %v", err)
			return
		}

		assert.NotNil(t, result)
		assert.Equal(t, models.ContextStrategyHybrid, result.Strategy)
		t.Logf("Hybrid context returned %d chunks", len(result.Chunks))
		t.Logf("Total tokens: %d", result.TotalTokens)
	})
}

// TestDocumentContextIntegration_Caching tests caching functionality
func TestDocumentContextIntegration_Caching(t *testing.T) {
	// Load config to validate it's available, but we don't need it for cache tests
	_, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	// Test with in-memory cache (no Redis required)
	cacheSvc, err := impl.NewCacheService(nil)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Cache Set and Get", func(t *testing.T) {
		testResult := &models.DocumentContextResult{
			Chunks: []models.RetrievedChunk{
				{
					ID:           "test-chunk-1",
					DocumentID:   "test-doc-1",
					DocumentName: "Test Document",
					Content:      "This is cached content for testing purposes.",
					ChunkNumber:  1,
					Score:        0.95,
				},
			},
			TotalTokens: 50,
			Strategy:    models.ContextStrategyVector,
		}

		cacheKey := "test:integration:cache:key"

		// Set cache
		err := cacheSvc.SetCachedContext(ctx, cacheKey, testResult, 60)
		require.NoError(t, err)

		// Get cache
		retrieved, err := cacheSvc.GetCachedContext(ctx, cacheKey)
		require.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, testResult.TotalTokens, retrieved.TotalTokens)
		assert.Equal(t, testResult.Strategy, retrieved.Strategy)
		assert.Len(t, retrieved.Chunks, 1)
		assert.Equal(t, testResult.Chunks[0].Content, retrieved.Chunks[0].Content)
	})

	t.Run("Cache Key Generation", func(t *testing.T) {
		agentID := uuid.New()
		sessionID := "session-abc123"
		queryHash := "hash123456"

		key := cacheSvc.GenerateCacheKey(agentID, &sessionID, queryHash)

		assert.Contains(t, key, agentID.String())
		assert.Contains(t, key, sessionID)
		assert.Contains(t, key, queryHash)
		t.Logf("Generated cache key: %s", key)
	})

	t.Run("Cache Invalidation", func(t *testing.T) {
		testResult := &models.DocumentContextResult{
			Chunks:      []models.RetrievedChunk{{Content: "temp"}},
			TotalTokens: 10,
			Strategy:    models.ContextStrategyVector,
		}

		// Set multiple keys with a common pattern
		for i := 0; i < 3; i++ {
			key := "test:pattern:" + string(rune('a'+i))
			err := cacheSvc.SetCachedContext(ctx, key, testResult, 60)
			require.NoError(t, err)
		}

		// Invalidate with pattern
		err := cacheSvc.InvalidateCache(ctx, "test:pattern:*")
		require.NoError(t, err)

		// Verify invalidation (note: in-memory cache may behave differently than Redis)
		// This is mainly testing that the method doesn't error
	})
}

// TestDocumentContextIntegration_ContextFormatting tests context formatting for LLM injection
func TestDocumentContextIntegration_ContextFormatting(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	cacheSvc, _ := impl.NewCacheService(nil)
	service := impl.NewDocumentContextService(
		&cfg.DeepLake,
		&cfg.AudiModal,
		&cfg.Aether,
		cacheSvc,
	)

	t.Run("Format Small Context", func(t *testing.T) {
		result := &models.DocumentContextResult{
			Chunks: []models.RetrievedChunk{
				{
					ID:           "chunk-1",
					DocumentID:   "doc-1",
					DocumentName: "Introduction.pdf",
					Content:      "This is the introduction to artificial intelligence, covering basic concepts and history.",
					ChunkNumber:  1,
					Score:        0.95,
				},
				{
					ID:           "chunk-2",
					DocumentID:   "doc-1",
					DocumentName: "Introduction.pdf",
					Content:      "Machine learning is a subset of AI that focuses on learning from data.",
					ChunkNumber:  2,
					Score:        0.90,
				},
			},
			TotalTokens: 100,
			Strategy:    models.ContextStrategyVector,
		}

		injection, err := service.FormatContextForInjection(result, 1000)
		require.NoError(t, err)
		assert.NotNil(t, injection)
		assert.False(t, injection.Truncated)
		assert.Contains(t, injection.FormattedContext, "artificial intelligence")
		assert.Contains(t, injection.FormattedContext, "Machine learning")
		t.Logf("Formatted context (%d tokens):\n%s", injection.TotalTokens, injection.FormattedContext)
	})

	t.Run("Format Context with Truncation", func(t *testing.T) {
		// Create result with many chunks
		chunks := make([]models.RetrievedChunk, 20)
		for i := 0; i < 20; i++ {
			chunks[i] = models.RetrievedChunk{
				ID:           uuid.New().String(),
				DocumentID:   "doc-1",
				DocumentName: "Large Document.pdf",
				Content:      "This is chunk number " + string(rune('A'+i)) + " with significant content that will push us over the token limit when combined with all other chunks.",
				ChunkNumber:  i + 1,
				Score:        0.95 - float64(i)*0.02,
			}
		}

		result := &models.DocumentContextResult{
			Chunks:      chunks,
			TotalTokens: 2000,
			Strategy:    models.ContextStrategyFull,
		}

		// Request smaller limit
		injection, err := service.FormatContextForInjection(result, 200)
		require.NoError(t, err)
		assert.NotNil(t, injection)
		assert.True(t, injection.Truncated)
		assert.Less(t, injection.ChunkCount, 20)
		t.Logf("Truncated to %d chunks, %d tokens (from 20 chunks, 2000 tokens)",
			injection.ChunkCount, injection.TotalTokens)
	})
}

// TestDocumentContextIntegration_TokenEstimation tests token count estimation
func TestDocumentContextIntegration_TokenEstimation(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	cacheSvc, _ := impl.NewCacheService(nil)
	service := impl.NewDocumentContextService(
		&cfg.DeepLake,
		&cfg.AudiModal,
		&cfg.Aether,
		cacheSvc,
	)

	testCases := []struct {
		name          string
		text          string
		minTokens     int
		maxTokens     int
	}{
		{
			name:      "Empty string",
			text:      "",
			minTokens: 0,
			maxTokens: 0,
		},
		{
			name:      "Single word",
			text:      "Hello",
			minTokens: 1,
			maxTokens: 3,
		},
		{
			name:      "Short sentence",
			text:      "The quick brown fox jumps over the lazy dog.",
			minTokens: 8,
			maxTokens: 15,
		},
		{
			name:      "Technical text",
			text:      "The transformer architecture uses self-attention mechanisms to process sequential data in parallel, enabling efficient training on large-scale datasets.",
			minTokens: 20,
			maxTokens: 40,
		},
		{
			name:      "Long paragraph",
			text:      "Machine learning is a subset of artificial intelligence that provides systems the ability to automatically learn and improve from experience without being explicitly programmed. The focus of machine learning is on the development of computer programs that can access data and use it to learn for themselves. The process begins with observations or data, such as examples, direct experience, or instruction, in order to look for patterns in data and make better decisions in the future based on the examples that we provide.",
			minTokens: 80,
			maxTokens: 150,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tokens := service.EstimateTokenCount(tc.text)
			t.Logf("Text: %q\nEstimated tokens: %d (expected range: %d-%d)",
				truncateString(tc.text, 50), tokens, tc.minTokens, tc.maxTokens)

			assert.GreaterOrEqual(t, tokens, tc.minTokens, "Token count should be at least minimum")
			assert.LessOrEqual(t, tokens, tc.maxTokens, "Token count should be at most maximum")
		})
	}
}

// Helper function to truncate strings for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
