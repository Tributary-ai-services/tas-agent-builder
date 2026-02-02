# Agent Creation Test Improvements - Comprehensive Plan

## Executive Summary

This document outlines a comprehensive test improvement strategy for the TAS Agent Builder, with **primary focus on pluggable agent interface testing** to ensure true universal agent support as designed. The plan addresses critical gaps in current test coverage while maintaining alignment with the original architectural vision of supporting built-in agents, external API integrations, hybrid implementations, and legacy system adapters through a unified interface abstraction.

**Key Principles:**
- **Interface-First Testing**: Validate contracts, not implementations
- **Universal Compatibility**: Any compliant agent must work seamlessly
- **Progressive Enhancement**: Start with core interfaces, expand to advanced patterns

## Priority 1: Pluggable Agent Interface Tests (Week 1) 🔌

### Rationale
The original design emphasizes "Pluggable agent interface abstraction" as a core capability. This MUST be tested first to ensure the platform can truly support any agent implementation that conforms to the interface.

### 1.1 Interface Compliance Test Suite (`test/agent_interface_compliance_test.go`)

#### Core Interface Contract Tests
```go
// Test that any implementation satisfying AgentService interface works
type TestAgentServiceCompliance struct {
    // Verify all interface methods
    CreateAgent(ctx, req, ownerID, tenantID) (*Agent, error)
    GetAgent(ctx, id, userID) (*Agent, error)
    UpdateAgent(ctx, id, req, ownerID) (*Agent, error)
    DeleteAgent(ctx, id, ownerID) error
    // ... all other interface methods
}
```

#### Minimal Compliant Implementation
```go
// Absolute minimum to satisfy interface
type MinimalAgent struct {
    agents map[uuid.UUID]*models.Agent
}

// Test that minimal implementation works
func TestMinimalAgentCompliance(t *testing.T) {
    agent := &MinimalAgent{}
    // Verify it satisfies AgentService interface
    var _ services.AgentService = agent
    // Test all interface methods work
}
```

#### Interface Substitutability Tests
```go
func TestInterfaceSubstitutability(t *testing.T) {
    implementations := []services.AgentService{
        &BuiltInAgent{},      // Standard implementation
        &ExternalAPIAgent{},  // External service proxy
        &HybridAgent{},       // Combined implementation
        &MockAgent{},         // Test mock
    }
    
    for _, impl := range implementations {
        // All should pass same contract tests
        testAgentContract(t, impl)
    }
}
```

### 1.2 Mock Agent Implementations (`test/mock_agents/`)

#### Non-Persistent Mock Agent
```go
// In-memory only, no database dependency
type NonPersistentAgent struct {
    agents sync.Map
}

func (n *NonPersistentAgent) CreateAgent(ctx context.Context, req models.CreateAgentRequest, ownerID uuid.UUID, tenantID string) (*models.Agent, error) {
    agent := &models.Agent{
        ID:       uuid.New(),
        Name:     req.Name,
        OwnerID:  ownerID,
        TenantID: tenantID,
    }
    n.agents.Store(agent.ID, agent)
    return agent, nil
}
```

#### External API Mock Agent
```go
// Simulates external service integration
type ExternalAPIMockAgent struct {
    endpoint string
    client   *http.Client
}

func (e *ExternalAPIMockAgent) CreateAgent(ctx context.Context, req models.CreateAgentRequest, ownerID uuid.UUID, tenantID string) (*models.Agent, error) {
    // Forward to external service
    resp, err := e.client.Post(e.endpoint, "application/json", marshalRequest(req))
    // Parse and return
}
```

### 1.3 Contract Testing Framework (`test/agent_contract_test.go`)

#### Define Universal Contracts
```go
type AgentContract struct {
    Name        string
    InputSchema Schema
    OutputSchema Schema
    Behaviors   []BehaviorTest
}

var UniversalAgentContracts = []AgentContract{
    {
        Name: "CreateAgentContract",
        Behaviors: []BehaviorTest{
            {"Returns valid UUID", testValidUUID},
            {"Preserves agent name", testNamePreservation},
            {"Enforces owner permissions", testOwnerPermissions},
        },
    },
}
```

#### Contract Validation
```go
func ValidateAgentContract(t *testing.T, agent services.AgentService, contract AgentContract) {
    for _, behavior := range contract.Behaviors {
        t.Run(behavior.Name, func(t *testing.T) {
            behavior.Test(t, agent)
        })
    }
}
```

### 1.4 Success Metrics for Week 1
- ✅ 100% interface method coverage
- ✅ 3+ different agent implementations passing same tests
- ✅ Contract validation framework operational
- ✅ Zero coupling to specific implementations

## Priority 2: Core Agent Creation Enhancement (Week 2) 🏗️

### 2.1 Bulk/Concurrent Creation Tests (`test/agent_creation_stress_test.go`)

#### High Volume Creation
```go
func TestBulkAgentCreation(t *testing.T) {
    const numAgents = 100
    agents := make([]*models.Agent, 0, numAgents)
    
    start := time.Now()
    for i := 0; i < numAgents; i++ {
        agent, err := service.CreateAgent(ctx, generateRequest(i), userID, tenantID)
        require.NoError(t, err)
        agents = append(agents, agent)
    }
    duration := time.Since(start)
    
    // Verify all unique IDs
    ids := make(map[uuid.UUID]bool)
    for _, agent := range agents {
        assert.False(t, ids[agent.ID], "Duplicate ID found")
        ids[agent.ID] = true
    }
    
    // Performance assertion
    assert.Less(t, duration, 10*time.Second, "Bulk creation too slow")
}
```

#### Concurrent Creation
```go
func TestConcurrentAgentCreation(t *testing.T) {
    const numGoroutines = 50
    var wg sync.WaitGroup
    errors := make(chan error, numGoroutines)
    
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(index int) {
            defer wg.Done()
            _, err := service.CreateAgent(ctx, generateRequest(index), userID, tenantID)
            if err != nil {
                errors <- err
            }
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    // No errors expected
    for err := range errors {
        t.Errorf("Concurrent creation failed: %v", err)
    }
}
```

### 2.2 Duplicate Detection (`test/agent_duplicate_test.go`)

#### Name Uniqueness per Space
```go
func TestDuplicateNameDetection(t *testing.T) {
    // Create first agent
    req := models.CreateAgentRequest{
        Name:     "Unique Agent",
        SpaceID:  spaceID,
    }
    agent1, err := service.CreateAgent(ctx, req, userID, tenantID)
    require.NoError(t, err)
    
    // Attempt duplicate in same space
    agent2, err := service.CreateAgent(ctx, req, userID, tenantID)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "duplicate name")
    
    // Allow in different space
    req.SpaceID = differentSpaceID
    agent3, err := service.CreateAgent(ctx, req, userID, tenantID)
    assert.NoError(t, err)
}
```

### 2.3 Resource Limits (`test/agent_limits_test.go`)

#### User Quota Enforcement
```go
func TestMaxAgentsPerUser(t *testing.T) {
    const userLimit = 100
    
    // Create up to limit
    for i := 0; i < userLimit; i++ {
        _, err := service.CreateAgent(ctx, generateRequest(i), userID, tenantID)
        require.NoError(t, err)
    }
    
    // Attempt to exceed limit
    _, err := service.CreateAgent(ctx, generateRequest(userLimit), userID, tenantID)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "quota exceeded")
}
```

### 2.4 Edge Cases (`test/agent_edge_cases_test.go`)

#### Special Characters and Injection Attempts
```go
func TestSpecialCharacterHandling(t *testing.T) {
    testCases := []struct {
        name     string
        input    string
        expected string
    }{
        {"Unicode", "🤖 Agent", "🤖 Agent"},
        {"SQL Injection", "Agent'; DROP TABLE--", "Agent'; DROP TABLE--"},
        {"HTML Tags", "<script>alert('xss')</script>", "&lt;script&gt;alert('xss')&lt;/script&gt;"},
        {"Very Long", strings.Repeat("a", 1000), strings.Repeat("a", 255)},
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            req := models.CreateAgentRequest{Name: tc.input}
            agent, err := service.CreateAgent(ctx, req, userID, tenantID)
            require.NoError(t, err)
            assert.Equal(t, tc.expected, agent.Name)
        })
    }
}
```

## Priority 3: Custom Agent Implementations (Week 3) 🔧

### 3.1 External API Agents (`test/external_api_agent_test.go`)

#### External Service Integration
```go
func TestExternalAPIAgentCreation(t *testing.T) {
    // Setup mock external service
    mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]interface{}{
            "id":     uuid.New(),
            "status": "created",
        })
    }))
    defer mockServer.Close()
    
    config := models.AgentLLMConfig{
        Provider: "external",
        Metadata: map[string]interface{}{
            "endpoint": mockServer.URL,
            "auth_type": "bearer",
            "credentials": encryptedCredentials,
        },
    }
    
    req := models.CreateAgentRequest{
        Name:      "External API Agent",
        LLMConfig: config,
    }
    
    agent, err := service.CreateAgent(ctx, req, userID, tenantID)
    require.NoError(t, err)
    assert.Equal(t, "external", agent.LLMConfig.Provider)
}
```

#### Credential Management
```go
func TestExternalAPICredentialHandling(t *testing.T) {
    // Test credential encryption
    plainCreds := "secret-api-key"
    agent := createExternalAgent(plainCreds)
    
    // Verify credentials are encrypted in storage
    stored := getStoredAgent(agent.ID)
    assert.NotEqual(t, plainCreds, stored.LLMConfig.Metadata["credentials"])
    assert.True(t, isEncrypted(stored.LLMConfig.Metadata["credentials"]))
    
    // Verify decryption on retrieval
    retrieved, _ := service.GetAgent(ctx, agent.ID, userID)
    // Credentials should be decrypted for authorized user
    assert.Equal(t, plainCreds, decryptCredentials(retrieved))
}
```

### 3.2 Hybrid Agents (`test/hybrid_agent_test.go`)

#### Component Composition
```go
type HybridAgent struct {
    builtIn  services.AgentService
    external ExternalAPIClient
    custom   CustomLogicProcessor
}

func TestHybridAgentCreation(t *testing.T) {
    hybrid := &HybridAgent{
        builtIn:  impl.NewAgentService(db),
        external: NewExternalClient(endpoint),
        custom:   NewCustomProcessor(rules),
    }
    
    // Create through standard interface
    req := models.CreateAgentRequest{
        Name: "Hybrid Agent",
        LLMConfig: models.AgentLLMConfig{
            Provider: "hybrid",
            Metadata: map[string]interface{}{
                "components": []string{"builtin", "external", "custom"},
            },
        },
    }
    
    agent, err := hybrid.CreateAgent(ctx, req, userID, tenantID)
    require.NoError(t, err)
    
    // Verify all components initialized
    assert.Contains(t, agent.LLMConfig.Metadata["components"], "builtin")
    assert.Contains(t, agent.LLMConfig.Metadata["components"], "external")
    assert.Contains(t, agent.LLMConfig.Metadata["components"], "custom")
}
```

### 3.3 Plugin Architecture (`test/plugin_agent_test.go`)

#### Dynamic Plugin Loading
```go
func TestPluginAgentRegistration(t *testing.T) {
    registry := NewPluginRegistry()
    
    // Register plugin
    plugin := &CustomPlugin{
        Name:    "sentiment-analyzer",
        Version: "1.0.0",
        Handler: sentimentHandler,
    }
    
    err := registry.Register(plugin)
    require.NoError(t, err)
    
    // Create agent using plugin
    req := models.CreateAgentRequest{
        Name: "Plugin-based Agent",
        LLMConfig: models.AgentLLMConfig{
            Provider: "plugin",
            Metadata: map[string]interface{}{
                "plugin_name": "sentiment-analyzer",
                "plugin_version": "1.0.0",
            },
        },
    }
    
    agent, err := service.CreateAgent(ctx, req, userID, tenantID)
    require.NoError(t, err)
    
    // Verify plugin is active
    assert.True(t, registry.IsActive(plugin.Name))
}
```

## Priority 4: Advanced Testing (Week 4) 🚀

### 4.1 Version Management (`test/agent_versioning_test.go`)

#### Version History Tracking
```go
func TestAgentVersioning(t *testing.T) {
    // Create v1
    agent := createAgent("v1 config")
    v1ID := agent.ID
    
    // Update to v2
    updateReq := models.UpdateAgentRequest{
        SystemPrompt: stringPtr("v2 prompt"),
    }
    agent, _ = service.UpdateAgent(ctx, v1ID, updateReq, userID)
    
    // Get version history
    versions := service.GetAgentVersions(ctx, v1ID, userID)
    assert.Len(t, versions, 2)
    assert.Equal(t, "v1 config", versions[0].Config)
    assert.Equal(t, "v2 prompt", versions[1].SystemPrompt)
    
    // Rollback to v1
    agent, _ = service.RollbackAgent(ctx, v1ID, versions[0].Version, userID)
    assert.Equal(t, "v1 config", agent.Config)
}
```

### 4.2 Security & Permissions (`test/agent_creation_security_test.go`)

#### Permission Boundaries
```go
func TestCrossTenantCreationPrevention(t *testing.T) {
    // User from tenant A
    userA := uuid.New()
    tenantA := "tenant-a"
    
    // Try to create in tenant B's space
    tenantBSpace := uuid.New() // Known to be tenant B's space
    req := models.CreateAgentRequest{
        Name:    "Malicious Agent",
        SpaceID: tenantBSpace,
    }
    
    _, err := service.CreateAgent(ctx, req, userA, tenantA)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "permission denied")
}
```

### 4.3 Integration Patterns (`test/agent_integration_patterns_test.go`)

#### Sidecar Pattern
```go
func TestSidecarAgentPattern(t *testing.T) {
    // Main application
    mainApp := &Application{Port: 8080}
    
    // Sidecar agent
    sidecar := &SidecarAgent{
        mainApp: mainApp,
        agent:   service,
    }
    
    // Intercept and enhance requests
    req := createRequest()
    enhanced := sidecar.Intercept(req)
    
    assert.NotNil(t, enhanced.TraceContext)
    assert.NotNil(t, enhanced.SecurityContext)
}
```

## Implementation Timeline

### Week 1: Pluggable Interface Foundation ✅
- Days 1-2: Interface compliance suite
- Days 3-4: Mock implementations
- Day 5: Contract testing framework

### Week 2: Core Creation Testing ✅
- Days 1-2: Bulk/concurrent tests
- Day 3: Duplicate detection
- Days 4-5: Resource limits & edge cases

### Week 3: Custom Implementations ✅
- Days 1-2: External API agents
- Days 3-4: Hybrid agents
- Day 5: Plugin architecture

### Week 4: Advanced Features ✅
- Days 1-2: Version management
- Days 3-4: Security testing
- Day 5: Integration patterns

## Success Metrics

### Overall Coverage Goals
- **Line Coverage**: 90%+ for agent creation paths
- **Branch Coverage**: 85%+ for decision points
- **Interface Compliance**: 100% of methods tested

### Performance Targets
- Single agent creation: < 100ms (p99)
- Bulk creation (100 agents): < 5 seconds
- Concurrent creation (50 goroutines): No deadlocks
- Interface overhead: < 5ms per call

### Quality Metrics
- All agent types (Built-in, External, Hybrid, Legacy) fully tested
- Zero hardcoded implementation dependencies
- 100% contract compliance for all implementations
- Graceful handling of all edge cases

## Key Testing Principles

### 1. Interface-First Approach
- Test contracts, not implementations
- Ensure substitutability (Liskov principle)
- Validate behavior, not internals

### 2. Universal Compatibility
- Any compliant implementation must pass
- No special cases for specific implementations
- Support future agent types without test changes

### 3. Progressive Enhancement
- Start with minimal compliance
- Add complexity incrementally
- Maintain backward compatibility

## Conclusion

This comprehensive test improvement plan ensures the TAS Agent Builder truly delivers on its promise of "Universal Agent Support" through pluggable interface abstraction. By prioritizing interface compliance testing, we validate that any agent implementation—whether built-in, external API, hybrid, or legacy—can integrate seamlessly with the platform.

The phased approach allows for systematic improvement while maintaining development velocity, with clear success metrics and timelines for each phase. This plan aligns perfectly with the original architectural vision while addressing current testing gaps.