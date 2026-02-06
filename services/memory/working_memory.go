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

// WorkingMemoryServiceImpl implements WorkingMemoryService using Redis
type WorkingMemoryServiceImpl struct {
	redis     *redis.Client
	config    *models.MemoryConfig
	keyPrefix string
}

// NewWorkingMemoryService creates a new working memory service
func NewWorkingMemoryService(redisClient *redis.Client, config *models.MemoryConfig) *WorkingMemoryServiceImpl {
	if config == nil {
		config = models.DefaultMemoryConfig()
	}
	return &WorkingMemoryServiceImpl{
		redis:     redisClient,
		config:    config,
		keyPrefix: "memory:working",
	}
}

// memoryKey generates the Redis key for a session's working memory
func (s *WorkingMemoryServiceImpl) memoryKey(sessionID string, agentID uuid.UUID) string {
	return fmt.Sprintf("%s:%s:%s", s.keyPrefix, agentID.String(), sessionID)
}

// GetWorkingMemory retrieves the working memory for a session
func (s *WorkingMemoryServiceImpl) GetWorkingMemory(ctx context.Context, sessionID string, agentID uuid.UUID) (*models.WorkingMemory, error) {
	key := s.memoryKey(sessionID, agentID)

	data, err := s.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			// Return empty memory if not found
			return &models.WorkingMemory{
				SessionID:       sessionID,
				AgentID:         agentID,
				LoadedDocuments: []models.LoadedDocument{},
				RetrievedChunks: []models.RetrievedChunk{},
				TotalTokens:     0,
				MaxTokens:       s.config.WorkingMemoryMaxTokens,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
				ExpiresAt:       time.Now().Add(s.config.WorkingMemoryTTL),
			}, nil
		}
		return nil, fmt.Errorf("failed to get working memory: %w", err)
	}

	var memory models.WorkingMemory
	if err := json.Unmarshal(data, &memory); err != nil {
		return nil, fmt.Errorf("failed to unmarshal working memory: %w", err)
	}

	return &memory, nil
}

// SetDocumentContext sets the document chunks in working memory
func (s *WorkingMemoryServiceImpl) SetDocumentContext(ctx context.Context, sessionID string, agentID uuid.UUID, chunks []models.RetrievedChunk) error {
	key := s.memoryKey(sessionID, agentID)

	// Get existing memory or create new
	memory, err := s.GetWorkingMemory(ctx, sessionID, agentID)
	if err != nil {
		return fmt.Errorf("failed to get existing working memory: %w", err)
	}

	// Calculate token count for chunks
	totalTokens := 0
	for _, chunk := range chunks {
		totalTokens += estimateTokenCount(chunk.Content)
	}

	// Trim chunks if exceeds token limit
	if totalTokens > s.config.WorkingMemoryMaxTokens {
		chunks, totalTokens = s.trimChunks(chunks, s.config.WorkingMemoryMaxTokens)
	}

	memory.RetrievedChunks = chunks
	memory.TotalTokens = totalTokens
	memory.UpdatedAt = time.Now()

	// Store
	data, err := json.Marshal(memory)
	if err != nil {
		return fmt.Errorf("failed to marshal working memory: %w", err)
	}

	return s.redis.Set(ctx, key, data, s.config.WorkingMemoryTTL).Err()
}

// LoadDocument loads a document into working memory
func (s *WorkingMemoryServiceImpl) LoadDocument(ctx context.Context, sessionID string, agentID uuid.UUID, doc models.LoadedDocument) error {
	memory, err := s.GetWorkingMemory(ctx, sessionID, agentID)
	if err != nil {
		return err
	}

	// Check if document already loaded
	for i, existing := range memory.LoadedDocuments {
		if existing.DocumentID == doc.DocumentID {
			// Update existing entry
			memory.LoadedDocuments[i] = doc
			return s.saveMemory(ctx, memory)
		}
	}

	// Check max documents limit
	if len(memory.LoadedDocuments) >= s.config.MaxLoadedDocuments {
		// Remove oldest document
		memory.LoadedDocuments = memory.LoadedDocuments[1:]
	}

	doc.LoadedAt = time.Now()
	memory.LoadedDocuments = append(memory.LoadedDocuments, doc)
	memory.UpdatedAt = time.Now()

	return s.saveMemory(ctx, memory)
}

// UnloadDocument removes a document from working memory
func (s *WorkingMemoryServiceImpl) UnloadDocument(ctx context.Context, sessionID string, agentID uuid.UUID, documentID uuid.UUID) error {
	memory, err := s.GetWorkingMemory(ctx, sessionID, agentID)
	if err != nil {
		return err
	}

	// Find and remove document
	for i, doc := range memory.LoadedDocuments {
		if doc.DocumentID == documentID {
			memory.LoadedDocuments = append(memory.LoadedDocuments[:i], memory.LoadedDocuments[i+1:]...)
			memory.UpdatedAt = time.Now()

			// Also remove associated chunks
			var remainingChunks []models.RetrievedChunk
			for _, chunk := range memory.RetrievedChunks {
				if chunk.DocumentID != documentID.String() {
					remainingChunks = append(remainingChunks, chunk)
				}
			}
			memory.RetrievedChunks = remainingChunks

			// Recalculate tokens
			memory.TotalTokens = 0
			for _, chunk := range memory.RetrievedChunks {
				memory.TotalTokens += estimateTokenCount(chunk.Content)
			}

			return s.saveMemory(ctx, memory)
		}
	}

	return nil // Document not found, no-op
}

// UpdateLastQuery updates the last query that retrieved this context
func (s *WorkingMemoryServiceImpl) UpdateLastQuery(ctx context.Context, sessionID string, agentID uuid.UUID, query string) error {
	memory, err := s.GetWorkingMemory(ctx, sessionID, agentID)
	if err != nil {
		return err
	}

	memory.LastQuery = query
	now := time.Now()
	memory.LastQueryTime = &now
	memory.UpdatedAt = now

	return s.saveMemory(ctx, memory)
}

// IsContextStale checks if the working memory needs refresh based on new query
func (s *WorkingMemoryServiceImpl) IsContextStale(ctx context.Context, sessionID string, agentID uuid.UUID, newQuery string, threshold float64) (bool, error) {
	memory, err := s.GetWorkingMemory(ctx, sessionID, agentID)
	if err != nil {
		return true, err
	}

	// If no previous query, context is stale
	if memory.LastQuery == "" {
		return true, nil
	}

	// If no chunks, context is stale
	if len(memory.RetrievedChunks) == 0 {
		return true, nil
	}

	// Simple heuristic: if queries are similar, context is not stale
	// In production, you'd want to use embedding similarity
	similarity := calculateSimpleSimilarity(memory.LastQuery, newQuery)
	return similarity < threshold, nil
}

// ClearWorkingMemory clears all working memory for a session
func (s *WorkingMemoryServiceImpl) ClearWorkingMemory(ctx context.Context, sessionID string, agentID uuid.UUID) error {
	key := s.memoryKey(sessionID, agentID)
	return s.redis.Del(ctx, key).Err()
}

// saveMemory saves the working memory to Redis
func (s *WorkingMemoryServiceImpl) saveMemory(ctx context.Context, memory *models.WorkingMemory) error {
	key := s.memoryKey(memory.SessionID, memory.AgentID)
	data, err := json.Marshal(memory)
	if err != nil {
		return fmt.Errorf("failed to marshal working memory: %w", err)
	}

	ttl := s.redis.TTL(ctx, key).Val()
	if ttl <= 0 {
		ttl = s.config.WorkingMemoryTTL
	}

	return s.redis.Set(ctx, key, data, ttl).Err()
}

// trimChunks trims chunks to fit within token limit
func (s *WorkingMemoryServiceImpl) trimChunks(chunks []models.RetrievedChunk, maxTokens int) ([]models.RetrievedChunk, int) {
	var result []models.RetrievedChunk
	totalTokens := 0

	// Sort by score (descending) to keep most relevant chunks
	// For simplicity, assuming chunks are already sorted by relevance
	for _, chunk := range chunks {
		chunkTokens := estimateTokenCount(chunk.Content)
		if totalTokens+chunkTokens > maxTokens {
			break
		}
		result = append(result, chunk)
		totalTokens += chunkTokens
	}

	return result, totalTokens
}

// FormatForContext formats the working memory for context injection
func (s *WorkingMemoryServiceImpl) FormatForContext(memory *models.WorkingMemory) string {
	if memory == nil || len(memory.RetrievedChunks) == 0 {
		return ""
	}

	var formatted string
	formatted += "--- Retrieved Document Context ---\n\n"

	currentDoc := ""
	for _, chunk := range memory.RetrievedChunks {
		if chunk.DocumentName != currentDoc {
			if currentDoc != "" {
				formatted += "\n"
			}
			formatted += fmt.Sprintf("### Document: %s\n", chunk.DocumentName)
			currentDoc = chunk.DocumentName
		}
		formatted += chunk.Content + "\n"
	}

	formatted += "\n--- End Document Context ---\n"
	return formatted
}

// estimateTokenCount provides a rough token estimate (chars / 4)
func estimateTokenCount(text string) int {
	return len(text) / 4
}

// calculateSimpleSimilarity calculates simple word overlap similarity
func calculateSimpleSimilarity(query1, query2 string) float64 {
	if query1 == "" || query2 == "" {
		return 0.0
	}

	// Simple word overlap calculation
	words1 := make(map[string]bool)
	words2 := make(map[string]bool)

	for _, word := range splitWords(query1) {
		words1[word] = true
	}
	for _, word := range splitWords(query2) {
		words2[word] = true
	}

	overlap := 0
	for word := range words1 {
		if words2[word] {
			overlap++
		}
	}

	total := len(words1) + len(words2) - overlap
	if total == 0 {
		return 0.0
	}

	return float64(overlap) / float64(total)
}

// splitWords splits a string into words (simple implementation)
func splitWords(s string) []string {
	var words []string
	var current string
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '.' || r == ',' {
			if current != "" {
				words = append(words, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		words = append(words, current)
	}
	return words
}
