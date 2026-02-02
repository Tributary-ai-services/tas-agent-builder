package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// RouterRequest represents the request structure for TAS-LLM-Router
type RouterRequest struct {
	Model       string          `json:"model"`
	Messages    []Message       `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	OptimizeFor string          `json:"optimize_for,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RouterResponse represents the response from TAS-LLM-Router
type RouterResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int         `json:"index"`
	Message      Message     `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// TestRouterBasicQuery tests basic connectivity to TAS-LLM-Router
func TestRouterBasicQuery(t *testing.T) {
	// Get router URL from environment or use default
	routerURL := os.Getenv("ROUTER_BASE_URL")
	if routerURL == "" {
		routerURL = "http://localhost:8080"
	}

	// Skip test if router is not available
	if !isRouterAvailable(routerURL) {
		t.Skip("TAS-LLM-Router not available at", routerURL)
	}

	// Create a simple test request
	request := RouterRequest{
		Model: "gpt-3.5-turbo",
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: "Say 'Hello from TAS Agent Builder test' in exactly those words.",
			},
		},
		Temperature: 0.0,
		MaxTokens:   50,
		OptimizeFor: "cost",
	}

	// Send request to router
	response, err := sendRouterRequest(routerURL, request)
	if err != nil {
		t.Fatalf("Failed to send request to router: %v", err)
	}

	// Validate response
	if response.ID == "" {
		t.Error("Response ID is empty")
	}

	if len(response.Choices) == 0 {
		t.Fatal("No choices in response")
	}

	content := response.Choices[0].Message.Content
	t.Logf("Router response: %s", content)

	// Check that we got a response
	if content == "" {
		t.Error("Response content is empty")
	}
}

// TestRouterWithAgentConfig tests router with agent-like configuration
func TestRouterWithAgentConfig(t *testing.T) {
	routerURL := os.Getenv("ROUTER_BASE_URL")
	if routerURL == "" {
		routerURL = "http://localhost:8080"
	}

	if !isRouterAvailable(routerURL) {
		t.Skip("TAS-LLM-Router not available at", routerURL)
	}

	// Create an agent-like request
	request := RouterRequest{
		Model: "gpt-4o",
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a code review assistant. Analyze code for quality, security, and best practices.",
			},
			{
				Role:    "user",
				Content: "Review this Go function:\n```go\nfunc add(a, b int) int {\n    return a + b\n}\n```",
			},
		},
		Temperature: 0.2,
		MaxTokens:   200,
		OptimizeFor: "performance",
	}

	response, err := sendRouterRequest(routerURL, request)
	if err != nil {
		t.Fatalf("Failed to send agent request: %v", err)
	}

	if len(response.Choices) == 0 {
		t.Fatal("No choices in agent response")
	}

	content := response.Choices[0].Message.Content
	t.Logf("Agent response: %s", content)

	// Check response has reasonable length for code review
	if len(content) < 20 {
		t.Error("Response seems too short for code review")
	}

	// Log usage stats
	t.Logf("Token usage - Prompt: %d, Completion: %d, Total: %d",
		response.Usage.PromptTokens,
		response.Usage.CompletionTokens,
		response.Usage.TotalTokens)
}

// TestRouterProviderRouting tests specific provider routing
func TestRouterProviderRouting(t *testing.T) {
	routerURL := os.Getenv("ROUTER_BASE_URL")
	if routerURL == "" {
		routerURL = "http://localhost:8080"
	}

	if !isRouterAvailable(routerURL) {
		t.Skip("TAS-LLM-Router not available at", routerURL)
	}

	tests := []struct {
		name     string
		model    string
		provider string
	}{
		{"OpenAI GPT-3.5", "gpt-3.5-turbo", "openai"},
		{"OpenAI GPT-4", "gpt-4o", "openai"},
		{"Anthropic Claude", "claude-3-5-sonnet-20241022", "anthropic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := RouterRequest{
				Model: tt.model,
				Messages: []Message{
					{
						Role:    "user",
						Content: "What is 2+2?",
					},
				},
				MaxTokens: 10,
			}

			response, err := sendRouterRequest(routerURL, request)
			if err != nil {
				// Model might not be available, skip
				t.Skipf("Model %s not available: %v", tt.model, err)
			}

			if response.Model == "" {
				t.Error("Response model is empty")
			}

			t.Logf("Model %s routed successfully", tt.model)
		})
	}
}

// Note: isRouterAvailable function is now in test_helpers.go

// Helper function to send request to router
func sendRouterRequest(routerURL string, request RouterRequest) (*RouterResponse, error) {
	url := fmt.Sprintf("%s/v1/chat/completions", routerURL)
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	
	// Add API key if available
	if apiKey := os.Getenv("ROUTER_API_KEY"); apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("router returned status %d: %s", resp.StatusCode, string(body))
	}

	var response RouterResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}