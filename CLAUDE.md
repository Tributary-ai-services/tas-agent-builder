# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**TAS Agent Builder** is a microservice for building and executing AI agents within the TAS (Tributary AI System) ecosystem. It provides agent creation and management, execution tracking, and seamless integration with TAS-LLM-Router for intelligent routing to multiple LLM providers. Built with Go, it supports multi-tenant architecture with space-based isolation.

## Data Models & Schema Reference

### Service-Specific Data Models
This service's data models are comprehensively documented in the centralized data models repository:

**Location**: `../aether-shared/data-models/tas-agent-builder/`

#### Key Entity Models:
The data models for TAS Agent Builder are organized in the following subdirectories:
- **`entities/`** - Core agent and execution entity models (PostgreSQL)
- **`api/`** - API request/response types and endpoint schemas
- **`schemas/`** - JSON schemas for agent configurations and tool definitions

#### Cross-Service Integration:
- **Platform ERD** (`../aether-shared/data-models/cross-service/diagrams/platform-erd.md`) - Complete entity relationship diagram
- **Architecture Overview** (`../aether-shared/data-models/cross-service/diagrams/architecture-overview.md`) - Agent Builder in system architecture
- **ID Mapping Chain** (`../aether-shared/data-models/cross-service/mappings/id-mapping-chain.md`) - Cross-service identifier relationships

#### When to Reference Data Models:
1. Before making schema changes to agent or execution entities
2. When implementing new API endpoints or modifying request/response formats
3. When debugging agent execution issues or LLM integration problems
4. When onboarding new developers to understand the agent architecture
5. Before adding new tool configurations or agent capabilities

**Main Documentation Hub**: `../aether-shared/data-models/README.md` - Complete navigation for all 38 data model files

## Technology Stack

- **Language**: Go 1.21+
- **Database**: PostgreSQL (shared TAS infrastructure)
- **Cache**: Redis (shared TAS infrastructure)
- **Framework**: Gin HTTP framework
- **LLM Integration**: TAS-LLM-Router for multi-provider access
- **Monitoring**: Prometheus metrics + Grafana dashboards

## Key Features

### Agent Creation & Management
- **Custom Agents**: Define agents with custom prompts and LLM configurations
- **Tool Integration**: Attach tools and capabilities to agents
- **Version Control**: Agent configuration versioning and rollback
- **Template Library**: Pre-built agent templates for common use cases

### TAS-LLM-Router Integration
- **Intelligent Routing**: Automatic routing to optimal LLM provider
- **Multi-Provider**: Support for OpenAI, Anthropic, and other providers
- **Cost Optimization**: Automatic cost-based provider selection
- **Failover**: Automatic failover to backup providers

### Execution Tracking
- **Performance Monitoring**: Track agent execution time, token usage, and costs
- **Usage Statistics**: Per-agent and per-tenant usage analytics
- **Execution History**: Complete audit trail of agent executions
- **Error Tracking**: Detailed error logs and failure analysis

### Multi-Tenant Architecture
- **Space-Based Isolation**: Secure separation of tenant data and resources
- **Resource Limits**: Per-tenant quotas and rate limiting
- **User Management**: Role-based access control for agent management
- **Shared Database**: Integration with TAS shared PostgreSQL infrastructure

## Common Commands

```bash
# Install dependencies
make deps

# Run database migrations
make db-migrate-up

# Rollback database migrations
make db-migrate-down

# Check TAS-LLM-Router connectivity
make check-router

# Test LLM provider integration
make test-providers

# Run example agent via router
make example-router

# Run integration tests
make test-router

# Build the application
make build

# Run tests
make test

# Run with hot reload (development)
make dev
```

## API Endpoints

### Agent Management
- `POST /agents` - Create new agent
- `GET /agents` - List agents (filtered by space/tenant)
- `GET /agents/{id}` - Get agent details
- `PUT /agents/{id}` - Update agent configuration
- `DELETE /agents/{id}` - Delete agent

### Agent Execution
- `POST /agents/{id}/execute` - Execute agent with input
- `GET /executions` - List execution history
- `GET /executions/{id}` - Get execution details
- `GET /executions/{id}/logs` - Get execution logs

### Agent Templates
- `GET /templates` - List available agent templates
- `GET /templates/{id}` - Get template details
- `POST /templates/{id}/instantiate` - Create agent from template

### Management
- `GET /health` - Health check endpoint
- `GET /metrics` - Prometheus metrics endpoint
- `GET /stats` - Usage statistics and analytics

## Integration Points

- **TAS-LLM-Router**: Primary LLM provider integration (required)
- **Aether Backend**: Agent invocation from notebooks and workflows
- **TAS Workflow Builder**: Agents as workflow steps
- **TAS MCP**: MCP server capabilities as agent tools
- **PostgreSQL**: Shared database for agent and execution storage
- **Redis**: Caching for agent configurations and execution results

## Configuration

Configuration is managed via environment variables and `.env` file:

```bash
# Database Configuration
DATABASE_URL=postgresql://tasuser:taspassword@localhost:5432/tas_shared

# Redis Configuration
REDIS_URL=redis://localhost:6379/0

# TAS-LLM-Router Configuration
LLM_ROUTER_URL=http://localhost:8080
LLM_ROUTER_TIMEOUT=300s

# Server Configuration
SERVER_PORT=8087
SERVER_TIMEOUT=60s

# Multi-Tenancy
DEFAULT_SPACE_ID=default
ENABLE_SPACE_ISOLATION=true
```

## Architecture: Aether-BE as Single Entry Point

**IMPORTANT**: All agent operations should go through **aether-be**, not directly to agent-builder.

```
Frontend/API Clients → aether-be (API Gateway) → agent-builder (internal only)
                           ↓
                        Neo4j (relationships, metadata)
```

### Why This Architecture?

Agents are stored in two databases:
1. **PostgreSQL** (agent-builder) - Agent definitions, LLM configs, executions
2. **Neo4j** (aether-be) - Relationships (ownership, teams), permissions, graph queries

aether-be acts as a synchronization layer:
- **Create**: Creates in agent-builder → Creates in Neo4j with `agent_builder_id` mapping
- **Update**: Updates agent-builder → Syncs changes to Neo4j
- **Delete**: Deletes from agent-builder → Removes from Neo4j

### Network Access

- **External Ingress**: REMOVED (was `agent-builder.tas.scharber.com`)
- **Internal Only**: `http://agent-builder.tas-agent-builder:8087/api/v1`

Only aether-be should call agent-builder directly. Direct external access is blocked to ensure database consistency.

## Important Notes

- **API Keys**: LLM provider API keys (OpenAI, Anthropic) are configured in TAS-LLM-Router, NOT in this service
- **Router Dependency**: TAS-LLM-Router must be running and accessible before starting this service
- **Space Isolation**: Always verify space context when creating or executing agents
- **Database Migrations**: Run migrations before first use with `make db-migrate-up`
- **Shared Infrastructure**: Requires TAS shared PostgreSQL and Redis services
- **No External Access**: This service is internal-only; all requests must go through aether-be
- Integration with shared TAS infrastructure via `tas-shared-network` Docker network

## Development Workflow

1. **Start shared infrastructure**: `cd ../aether-shared && ./start-shared-services.sh`
2. **Start TAS-LLM-Router**: `cd ../tas-llm-router && make dev`
3. **Verify router connectivity**: `make check-router`
4. **Run database migrations**: `make db-migrate-up`
5. **Start development server**: `make dev` (with hot reload)
6. **Run tests**: `make test` and `make test-router`

## Testing

```bash
# Run all tests
make test

# Run router integration tests
make test-router

# Test LLM provider connectivity
make test-providers

# Run example agent execution
make example-router

# Run with coverage
make test-coverage
```
