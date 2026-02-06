package impl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
)

// mcpContextServiceImpl implements MCPContextService
type mcpContextServiceImpl struct {
	mcpServerURL string
	httpClient   *http.Client
	config       *models.MCPConfig
}

// NewMCPContextService creates a new MCP context service
func NewMCPContextService(mcpServerURL string, config *models.MCPConfig) services.MCPContextService {
	if config == nil {
		config = models.DefaultMCPConfig()
	}
	if mcpServerURL != "" {
		config.ServerURL = mcpServerURL
	}

	return &mcpContextServiceImpl{
		mcpServerURL: config.ServerURL,
		config:       config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.TimeoutMs) * time.Millisecond,
		},
	}
}

// InvokeTool invokes an MCP tool and returns the result
func (s *mcpContextServiceImpl) InvokeTool(ctx context.Context, req models.MCPToolRequest) (*models.MCPToolResponse, error) {
	startTime := time.Now()

	// Build the MCP tool call request
	mcpRequest := map[string]any{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  "tools/call",
		"params": map[string]any{
			"name":      req.ToolName,
			"arguments": req.Parameters,
		},
	}

	reqBody, err := json.Marshal(mcpRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MCP request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", s.mcpServerURL+"/mcp/tools/call", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if req.TenantID != "" {
		httpReq.Header.Set("X-Tenant-ID", req.TenantID)
	}

	// Execute request
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return &models.MCPToolResponse{
			ToolName:    req.ToolName,
			Success:     false,
			Error:       fmt.Sprintf("HTTP request failed: %v", err),
			ExecutionMs: int(time.Since(startTime).Milliseconds()),
		}, nil
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &models.MCPToolResponse{
			ToolName:    req.ToolName,
			Success:     false,
			Error:       fmt.Sprintf("failed to read response: %v", err),
			ExecutionMs: int(time.Since(startTime).Milliseconds()),
		}, nil
	}

	// Parse MCP response
	var mcpResp struct {
		Result any    `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &mcpResp); err != nil {
		return &models.MCPToolResponse{
			ToolName:    req.ToolName,
			Success:     false,
			Error:       fmt.Sprintf("failed to parse MCP response: %v", err),
			ExecutionMs: int(time.Since(startTime).Milliseconds()),
		}, nil
	}

	if mcpResp.Error != nil {
		return &models.MCPToolResponse{
			ToolName:    req.ToolName,
			Success:     false,
			Error:       mcpResp.Error.Message,
			ExecutionMs: int(time.Since(startTime).Milliseconds()),
		}, nil
	}

	return &models.MCPToolResponse{
		ToolName:    req.ToolName,
		Success:     true,
		Result:      mcpResp.Result,
		ExecutionMs: int(time.Since(startTime).Milliseconds()),
	}, nil
}

// ListAvailableTools lists all available MCP tools for document retrieval
func (s *mcpContextServiceImpl) ListAvailableTools(ctx context.Context) ([]models.MCPToolDefinition, error) {
	// Build the MCP tools/list request
	mcpRequest := map[string]any{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  "tools/list",
	}

	reqBody, err := json.Marshal(mcpRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MCP request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", s.mcpServerURL+"/mcp/tools/list", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP tools: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse MCP response
	var mcpResp struct {
		Result struct {
			Tools []struct {
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				InputSchema map[string]interface{} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &mcpResp); err != nil {
		return nil, fmt.Errorf("failed to parse MCP response: %w", err)
	}

	// Convert to our tool definition format
	tools := make([]models.MCPToolDefinition, 0, len(mcpResp.Result.Tools))
	for _, t := range mcpResp.Result.Tools {
		// Only include document retrieval tools
		if s.isEnabledTool(t.Name) {
			tools = append(tools, models.MCPToolDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
				Server:      "tas-mcp",
			})
		}
	}

	return tools, nil
}

// isEnabledTool checks if a tool is in the enabled tools list
func (s *mcpContextServiceImpl) isEnabledTool(toolName string) bool {
	for _, enabled := range s.config.EnabledTools {
		if enabled == toolName {
			return true
		}
	}
	return false
}

// SearchDocuments searches documents using MCP search tool
func (s *mcpContextServiceImpl) SearchDocuments(ctx context.Context, req models.MCPSearchRequest) (*models.DocumentContextResult, error) {
	startTime := time.Now()

	// Build search parameters
	params := map[string]any{
		"query":     req.Query,
		"tenant_id": req.TenantID,
	}
	if len(req.NotebookIDs) > 0 {
		notebookStrs := make([]string, len(req.NotebookIDs))
		for i, id := range req.NotebookIDs {
			notebookStrs[i] = id.String()
		}
		params["notebook_ids"] = notebookStrs
	}
	if req.TopK > 0 {
		params["top_k"] = req.TopK
	}
	if req.MinScore > 0 {
		params["min_score"] = req.MinScore
	}

	// Invoke the search_documents MCP tool
	toolResp, err := s.InvokeTool(ctx, models.MCPToolRequest{
		ToolName:   "search_documents",
		Parameters: params,
		TenantID:   req.TenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to invoke search_documents tool: %w", err)
	}

	if !toolResp.Success {
		return nil, fmt.Errorf("search_documents tool failed: %s", toolResp.Error)
	}

	// Parse the search results
	chunks, totalTokens, err := s.parseSearchResults(toolResp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	return &models.DocumentContextResult{
		Chunks:          chunks,
		TotalTokens:     totalTokens,
		Strategy:        models.ContextStrategyMCP,
		RetrievalTimeMs: int(time.Since(startTime).Milliseconds()),
		Metadata: map[string]interface{}{
			"tool_used":     "search_documents",
			"mcp_server":    s.mcpServerURL,
			"execution_ms":  toolResp.ExecutionMs,
		},
	}, nil
}

// parseSearchResults converts MCP search results to RetrievedChunks
func (s *mcpContextServiceImpl) parseSearchResults(result any) ([]models.RetrievedChunk, int, error) {
	// Result should be a map with results array
	resultMap, ok := result.(map[string]any)
	if !ok {
		return nil, 0, fmt.Errorf("unexpected result type: %T", result)
	}

	resultsRaw, ok := resultMap["results"].([]any)
	if !ok {
		// Try to get content directly if it's a different format
		if content, ok := resultMap["content"].(string); ok {
			// Single content result
			chunk := models.RetrievedChunk{
				ID:      "mcp-result-0",
				Content: content,
			}
			tokens := len(content) / 4 // Rough estimate
			return []models.RetrievedChunk{chunk}, tokens, nil
		}
		return nil, 0, nil // Empty results
	}

	chunks := make([]models.RetrievedChunk, 0, len(resultsRaw))
	totalTokens := 0

	for i, r := range resultsRaw {
		item, ok := r.(map[string]any)
		if !ok {
			continue
		}

		chunk := models.RetrievedChunk{
			ID: fmt.Sprintf("mcp-result-%d", i),
		}

		if docID, ok := item["document_id"].(string); ok {
			chunk.DocumentID = docID
		}
		if content, ok := item["content"].(string); ok {
			chunk.Content = content
			totalTokens += len(content) / 4 // Rough token estimate
		}
		if score, ok := item["score"].(float64); ok {
			chunk.Score = score
		}
		if chunkNum, ok := item["chunk_number"].(float64); ok {
			chunk.ChunkNumber = int(chunkNum)
		}
		if metadata, ok := item["metadata"].(map[string]any); ok {
			chunk.Metadata = metadata
		}

		chunks = append(chunks, chunk)
	}

	return chunks, totalTokens, nil
}

// GetDocumentContent retrieves full document content via MCP
func (s *mcpContextServiceImpl) GetDocumentContent(ctx context.Context, documentID string, tenantID string) (*models.DocumentContextResult, error) {
	startTime := time.Now()

	// Invoke the get_document_content MCP tool
	toolResp, err := s.InvokeTool(ctx, models.MCPToolRequest{
		ToolName: "get_document_content",
		Parameters: map[string]any{
			"document_id": documentID,
			"format":      "chunks",
		},
		TenantID: tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to invoke get_document_content tool: %w", err)
	}

	if !toolResp.Success {
		return nil, fmt.Errorf("get_document_content tool failed: %s", toolResp.Error)
	}

	// Parse the content results
	chunks, totalTokens, err := s.parseSearchResults(toolResp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse content results: %w", err)
	}

	return &models.DocumentContextResult{
		Chunks:          chunks,
		TotalTokens:     totalTokens,
		Strategy:        models.ContextStrategyMCP,
		RetrievalTimeMs: int(time.Since(startTime).Milliseconds()),
		Metadata: map[string]interface{}{
			"tool_used":    "get_document_content",
			"document_id":  documentID,
			"mcp_server":   s.mcpServerURL,
			"execution_ms": toolResp.ExecutionMs,
		},
	}, nil
}

// GetDocumentSummary retrieves cached document summary via MCP
func (s *mcpContextServiceImpl) GetDocumentSummary(ctx context.Context, documentID string, tenantID string) (string, error) {
	// Invoke the get_document_summary MCP tool
	toolResp, err := s.InvokeTool(ctx, models.MCPToolRequest{
		ToolName: "get_document_summary",
		Parameters: map[string]any{
			"document_id": documentID,
		},
		TenantID: tenantID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to invoke get_document_summary tool: %w", err)
	}

	if !toolResp.Success {
		return "", fmt.Errorf("get_document_summary tool failed: %s", toolResp.Error)
	}

	// Extract summary from result
	if resultMap, ok := toolResp.Result.(map[string]any); ok {
		if summary, ok := resultMap["summary"].(string); ok {
			return summary, nil
		}
	}

	// Try direct string result
	if summary, ok := toolResp.Result.(string); ok {
		return summary, nil
	}

	return "", fmt.Errorf("unexpected summary result format")
}

// RetrieveMCPContext performs autonomous context retrieval using MCP tools
func (s *mcpContextServiceImpl) RetrieveMCPContext(ctx context.Context, query string, notebookIDs []uuid.UUID, tenantID string, maxTokens int) (*models.MCPContextResult, error) {
	startTime := time.Now()

	result := &models.MCPContextResult{
		DocumentContextResult: &models.DocumentContextResult{
			Chunks:   make([]models.RetrievedChunk, 0),
			Strategy: models.ContextStrategyMCP,
		},
		ToolsUsed:       make([]models.MCPToolInvocation, 0),
		AutonomousSteps: make([]models.MCPAutonomousStep, 0),
	}

	currentTokens := 0
	stepNumber := 0

	// Step 1: Initial semantic search
	stepNumber++
	searchReq := models.MCPSearchRequest{
		Query:       query,
		NotebookIDs: notebookIDs,
		TenantID:    tenantID,
		TopK:        20,
		MinScore:    0.5,
	}

	searchResult, err := s.SearchDocuments(ctx, searchReq)
	chunksAdded := 0
	if err == nil && len(searchResult.Chunks) > 0 {
		// Add chunks that fit within token budget
		for _, chunk := range searchResult.Chunks {
			chunkTokens := len(chunk.Content) / 4
			if currentTokens+chunkTokens <= maxTokens {
				result.Chunks = append(result.Chunks, chunk)
				currentTokens += chunkTokens
				chunksAdded++
			}
		}
	}

	result.AutonomousSteps = append(result.AutonomousSteps, models.MCPAutonomousStep{
		StepNumber:  stepNumber,
		Action:      "search",
		Reasoning:   "Perform initial semantic search to find relevant document chunks",
		ToolUsed:    "search_documents",
		Success:     err == nil,
		ChunksAdded: chunksAdded,
	})

	result.ToolsUsed = append(result.ToolsUsed, models.MCPToolInvocation{
		ToolName:    "search_documents",
		Parameters:  searchReq,
		Success:     err == nil,
		ExecutionMs: searchResult.RetrievalTimeMs,
		ChunksFound: len(searchResult.Chunks),
	})

	// Step 2: If we have room and found relevant documents, get summaries
	if currentTokens < maxTokens*80/100 && len(result.Chunks) > 0 && stepNumber < s.config.MaxAutonomousSteps {
		stepNumber++
		documentsSeen := make(map[string]bool)
		summariesAdded := 0

		for _, chunk := range result.Chunks {
			if chunk.DocumentID != "" && !documentsSeen[chunk.DocumentID] {
				documentsSeen[chunk.DocumentID] = true

				summary, err := s.GetDocumentSummary(ctx, chunk.DocumentID, tenantID)
				if err == nil && summary != "" {
					summaryTokens := len(summary) / 4
					if currentTokens+summaryTokens <= maxTokens {
						// Add summary as a special chunk
						summaryChunk := models.RetrievedChunk{
							ID:           fmt.Sprintf("summary-%s", chunk.DocumentID),
							DocumentID:   chunk.DocumentID,
							Content:      fmt.Sprintf("[Document Summary]\n%s", summary),
							ContentType:  "summary",
							Score:        1.0, // Summaries get high relevance
						}
						result.Chunks = append(result.Chunks, summaryChunk)
						currentTokens += summaryTokens
						summariesAdded++
					}
				}

				// Limit summary fetching
				if summariesAdded >= 3 {
					break
				}
			}
		}

		result.AutonomousSteps = append(result.AutonomousSteps, models.MCPAutonomousStep{
			StepNumber:  stepNumber,
			Action:      "get_summary",
			Reasoning:   "Retrieve document summaries to provide broader context",
			ToolUsed:    "get_document_summary",
			Success:     summariesAdded > 0,
			ChunksAdded: summariesAdded,
		})
	}

	// Update final metadata
	result.TotalTokens = currentTokens
	result.RetrievalTimeMs = int(time.Since(startTime).Milliseconds())
	result.TotalToolCalls = len(result.ToolsUsed)
	result.Metadata = map[string]interface{}{
		"strategy":         "mcp_autonomous",
		"autonomous_steps": len(result.AutonomousSteps),
		"total_tool_calls": result.TotalToolCalls,
		"token_budget":     maxTokens,
		"tokens_used":      currentTokens,
	}

	return result, nil
}
