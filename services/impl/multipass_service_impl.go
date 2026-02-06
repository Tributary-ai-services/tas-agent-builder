package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
)

// MultiPassService handles multi-pass document processing for large documents
type MultiPassService struct {
	routerService    services.RouterService
	documentService  services.DocumentContextService
}

// NewMultiPassService creates a new MultiPassService instance
func NewMultiPassService(routerSvc services.RouterService, docSvc services.DocumentContextService) *MultiPassService {
	return &MultiPassService{
		routerService:   routerSvc,
		documentService: docSvc,
	}
}

// ExecuteMultiPass processes large documents in multiple passes
func (s *MultiPassService) ExecuteMultiPass(
	ctx context.Context,
	agent *models.Agent,
	documents *models.DocumentContextResult,
	userInput string,
	userID uuid.UUID,
) (*models.MultiPassResult, error) {
	startTime := time.Now()

	config := s.getMultiPassConfig(agent)
	if !config.Enabled {
		return nil, fmt.Errorf("multi-pass execution is not enabled for this agent")
	}

	// Segment documents into chunks that fit the context window
	segments := s.segmentDocuments(documents, config.SegmentSize, config.OverlapTokens)

	if len(segments) == 0 {
		return nil, fmt.Errorf("no document segments to process")
	}

	// Limit number of passes
	if len(segments) > config.MaxPasses {
		segments = segments[:config.MaxPasses]
	}

	// Process each segment
	results := make([]models.SegmentResult, len(segments))
	totalTokens := 0

	for i, segment := range segments {
		segmentStart := time.Now()

		// Build extraction prompt for this segment
		extractionPrompt := s.buildExtractionPrompt(agent, segment, userInput, i+1, len(segments))

		messages := []services.Message{
			{
				Role:    "system",
				Content: s.buildSegmentSystemPrompt(agent),
			},
			{
				Role:    "user",
				Content: extractionPrompt,
			},
		}

		// Call LLM for this segment
		response, err := s.routerService.SendRequest(ctx, agent.LLMConfig, messages, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to process segment %d: %w", i+1, err)
		}

		results[i] = models.SegmentResult{
			SegmentNumber:    i + 1,
			Content:          segment.FormattedContext,
			PartialResult:    response.Content,
			TokensUsed:       response.TokenUsage,
			ProcessingTimeMs: int(time.Since(segmentStart).Milliseconds()),
		}

		totalTokens += response.TokenUsage
	}

	// Aggregate results if we have multiple segments
	var aggregatedResult string
	if len(results) > 1 {
		aggregatedResult, err := s.aggregateResults(ctx, agent, results, userInput, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to aggregate results: %w", err)
		}
		return &models.MultiPassResult{
			Segments:         results,
			AggregatedResult: aggregatedResult,
			TotalPasses:      len(segments),
			TotalTokens:      totalTokens,
			ProcessingTimeMs: int(time.Since(startTime).Milliseconds()),
		}, nil
	}

	// Single segment, use its result directly
	aggregatedResult = results[0].PartialResult

	return &models.MultiPassResult{
		Segments:         results,
		AggregatedResult: aggregatedResult,
		TotalPasses:      len(segments),
		TotalTokens:      totalTokens,
		ProcessingTimeMs: int(time.Since(startTime).Milliseconds()),
	}, nil
}

// getMultiPassConfig returns the multi-pass configuration for an agent
func (s *MultiPassService) getMultiPassConfig(agent *models.Agent) *models.MultiPassConfig {
	if agent.DocumentContext != nil && agent.DocumentContext.MultiPass != nil {
		return agent.DocumentContext.MultiPass
	}

	// Default configuration
	return &models.MultiPassConfig{
		Enabled:       false,
		SegmentSize:   8000,
		OverlapTokens: 500,
		MaxPasses:     10,
	}
}

// segmentDocuments splits document chunks into segments that fit the context window
func (s *MultiPassService) segmentDocuments(
	documents *models.DocumentContextResult,
	segmentSize int,
	overlapTokens int,
) []*models.ContextInjectionResult {
	if documents == nil || len(documents.Chunks) == 0 {
		return nil
	}

	var segments []*models.ContextInjectionResult
	var currentChunks []models.RetrievedChunk
	currentTokens := 0

	for _, chunk := range documents.Chunks {
		chunkTokens := s.documentService.EstimateTokenCount(chunk.Content)

		// Check if adding this chunk would exceed segment size
		if currentTokens+chunkTokens > segmentSize && len(currentChunks) > 0 {
			// Create segment from current chunks
			segment := s.formatSegment(currentChunks, currentTokens)
			segments = append(segments, segment)

			// Start new segment with overlap
			overlapChunks := s.getOverlapChunks(currentChunks, overlapTokens)
			currentChunks = overlapChunks
			currentTokens = 0
			for _, c := range overlapChunks {
				currentTokens += s.documentService.EstimateTokenCount(c.Content)
			}
		}

		currentChunks = append(currentChunks, chunk)
		currentTokens += chunkTokens
	}

	// Add final segment if there are remaining chunks
	if len(currentChunks) > 0 {
		segment := s.formatSegment(currentChunks, currentTokens)
		segments = append(segments, segment)
	}

	return segments
}

// formatSegment formats a list of chunks into a context injection result
func (s *MultiPassService) formatSegment(chunks []models.RetrievedChunk, totalTokens int) *models.ContextInjectionResult {
	var builder strings.Builder
	documentSet := make(map[string]bool)
	currentDocID := ""

	builder.WriteString("\n--- DOCUMENT SEGMENT ---\n\n")

	for _, chunk := range chunks {
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

		builder.WriteString(chunk.Content)
		builder.WriteString("\n")
	}

	builder.WriteString("\n--- END SEGMENT ---\n")

	return &models.ContextInjectionResult{
		FormattedContext: builder.String(),
		ChunkCount:       len(chunks),
		DocumentCount:    len(documentSet),
		TotalTokens:      totalTokens,
		Strategy:         models.ContextStrategyFull,
		Truncated:        false,
	}
}

// getOverlapChunks returns chunks from the end of the list to provide context overlap
func (s *MultiPassService) getOverlapChunks(chunks []models.RetrievedChunk, overlapTokens int) []models.RetrievedChunk {
	if len(chunks) == 0 || overlapTokens <= 0 {
		return nil
	}

	var overlapChunks []models.RetrievedChunk
	tokens := 0

	// Work backwards from the end
	for i := len(chunks) - 1; i >= 0 && tokens < overlapTokens; i-- {
		chunkTokens := s.documentService.EstimateTokenCount(chunks[i].Content)
		overlapChunks = append([]models.RetrievedChunk{chunks[i]}, overlapChunks...)
		tokens += chunkTokens
	}

	return overlapChunks
}

// buildSegmentSystemPrompt builds the system prompt for segment processing
func (s *MultiPassService) buildSegmentSystemPrompt(agent *models.Agent) string {
	basePrompt := "You are a document analysis assistant. Your task is to extract relevant information from the provided document segment."

	if agent.SystemPrompt != "" {
		basePrompt = agent.SystemPrompt + "\n\nFor this segment analysis task: " + basePrompt
	}

	return basePrompt
}

// buildExtractionPrompt builds the user prompt for extracting information from a segment
func (s *MultiPassService) buildExtractionPrompt(
	agent *models.Agent,
	segment *models.ContextInjectionResult,
	userInput string,
	segmentNum int,
	totalSegments int,
) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("This is segment %d of %d from the document(s).\n\n", segmentNum, totalSegments))
	builder.WriteString("DOCUMENT CONTENT:\n")
	builder.WriteString(segment.FormattedContext)
	builder.WriteString("\n\nUSER QUESTION/TASK:\n")
	builder.WriteString(userInput)
	builder.WriteString("\n\nINSTRUCTIONS:\n")
	builder.WriteString("1. Analyze the document content in this segment.\n")
	builder.WriteString("2. Extract any information relevant to the user's question/task.\n")
	builder.WriteString("3. If this segment contains relevant information, provide a detailed response.\n")
	builder.WriteString("4. If this segment does not contain relevant information, indicate that briefly.\n")
	builder.WriteString("5. Note any partial information that might need context from other segments.\n")

	return builder.String()
}

// aggregateResults combines partial results from multiple segments into a final response
func (s *MultiPassService) aggregateResults(
	ctx context.Context,
	agent *models.Agent,
	results []models.SegmentResult,
	userInput string,
	userID uuid.UUID,
) (string, error) {
	// Build aggregation prompt
	var builder strings.Builder

	builder.WriteString("You are synthesizing information from multiple document segments to answer the user's question.\n\n")
	builder.WriteString("USER QUESTION/TASK:\n")
	builder.WriteString(userInput)
	builder.WriteString("\n\nPARTIAL RESULTS FROM DOCUMENT SEGMENTS:\n\n")

	for i, result := range results {
		builder.WriteString(fmt.Sprintf("--- Segment %d Result ---\n", i+1))
		builder.WriteString(result.PartialResult)
		builder.WriteString("\n\n")
	}

	builder.WriteString("INSTRUCTIONS:\n")
	builder.WriteString("1. Synthesize the information from all segment results.\n")
	builder.WriteString("2. Provide a comprehensive, well-organized response to the user's question.\n")
	builder.WriteString("3. Remove any redundancy or duplicate information.\n")
	builder.WriteString("4. If there are conflicting pieces of information, note them.\n")
	builder.WriteString("5. Ensure the response is complete and addresses the user's original question/task.\n")

	// Use custom aggregation prompt if provided
	aggregationPrompt := builder.String()
	if agent.DocumentContext != nil && agent.DocumentContext.MultiPass != nil && agent.DocumentContext.MultiPass.AggregationPrompt != "" {
		aggregationPrompt = agent.DocumentContext.MultiPass.AggregationPrompt + "\n\n" + builder.String()
	}

	messages := []services.Message{
		{
			Role:    "system",
			Content: "You are an expert at synthesizing and summarizing information from multiple sources.",
		},
		{
			Role:    "user",
			Content: aggregationPrompt,
		},
	}

	response, err := s.routerService.SendRequest(ctx, agent.LLMConfig, messages, userID)
	if err != nil {
		return "", fmt.Errorf("aggregation request failed: %w", err)
	}

	return response.Content, nil
}
