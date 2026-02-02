package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RouterProxyHandler proxies requests to the TAS Router service
type RouterProxyHandler struct {
	routerBaseURL string
}

// NewRouterProxyHandler creates a new router proxy handler
func NewRouterProxyHandler(routerBaseURL string) *RouterProxyHandler {
	return &RouterProxyHandler{
		routerBaseURL: strings.TrimSuffix(routerBaseURL, "/"),
	}
}

// ProxyToRouter forwards requests to the router service
func (h *RouterProxyHandler) ProxyToRouter(c *gin.Context) {
	// Get the path after /api/v1/router/
	path := c.Param("path")
	
	// Construct the target URL
	targetURL := fmt.Sprintf("%s/%s", h.routerBaseURL, path)
	
	// Add query parameters if any
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}
	
	// Create a new request
	req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create proxy request"})
		return
	}
	
	// Copy headers
	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	
	// Remove hop-by-hop headers
	req.Header.Del("Connection")
	req.Header.Del("Keep-Alive")
	req.Header.Del("Proxy-Authenticate")
	req.Header.Del("Proxy-Authorization")
	req.Header.Del("TE")
	req.Header.Del("Trailers")
	req.Header.Del("Transfer-Encoding")
	req.Header.Del("Upgrade")
	
	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to reach router service"})
		return
	}
	defer resp.Body.Close()
	
	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	
	// Set the status code
	c.Status(resp.StatusCode)
	
	// Copy the response body
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		// Log error but don't write response as headers are already sent
		fmt.Printf("Error copying response body: %v\n", err)
	}
}

// GetProviders returns available LLM providers from the router
func (h *RouterProxyHandler) GetProviders(c *gin.Context) {
	// For now, return a static list if router is not available
	// This allows the frontend to work even without the router running
	providers := []map[string]interface{}{
		{
			"id":          "openai",
			"name":        "OpenAI",
			"status":      "available",
			"models":      []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"},
			"features":    []string{"chat", "functions", "vision"},
			"maxTokens":   128000,
			"costPerMillion": 5.0,
		},
		{
			"id":          "anthropic",
			"name":        "Anthropic",
			"status":      "available",
			"models":      []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"},
			"features":    []string{"chat", "vision"},
			"maxTokens":   200000,
			"costPerMillion": 15.0,
		},
		{
			"id":          "google",
			"name":        "Google",
			"status":      "available",
			"models":      []string{"gemini-pro", "gemini-pro-vision"},
			"features":    []string{"chat", "vision"},
			"maxTokens":   32768,
			"costPerMillion": 0.5,
		},
	}
	
	// Try to get real data from router
	targetURL := fmt.Sprintf("%s/providers", h.routerBaseURL)
	resp, err := http.Get(targetURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		// Forward the real response
		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
		return
	}
	
	// Return static data if router is not available
	c.JSON(http.StatusOK, gin.H{
		"providers": providers,
		"total":     len(providers),
	})
}

// GetProviderModels returns models for a specific provider
func (h *RouterProxyHandler) GetProviderModels(c *gin.Context) {
	provider := c.Param("provider")
	
	// Static model data
	modelsMap := map[string][]map[string]interface{}{
		"openai": {
			{"id": "gpt-4", "name": "GPT-4", "context": 8192, "costPerMillion": 30.0},
			{"id": "gpt-4-turbo", "name": "GPT-4 Turbo", "context": 128000, "costPerMillion": 10.0},
			{"id": "gpt-3.5-turbo", "name": "GPT-3.5 Turbo", "context": 16385, "costPerMillion": 0.5},
		},
		"anthropic": {
			{"id": "claude-3-opus", "name": "Claude 3 Opus", "context": 200000, "costPerMillion": 15.0},
			{"id": "claude-3-sonnet", "name": "Claude 3 Sonnet", "context": 200000, "costPerMillion": 3.0},
			{"id": "claude-3-haiku", "name": "Claude 3 Haiku", "context": 200000, "costPerMillion": 0.25},
		},
		"google": {
			{"id": "gemini-pro", "name": "Gemini Pro", "context": 32768, "costPerMillion": 0.5},
			{"id": "gemini-pro-vision", "name": "Gemini Pro Vision", "context": 32768, "costPerMillion": 0.5},
		},
	}
	
	models, exists := modelsMap[provider]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"models": models,
		"total":  len(models),
	})
}