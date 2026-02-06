package impl

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
)

// documentContextServiceImpl implements DocumentContextService
type documentContextServiceImpl struct {
	deeplakeConfig  *config.DeepLakeConfig
	audimodalConfig *config.AudiModalConfig
	aetherConfig    *config.AetherConfig
	httpClient      *http.Client
	cacheService    services.CacheService
}

// NewDocumentContextService creates a new DocumentContextService instance
func NewDocumentContextService(
	deeplakeCfg *config.DeepLakeConfig,
	audimodalCfg *config.AudiModalConfig,
	aetherCfg *config.AetherConfig,
	cacheSvc services.CacheService,
) services.DocumentContextService {
	return &documentContextServiceImpl{
		deeplakeConfig:  deeplakeCfg,
		audimodalConfig: audimodalCfg,
		aetherConfig:    aetherCfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cacheService: cacheSvc,
	}
}

// RetrieveVectorContext performs vector search to retrieve relevant document chunks
func (s *documentContextServiceImpl) RetrieveVectorContext(ctx context.Context, req models.VectorSearchRequest) (*models.DocumentContextResult, error) {
	startTime := time.Now()

	// Build DeepLake search request
	searchReq := map[string]interface{}{
		"query_text": req.QueryText,
		"options": map[string]interface{}{
			"top_k":            req.Options.TopK,
			"include_content":  true,
			"include_metadata": true,
		},
	}

	if req.Options.MinScore > 0 {
		searchReq["options"].(map[string]interface{})["min_score"] = req.Options.MinScore
	}

	// Determine dataset ID based on configuration
	datasetID := req.DatasetID
	if datasetID == "" {
		datasetID = "documents" // Default shared dataset
	}

	// Make request to DeepLake API
	url := fmt.Sprintf("%s/api/v1/datasets/%s/search/text", s.deeplakeConfig.BaseURL, datasetID)

	jsonData, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if s.deeplakeConfig.APIKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("ApiKey %s", s.deeplakeConfig.APIKey))
	}
	if req.TenantID != "" {
		httpReq.Header.Set("X-Tenant-ID", req.TenantID)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DeepLake search returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var searchResp struct {
		Results []struct {
			Vector struct {
				ID          string                 `json:"id"`
				DocumentID  string                 `json:"document_id"`
				ChunkID     string                 `json:"chunk_id"`
				Content     string                 `json:"content"`
				ContentHash string                 `json:"content_hash"`
				Metadata    map[string]interface{} `json:"metadata"`
				ChunkIndex  *int                   `json:"chunk_index"`
				ChunkCount  *int                   `json:"chunk_count"`
			} `json:"vector"`
			Score    float64 `json:"score"`
			Distance float64 `json:"distance"`
			Rank     int     `json:"rank"`
		} `json:"results"`
		TotalFound      int     `json:"total_found"`
		HasMore         bool    `json:"has_more"`
		QueryTimeMs     float64 `json:"query_time_ms"`
		EmbeddingTimeMs float64 `json:"embedding_time_ms"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	// Convert to RetrievedChunk format
	chunks := make([]models.RetrievedChunk, 0, len(searchResp.Results))
	totalTokens := 0

	for _, result := range searchResp.Results {
		chunk := models.RetrievedChunk{
			ID:          result.Vector.ID,
			DocumentID:  result.Vector.DocumentID,
			Content:     result.Vector.Content,
			Score:       result.Score,
			Distance:    result.Distance,
			Metadata:    result.Vector.Metadata,
		}

		// Extract chunk metadata
		if result.Vector.ChunkIndex != nil {
			chunk.ChunkNumber = *result.Vector.ChunkIndex
		}
		if result.Vector.ChunkCount != nil {
			chunk.TotalChunks = *result.Vector.ChunkCount
		}

		// Extract document name from metadata
		if result.Vector.Metadata != nil {
			if name, ok := result.Vector.Metadata["document_name"].(string); ok {
				chunk.DocumentName = name
			}
			if contentType, ok := result.Vector.Metadata["content_type"].(string); ok {
				chunk.ContentType = contentType
			}
			if lang, ok := result.Vector.Metadata["language"].(string); ok {
				chunk.Language = lang
			}
			if pageNum, ok := result.Vector.Metadata["page_number"].(float64); ok {
				pn := int(pageNum)
				chunk.PageNumber = &pn
			}
		}

		chunks = append(chunks, chunk)
		totalTokens += s.EstimateTokenCount(chunk.Content)
	}

	retrievalTime := int(time.Since(startTime).Milliseconds())

	return &models.DocumentContextResult{
		Chunks:          chunks,
		TotalTokens:     totalTokens,
		Strategy:        models.ContextStrategyVector,
		RetrievalTimeMs: retrievalTime,
		Metadata: map[string]interface{}{
			"total_found":       searchResp.TotalFound,
			"has_more":          searchResp.HasMore,
			"query_time_ms":     searchResp.QueryTimeMs,
			"embedding_time_ms": searchResp.EmbeddingTimeMs,
		},
	}, nil
}

// RetrieveFullDocuments retrieves complete document content for injection
func (s *documentContextServiceImpl) RetrieveFullDocuments(ctx context.Context, req models.ChunkRetrievalRequest) (*models.DocumentContextResult, error) {
	startTime := time.Now()

	log.Printf("[DEBUG] RetrieveFullDocuments called: tenant_id=%s, file_ids_count=%d, auth_token_length=%d",
		req.TenantID, len(req.FileIDs), len(req.AuthToken))

	// If we have file IDs, retrieve chunks for each file using the /files/{id}/chunks endpoint
	// This is more reliable than the query parameter approach
	var allChunks []audiModalChunk
	var totalCount int64

	if len(req.FileIDs) > 0 {
		for _, fileID := range req.FileIDs {
			log.Printf("[DEBUG] Fetching chunks for file: %s", fileID.String())
			chunks, count, err := s.fetchChunksForFile(ctx, req.TenantID, fileID.String(), req.AuthToken, req.Limit, req.Offset)
			if err != nil {
				log.Printf("[DEBUG] Failed to fetch chunks for file %s: %v", fileID.String(), err)
				continue // Skip this file but try others
			}
			allChunks = append(allChunks, chunks...)
			totalCount += count
			log.Printf("[DEBUG] Retrieved %d chunks for file %s", len(chunks), fileID.String())
		}
	} else {
		log.Printf("[DEBUG] No file IDs provided - fetching all chunks for tenant")
		chunks, count, err := s.fetchAllChunksForTenant(ctx, req.TenantID, req.AuthToken, req.Limit, req.Offset)
		if err != nil {
			return nil, err
		}
		allChunks = chunks
		totalCount = count
	}

	log.Printf("[DEBUG] Total chunks retrieved: %d, total_count=%d", len(allChunks), totalCount)

	// Convert to RetrievedChunk format and sort by document/chunk order
	chunks := make([]models.RetrievedChunk, 0, len(allChunks))
	totalTokens := 0

	for _, c := range allChunks {
		chunk := models.RetrievedChunk{
			ID:          c.ID,
			DocumentID:  c.FileID,
			Content:     c.Content,
			ChunkNumber: c.ChunkNumber,
			ContentType: c.ChunkType,
			PageNumber:  c.PageNumber,
			Metadata:    c.Metadata,
		}

		chunks = append(chunks, chunk)
		totalTokens += s.EstimateTokenCount(chunk.Content)
	}

	// Sort chunks by document ID and chunk number for proper ordering
	sort.Slice(chunks, func(i, j int) bool {
		if chunks[i].DocumentID != chunks[j].DocumentID {
			return chunks[i].DocumentID < chunks[j].DocumentID
		}
		return chunks[i].ChunkNumber < chunks[j].ChunkNumber
	})

	retrievalTime := int(time.Since(startTime).Milliseconds())

	return &models.DocumentContextResult{
		Chunks:          chunks,
		TotalTokens:     totalTokens,
		Strategy:        models.ContextStrategyFull,
		RetrievalTimeMs: retrievalTime,
		Metadata: map[string]interface{}{
			"total":    totalCount,
			"has_more": false,
		},
	}, nil
}

// audiModalChunk represents a chunk from AudiModal's response
type audiModalChunk struct {
	ID          string
	FileID      string
	ChunkID     string
	ChunkType   string
	ChunkNumber int
	Content     string
	PageNumber  *int
	Metadata    map[string]interface{}
}

// fetchChunksForFile fetches ALL chunks for a specific file using pagination
// It fetches pages until all chunks are retrieved or token budget is reached
func (s *documentContextServiceImpl) fetchChunksForFile(ctx context.Context, tenantID, fileID, authToken string, limit, offset int) ([]audiModalChunk, int64, error) {
	const maxPageSize = 100      // AudiModal's max page size
	const maxTokenBudget = 50000 // Max tokens to retrieve (leaves room for system prompt)

	var allChunks []audiModalChunk
	var totalCount int64
	currentPage := 1
	currentTokens := 0

	for {
		// Build URL with pagination
		url := fmt.Sprintf("%s/api/v1/tenants/%s/files/%s/chunks?page=%d&page_size=%d",
			s.audimodalConfig.BaseURL, tenantID, fileID, currentPage, maxPageSize)

		log.Printf("[DEBUG] Fetching chunks page %d from URL: %s", currentPage, url)

		httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create HTTP request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if authToken != "" {
			httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
		} else if s.audimodalConfig.APIKey != "" {
			httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.audimodalConfig.APIKey))
		}

		resp, err := s.httpClient.Do(httpReq)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to execute request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, 0, fmt.Errorf("AudiModal returned status %d: %s", resp.StatusCode, string(body))
		}

		chunks, count, hasNext, err := s.parseAudiModalResponseWithPagination(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, 0, err
		}

		totalCount = count

		// Add chunks while tracking token budget
		for _, chunk := range chunks {
			chunkTokens := s.EstimateTokenCount(chunk.Content)
			if currentTokens+chunkTokens > maxTokenBudget {
				log.Printf("[DEBUG] Token budget reached: %d tokens, stopping pagination", currentTokens)
				return allChunks, totalCount, nil
			}
			allChunks = append(allChunks, chunk)
			currentTokens += chunkTokens
		}

		log.Printf("[DEBUG] Page %d: fetched %d chunks, total so far: %d, tokens: %d, hasNext: %v",
			currentPage, len(chunks), len(allChunks), currentTokens, hasNext)

		// Check if we should continue
		if !hasNext || len(chunks) == 0 {
			break
		}

		currentPage++

		// Safety limit to prevent infinite loops
		if currentPage > 100 {
			log.Printf("[DEBUG] Safety limit reached at page %d", currentPage)
			break
		}
	}

	return allChunks, totalCount, nil
}

// fetchAllChunksForTenant fetches all chunks for a tenant (when no file IDs are specified)
func (s *documentContextServiceImpl) fetchAllChunksForTenant(ctx context.Context, tenantID, authToken string, limit, offset int) ([]audiModalChunk, int64, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/chunks", s.audimodalConfig.BaseURL, tenantID)
	params := make([]string, 0)
	if limit > 0 {
		params = append(params, fmt.Sprintf("page_size=%d", limit))
	}
	if offset > 0 {
		params = append(params, fmt.Sprintf("offset=%d", offset))
	}
	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}

	log.Printf("[DEBUG] Fetching all chunks from URL: %s", url)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
	} else if s.audimodalConfig.APIKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.audimodalConfig.APIKey))
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("AudiModal returned status %d: %s", resp.StatusCode, string(body))
	}

	return s.parseAudiModalResponse(resp.Body)
}

// parseAudiModalResponse parses the AudiModal API response format
// AudiModal returns: {"success": true, "data": [...], "meta": {"pagination": {...}, "count": N}}
func (s *documentContextServiceImpl) parseAudiModalResponse(body io.Reader) ([]audiModalChunk, int64, error) {
	var apiResp struct {
		Success bool `json:"success"`
		Data    []struct {
			ID          string                 `json:"id"`
			TenantID    string                 `json:"tenant_id"`
			FileID      string                 `json:"file_id"`
			ChunkID     string                 `json:"chunk_id"`
			ChunkType   string                 `json:"chunk_type"`
			ChunkNumber int                    `json:"chunk_number"`
			Content     string                 `json:"content"`
			PageNumber  *int                   `json:"page_number"`
			Metadata    map[string]interface{} `json:"metadata"`
		} `json:"data"`
		Meta struct {
			Pagination struct {
				TotalCount int64 `json:"total_count"`
				Page       int   `json:"page"`
				PageSize   int   `json:"page_size"`
				HasNext    bool  `json:"has_next"`
			} `json:"pagination"`
			Count *int64 `json:"count"`
		} `json:"meta"`
	}

	if err := json.NewDecoder(body).Decode(&apiResp); err != nil {
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[DEBUG] Parsed AudiModal response: success=%v, data_count=%d, total_count=%d",
		apiResp.Success, len(apiResp.Data), apiResp.Meta.Pagination.TotalCount)

	// Convert to our internal format
	chunks := make([]audiModalChunk, 0, len(apiResp.Data))
	for _, c := range apiResp.Data {
		chunks = append(chunks, audiModalChunk{
			ID:          c.ID,
			FileID:      c.FileID,
			ChunkID:     c.ChunkID,
			ChunkType:   c.ChunkType,
			ChunkNumber: c.ChunkNumber,
			Content:     c.Content,
			PageNumber:  c.PageNumber,
			Metadata:    c.Metadata,
		})
	}

	totalCount := apiResp.Meta.Pagination.TotalCount
	if apiResp.Meta.Count != nil && *apiResp.Meta.Count > totalCount {
		totalCount = *apiResp.Meta.Count
	}

	return chunks, totalCount, nil
}

// parseAudiModalResponseWithPagination parses the response and returns pagination info
func (s *documentContextServiceImpl) parseAudiModalResponseWithPagination(body io.Reader) ([]audiModalChunk, int64, bool, error) {
	var apiResp struct {
		Success bool `json:"success"`
		Data    []struct {
			ID          string                 `json:"id"`
			TenantID    string                 `json:"tenant_id"`
			FileID      string                 `json:"file_id"`
			ChunkID     string                 `json:"chunk_id"`
			ChunkType   string                 `json:"chunk_type"`
			ChunkNumber int                    `json:"chunk_number"`
			Content     string                 `json:"content"`
			PageNumber  *int                   `json:"page_number"`
			Metadata    map[string]interface{} `json:"metadata"`
		} `json:"data"`
		Meta struct {
			Pagination struct {
				TotalCount int64 `json:"total_count"`
				Page       int   `json:"page"`
				PageSize   int   `json:"page_size"`
				HasNext    bool  `json:"has_next"`
			} `json:"pagination"`
			Count *int64 `json:"count"`
		} `json:"meta"`
	}

	if err := json.NewDecoder(body).Decode(&apiResp); err != nil {
		return nil, 0, false, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to our internal format
	chunks := make([]audiModalChunk, 0, len(apiResp.Data))
	for _, c := range apiResp.Data {
		chunks = append(chunks, audiModalChunk{
			ID:          c.ID,
			FileID:      c.FileID,
			ChunkID:     c.ChunkID,
			ChunkType:   c.ChunkType,
			ChunkNumber: c.ChunkNumber,
			Content:     c.Content,
			PageNumber:  c.PageNumber,
			Metadata:    c.Metadata,
		})
	}

	totalCount := apiResp.Meta.Pagination.TotalCount
	if apiResp.Meta.Count != nil && *apiResp.Meta.Count > totalCount {
		totalCount = *apiResp.Meta.Count
	}

	return chunks, totalCount, apiResp.Meta.Pagination.HasNext, nil
}

// RetrieveHybridContext combines vector search with full document sections
func (s *documentContextServiceImpl) RetrieveHybridContext(
	ctx context.Context,
	query string,
	req models.ChunkRetrievalRequest,
	vectorWeight, fullDocWeight float64,
) (*models.DocumentContextResult, error) {
	startTime := time.Now()

	// Perform vector search
	vectorReq := models.VectorSearchRequest{
		QueryText: query,
		TenantID:  req.TenantID,
		Options: models.SearchOptions{
			TopK:          20, // Get more results for hybrid merging
			MinScore:      0.6,
			IncludeChunks: true,
		},
	}

	vectorResult, err := s.RetrieveVectorContext(ctx, vectorReq)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// Perform full document retrieval
	fullResult, err := s.RetrieveFullDocuments(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("full document retrieval failed: %w", err)
	}

	// Merge and deduplicate results
	mergedChunks := s.mergeAndRankChunks(vectorResult.Chunks, fullResult.Chunks, vectorWeight, fullDocWeight)

	// Calculate total tokens
	totalTokens := 0
	for _, chunk := range mergedChunks {
		totalTokens += s.EstimateTokenCount(chunk.Content)
	}

	retrievalTime := int(time.Since(startTime).Milliseconds())

	return &models.DocumentContextResult{
		Chunks:          mergedChunks,
		TotalTokens:     totalTokens,
		Strategy:        models.ContextStrategyHybrid,
		RetrievalTimeMs: retrievalTime,
		Metadata: map[string]interface{}{
			"vector_count":    len(vectorResult.Chunks),
			"full_doc_count":  len(fullResult.Chunks),
			"merged_count":    len(mergedChunks),
			"vector_weight":   vectorWeight,
			"full_doc_weight": fullDocWeight,
		},
	}, nil
}

// RetrieveHybridContextWithConfig combines vector search with full document sections using advanced configuration
func (s *documentContextServiceImpl) RetrieveHybridContextWithConfig(
	ctx context.Context,
	query string,
	req models.ChunkRetrievalRequest,
	config *models.HybridContextConfig,
) (*models.HybridContextResult, error) {
	if config == nil {
		config = models.DefaultHybridContextConfig()
	}

	// Perform vector search with configured parameters
	vectorReq := models.VectorSearchRequest{
		QueryText: query,
		TenantID:  req.TenantID,
		Options: models.SearchOptions{
			TopK:          config.VectorTopK,
			MinScore:      config.VectorMinScore,
			IncludeChunks: true,
		},
	}

	vectorResult, err := s.RetrieveVectorContext(ctx, vectorReq)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// Limit full doc chunks if configured
	if config.FullDocMaxChunks > 0 && req.Limit == 0 {
		req.Limit = config.FullDocMaxChunks
	}

	// Perform full document retrieval
	fullResult, err := s.RetrieveFullDocuments(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("full document retrieval failed: %w", err)
	}

	// Use the HybridContextBuilder to merge and rank
	builder := NewHybridContextBuilder(config)
	return builder.BuildHybridContext(ctx, vectorResult.Chunks, fullResult.Chunks, s.EstimateTokenCount)
}

// mergeAndRankChunks merges and deduplicates chunks from vector and full doc retrieval
func (s *documentContextServiceImpl) mergeAndRankChunks(
	vectorChunks, fullDocChunks []models.RetrievedChunk,
	vectorWeight, fullDocWeight float64,
) []models.RetrievedChunk {
	// Create a map to track unique chunks and their combined scores
	chunkMap := make(map[string]*models.RetrievedChunk)

	// Add vector search results with weighted scores
	for i := range vectorChunks {
		chunk := vectorChunks[i]
		key := chunk.DocumentID + "_" + fmt.Sprintf("%d", chunk.ChunkNumber)
		chunk.Score = chunk.Score * vectorWeight
		chunkMap[key] = &chunk
	}

	// Add full document chunks, combining scores for duplicates
	for i := range fullDocChunks {
		chunk := fullDocChunks[i]
		key := chunk.DocumentID + "_" + fmt.Sprintf("%d", chunk.ChunkNumber)

		if existing, exists := chunkMap[key]; exists {
			// Combine scores
			existing.Score += (1.0 - float64(chunk.ChunkNumber)/100.0) * fullDocWeight
		} else {
			// Calculate position-based score for full doc chunks
			chunk.Score = (1.0 - float64(chunk.ChunkNumber)/100.0) * fullDocWeight
			chunkMap[key] = &chunk
		}
	}

	// Convert to slice and sort by score
	result := make([]models.RetrievedChunk, 0, len(chunkMap))
	for _, chunk := range chunkMap {
		result = append(result, *chunk)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result
}

// GetNotebookDocuments retrieves document list for notebooks
func (s *documentContextServiceImpl) GetNotebookDocuments(
	ctx context.Context,
	notebookIDs []uuid.UUID,
	tenantID string,
	includeSubNotebooks bool,
) ([]models.NotebookDocument, error) {
	var allDocuments []models.NotebookDocument
	seenDocs := make(map[string]bool) // Track seen document IDs to avoid duplicates

	for _, notebookID := range notebookIDs {
		var endpoint string
		if includeSubNotebooks {
			// Use the recursive endpoint for sub-notebook document retrieval
			endpoint = fmt.Sprintf("%s/api/v1/internal/notebooks/%s/documents/recursive?tenant_id=%s",
				s.aetherConfig.BaseURL, notebookID.String(), tenantID)
		} else {
			// Use the flat endpoint for single notebook
			endpoint = fmt.Sprintf("%s/api/v1/internal/notebooks/%s/documents?tenant_id=%s",
				s.aetherConfig.BaseURL, notebookID.String(), tenantID)
		}

		httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if s.aetherConfig.APIKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+s.aetherConfig.APIKey)
		}

		resp, err := s.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("failed to execute notebook documents request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("Aether-BE notebook documents returned status %d: %s", resp.StatusCode, string(body))
		}

		var docsResp struct {
			NotebookID string                    `json:"notebook_id"`
			Documents  []models.NotebookDocument `json:"documents"`
			Total      int                       `json:"total"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&docsResp); err != nil {
			return nil, fmt.Errorf("failed to decode documents response: %w", err)
		}

		// Add documents, avoiding duplicates
		for _, doc := range docsResp.Documents {
			docIDStr := doc.ID.String()
			if !seenDocs[docIDStr] {
				seenDocs[docIDStr] = true
				allDocuments = append(allDocuments, doc)
			}
		}
	}

	return allDocuments, nil
}

// FormatContextForInjection formats retrieved chunks into a string ready for prompt injection
func (s *documentContextServiceImpl) FormatContextForInjection(
	result *models.DocumentContextResult,
	maxTokens int,
) (*models.ContextInjectionResult, error) {
	if result == nil || len(result.Chunks) == 0 {
		return &models.ContextInjectionResult{
			FormattedContext: "",
			ChunkCount:       0,
			DocumentCount:    0,
			TotalTokens:      0,
			Strategy:         models.ContextStrategyNone,
			Truncated:        false,
		}, nil
	}

	var builder strings.Builder
	builder.WriteString("\n--- RELEVANT CONTEXT ---\n\n")

	currentTokens := s.EstimateTokenCount(builder.String())
	truncated := false
	includedChunks := 0
	documentSet := make(map[string]bool)
	currentDocID := ""

	for _, chunk := range result.Chunks {
		chunkTokens := s.EstimateTokenCount(chunk.Content)

		// Check if adding this chunk would exceed the token limit
		if maxTokens > 0 && currentTokens+chunkTokens > maxTokens {
			truncated = true
			break
		}

		// Add document separator if switching documents
		if chunk.DocumentID != currentDocID && chunk.DocumentID != "" {
			if currentDocID != "" {
				builder.WriteString("\n")
			}
			docName := chunk.DocumentName
			if docName == "" {
				docName = chunk.DocumentID
			}
			builder.WriteString(fmt.Sprintf("--- Document: %s ---\n", docName))
			currentDocID = chunk.DocumentID
			documentSet[chunk.DocumentID] = true
		}

		// Add chunk content
		builder.WriteString(chunk.Content)
		builder.WriteString("\n")

		currentTokens += chunkTokens
		includedChunks++
	}

	builder.WriteString("\n--- END CONTEXT ---\n")

	formattedContext := builder.String()
	totalTokens := s.EstimateTokenCount(formattedContext)

	return &models.ContextInjectionResult{
		FormattedContext: formattedContext,
		ChunkCount:       includedChunks,
		DocumentCount:    len(documentSet),
		TotalTokens:      totalTokens,
		Strategy:         result.Strategy,
		Truncated:        truncated,
		Metadata: map[string]interface{}{
			"original_chunk_count": len(result.Chunks),
			"truncated_count":      len(result.Chunks) - includedChunks,
		},
	}, nil
}

// EstimateTokenCount provides a rough estimate of token count
// Uses a simple heuristic: ~4 characters per token for English text
func (s *documentContextServiceImpl) EstimateTokenCount(text string) int {
	if text == "" {
		return 0
	}
	// Rough estimate: 4 characters per token on average
	return len(text) / 4
}

// Helper function to generate cache key
func GenerateContextCacheKey(agentID uuid.UUID, sessionID *string, query string) string {
	h := sha256.New()
	h.Write([]byte(agentID.String()))
	if sessionID != nil {
		h.Write([]byte(*sessionID))
	}
	h.Write([]byte(query))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
