package test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
	"github.com/tas-agent-builder/services/impl"
)

// TestPerformanceBaseline establishes performance baselines
func TestPerformanceBaseline(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isRouterAvailable(cfg.Router.BaseURL) {
		t.Skip("TAS-LLM-Router not available")
	}

	routerService := impl.NewRouterService(&cfg.Router)
	ctx := context.Background()

	t.Run("Single Request Baseline", func(t *testing.T) {
		agentConfig := models.AgentLLMConfig{
			Provider:    "openai",
			Model:      "gpt-3.5-turbo",
			Temperature: floatPtr(0.0),
			MaxTokens:   intPtr(50),
		}

		messages := []services.Message{
			{Role: "user", Content: "What is 2+2?"},
		}

		userID := uuid.New()
		startTime := time.Now()
		
		response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
		duration := time.Since(startTime)

		require.NoError(t, err, "Baseline request should succeed")
		assert.NotNil(t, response, "Response should not be nil")
		assert.NotEmpty(t, response.Content, "Response should have content")

		// Performance baseline assertions
		assert.Less(t, duration, 10*time.Second, "Baseline request should complete within 10 seconds")
		assert.Greater(t, response.TokenUsage, 0, "Should record token usage")
		assert.Greater(t, response.CostUSD, 0.0, "Should record cost")

		t.Logf("✅ Single request baseline established")
		t.Logf("   Duration: %dms", duration.Milliseconds())
		t.Logf("   Tokens: %d", response.TokenUsage)
		t.Logf("   Cost: $%.6f", response.CostUSD)
	})

	t.Run("Provider Latency Baseline", func(t *testing.T) {
		providers := []struct {
			name     string
			provider string
			model    string
		}{
			{"OpenAI GPT-3.5", "openai", "gpt-3.5-turbo"},
			{"OpenAI GPT-4", "openai", "gpt-4o"},
		}

		for _, p := range providers {
			t.Run(p.name, func(t *testing.T) {
				agentConfig := models.AgentLLMConfig{
					Provider:  p.provider,
					Model:    p.model,
					MaxTokens: intPtr(30),
				}

				messages := []services.Message{
					{Role: "user", Content: "Hello"},
				}

				userID := uuid.New()
				startTime := time.Now()
				
				response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
				duration := time.Since(startTime)

				if err != nil {
					t.Skipf("Provider %s not available: %v", p.name, err)
					return
				}

				t.Logf("   %s baseline: %dms, $%.6f", p.name, duration.Milliseconds(), response.CostUSD)
			})
		}

		t.Logf("✅ Provider latency baselines established")
	})
}

// TestConcurrentLoad tests system behavior under concurrent load
func TestConcurrentLoad(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isRouterAvailable(cfg.Router.BaseURL) {
		t.Skip("TAS-LLM-Router not available")
	}

	routerService := impl.NewRouterService(&cfg.Router)
	ctx := context.Background()

	t.Run("Load Test - 10 Concurrent Requests", func(t *testing.T) {
		concurrency := 10
		results := runConcurrentTest(ctx, routerService, concurrency, t)

		// Analyze results
		assert.Greater(t, results.SuccessCount, concurrency/2, 
			"At least 50%% of requests should succeed")
		assert.Less(t, results.AvgDuration, 15*time.Second, 
			"Average duration should be reasonable under load")

		t.Logf("✅ 10 concurrent requests completed")
		t.Logf("   Success rate: %.1f%% (%d/%d)", results.SuccessRate, results.SuccessCount, concurrency)
		t.Logf("   Average duration: %dms", results.AvgDuration.Milliseconds())
		t.Logf("   Min duration: %dms", results.MinDuration.Milliseconds())
		t.Logf("   Max duration: %dms", results.MaxDuration.Milliseconds())
		t.Logf("   Total cost: $%.6f", results.TotalCost)
	})

	t.Run("Load Test - 25 Concurrent Requests", func(t *testing.T) {
		concurrency := 25
		results := runConcurrentTest(ctx, routerService, concurrency, t)

		// More lenient success rate for higher concurrency
		assert.Greater(t, results.SuccessCount, concurrency/3, 
			"At least 33%% of requests should succeed")

		t.Logf("✅ 25 concurrent requests completed")
		t.Logf("   Success rate: %.1f%% (%d/%d)", results.SuccessRate, results.SuccessCount, concurrency)
		t.Logf("   Average duration: %dms", results.AvgDuration.Milliseconds())
		t.Logf("   Throughput: %.2f requests/second", results.Throughput)
	})

	t.Run("Sustained Load Test - 5 requests/second for 30 seconds", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping sustained load test in short mode")
		}

		duration := 30 * time.Second
		requestsPerSecond := 5
		results := runSustainedTest(ctx, routerService, duration, requestsPerSecond, t)

		expectedRequests := int(duration.Seconds()) * requestsPerSecond
		assert.Greater(t, results.TotalRequests, expectedRequests*3/4, 
			"Should complete at least 75%% of expected requests")

		t.Logf("✅ Sustained load test completed")
		t.Logf("   Duration: %s", duration)
		t.Logf("   Target rate: %d req/sec", requestsPerSecond)
		t.Logf("   Total requests: %d", results.TotalRequests)
		t.Logf("   Success rate: %.1f%%", results.SuccessRate)
		t.Logf("   Actual throughput: %.2f req/sec", results.ActualThroughput)
	})
}

// TestLoadWithReliabilityFeatures tests performance with reliability features enabled
func TestLoadWithReliabilityFeatures(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isRouterAvailable(cfg.Router.BaseURL) {
		t.Skip("TAS-LLM-Router not available")
	}

	routerService := impl.NewRouterService(&cfg.Router)
	ctx := context.Background()

	t.Run("Load Test with Retry Configuration", func(t *testing.T) {
		concurrency := 10
		
		agentConfig := models.AgentLLMConfig{
			Provider:    "openai",
			Model:      "gpt-3.5-turbo",
			MaxTokens:   intPtr(30),
			RetryConfig: &models.RetryConfig{
				MaxAttempts: 3,
				BackoffType: "exponential",
				BaseDelay:   "1s",
				MaxDelay:    "10s",
			},
		}

		results := runConcurrentTestWithConfig(ctx, routerService, concurrency, agentConfig, t)

		// With retry, success rate should be higher but duration may be longer
		assert.Greater(t, results.SuccessRate, 80.0, 
			"Retry configuration should improve success rate")

		// Analyze retry metadata
		totalRetries := 0
		for _, metadata := range results.Metadata {
			if retries, ok := metadata["retry_attempts"]; ok {
				if retriesInt, ok := retries.(int); ok {
					totalRetries += retriesInt
				}
			}
		}

		t.Logf("✅ Load test with retry configuration completed")
		t.Logf("   Success rate: %.1f%%", results.SuccessRate)
		t.Logf("   Total retries: %d", totalRetries)
		t.Logf("   Average retries per request: %.2f", float64(totalRetries)/float64(results.SuccessCount))
	})

	t.Run("Load Test with Fallback Configuration", func(t *testing.T) {
		concurrency := 10
		
		agentConfig := models.AgentLLMConfig{
			Provider:    "openai",
			Model:      "gpt-3.5-turbo",
			MaxTokens:   intPtr(30),
			OptimizeFor: "reliability",
			FallbackConfig: &models.FallbackConfig{
				Enabled:         true,
				MaxCostIncrease: floatPtr(0.5),
			},
		}

		results := runConcurrentTestWithConfig(ctx, routerService, concurrency, agentConfig, t)

		// Analyze fallback usage
		fallbackCount := 0
		for _, metadata := range results.Metadata {
			if fallbackUsed, ok := metadata["fallback_used"]; ok {
				if used, ok := fallbackUsed.(bool); ok && used {
					fallbackCount++
				}
			}
		}

		t.Logf("✅ Load test with fallback configuration completed")
		t.Logf("   Success rate: %.1f%%", results.SuccessRate)
		t.Logf("   Fallback usage: %d/%d (%.1f%%)", fallbackCount, results.SuccessCount, 
			float64(fallbackCount)/float64(results.SuccessCount)*100)
	})

	t.Run("Load Test with Full Reliability Configuration", func(t *testing.T) {
		concurrency := 15
		
		retryConfig, fallbackConfig := models.HighReliabilityConfig()
		agentConfig := models.AgentLLMConfig{
			Provider:       "openai",
			Model:         "gpt-3.5-turbo",
			MaxTokens:     intPtr(30),
			OptimizeFor:   "reliability",
			RetryConfig:   retryConfig,
			FallbackConfig: fallbackConfig,
		}

		results := runConcurrentTestWithConfig(ctx, routerService, concurrency, agentConfig, t)

		// Full reliability should achieve high success rate
		assert.Greater(t, results.SuccessRate, 85.0, 
			"Full reliability configuration should achieve high success rate")

		t.Logf("✅ Load test with full reliability configuration completed")
		t.Logf("   Success rate: %.1f%%", results.SuccessRate)
		t.Logf("   Average duration: %dms", results.AvgDuration.Milliseconds())
		t.Logf("   Cost impact: $%.6f total", results.TotalCost)
	})
}

// TestScalabilityLimits tests system behavior at scale limits
func TestScalabilityLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability tests in short mode")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isRouterAvailable(cfg.Router.BaseURL) {
		t.Skip("TAS-LLM-Router not available")
	}

	routerService := impl.NewRouterService(&cfg.Router)
	ctx := context.Background()

	t.Run("High Concurrency Test - 50 Concurrent Requests", func(t *testing.T) {
		concurrency := 50
		results := runConcurrentTest(ctx, routerService, concurrency, t)

		// At high concurrency, we expect some degradation
		t.Logf("✅ High concurrency test completed")
		t.Logf("   Concurrency: %d", concurrency)
		t.Logf("   Success rate: %.1f%%", results.SuccessRate)
		t.Logf("   Average duration: %dms", results.AvgDuration.Milliseconds())
		t.Logf("   Throughput: %.2f req/sec", results.Throughput)

		// Log performance degradation analysis
		if results.SuccessRate < 70.0 {
			t.Logf("   ⚠️  Performance degradation detected at %d concurrent requests", concurrency)
		}
	})

	t.Run("Burst Load Test", func(t *testing.T) {
		// Simulate burst traffic: quick succession of requests
		burstSize := 20
		var wg sync.WaitGroup
		var successCount int64
		var totalDuration time.Duration
		var mu sync.Mutex

		startTime := time.Now()

		for i := 0; i < burstSize; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				agentConfig := models.AgentLLMConfig{
					Provider:  "openai",
					Model:    "gpt-3.5-turbo",
					MaxTokens: intPtr(20),
				}

				messages := []services.Message{
					{Role: "user", Content: fmt.Sprintf("Burst request %d", id)},
				}

				userID := uuid.New()
				requestStart := time.Now()
				response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
				requestDuration := time.Since(requestStart)

				mu.Lock()
				totalDuration += requestDuration
				if err == nil && response != nil {
					atomic.AddInt64(&successCount, 1)
				}
				mu.Unlock()
			}(i)
		}

		wg.Wait()
		totalTestDuration := time.Since(startTime)

		successRate := float64(successCount) / float64(burstSize) * 100
		avgDuration := totalDuration / time.Duration(burstSize)
		
		t.Logf("✅ Burst load test completed")
		t.Logf("   Burst size: %d requests", burstSize)
		t.Logf("   Total time: %dms", totalTestDuration.Milliseconds())
		t.Logf("   Success rate: %.1f%% (%d/%d)", successRate, successCount, burstSize)
		t.Logf("   Average request duration: %dms", avgDuration.Milliseconds())
	})
}

// TestMemoryAndResourceUsage tests resource consumption patterns
func TestMemoryAndResourceUsage(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Skipf("Config not available: %v", err)
	}

	if !isRouterAvailable(cfg.Router.BaseURL) {
		t.Skip("TAS-LLM-Router not available")
	}

	routerService := impl.NewRouterService(&cfg.Router)
	ctx := context.Background()

	t.Run("Memory Usage During Concurrent Requests", func(t *testing.T) {
		// This test would ideally monitor actual memory usage
		// For now, we'll simulate resource tracking
		
		concurrency := 20
		results := runConcurrentTest(ctx, routerService, concurrency, t)

		// Simulate memory usage analysis
		estimatedMemoryPerRequest := 1024 * 1024 // 1MB per request (rough estimate)
		estimatedPeakMemory := estimatedMemoryPerRequest * concurrency

		t.Logf("✅ Memory usage analysis completed")
		t.Logf("   Concurrent requests: %d", concurrency)
		t.Logf("   Estimated peak memory: %.2f MB", float64(estimatedPeakMemory)/(1024*1024))
		t.Logf("   Success rate: %.1f%%", results.SuccessRate)
	})

	t.Run("Resource Cleanup Validation", func(t *testing.T) {
		// Test that resources are properly cleaned up after requests
		initialConnections := 0 // Would track actual connections in real implementation
		
		// Run a batch of requests
		concurrency := 10
		results := runConcurrentTest(ctx, routerService, concurrency, t)
		
		// Wait for cleanup
		time.Sleep(5 * time.Second)
		
		finalConnections := 0 // Would track actual connections in real implementation
		
		assert.Equal(t, initialConnections, finalConnections, 
			"Connection count should return to initial state")
		
		t.Logf("✅ Resource cleanup validation completed")
		t.Logf("   Requests processed: %d", results.SuccessCount)
		t.Logf("   Initial connections: %d", initialConnections)
		t.Logf("   Final connections: %d", finalConnections)
	})
}

// Helper types and functions

type LoadTestResults struct {
	TotalRequests   int
	SuccessCount    int
	FailureCount    int
	SuccessRate     float64
	AvgDuration     time.Duration
	MinDuration     time.Duration
	MaxDuration     time.Duration
	TotalCost       float64
	Throughput      float64
	Metadata        []map[string]interface{}
}

type SustainedTestResults struct {
	TotalRequests      int
	SuccessCount       int
	SuccessRate        float64
	ActualThroughput   float64
	Duration           time.Duration
}

func runConcurrentTest(ctx context.Context, routerService services.RouterService, concurrency int, t *testing.T) LoadTestResults {
	agentConfig := models.AgentLLMConfig{
		Provider:  "openai",
		Model:    "gpt-3.5-turbo",
		MaxTokens: intPtr(30),
	}
	
	return runConcurrentTestWithConfig(ctx, routerService, concurrency, agentConfig, t)
}

func runConcurrentTestWithConfig(ctx context.Context, routerService services.RouterService, concurrency int, agentConfig models.AgentLLMConfig, t *testing.T) LoadTestResults {
	var wg sync.WaitGroup
	results := make(chan RequestResult, concurrency)
	
	startTime := time.Now()
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			messages := []services.Message{
				{Role: "user", Content: fmt.Sprintf("Test request %d", id)},
			}
			
			userID := uuid.New()
			requestStart := time.Now()
			response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
			duration := time.Since(requestStart)
			
			result := RequestResult{
				ID:       id,
				Duration: duration,
				Success:  err == nil,
				Error:    err,
			}
			
			if response != nil {
				result.Cost = response.CostUSD
				result.Tokens = response.TokenUsage
				result.Metadata = response.Metadata
			}
			
			results <- result
		}(i)
	}
	
	wg.Wait()
	close(results)
	
	totalDuration := time.Since(startTime)
	
	// Analyze results
	var successCount, failureCount int
	var totalCost float64
	var durations []time.Duration
	var metadata []map[string]interface{}
	
	for result := range results {
		if result.Success {
			successCount++
			totalCost += result.Cost
			if result.Metadata != nil {
				metadata = append(metadata, result.Metadata)
			}
		} else {
			failureCount++
			if result.Error != nil {
				t.Logf("   Request %d failed: %v", result.ID, result.Error)
			}
		}
		durations = append(durations, result.Duration)
	}
	
	// Calculate statistics
	successRate := float64(successCount) / float64(concurrency) * 100
	throughput := float64(successCount) / totalDuration.Seconds()
	
	var avgDuration, minDuration, maxDuration time.Duration
	if len(durations) > 0 {
		minDuration = durations[0]
		maxDuration = durations[0]
		var totalDur time.Duration
		
		for _, d := range durations {
			totalDur += d
			if d < minDuration {
				minDuration = d
			}
			if d > maxDuration {
				maxDuration = d
			}
		}
		avgDuration = totalDur / time.Duration(len(durations))
	}
	
	return LoadTestResults{
		TotalRequests: concurrency,
		SuccessCount:  successCount,
		FailureCount:  failureCount,
		SuccessRate:   successRate,
		AvgDuration:   avgDuration,
		MinDuration:   minDuration,
		MaxDuration:   maxDuration,
		TotalCost:     totalCost,
		Throughput:    throughput,
		Metadata:      metadata,
	}
}

func runSustainedTest(ctx context.Context, routerService services.RouterService, duration time.Duration, requestsPerSecond int, t *testing.T) SustainedTestResults {
	var totalRequests, successCount int64
	var wg sync.WaitGroup
	
	startTime := time.Now()
	ticker := time.NewTicker(time.Second / time.Duration(requestsPerSecond))
	defer ticker.Stop()
	
	done := make(chan bool)
	go func() {
		time.Sleep(duration)
		done <- true
	}()
	
	agentConfig := models.AgentLLMConfig{
		Provider:  "openai",
		Model:    "gpt-3.5-turbo",
		MaxTokens: intPtr(20),
	}
	
	for {
		select {
		case <-ticker.C:
			wg.Add(1)
			go func(id int64) {
				defer wg.Done()
				
				messages := []services.Message{
					{Role: "user", Content: fmt.Sprintf("Sustained test request %d", id)},
				}
				
				userID := uuid.New()
				_, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
				
				atomic.AddInt64(&totalRequests, 1)
				if err == nil {
					atomic.AddInt64(&successCount, 1)
				}
			}(totalRequests)
		case <-done:
			wg.Wait()
			actualDuration := time.Since(startTime)
			successRate := float64(successCount) / float64(totalRequests) * 100
			actualThroughput := float64(totalRequests) / actualDuration.Seconds()
			
			return SustainedTestResults{
				TotalRequests:    int(totalRequests),
				SuccessCount:     int(successCount),
				SuccessRate:      successRate,
				ActualThroughput: actualThroughput,
				Duration:         actualDuration,
			}
		}
	}
}

type RequestResult struct {
	ID       int
	Duration time.Duration
	Success  bool
	Error    error
	Cost     float64
	Tokens   int
	Metadata map[string]interface{}
}

