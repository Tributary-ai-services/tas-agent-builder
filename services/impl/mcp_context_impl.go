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
// Uses plain JSON POST to napkin-mcp's /mcp/tools/call endpoint
func (s *mcpContextServiceImpl) InvokeTool(ctx context.Context, req models.MCPToolRequest) (*models.MCPToolResponse, error) {
	startTime := time.Now()

	// Build plain JSON request (not JSON-RPC)
	mcpRequest := map[string]any{
		"name":      req.ToolName,
		"arguments": req.Parameters,
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

	// Parse napkin-mcp response: {"content":[{"type":"text","text":"..."}], "isError":bool}
	var mcpResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	if err := json.Unmarshal(body, &mcpResp); err != nil {
		return &models.MCPToolResponse{
			ToolName:    req.ToolName,
			Success:     false,
			Error:       fmt.Sprintf("failed to parse MCP response: %v (body: %s)", err, string(body)),
			ExecutionMs: int(time.Since(startTime).Milliseconds()),
		}, nil
	}

	// Extract text content from response
	var resultText string
	for _, c := range mcpResp.Content {
		if c.Type == "text" {
			resultText = c.Text
			break
		}
	}

	if mcpResp.IsError {
		return &models.MCPToolResponse{
			ToolName:    req.ToolName,
			Success:     false,
			Error:       resultText,
			ExecutionMs: int(time.Since(startTime).Milliseconds()),
		}, nil
	}

	// Try to parse the text as JSON for structured results
	var resultObj interface{}
	if err := json.Unmarshal([]byte(resultText), &resultObj); err != nil {
		// Not JSON, use the raw text
		resultObj = resultText
	}

	return &models.MCPToolResponse{
		ToolName:    req.ToolName,
		Success:     true,
		Result:      resultObj,
		ExecutionMs: int(time.Since(startTime).Milliseconds()),
	}, nil
}

// ListAvailableTools lists all available MCP tools
// Uses GET /mcp/tools/list on napkin-mcp's HTTP endpoint
func (s *mcpContextServiceImpl) ListAvailableTools(ctx context.Context) ([]models.MCPToolDefinition, error) {
	// Create GET request (napkin-mcp uses plain HTTP, not JSON-RPC)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", s.mcpServerURL+"/mcp/tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

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

	// Parse response: {"tools": [{name, description, inputSchema}, ...]}
	var mcpResp struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(body, &mcpResp); err != nil {
		return nil, fmt.Errorf("failed to parse MCP response: %w (body: %s)", err, string(body))
	}

	// Convert to our tool definition format
	tools := make([]models.MCPToolDefinition, 0, len(mcpResp.Tools))
	for _, t := range mcpResp.Tools {
		if s.isEnabledTool(t.Name) {
			tools = append(tools, models.MCPToolDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
				Server:      "napkin-mcp",
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

// ListToolsForLLM returns tools in OpenAI function-calling format for LLM requests
func (s *mcpContextServiceImpl) ListToolsForLLM(ctx context.Context) ([]services.ToolDefinition, error) {
	mcpTools, err := s.ListAvailableTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP tools: %w", err)
	}

	tools := make([]services.ToolDefinition, len(mcpTools))
	for i, t := range mcpTools {
		// Use the inputSchema as parameters, or provide a permissive default
		params := interface{}(t.Parameters)
		if params == nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}
		tools[i] = services.ToolDefinition{
			Type: "function",
			Function: services.ToolFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		}
	}

	return tools, nil
}

// ListToolsFromServer lists tools from an arbitrary MCP server URL
// Unlike ListToolsForLLM which uses the configured server, this accepts any server URL
func (s *mcpContextServiceImpl) ListToolsFromServer(ctx context.Context, serverURL string) ([]services.ToolDefinition, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", serverURL+"/mcp/tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request for %s: %w", serverURL, err)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools from %s: %w", serverURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from %s: %w", serverURL, err)
	}

	var mcpResp struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(body, &mcpResp); err != nil {
		return nil, fmt.Errorf("failed to parse response from %s: %w (body: %s)", serverURL, err, string(body))
	}

	tools := make([]services.ToolDefinition, 0, len(mcpResp.Tools))
	for _, t := range mcpResp.Tools {
		params := interface{}(t.InputSchema)
		if params == nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}
		tools = append(tools, services.ToolDefinition{
			Type: "function",
			Function: services.ToolFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}

	return tools, nil
}

// InvokeToolOnServer invokes a tool on an arbitrary MCP server
func (s *mcpContextServiceImpl) InvokeToolOnServer(ctx context.Context, serverURL string, req models.MCPToolRequest) (*models.MCPToolResponse, error) {
	startTime := time.Now()

	mcpRequest := map[string]any{
		"name":      req.ToolName,
		"arguments": req.Parameters,
	}

	reqBody, err := json.Marshal(mcpRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MCP request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/mcp/tools/call", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if req.TenantID != "" {
		httpReq.Header.Set("X-Tenant-ID", req.TenantID)
	}

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &models.MCPToolResponse{
			ToolName:    req.ToolName,
			Success:     false,
			Error:       fmt.Sprintf("failed to read response: %v", err),
			ExecutionMs: int(time.Since(startTime).Milliseconds()),
		}, nil
	}

	var mcpResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	if err := json.Unmarshal(body, &mcpResp); err != nil {
		return &models.MCPToolResponse{
			ToolName:    req.ToolName,
			Success:     false,
			Error:       fmt.Sprintf("failed to parse MCP response: %v (body: %s)", err, string(body)),
			ExecutionMs: int(time.Since(startTime).Milliseconds()),
		}, nil
	}

	var resultText string
	for _, c := range mcpResp.Content {
		if c.Type == "text" {
			resultText = c.Text
			break
		}
	}

	if mcpResp.IsError {
		return &models.MCPToolResponse{
			ToolName:    req.ToolName,
			Success:     false,
			Error:       resultText,
			ExecutionMs: int(time.Since(startTime).Milliseconds()),
		}, nil
	}

	var resultObj interface{}
	if err := json.Unmarshal([]byte(resultText), &resultObj); err != nil {
		resultObj = resultText
	}

	return &models.MCPToolResponse{
		ToolName:    req.ToolName,
		Success:     true,
		Result:      resultObj,
		ExecutionMs: int(time.Since(startTime).Milliseconds()),
	}, nil
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
