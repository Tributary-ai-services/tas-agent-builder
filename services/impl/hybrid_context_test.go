package impl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tas-agent-builder/models"
)

// mockTokenEstimator provides a simple token estimation for testing
func mockTokenEstimator(text string) int {
	return len(text) / 4
}

func TestNewHybridContextBuilder(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		builder := NewHybridContextBuilder(nil)
		assert.NotNil(t, builder)
		assert.NotNil(t, builder.config)
		assert.Equal(t, 0.6, builder.config.VectorWeight)
		assert.Equal(t, 0.3, builder.config.FullDocWeight)
		assert.Equal(t, 0.1, builder.config.PositionWeight)
	})

	t.Run("with custom config uses provided values", func(t *testing.T) {
		config := &models.HybridContextConfig{
			VectorWeight:  0.8,
			FullDocWeight: 0.2,
			TokenBudget:   10000,
		}
		builder := NewHybridContextBuilder(config)
		assert.Equal(t, 0.8, builder.config.VectorWeight)
		assert.Equal(t, 0.2, builder.config.FullDocWeight)
		assert.Equal(t, 10000, builder.config.TokenBudget)
	})
}

func TestHybridContextBuilder_BuildHybridContext(t *testing.T) {
	config := models.DefaultHybridContextConfig()
	builder := NewHybridContextBuilder(config)

	vectorChunks := []models.RetrievedChunk{
		{
			ID:           "v1",
			DocumentID:   "doc1",
			DocumentName: "Document 1",
			Content:      "Vector chunk 1 content for testing",
			ChunkNumber:  1,
			TotalChunks:  10,
			Score:        0.95,
		},
		{
			ID:           "v2",
			DocumentID:   "doc1",
			DocumentName: "Document 1",
			Content:      "Vector chunk 2 content for testing",
			ChunkNumber:  2,
			TotalChunks:  10,
			Score:        0.85,
		},
	}

	fullDocChunks := []models.RetrievedChunk{
		{
			ID:           "f1",
			DocumentID:   "doc1",
			DocumentName: "Document 1",
			Content:      "Full doc chunk 1 content",
			ChunkNumber:  1,
			TotalChunks:  10,
		},
		{
			ID:           "f3",
			DocumentID:   "doc1",
			DocumentName: "Document 1",
			Content:      "Full doc chunk 3 content",
			ChunkNumber:  3,
			TotalChunks:  10,
		},
	}

	result, err := builder.BuildHybridContext(context.Background(), vectorChunks, fullDocChunks, mockTokenEstimator)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.DocumentContextResult)
	assert.Equal(t, models.ContextStrategyHybrid, result.Strategy)
	assert.Greater(t, len(result.Chunks), 0)
	assert.Greater(t, result.VectorChunkCount, 0)
}

func TestHybridContextBuilder_ScoreAllChunks(t *testing.T) {
	config := models.DefaultHybridContextConfig()
	builder := NewHybridContextBuilder(config)

	vectorChunks := []models.RetrievedChunk{
		{
			ID:          "v1",
			DocumentID:  "doc1",
			Content:     "Vector content",
			ChunkNumber: 1,
			TotalChunks: 10,
			Score:       0.9,
		},
	}

	fullDocChunks := []models.RetrievedChunk{
		{
			ID:          "f1",
			DocumentID:  "doc1",
			Content:     "Full doc content",
			ChunkNumber: 1,
			TotalChunks: 10,
		},
	}

	scored := builder.scoreAllChunks(vectorChunks, fullDocChunks, mockTokenEstimator)

	assert.Greater(t, len(scored), 0)

	// Verify scored chunks have proper sources
	for _, sc := range scored {
		if sc.Source == "both" {
			assert.Greater(t, sc.VectorScore, 0.0)
		}
	}
	// Since IDs differ but chunk keys are same (doc1_1), one should be merged
	assert.True(t, len(scored) >= 1)
}

func TestHybridContextBuilder_DeduplicateByContent(t *testing.T) {
	config := models.DefaultHybridContextConfig()
	config.DeduplicateByContent = true
	builder := NewHybridContextBuilder(config)

	chunks := []models.ScoredChunk{
		{
			Chunk:         models.RetrievedChunk{ID: "1", Content: "duplicate content"},
			CombinedScore: 0.9,
		},
		{
			Chunk:         models.RetrievedChunk{ID: "2", Content: "duplicate content"},
			CombinedScore: 0.8,
		},
		{
			Chunk:         models.RetrievedChunk{ID: "3", Content: "unique content"},
			CombinedScore: 0.7,
		},
	}

	result, duplicatesRemoved := builder.deduplicateByContent(chunks)

	assert.Equal(t, 1, duplicatesRemoved)
	assert.Len(t, result, 2)

	// The higher scoring duplicate should be kept
	for _, sc := range result {
		if sc.Chunk.Content == "duplicate content" {
			assert.Equal(t, 0.9, sc.CombinedScore)
		}
	}
}

func TestHybridContextBuilder_AssignPriorityTiers(t *testing.T) {
	config := models.DefaultHybridContextConfig()
	builder := NewHybridContextBuilder(config)

	chunks := []models.ScoredChunk{
		{Chunk: models.RetrievedChunk{ID: "1"}, CombinedScore: 0.9},
		{Chunk: models.RetrievedChunk{ID: "2"}, CombinedScore: 0.7},
		{Chunk: models.RetrievedChunk{ID: "3"}, CombinedScore: 0.4},
	}

	result := builder.assignPriorityTiers(chunks)

	assert.Len(t, result, 3)
	assert.Equal(t, "high_relevance", result[0].PriorityTier)
	assert.Equal(t, "medium_relevance", result[1].PriorityTier)
	assert.Equal(t, "context", result[2].PriorityTier)
}

func TestHybridContextBuilder_FitToTokenBudget(t *testing.T) {
	config := &models.HybridContextConfig{
		TokenBudget: 100,
		PriorityTiers: []models.HybridPriorityTier{
			{Name: "high", MinScore: 0.8, Percentage: 0.6},
			{Name: "medium", MinScore: 0.5, Percentage: 0.3},
			{Name: "low", MinScore: 0.0, Percentage: 0.1},
		},
	}
	builder := NewHybridContextBuilder(config)

	chunks := []models.ScoredChunk{
		{Chunk: models.RetrievedChunk{ID: "1"}, CombinedScore: 0.9, PriorityTier: "high", EstimatedTokens: 30},
		{Chunk: models.RetrievedChunk{ID: "2"}, CombinedScore: 0.85, PriorityTier: "high", EstimatedTokens: 30},
		{Chunk: models.RetrievedChunk{ID: "3"}, CombinedScore: 0.6, PriorityTier: "medium", EstimatedTokens: 25},
		{Chunk: models.RetrievedChunk{ID: "4"}, CombinedScore: 0.3, PriorityTier: "low", EstimatedTokens: 50},
	}

	result, tierBreakdown := builder.fitToTokenBudget(chunks)

	assert.NotNil(t, tierBreakdown)
	totalTokens := 0
	for _, sc := range result {
		totalTokens += sc.EstimatedTokens
	}
	assert.LessOrEqual(t, totalTokens, config.TokenBudget)
}

func TestHybridContextBuilder_PositionScore(t *testing.T) {
	config := models.DefaultHybridContextConfig()
	builder := NewHybridContextBuilder(config)

	// First chunk should have highest position score
	score1 := builder.calculatePositionScore(0, 10)
	score5 := builder.calculatePositionScore(5, 10)
	score9 := builder.calculatePositionScore(9, 10)

	assert.Greater(t, score1, score5)
	assert.Greater(t, score5, score9)
}

func TestHybridContextBuilder_SummaryBoost(t *testing.T) {
	config := models.DefaultHybridContextConfig()
	config.IncludeSummaries = true
	config.SummaryBoost = 1.5
	builder := NewHybridContextBuilder(config)

	regularChunk := &models.RetrievedChunk{
		ID:      "1",
		Content: "Regular content",
	}

	summaryChunk := &models.RetrievedChunk{
		ID:       "2",
		Content:  "Summary content",
		Metadata: map[string]interface{}{"is_summary": true},
	}

	regularBoost := builder.calculateSummaryBoost(regularChunk)
	summaryBoostValue := builder.calculateSummaryBoost(summaryChunk)

	assert.Equal(t, 1.0, regularBoost)
	assert.Equal(t, 1.5, summaryBoostValue)
}

func TestHybridContextBuilder_CombinedScore(t *testing.T) {
	config := &models.HybridContextConfig{
		VectorWeight:   0.6,
		FullDocWeight:  0.3,
		PositionWeight: 0.1,
	}
	builder := NewHybridContextBuilder(config)

	chunk := &models.ScoredChunk{
		VectorScore:   1.0,
		FullDocScore:  0.5,
		PositionScore: 0.8,
		SummaryBoost:  1.0,
	}

	score := builder.calculateCombinedScore(chunk)

	// (1.0 * 0.6) + (0.5 * 0.3) + (0.8 * 0.1) = 0.6 + 0.15 + 0.08 = 0.83
	expectedScore := (1.0 * 0.6) + (0.5 * 0.3) + (0.8 * 0.1)
	assert.InDelta(t, expectedScore, score, 0.001)
}

func TestHybridContextBuilder_WithMethods(t *testing.T) {
	original := NewHybridContextBuilder(models.DefaultHybridContextConfig())

	t.Run("WithVectorWeight", func(t *testing.T) {
		modified := original.WithVectorWeight(0.9)
		assert.Equal(t, 0.9, modified.config.VectorWeight)
		assert.Equal(t, 0.6, original.config.VectorWeight) // Original unchanged
	})

	t.Run("WithFullDocWeight", func(t *testing.T) {
		modified := original.WithFullDocWeight(0.5)
		assert.Equal(t, 0.5, modified.config.FullDocWeight)
		assert.Equal(t, 0.3, original.config.FullDocWeight) // Original unchanged
	})

	t.Run("WithTokenBudget", func(t *testing.T) {
		modified := original.WithTokenBudget(16000)
		assert.Equal(t, 16000, modified.config.TokenBudget)
		assert.Equal(t, 8000, original.config.TokenBudget) // Original unchanged
	})
}

func TestHybridContextBuilder_EmptyInput(t *testing.T) {
	config := models.DefaultHybridContextConfig()
	builder := NewHybridContextBuilder(config)

	t.Run("empty vector chunks", func(t *testing.T) {
		result, err := builder.BuildHybridContext(
			context.Background(),
			[]models.RetrievedChunk{},
			[]models.RetrievedChunk{{ID: "1", Content: "test"}},
			mockTokenEstimator,
		)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("empty full doc chunks", func(t *testing.T) {
		result, err := builder.BuildHybridContext(
			context.Background(),
			[]models.RetrievedChunk{{ID: "1", Content: "test", Score: 0.9}},
			[]models.RetrievedChunk{},
			mockTokenEstimator,
		)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("both empty", func(t *testing.T) {
		result, err := builder.BuildHybridContext(
			context.Background(),
			[]models.RetrievedChunk{},
			[]models.RetrievedChunk{},
			mockTokenEstimator,
		)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Chunks)
	})
}

func TestHybridContextBuilder_LargeTokenBudget(t *testing.T) {
	config := &models.HybridContextConfig{
		VectorWeight:         0.6,
		FullDocWeight:        0.3,
		PositionWeight:       0.1,
		TokenBudget:          100000, // Very large budget
		DeduplicateByContent: true,
		PriorityTiers: []models.HybridPriorityTier{
			{Name: "high", MinScore: 0.8, Percentage: 0.5},
			{Name: "medium", MinScore: 0.5, Percentage: 0.3},
			{Name: "low", MinScore: 0.0, Percentage: 0.2},
		},
	}
	builder := NewHybridContextBuilder(config)

	// Create many chunks
	vectorChunks := make([]models.RetrievedChunk, 50)
	for i := 0; i < 50; i++ {
		vectorChunks[i] = models.RetrievedChunk{
			ID:          string(rune('a' + i)),
			DocumentID:  "doc1",
			Content:     "Vector chunk content " + string(rune('0'+i)),
			ChunkNumber: i,
			TotalChunks: 100,
			Score:       float64(50-i) / 50.0,
		}
	}

	result, err := builder.BuildHybridContext(context.Background(), vectorChunks, nil, mockTokenEstimator)

	require.NoError(t, err)
	assert.NotNil(t, result)
	// All chunks should be included since budget is very large
	assert.Equal(t, 50, len(result.Chunks))
}

func TestHybridContextBuilder_ZeroTokenBudget(t *testing.T) {
	config := models.DefaultHybridContextConfig()
	config.TokenBudget = 0 // No budget limit
	builder := NewHybridContextBuilder(config)

	chunks := []models.RetrievedChunk{
		{ID: "1", Content: "Test content 1", Score: 0.9},
		{ID: "2", Content: "Test content 2", Score: 0.8},
	}

	result, err := builder.BuildHybridContext(context.Background(), chunks, nil, mockTokenEstimator)

	require.NoError(t, err)
	// All chunks should be included when budget is 0 (unlimited)
	assert.Equal(t, 2, len(result.Chunks))
}

func TestHybridContextBuilder_ContentHashDedup(t *testing.T) {
	config := models.DefaultHybridContextConfig()
	builder := NewHybridContextBuilder(config)

	content1 := "This is some content"
	content2 := "This is some content" // Same content
	content3 := "This is different content"

	hash1 := builder.hashContent(content1)
	hash2 := builder.hashContent(content2)
	hash3 := builder.hashContent(content3)

	assert.Equal(t, hash1, hash2)
	assert.NotEqual(t, hash1, hash3)
}

func TestHybridContextBuilder_FullDocScore(t *testing.T) {
	config := models.DefaultHybridContextConfig()
	builder := NewHybridContextBuilder(config)

	// First chunk should have higher score
	chunk1 := &models.RetrievedChunk{ChunkNumber: 0}
	chunk5 := &models.RetrievedChunk{ChunkNumber: 5}

	score1 := builder.calculateFullDocScore(chunk1)
	score5 := builder.calculateFullDocScore(chunk5)

	assert.Greater(t, score1, score5)

	// Header chunks should have boost
	headerChunk := &models.RetrievedChunk{
		ChunkNumber: 5,
		Metadata:    map[string]interface{}{"is_header": true},
	}
	headerScore := builder.calculateFullDocScore(headerChunk)
	assert.Greater(t, headerScore, score5)
}

func TestHybridContextBuilder_TierBreakdown(t *testing.T) {
	config := &models.HybridContextConfig{
		TokenBudget: 200,
		PriorityTiers: []models.HybridPriorityTier{
			{Name: "high", MinScore: 0.8, Percentage: 0.5},
			{Name: "medium", MinScore: 0.5, Percentage: 0.3},
			{Name: "low", MinScore: 0.0, Percentage: 0.2},
		},
	}
	builder := NewHybridContextBuilder(config)

	chunks := []models.ScoredChunk{
		{Chunk: models.RetrievedChunk{ID: "1"}, CombinedScore: 0.9, PriorityTier: "high", EstimatedTokens: 40},
		{Chunk: models.RetrievedChunk{ID: "2"}, CombinedScore: 0.85, PriorityTier: "high", EstimatedTokens: 40},
		{Chunk: models.RetrievedChunk{ID: "3"}, CombinedScore: 0.6, PriorityTier: "medium", EstimatedTokens: 30},
		{Chunk: models.RetrievedChunk{ID: "4"}, CombinedScore: 0.55, PriorityTier: "medium", EstimatedTokens: 30},
		{Chunk: models.RetrievedChunk{ID: "5"}, CombinedScore: 0.3, PriorityTier: "low", EstimatedTokens: 20},
	}

	_, tierBreakdown := builder.fitToTokenBudget(chunks)

	assert.NotNil(t, tierBreakdown)
	// Verify that tier breakdown contains the expected tiers
	_, hasHigh := tierBreakdown["high"]
	_, hasMedium := tierBreakdown["medium"]
	_, hasLow := tierBreakdown["low"]

	assert.True(t, hasHigh || hasMedium || hasLow, "Should have at least one tier in breakdown")
}
