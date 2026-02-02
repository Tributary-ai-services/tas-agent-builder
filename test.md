# TAS-LLM-Router Integration Testing

This document provides complete instructions for testing the TAS Agent Builder integration with TAS-LLM-Router.

## ✅ **Router Integration Complete**

### **1. Integration Test Suite** (`test/router_integration_test.go`)
- **Basic connectivity test** - Verifies router is available and responds
- **Agent configuration test** - Tests realistic agent scenarios (code review)  
- **Provider routing test** - Tests OpenAI and Anthropic model routing
- **Error handling** - Graceful handling when router unavailable

### **2. Router Client Service** (`services/impl/router_service_impl.go`)
- **Full HTTP client** with retry logic and exponential backoff
- **Request/response mapping** between agent models and router API
- **Provider validation** - Check available providers and models
- **Cost calculation** - Basic cost estimation for different models
- **Metadata handling** - Support for routing preferences and optimization

### **3. Example Application** (`examples/router_example.go`)
- **4-step integration demo**:
  1. Test router connectivity
  2. Validate agent configurations
  3. Send simple queries
  4. Execute agent-like interactions
- **Real usage patterns** - Shows how agents will interact with router

### **4. Development Tools**
- **Makefile** - Easy commands for testing and development
- **Documentation** - Complete setup and usage guide
- **Environment configuration** - Proper shared database integration

## **Testing Instructions**

### **Prerequisites**
1. **TAS-LLM-Router must be running** on `http://localhost:8080` with API keys already configured
2. **Go 1.21+** installed for running tests

> **Note**: API keys should be configured in TAS-LLM-Router, not in the test environment. The router handles all provider authentication.

### **Quick Test Commands**

```bash
# Check if router is running
make check-router

# Run integration tests  
make test-router

# Run full example application
make example-router

# NEW: Test both OpenAI and Anthropic providers
make test-providers

# Quick provider validation
make test-providers-quick
```

### **Detailed Testing Steps**

#### **Step 1: Verify Router Availability**
```bash
# Check router health endpoint
curl http://localhost:8080/health

# Expected response: HTTP 200 OK
```

#### **Step 2: Run Basic Integration Tests**
```bash
# Run all router integration tests
ROUTER_BASE_URL=http://localhost:8080 go test -v ./test -run TestRouter

# Individual test cases:
# - TestRouterBasicQuery: Simple "hello world" test
# - TestRouterWithAgentConfig: Code review agent scenario  
# - TestRouterProviderRouting: Test different models (GPT, Claude)
```

#### **Step 3: Run Example Application**
```bash
# Run comprehensive example with detailed output
ROUTER_BASE_URL=http://localhost:8080 go run examples/router_example.go

# Expected output:
# ✅ Connected to router at http://localhost:8080
# 📋 Available providers: 2
#    - OpenAI (openai) - 3 models
#    - Anthropic (anthropic) - 2 models
# ✅ Agent configuration is valid
# ✅ Query successful!
#    Response: Hello from TAS Agent Builder test
#    Provider: openai
#    Cost: $0.000150
```

#### **Step 4: Test Different Scenarios**

**Test OpenAI Models:**
```bash
# Test GPT-3.5 Turbo
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello!"}],
    "optimize_for": "cost"
  }'

# Test GPT-4
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o", 
    "messages": [{"role": "user", "content": "Hello!"}],
    "optimize_for": "performance"
  }'
```

**Test Anthropic Models:**
```bash
# Test Claude
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## **Expected Test Results**

### **Successful Integration Test Output**
```
=== RUN   TestRouterBasicQuery
    router_integration_test.go:89: Router response: Hello from TAS Agent Builder test
    router_integration_test.go:101: Token usage - Prompt: 25, Completion: 8, Total: 33
--- PASS: TestRouterBasicQuery (2.34s)

=== RUN   TestRouterWithAgentConfig  
    router_integration_test.go:134: Agent response: This Go function is simple and correct...
    router_integration_test.go:143: Token usage - Prompt: 45, Completion: 67, Total: 112
--- PASS: TestRouterWithAgentConfig (3.21s)

=== RUN   TestRouterProviderRouting
=== RUN   TestRouterProviderRouting/OpenAI_GPT-3.5
    router_integration_test.go:178: Model gpt-3.5-turbo routed successfully
--- PASS: TestRouterProviderRouting/OpenAI_GPT-3.5 (1.45s)
```

### **Example Application Success Output**
```
TAS Agent Builder - Router Integration Example
==============================================

1. Testing Router Connectivity...
✅ Connected to router at http://localhost:8080
📋 Available providers: 2
   - OpenAI (openai) - 3 models
   - Anthropic (anthropic) - 2 models

2. Testing Agent Configuration...
✅ Agent configuration is valid
   Provider: openai, Model: gpt-3.5-turbo

3. Testing Simple Query...
✅ Query successful!
   Response: Hello and confirm you're working with TAS Agent Builder. I'm functioning properly and ready to assist with TAS Agent Builder testing!
   Provider: openai
   Model: gpt-3.5-turbo
   Tokens: 42
   Cost: $0.000042
   Response Time: 1456ms

4. Testing Agent-like Interaction...  
✅ Agent query successful!
   Response: ## Code Review Analysis

**Security Issue - SQL Injection Vulnerability:**
The function concatenates user input directly into SQL query, creating a critical SQL injection vulnerability.

**Recommended Fix:**
```go
func ProcessUser(db *sql.DB, userID string) error {
    query := "SELECT * FROM users WHERE id = $1"
    rows, err := db.Query(query, userID)
    // ... rest of function
}
```

**Other Improvements:**
- Add input validation for userID
- Handle the retrieved data (currently unused)
- Consider using prepared statements for better performance

   Provider: openai
   Routing: performance  
   Tokens: 156
   Cost: $0.004680

✅ Router integration test completed successfully!

Next Steps:
- Integrate this router service into agent execution
- Add conversation memory and context  
- Implement knowledge retrieval integration
- Add streaming support for real-time responses
```

## **Troubleshooting**

### **Router Not Available**
```bash
# Check if router is running
curl http://localhost:8080/health

# If not running, start the router:
cd /path/to/tas-llm-router
go run cmd/llm-router/main.go
```

### **API Key Issues**
```bash
# Check router logs for authentication errors
# Ensure OPENAI_API_KEY and ANTHROPIC_API_KEY are set in router environment
```

### **Test Failures**
```bash
# Run tests with verbose output
go test -v ./test -run TestRouter

# Check specific error messages
# Common issues: network timeouts, model not available, quota exceeded
```

## **Key Integration Features**

✅ **Multi-provider routing** - OpenAI, Anthropic via single interface  
✅ **Cost optimization** - Automatic routing for cost/performance  
✅ **Retry logic** - Handles transient failures gracefully  
✅ **User context** - Passes user IDs for tracking/analytics  
✅ **Configuration validation** - Ensures valid provider/model combinations  
✅ **Comprehensive logging** - Request/response metrics and metadata  

## **Usage in Agent Development**

```go
// Create router service
routerService := impl.NewRouterService(&cfg.Router)

// Define agent configuration
agentConfig := models.AgentLLMConfig{
    Provider:    "openai",
    Model:       "gpt-3.5-turbo", 
    Temperature: float64Ptr(0.7),
    MaxTokens:   intPtr(150),
}

// Send query to router
messages := []services.Message{
    {Role: "system", Content: "You are a helpful assistant."},
    {Role: "user", Content: "Hello!"},
}

response, err := routerService.SendRequest(ctx, agentConfig, messages, userID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Response: %s\n", response.Content)
fmt.Printf("Provider: %s, Model: %s\n", response.Provider, response.Model) 
fmt.Printf("Cost: $%.6f, Tokens: %d\n", response.CostUSD, response.TokenUsage)
```

This provides a solid foundation for agent execution. The router integration handles all LLM communication, allowing agents to focus on conversation logic and knowledge retrieval.

**Next step**: You can now build agents that use this router service to handle LLM requests across multiple providers with intelligent routing and cost optimization!