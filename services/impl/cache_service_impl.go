package impl

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
)

const (
	// CacheKeyPrefix is the prefix for all document context cache keys
	CacheKeyPrefix = "doc_context"

	// DefaultCacheTTL is the default TTL for cached context (30 minutes)
	DefaultCacheTTL = 30 * 60

	// MaxCacheTTL is the maximum allowed TTL (24 hours)
	MaxCacheTTL = 24 * 60 * 60
)

// cacheServiceImpl implements CacheService using either in-memory or Redis cache
type cacheServiceImpl struct {
	// In-memory cache (fallback when Redis is unavailable)
	memCache map[string]cacheEntry
	mu       sync.RWMutex

	// Redis cache (production)
	redis *redis.Client

	config    *config.RedisConfig
	enabled   bool
	useRedis  bool
}

type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

// NewCacheService creates a new CacheService instance
// Uses Redis if available, falls back to in-memory cache
func NewCacheService(cfg *config.RedisConfig) (services.CacheService, error) {
	if cfg == nil || !cfg.EnableContextCache {
		return &cacheServiceImpl{
			enabled: false,
		}, nil
	}

	svc := &cacheServiceImpl{
		memCache: make(map[string]cacheEntry),
		config:   cfg,
		enabled:  true,
		useRedis: false,
	}

	// Try to connect to Redis
	if cfg.Host != "" {
		redisClient := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Password: cfg.Password,
			DB:       cfg.DB,
		})

		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := redisClient.Ping(ctx).Err(); err == nil {
			svc.redis = redisClient
			svc.useRedis = true
		}
		// If Redis fails, fall back to in-memory (no error)
	}

	return svc, nil
}

// NewCacheServiceWithRedis creates a cache service with an existing Redis client
func NewCacheServiceWithRedis(redisClient *redis.Client, cfg *config.RedisConfig) services.CacheService {
	if redisClient == nil || cfg == nil || !cfg.EnableContextCache {
		return &cacheServiceImpl{
			memCache: make(map[string]cacheEntry),
			config:   cfg,
			enabled:  cfg != nil && cfg.EnableContextCache,
			useRedis: false,
		}
	}

	return &cacheServiceImpl{
		memCache: make(map[string]cacheEntry),
		redis:    redisClient,
		config:   cfg,
		enabled:  true,
		useRedis: true,
	}
}

// GetCachedContext retrieves cached context if available
func (s *cacheServiceImpl) GetCachedContext(ctx context.Context, cacheKey string) (*models.DocumentContextResult, error) {
	if !s.enabled {
		return nil, nil
	}

	prefixedKey := s.prefixKey(cacheKey)

	// Try Redis first if available
	if s.useRedis && s.redis != nil {
		data, err := s.redis.Get(ctx, prefixedKey).Bytes()
		if err == nil {
			var result models.DocumentContextResult
			if err := json.Unmarshal(data, &result); err != nil {
				// Invalid cache data - delete it
				s.redis.Del(ctx, prefixedKey)
				return nil, nil
			}
			return &result, nil
		}
		if err != redis.Nil {
			// Redis error - fall back to memory cache
			return s.getFromMemCache(prefixedKey)
		}
		return nil, nil // Cache miss
	}

	// Use in-memory cache
	return s.getFromMemCache(prefixedKey)
}

// getFromMemCache retrieves from in-memory cache
func (s *cacheServiceImpl) getFromMemCache(prefixedKey string) (*models.DocumentContextResult, error) {
	s.mu.RLock()
	entry, exists := s.memCache[prefixedKey]
	s.mu.RUnlock()

	if !exists {
		return nil, nil // Cache miss
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		// Entry expired, clean it up
		s.mu.Lock()
		delete(s.memCache, prefixedKey)
		s.mu.Unlock()
		return nil, nil
	}

	var result models.DocumentContextResult
	if err := json.Unmarshal(entry.data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached context: %w", err)
	}

	return &result, nil
}

// SetCachedContext stores context in cache with TTL
func (s *cacheServiceImpl) SetCachedContext(ctx context.Context, cacheKey string, result *models.DocumentContextResult, ttlSeconds int) error {
	if !s.enabled || result == nil {
		return nil
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal context for caching: %w", err)
	}

	ttl := time.Duration(ttlSeconds) * time.Second
	if ttlSeconds <= 0 && s.config != nil {
		ttl = time.Duration(s.config.ContextCacheTTL) * time.Second
	}
	if ttl <= 0 {
		ttl = time.Duration(DefaultCacheTTL) * time.Second
	}

	prefixedKey := s.prefixKey(cacheKey)

	// Use Redis if available
	if s.useRedis && s.redis != nil {
		if err := s.redis.Set(ctx, prefixedKey, data, ttl).Err(); err != nil {
			// Redis error - fall back to memory cache
			s.setInMemCache(prefixedKey, data, ttl)
			return nil
		}
		return nil
	}

	// Use in-memory cache
	s.setInMemCache(prefixedKey, data, ttl)
	return nil
}

// setInMemCache stores data in memory cache
func (s *cacheServiceImpl) setInMemCache(prefixedKey string, data []byte, ttl time.Duration) {
	s.mu.Lock()
	s.memCache[prefixedKey] = cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
	s.mu.Unlock()
}

// InvalidateCache invalidates cached context for specific patterns
func (s *cacheServiceImpl) InvalidateCache(ctx context.Context, pattern string) error {
	if !s.enabled {
		return nil
	}

	prefixedPattern := s.prefixKey(pattern)

	// Use Redis if available
	if s.useRedis && s.redis != nil {
		var cursor uint64
		for {
			keys, newCursor, err := s.redis.Scan(ctx, cursor, prefixedPattern, 100).Result()
			if err != nil {
				break // Redis error - silently fail
			}
			if len(keys) > 0 {
				s.redis.Del(ctx, keys...)
			}
			cursor = newCursor
			if cursor == 0 {
				break
			}
		}
	}

	// Always clear in-memory cache as well
	s.mu.Lock()
	defer s.mu.Unlock()

	for key := range s.memCache {
		if matchPattern(key, prefixedPattern) {
			delete(s.memCache, key)
		}
	}

	return nil
}

// matchPattern provides simple pattern matching (* as wildcard)
func matchPattern(key, pattern string) bool {
	// Simple implementation - matches if pattern prefix matches
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}
	return key == pattern
}

// GenerateCacheKey generates a cache key for context retrieval
func (s *cacheServiceImpl) GenerateCacheKey(agentID uuid.UUID, sessionID *string, queryHash string) string {
	h := sha256.New()
	h.Write([]byte(agentID.String()))
	if sessionID != nil {
		h.Write([]byte(*sessionID))
	}
	h.Write([]byte(queryHash))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// prefixKey adds a prefix to cache keys for namespacing
func (s *cacheServiceImpl) prefixKey(key string) string {
	return fmt.Sprintf("%s:%s", CacheKeyPrefix, key)
}

// HashQuery generates a hash of a query string for use in cache keys
func HashQuery(query string) string {
	hash := sha256.Sum256([]byte(query))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes for shorter key
}

// InvalidateAgentCache invalidates all cached context for a specific agent
func (s *cacheServiceImpl) InvalidateAgentCache(ctx context.Context, agentID uuid.UUID) error {
	pattern := fmt.Sprintf("%s:*", agentID.String())
	return s.InvalidateCache(ctx, pattern)
}

// InvalidateSessionCache invalidates all cached context for a specific session
func (s *cacheServiceImpl) InvalidateSessionCache(ctx context.Context, agentID uuid.UUID, sessionID string) error {
	pattern := fmt.Sprintf("%s:%s:*", agentID.String(), sessionID)
	return s.InvalidateCache(ctx, pattern)
}

// IsUsingRedis returns true if the cache is using Redis backend
func (s *cacheServiceImpl) IsUsingRedis() bool {
	return s.useRedis
}

// ============================================
// CachedContextService - Wrapper with Caching
// ============================================

// CachedContextService wraps DocumentContextService with caching
type CachedContextService struct {
	delegate services.DocumentContextService
	cache    services.CacheService
	ttl      int
}

// NewCachedContextService creates a new cached document context service
func NewCachedContextService(delegate services.DocumentContextService, cache services.CacheService, ttlSeconds int) *CachedContextService {
	if ttlSeconds <= 0 {
		ttlSeconds = DefaultCacheTTL
	}
	return &CachedContextService{
		delegate: delegate,
		cache:    cache,
		ttl:      ttlSeconds,
	}
}

// RetrieveVectorContext performs vector search with caching
func (s *CachedContextService) RetrieveVectorContext(ctx context.Context, req models.VectorSearchRequest) (*models.DocumentContextResult, error) {
	// Generate cache key from request
	cacheKey := s.generateVectorSearchCacheKey(req)

	// Try to get from cache
	cached, err := s.cache.GetCachedContext(ctx, cacheKey)
	if err == nil && cached != nil {
		// Cache hit - add metadata indicating cached response
		if cached.Metadata == nil {
			cached.Metadata = make(map[string]any)
		}
		cached.Metadata["cached"] = true
		cached.Metadata["cache_key"] = cacheKey
		return cached, nil
	}

	// Cache miss - execute the actual retrieval
	result, err := s.delegate.RetrieveVectorContext(ctx, req)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if result != nil {
		_ = s.cache.SetCachedContext(ctx, cacheKey, result, s.ttl)
	}

	return result, nil
}

// RetrieveFullDocuments retrieves complete document content with caching
func (s *CachedContextService) RetrieveFullDocuments(ctx context.Context, req models.ChunkRetrievalRequest) (*models.DocumentContextResult, error) {
	// Generate cache key from request
	cacheKey := s.generateChunkRetrievalCacheKey(req)

	// Try to get from cache
	cached, err := s.cache.GetCachedContext(ctx, cacheKey)
	if err == nil && cached != nil {
		if cached.Metadata == nil {
			cached.Metadata = make(map[string]any)
		}
		cached.Metadata["cached"] = true
		cached.Metadata["cache_key"] = cacheKey
		return cached, nil
	}

	// Cache miss - execute the actual retrieval
	result, err := s.delegate.RetrieveFullDocuments(ctx, req)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if result != nil {
		_ = s.cache.SetCachedContext(ctx, cacheKey, result, s.ttl)
	}

	return result, nil
}

// RetrieveHybridContext combines vector search with full document sections (with caching)
func (s *CachedContextService) RetrieveHybridContext(ctx context.Context, query string, req models.ChunkRetrievalRequest, vectorWeight, fullDocWeight float64) (*models.DocumentContextResult, error) {
	// Generate cache key
	cacheKey := s.generateHybridCacheKey(query, req, vectorWeight, fullDocWeight)

	// Try to get from cache
	cached, err := s.cache.GetCachedContext(ctx, cacheKey)
	if err == nil && cached != nil {
		if cached.Metadata == nil {
			cached.Metadata = make(map[string]any)
		}
		cached.Metadata["cached"] = true
		cached.Metadata["cache_key"] = cacheKey
		return cached, nil
	}

	// Cache miss - execute the actual retrieval
	result, err := s.delegate.RetrieveHybridContext(ctx, query, req, vectorWeight, fullDocWeight)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if result != nil {
		_ = s.cache.SetCachedContext(ctx, cacheKey, result, s.ttl)
	}

	return result, nil
}

// RetrieveHybridContextWithConfig combines vector search with full document sections using advanced configuration
func (s *CachedContextService) RetrieveHybridContextWithConfig(ctx context.Context, query string, req models.ChunkRetrievalRequest, config *models.HybridContextConfig) (*models.HybridContextResult, error) {
	// For hybrid with config, we don't cache as aggressively since configurations can be complex
	// Pass through to delegate
	return s.delegate.RetrieveHybridContextWithConfig(ctx, query, req, config)
}

// GetNotebookDocuments retrieves document list for a notebook (pass-through, no caching)
func (s *CachedContextService) GetNotebookDocuments(ctx context.Context, notebookIDs []uuid.UUID, tenantID string, includeSubNotebooks bool) ([]models.NotebookDocument, error) {
	return s.delegate.GetNotebookDocuments(ctx, notebookIDs, tenantID, includeSubNotebooks)
}

// FormatContextForInjection formats retrieved chunks into a string (pass-through)
func (s *CachedContextService) FormatContextForInjection(result *models.DocumentContextResult, maxTokens int) (*models.ContextInjectionResult, error) {
	return s.delegate.FormatContextForInjection(result, maxTokens)
}

// EstimateTokenCount estimates the number of tokens in a string (pass-through)
func (s *CachedContextService) EstimateTokenCount(text string) int {
	return s.delegate.EstimateTokenCount(text)
}

// Helper methods for cache key generation

func (s *CachedContextService) generateVectorSearchCacheKey(req models.VectorSearchRequest) string {
	// Create a deterministic hash of the request
	keyData := fmt.Sprintf("vector:%s:%s:%v:%v:%d:%.2f",
		req.TenantID,
		req.QueryText,
		req.NotebookIDs,
		req.DocumentIDs,
		req.Options.TopK,
		req.Options.MinScore,
	)
	return HashQuery(keyData)
}

func (s *CachedContextService) generateChunkRetrievalCacheKey(req models.ChunkRetrievalRequest) string {
	keyData := fmt.Sprintf("chunks:%s:%v:%v:%v:%d:%d",
		req.TenantID,
		req.FileIDs,
		req.NotebookIDs,
		req.ChunkTypes,
		req.Limit,
		req.Offset,
	)
	return HashQuery(keyData)
}

func (s *CachedContextService) generateHybridCacheKey(query string, req models.ChunkRetrievalRequest, vectorWeight, fullDocWeight float64) string {
	keyData := fmt.Sprintf("hybrid:%s:%s:%v:%v:%.2f:%.2f",
		req.TenantID,
		query,
		req.FileIDs,
		req.NotebookIDs,
		vectorWeight,
		fullDocWeight,
	)
	return HashQuery(keyData)
}
