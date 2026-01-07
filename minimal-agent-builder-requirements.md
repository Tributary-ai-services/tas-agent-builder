# Minimal Agent Builder Requirements

## Overview
This document defines the smallest set of features needed to replace the mock agent data in the Aether UI with a functional agent builder that can create, configure, and test basic AI agents using TAS-LLM-Router.

## Current UI Analysis

### Existing Components
Based on the Aether frontend code analysis:

**AgentsPage.jsx**: Grid view of agent cards with create button
**AgentCard.jsx**: Display card showing agent status, media support, recent analysis, runs, accuracy
**AgentDetailModal.jsx**: Detailed view with performance metrics, configuration, recent runs

### Mock Data Structure
Current mock agents have:
```javascript
{
  id: 1,
  name: "Legal Contract Analyzer",
  status: "active" | "training",
  runs: 1204,
  accuracy: 94,
  mediaSupport: ['document', 'image', 'handwriting'],
  recentAnalysis: 'Detected 12 key clauses in scanned contract'
}
```

## Minimal Viable Product (MVP) Features

### Core Agent Management
1. **Create Agent** - Basic configuration form
2. **List Agents** - Display user's agents
3. **View Agent Details** - Show configuration and status
4. **Test Agent** - Simple chat interface to test agent
5. **Delete Agent** - Remove agent

### Essential Agent Configuration
1. **Basic Identity**
   - Name (required)
   - Description (optional)

2. **LLM Configuration**
   - Model selection (gpt-4o, claude-3-5-sonnet, etc.)
   - System prompt (required)
   - Temperature (0.0-1.0)
   - Max tokens

3. **Knowledge Integration**
   - Notebook selection (from existing notebooks)
   - Simple toggle: Enable/Disable knowledge retrieval

4. **Memory Configuration**  
   - Simple toggle: Enable/Disable conversation memory

### Agent Execution
1. **Test Interface** - Simple chat UI to test agent
2. **Execution History** - Show recent test conversations
3. **Basic Metrics** - Response time, token usage, cost

## MVP UI Components

### 1. Agent Creation Form
Replace the simple "Create Agent" button with a proper form modal:

```jsx
// AgentCreateModal.jsx
<Modal title="Create Agent">
  <form>
    {/* Basic Information */}
    <input name="name" placeholder="Agent Name" required />
    <textarea name="description" placeholder="Description (optional)" />
    
    {/* LLM Configuration */}
    <select name="preferredModel">
      <option value="gpt-4o">GPT-4o</option>
      <option value="claude-3-5-sonnet">Claude 3.5 Sonnet</option>
    </select>
    <textarea name="systemPrompt" placeholder="System Prompt" required />
    <input type="number" name="temperature" min="0" max="1" step="0.1" />
    <input type="number" name="maxTokens" min="1" max="4000" />
    
    {/* Knowledge Integration */}
    <label>
      <input type="checkbox" name="enableKnowledge" />
      Enable Knowledge Retrieval
    </label>
    <select name="notebooks" multiple>
      {/* Populate from existing notebooks */}
    </select>
    
    {/* Memory */}
    <label>
      <input type="checkbox" name="enableMemory" />
      Enable Conversation Memory
    </label>
    
    <button type="submit">Create Agent</button>
  </form>
</Modal>
```

### 2. Enhanced Agent Card
Update AgentCard.jsx to show real agent data:

```jsx
// Updated AgentCard.jsx
<div className="agent-card">
  <div className="header">
    <h3>{agent.name}</h3>
    <span className={`status ${agent.status}`}>
      {agent.status} // "active", "draft", "error"
    </span>
  </div>
  
  <div className="model-info">
    Model: {agent.llmConfig.preferredModel}
  </div>
  
  <div className="capabilities">
    {agent.knowledgeConfig.enableVectorSearch && (
      <span className="badge">Knowledge</span>
    )}
    {agent.memoryConfig.enableShortTermMemory && (
      <span className="badge">Memory</span>
    )}
  </div>
  
  <div className="metrics">
    <div>
      <span>{agent.usageStats?.totalExecutions || 0}</span>
      <small>Total Runs</small>
    </div>
    <div>
      <span>${agent.usageStats?.totalCostUSD?.toFixed(3) || '0.000'}</span>
      <small>Total Cost</small>
    </div>
  </div>
  
  <div className="actions">
    <button onClick={() => onTest(agent)}>Test</button>
    <button onClick={() => onEdit(agent)}>Edit</button>
    <button onClick={() => onDelete(agent)}>Delete</button>
  </div>
</div>
```

### 3. Agent Test Modal
New component for testing agents:

```jsx
// AgentTestModal.jsx
<Modal title={`Test ${agent.name}`} size="large">
  <div className="chat-interface">
    {/* Chat Messages */}
    <div className="messages">
      {messages.map(msg => (
        <div key={msg.id} className={`message ${msg.role}`}>
          <div className="content">{msg.content}</div>
          <div className="meta">
            {msg.role === 'assistant' && (
              <span>Tokens: {msg.tokenUsage}, Cost: ${msg.costUSD}</span>
            )}
            <span>{msg.timestamp}</span>
          </div>
        </div>
      ))}
    </div>
    
    {/* Input */}
    <div className="input-area">
      <textarea 
        value={inputMessage}
        onChange={(e) => setInputMessage(e.target.value)}
        placeholder="Type your message..."
        onKeyPress={handleKeyPress}
      />
      <button onClick={sendMessage} disabled={isLoading}>
        {isLoading ? 'Sending...' : 'Send'}
      </button>
    </div>
  </div>
  
  {/* Test Controls */}
  <div className="test-controls">
    <button onClick={clearConversation}>Clear</button>
    <button onClick={exportConversation}>Export</button>
  </div>
</Modal>
```

### 4. Enhanced Agent Detail Modal
Update AgentDetailModal.jsx to show real configuration:

```jsx
// Enhanced AgentDetailModal.jsx sections:

// Configuration Section
<div className="configuration">
  <h3>LLM Configuration</h3>
  <div className="config-item">
    <label>Model:</label>
    <span>{agent.llmConfig.preferredModel}</span>
  </div>
  <div className="config-item">
    <label>Temperature:</label>
    <span>{agent.llmConfig.temperature}</span>
  </div>
  <div className="config-item">
    <label>Max Tokens:</label>
    <span>{agent.llmConfig.maxTokens}</span>
  </div>
  
  <h3>System Prompt</h3>
  <div className="system-prompt">
    {agent.llmConfig.systemPrompt}
  </div>
</div>

// Recent Executions (replace mock recent runs)
<div className="recent-executions">
  <h3>Recent Executions</h3>
  {agent.recentExecutions?.map(execution => (
    <div key={execution.id} className="execution">
      <div className="input">{execution.input.message}</div>
      <div className="output">{execution.output?.content}</div>
      <div className="meta">
        Duration: {execution.totalDuration}ms, 
        Cost: ${execution.costUSD}, 
        Tokens: {execution.tokenUsage}
      </div>
    </div>
  ))}
</div>
```

## Backend API Requirements

### Data Models (Simplified from full design)

```go
// Minimal Agent struct for MVP
type Agent struct {
    ID          string    `json:"id"`
    Name        string    `json:"name" validate:"required"`
    Description string    `json:"description,omitempty"`
    Status      string    `json:"status"` // "draft", "active", "error"
    
    // Owner information
    OwnerID     string    `json:"owner_id"`
    SpaceID     string    `json:"space_id"`
    
    // LLM Configuration (simplified)
    LLMConfig   AgentLLMConfig `json:"llm_config"`
    
    // Knowledge Integration (simplified)
    NotebookIDs []string  `json:"notebook_ids,omitempty"`
    EnableKnowledge bool  `json:"enable_knowledge"`
    
    // Memory (simplified)
    EnableMemory bool     `json:"enable_memory"`
    
    // Usage stats
    UsageStats  AgentUsageStats `json:"usage_stats,omitempty"`
    
    // Timestamps
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type AgentLLMConfig struct {
    PreferredModel  string  `json:"preferred_model"`
    SystemPrompt    string  `json:"system_prompt"`
    Temperature     float64 `json:"temperature"`
    MaxTokens       int     `json:"max_tokens"`
    RouterURL       string  `json:"router_url"`
    RouterAPIKey    string  `json:"router_api_key"`
}

type AgentUsageStats struct {
    TotalExecutions int     `json:"total_executions"`
    TotalCostUSD    float64 `json:"total_cost_usd"`
    AvgResponseTime int     `json:"avg_response_time_ms"`
    LastExecutedAt  *time.Time `json:"last_executed_at,omitempty"`
}

// Agent Execution for testing
type AgentExecution struct {
    ID              string    `json:"id"`
    AgentID         string    `json:"agent_id"`
    UserID          string    `json:"user_id"`
    
    Input           AgentInput    `json:"input"`
    Output          *AgentOutput  `json:"output,omitempty"`
    
    Status          string        `json:"status"` // "running", "completed", "failed"
    TotalDuration   int           `json:"total_duration_ms"`
    TokenUsage      int           `json:"token_usage,omitempty"`
    CostUSD         float64       `json:"cost_usd,omitempty"`
    
    ErrorMessage    string        `json:"error_message,omitempty"`
    
    CreatedAt       time.Time     `json:"created_at"`
    CompletedAt     *time.Time    `json:"completed_at,omitempty"`
}

type AgentInput struct {
    Message string `json:"message"`
}

type AgentOutput struct {
    Content string `json:"content"`
}
```

### Required API Endpoints

```http
# Agent Management
GET    /api/v1/agents                    # List user's agents
POST   /api/v1/agents                    # Create new agent  
GET    /api/v1/agents/{id}               # Get agent details
PUT    /api/v1/agents/{id}               # Update agent
DELETE /api/v1/agents/{id}               # Delete agent

# Agent Testing
POST   /api/v1/agents/{id}/execute       # Execute agent with input
GET    /api/v1/agents/{id}/executions    # Get execution history

# Support endpoints
GET    /api/v1/notebooks                 # List notebooks for knowledge selection
GET    /api/v1/llm/models                # List available models (from router)
```

## Implementation Plan

### Phase 1: Backend Foundation (Week 1)
1. **Database Schema**
   - Create `agents` table with simplified schema
   - Create `agent_executions` table
   - Migration scripts

2. **Basic CRUD API**
   - Implement agent CRUD endpoints
   - Integrate with existing authentication/authorization
   - Connect to existing notebook system

3. **TAS-LLM-Router Integration**
   - HTTP client for router communication
   - Basic agent execution engine
   - Simple memory storage (in database)

### Phase 2: UI Implementation (Week 2)
1. **Replace Mock Data**
   - Update `useAgents.js` hook to call real API
   - Update API service to point to backend

2. **Agent Creation Form**
   - Create `AgentCreateModal.jsx`
   - Form validation and submission
   - Integration with notebook selection

3. **Agent Testing Interface**
   - Create `AgentTestModal.jsx` 
   - Chat interface implementation
   - Real-time execution status

### Phase 3: Enhanced UI (Week 3)
1. **Improved Agent Cards**
   - Show real configuration data
   - Usage statistics display
   - Status indicators

2. **Agent Detail Modal Enhancements**
   - Real configuration display
   - Execution history
   - Performance metrics

3. **Error Handling & UX**
   - Loading states
   - Error messages
   - Form validation

### Phase 4: Testing & Polish (Week 4)
1. **Integration Testing**
   - End-to-end agent creation and testing
   - TAS-LLM-Router integration testing
   - Cost tracking validation

2. **UI Polish**
   - Responsive design
   - Accessibility improvements
   - Performance optimization

3. **Documentation**
   - User guide for agent creation
   - API documentation
   - Deployment guide

## Success Criteria

### Technical Requirements
- ✅ Create functional agent in < 2 minutes
- ✅ Agent executes successfully using TAS-LLM-Router
- ✅ Knowledge retrieval from selected notebooks works
- ✅ Conversation memory persists across test sessions
- ✅ Cost tracking shows accurate per-execution costs
- ✅ UI responsive on desktop and tablet

### User Experience Requirements
- ✅ Intuitive agent creation flow
- ✅ Clear feedback during agent execution
- ✅ Execution history easily accessible
- ✅ Error messages are helpful and actionable
- ✅ No data loss during creation process

### Performance Requirements
- ✅ Agent execution < 5 seconds (95th percentile)
- ✅ UI loads agent list < 2 seconds
- ✅ Form submission response < 1 second
- ✅ Real-time chat interface with < 100ms input lag

## Technical Dependencies

### Backend
- Existing Aether-BE infrastructure
- TAS-LLM-Router instance running
- Neo4j database for relationships
- Existing notebook/document system

### Frontend  
- Existing React/Vite setup
- Current component library (Modal, forms)
- Authentication context
- Routing system

### External Services
- TAS-LLM-Router API access
- Model provider access (via router)
- Vector database for knowledge retrieval

## Risk Mitigation

### High Risk
- **TAS-LLM-Router Integration**: Have fallback to direct API calls if router fails
- **Knowledge Retrieval Performance**: Implement caching and result limits
- **Cost Control**: Hard limits on execution costs per agent

### Medium Risk
- **Memory Storage**: Start with simple database storage, can enhance later
- **UI Complexity**: Keep initial UI simple, add features incrementally
- **Testing Load**: Implement rate limiting on agent execution

### Low Risk
- **Authentication**: Leverage existing system
- **Deployment**: Use existing infrastructure
- **Monitoring**: Use existing logging and metrics

This MVP provides a solid foundation that can be incrementally enhanced while delivering immediate value by replacing mock data with functional agent creation and testing capabilities.