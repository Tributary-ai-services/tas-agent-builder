# TAS Agent Builder

A microservice for building and executing AI agents within the TAS (Tributary AI System) ecosystem.

## Overview

The Agent Builder provides:
- 🤖 **Agent Creation & Management** - Define agents with custom prompts and LLM configurations
- 🔄 **TAS-LLM-Router Integration** - Intelligent routing to multiple LLM providers
- 📊 **Execution Tracking** - Monitor agent performance and usage statistics  
- 🗃️ **Multi-tenant Architecture** - Space-based isolation and user management
- 💾 **Shared Database** - Integrated with TAS shared PostgreSQL infrastructure

## Quick Start

### Prerequisites

1. **TAS Shared Infrastructure** - PostgreSQL, Redis, etc.
2. **TAS-LLM-Router** - Running on `http://localhost:8080` with API keys configured
3. **Go 1.21+** - For development

> **Note**: API keys (OpenAI, Anthropic) should be configured in TAS-LLM-Router, not in this service.

### Setup

1. **Clone and setup**:
   ```bash
   cd /path/to/TAS/tas-agent-builder
   make deps
   ```

2. **Configure environment**:
   ```bash
   cp .env.example .env
   # Edit .env with your database and router settings
   ```

3. **Run database migrations**:
   ```bash
   make db-migrate-up
   ```

4. **Test router integration**:
   ```bash
   make check-router
   make test-providers
   make example-router
   ```

## Router Integration

### Test Router Connectivity

```bash
# Check if router is available
make check-router

# Run integration tests
make test-router

# Run example queries
make example-router
```

### Example Usage

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

## Database Architecture

The agent builder uses the shared TAS PostgreSQL database with schema namespacing:

- **Database**: `tas_shared`
- **Schema**: `agent_builder`
- **Tables**: 
  - `agent_builder.agents` - Agent configurations
  - `agent_builder.agent_executions` - Execution history
  - `agent_builder.agent_usage_stats` - Performance metrics

### Multi-tenancy

All tables include proper tenant isolation:
- `tenant_id` - Tenant identifier
- `space_id` - Personal/organization space
- `owner_id` - User ownership

## Development

### Available Commands

```bash
make help              # Show all commands
make build             # Build application  
make test              # Run all tests
make test-router       # Test router integration
make example-router    # Run router example
make db-migrate-up     # Run migrations
make db-status         # Check migration status
make dev               # Run in development mode
```

### Project Structure

```
├── cmd/                    # Application entry points
├── config/                 # Configuration management
├── database/              
│   ├── migrations/        # Database migrations
│   └── migrate.sh         # Migration script
├── examples/              # Example applications
├── handlers/              # HTTP handlers
├── models/                # Data models
├── services/              # Business logic
│   ├── interfaces         # Service interfaces
│   └── impl/             # Service implementations
├── test/                  # Integration tests
└── Makefile              # Build automation
```

## Integration with TAS Ecosystem

### TAS-LLM-Router
- **Endpoint**: `/v1/chat/completions`
- **Features**: Multi-provider routing, cost optimization, health monitoring
- **Authentication**: Bearer token via `ROUTER_API_KEY`

### Shared Database
- **Connection**: PostgreSQL `tas_shared` database
- **Credentials**: `tasuser` / `taspassword` (from shared infrastructure)
- **Schema**: Namespaced as `agent_builder.*`

### Future Integrations
- **Aether-BE**: User/space management via APIs
- **AudiModal**: Document processing and DLP policies
- **DeepLake**: Vector embeddings for knowledge retrieval

## Configuration

Key environment variables:

```bash
# Database (Shared TAS Infrastructure)
DB_HOST=localhost
DB_PORT=5432
DB_USER=tasuser
DB_PASSWORD=taspassword  
DB_NAME=tas_shared

# TAS-LLM-Router
ROUTER_BASE_URL=http://localhost:8080
ROUTER_API_KEY=your_api_key_here
ROUTER_TIMEOUT=30

# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
```

## Next Steps

1. **Agent Execution Engine** - Implement full agent conversation logic
2. **Knowledge Integration** - Connect to notebook/document sources  
3. **Memory System** - Conversation history and context management
4. **Frontend Integration** - Replace Aether mock data with real APIs
5. **Streaming Support** - Real-time agent responses

## Contributing

1. Follow existing patterns for multi-tenancy and database access
2. Add tests for new functionality
3. Use the shared TAS infrastructure patterns
4. Ensure proper error handling and logging