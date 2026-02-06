package impl

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"github.com/tas-agent-builder/models"
)

// HybridContextBuilder builds hybrid context by combining vector search and full document retrieval
type HybridContextBuilder struct {
	config *models.HybridContextConfig
}

// NewHybridContextBuilder creates a new HybridContextBuilder with the given configuration
func NewHybridContextBuilder(config *models.HybridContextConfig) *HybridContextBuilder {
	if config == nil {
		config = models.DefaultHybridContextConfig()
	}
	return &HybridContextBuilder{
		config: config,
	}
}

// BuildHybridContext combines vector search results and full document chunks into an optimized context
func (b *HybridContextBuilder) BuildHybridContext(
	ctx context.Context,
	vectorChunks []models.RetrievedChunk,
	fullDocChunks []models.RetrievedChunk,
	tokenEstimator func(string) int,
) (*models.HybridContextResult, error) {
	startTime := time.Now()

	// Step 1: Score all chunks
	scoredChunks := b.scoreAllChunks(vectorChunks, fullDocChunks, tokenEstimator)

	// Step 2: Deduplicate by content hash if configured
	var duplicatesRemoved int
	if b.config.DeduplicateByContent {
		scoredChunks, duplicatesRemoved = b.deduplicateByContent(scoredChunks)
	}

	// Step 3: Assign priority tiers
	scoredChunks = b.assignPriorityTiers(scoredChunks)

	// Step 4: Fit to token budget using priority ranking
	selectedChunks, tierBreakdown := b.fitToTokenBudget(scoredChunks)

	// Step 5: Build result
	retrievedChunks := make([]models.RetrievedChunk, len(selectedChunks))
	totalTokens := 0
	vectorCount := 0
	fullDocCount := 0

	for i, sc := range selectedChunks {
		retrievedChunks[i] = sc.Chunk
		retrievedChunks[i].Score = sc.CombinedScore
		totalTokens += sc.EstimatedTokens

		switch sc.Source {
		case "vector":
			vectorCount++
		case "full_doc":
			fullDocCount++
		case "both":
			vectorCount++
			fullDocCount++
		}
	}

	retrievalTime := int(time.Since(startTime).Milliseconds())

	return &models.HybridContextResult{
		DocumentContextResult: &models.DocumentContextResult{
			Chunks:          retrievedChunks,
			TotalTokens:     totalTokens,
			Strategy:        models.ContextStrategyHybrid,
			RetrievalTimeMs: retrievalTime,
			Metadata: map[string]any{
				"vector_weight":      b.config.VectorWeight,
				"full_doc_weight":    b.config.FullDocWeight,
				"position_weight":    b.config.PositionWeight,
				"summary_boost":      b.config.SummaryBoost,
				"token_budget":       b.config.TokenBudget,
				"original_vector":    len(vectorChunks),
				"original_full_doc":  len(fullDocChunks),
			},
		},
		ScoredChunks:      selectedChunks,
		VectorChunkCount:  vectorCount,
		FullDocChunkCount: fullDocCount,
		DuplicatesRemoved: duplicatesRemoved,
		TierBreakdown:     tierBreakdown,
		Config:            b.config,
	}, nil
}

// scoreAllChunks creates ScoredChunk entries for all input chunks with computed scores
func (b *HybridContextBuilder) scoreAllChunks(
	vectorChunks []models.RetrievedChunk,
	fullDocChunks []models.RetrievedChunk,
	tokenEstimator func(string) int,
) []models.ScoredChunk {
	// Map to track chunks by their unique key (document_id + chunk_number)
	chunkMap := make(map[string]*models.ScoredChunk)

	// Process vector search chunks
	for i := range vectorChunks {
		chunk := vectorChunks[i]
		key := b.chunkKey(&chunk)

		scoredChunk := &models.ScoredChunk{
			Chunk:           chunk,
			VectorScore:     chunk.Score,
			PositionScore:   b.calculatePositionScore(chunk.ChunkNumber, chunk.TotalChunks),
			FullDocScore:    0,
			SummaryBoost:    b.calculateSummaryBoost(&chunk),
			Source:          "vector",
			EstimatedTokens: tokenEstimator(chunk.Content),
		}
		scoredChunk.CombinedScore = b.calculateCombinedScore(scoredChunk)
		chunkMap[key] = scoredChunk
	}

	// Process full document chunks
	for i := range fullDocChunks {
		chunk := fullDocChunks[i]
		key := b.chunkKey(&chunk)

		if existing, exists := chunkMap[key]; exists {
			// Chunk exists from vector search, update scores
			existing.FullDocScore = b.calculateFullDocScore(&chunk)
			existing.Source = "both"
			existing.CombinedScore = b.calculateCombinedScore(existing)
		} else {
			// New chunk from full doc retrieval
			scoredChunk := &models.ScoredChunk{
				Chunk:           chunk,
				VectorScore:     0,
				PositionScore:   b.calculatePositionScore(chunk.ChunkNumber, chunk.TotalChunks),
				FullDocScore:    b.calculateFullDocScore(&chunk),
				SummaryBoost:    b.calculateSummaryBoost(&chunk),
				Source:          "full_doc",
				EstimatedTokens: tokenEstimator(chunk.Content),
			}
			scoredChunk.CombinedScore = b.calculateCombinedScore(scoredChunk)
			chunkMap[key] = scoredChunk
		}
	}

	// Convert map to slice
	result := make([]models.ScoredChunk, 0, len(chunkMap))
	for _, sc := range chunkMap {
		result = append(result, *sc)
	}

	// Sort by combined score descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].CombinedScore > result[j].CombinedScore
	})

	return result
}

// chunkKey generates a unique key for a chunk based on document ID and chunk number
func (b *HybridContextBuilder) chunkKey(chunk *models.RetrievedChunk) string {
	if chunk.ID != "" {
		return chunk.ID
	}
	return chunk.DocumentID + "_" + string(rune(chunk.ChunkNumber))
}

// calculatePositionScore returns a score based on chunk position (earlier chunks score higher)
func (b *HybridContextBuilder) calculatePositionScore(chunkNumber, totalChunks int) float64 {
	if totalChunks <= 0 {
		totalChunks = 100 // Default assumption
	}
	// Earlier chunks get higher scores (1.0 for first chunk, 0.0 for last)
	return 1.0 - (float64(chunkNumber) / float64(totalChunks))
}

// calculateFullDocScore returns a score for full document chunks
func (b *HybridContextBuilder) calculateFullDocScore(chunk *models.RetrievedChunk) float64 {
	// Base score for full doc chunks
	baseScore := 0.5

	// Boost for first chunks of document (likely introduction/summary)
	if chunk.ChunkNumber <= 2 {
		baseScore += 0.3
	}

	// Boost for chunks with metadata indicating importance
	if chunk.Metadata != nil {
		if isHeader, ok := chunk.Metadata["is_header"].(bool); ok && isHeader {
			baseScore += 0.2
		}
		if isSummary, ok := chunk.Metadata["is_summary"].(bool); ok && isSummary {
			baseScore += 0.3
		}
	}

	return baseScore
}

// calculateSummaryBoost returns a boost multiplier for summary chunks
func (b *HybridContextBuilder) calculateSummaryBoost(chunk *models.RetrievedChunk) float64 {
	if !b.config.IncludeSummaries {
		return 1.0
	}

	if chunk.Metadata != nil {
		if isSummary, ok := chunk.Metadata["is_summary"].(bool); ok && isSummary {
			return b.config.SummaryBoost
		}
		if chunkType, ok := chunk.Metadata["chunk_type"].(string); ok {
			if chunkType == "summary" || chunkType == "abstract" || chunkType == "introduction" {
				return b.config.SummaryBoost
			}
		}
	}

	return 1.0
}

// calculateCombinedScore computes the final combined score for a chunk
func (b *HybridContextBuilder) calculateCombinedScore(sc *models.ScoredChunk) float64 {
	// Weighted combination of scores
	combinedScore := (sc.VectorScore * b.config.VectorWeight) +
		(sc.FullDocScore * b.config.FullDocWeight) +
		(sc.PositionScore * b.config.PositionWeight)

	// Apply summary boost
	combinedScore *= sc.SummaryBoost

	return combinedScore
}

// deduplicateByContent removes duplicate chunks based on content hash
func (b *HybridContextBuilder) deduplicateByContent(chunks []models.ScoredChunk) ([]models.ScoredChunk, int) {
	seen := make(map[string]int) // content hash -> index of best scoring chunk
	result := make([]models.ScoredChunk, 0, len(chunks))
	duplicatesRemoved := 0

	for i := range chunks {
		chunk := chunks[i]
		contentHash := b.hashContent(chunk.Chunk.Content)

		if existingIdx, exists := seen[contentHash]; exists {
			// Keep the higher scoring version
			if chunk.CombinedScore > result[existingIdx].CombinedScore {
				result[existingIdx] = chunk
			}
			duplicatesRemoved++
		} else {
			seen[contentHash] = len(result)
			result = append(result, chunk)
		}
	}

	return result, duplicatesRemoved
}

// hashContent generates a hash of the chunk content for deduplication
func (b *HybridContextBuilder) hashContent(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// assignPriorityTiers assigns each chunk to a priority tier based on its score
func (b *HybridContextBuilder) assignPriorityTiers(chunks []models.ScoredChunk) []models.ScoredChunk {
	if len(b.config.PriorityTiers) == 0 {
		// No tiers configured, assign all to "default"
		for i := range chunks {
			chunks[i].PriorityTier = "default"
		}
		return chunks
	}

	// Sort tiers by MinScore descending so we match highest tier first
	sortedTiers := make([]models.HybridPriorityTier, len(b.config.PriorityTiers))
	copy(sortedTiers, b.config.PriorityTiers)
	sort.Slice(sortedTiers, func(i, j int) bool {
		return sortedTiers[i].MinScore > sortedTiers[j].MinScore
	})

	for i := range chunks {
		assigned := false
		for _, tier := range sortedTiers {
			if chunks[i].CombinedScore >= tier.MinScore {
				chunks[i].PriorityTier = tier.Name
				assigned = true
				break
			}
		}
		if !assigned {
			// Assign to lowest tier
			chunks[i].PriorityTier = sortedTiers[len(sortedTiers)-1].Name
		}
	}

	return chunks
}

// fitToTokenBudget selects chunks to fit within token budget using priority tiers
func (b *HybridContextBuilder) fitToTokenBudget(chunks []models.ScoredChunk) ([]models.ScoredChunk, map[string]int) {
	if b.config.TokenBudget <= 0 {
		return chunks, nil
	}

	tierBreakdown := make(map[string]int)
	result := make([]models.ScoredChunk, 0)
	usedTokens := 0

	// Calculate token budgets per tier
	tierBudgets := make(map[string]int)
	for _, tier := range b.config.PriorityTiers {
		tierBudgets[tier.Name] = int(float64(b.config.TokenBudget) * tier.Percentage)
		if tier.MaxTokens > 0 && tierBudgets[tier.Name] > tier.MaxTokens {
			tierBudgets[tier.Name] = tier.MaxTokens
		}
	}

	// Group chunks by tier
	tierChunks := make(map[string][]models.ScoredChunk)
	for _, chunk := range chunks {
		tierChunks[chunk.PriorityTier] = append(tierChunks[chunk.PriorityTier], chunk)
	}

	// Process tiers in order (high_relevance, medium_relevance, context)
	tierOrder := []string{}
	for _, tier := range b.config.PriorityTiers {
		tierOrder = append(tierOrder, tier.Name)
	}

	// First pass: allocate within tier budgets
	remainingBudget := b.config.TokenBudget
	tierUsed := make(map[string]int)

	for _, tierName := range tierOrder {
		budget := min(tierBudgets[tierName], remainingBudget)

		tierUsedTokens := 0
		for _, chunk := range tierChunks[tierName] {
			if tierUsedTokens+chunk.EstimatedTokens <= budget {
				result = append(result, chunk)
				tierUsedTokens += chunk.EstimatedTokens
				usedTokens += chunk.EstimatedTokens
			}
		}

		tierUsed[tierName] = tierUsedTokens
		tierBreakdown[tierName] = tierUsedTokens
		remainingBudget -= tierUsedTokens
	}

	// Second pass: fill remaining budget with any leftover chunks
	if remainingBudget > 0 {
		usedChunks := make(map[string]bool)
		for _, chunk := range result {
			usedChunks[b.chunkKey(&chunk.Chunk)] = true
		}

		// Try to add more chunks from any tier
		for _, chunk := range chunks {
			key := b.chunkKey(&chunk.Chunk)
			if !usedChunks[key] && chunk.EstimatedTokens <= remainingBudget {
				result = append(result, chunk)
				usedChunks[key] = true
				remainingBudget -= chunk.EstimatedTokens
				tierBreakdown[chunk.PriorityTier] += chunk.EstimatedTokens
			}
		}
	}

	// Re-sort result by combined score
	sort.Slice(result, func(i, j int) bool {
		return result[i].CombinedScore > result[j].CombinedScore
	})

	return result, tierBreakdown
}

// GetConfig returns the current configuration
func (b *HybridContextBuilder) GetConfig() *models.HybridContextConfig {
	return b.config
}

// SetConfig updates the configuration
func (b *HybridContextBuilder) SetConfig(config *models.HybridContextConfig) {
	if config != nil {
		b.config = config
	}
}

// WithVectorWeight returns a new builder with updated vector weight
func (b *HybridContextBuilder) WithVectorWeight(weight float64) *HybridContextBuilder {
	newConfig := *b.config
	newConfig.VectorWeight = weight
	return &HybridContextBuilder{config: &newConfig}
}

// WithFullDocWeight returns a new builder with updated full doc weight
func (b *HybridContextBuilder) WithFullDocWeight(weight float64) *HybridContextBuilder {
	newConfig := *b.config
	newConfig.FullDocWeight = weight
	return &HybridContextBuilder{config: &newConfig}
}

// WithTokenBudget returns a new builder with updated token budget
func (b *HybridContextBuilder) WithTokenBudget(budget int) *HybridContextBuilder {
	newConfig := *b.config
	newConfig.TokenBudget = budget
	return &HybridContextBuilder{config: &newConfig}
}
