# TAS Agent Builder - Comprehensive Test Validation Guide

## Overview

This guide provides complete validation coverage for the TAS Agent Builder, ensuring all agent capabilities are thoroughly tested from basic functionality to production readiness.

## Test Architecture

### Test Categories

#### 🧪 **Unit Tests** (`test-unit`)
- **Purpose**: Validate individual components and models
- **Duration**: ~30 seconds
- **Coverage**: Configuration validation, model structures, retry/fallback logic
- **Files**: `reliability_test.go`

#### 🔗 **Integration Tests** (`test-integration`)  
- **Purpose**: Validate service integration and API endpoints
- **Duration**: ~5 minutes
- **Coverage**: Router service, API handlers, database operations, provider validation
- **Files**: 
  - `router_service_reliability_test.go`
  - `agent_handlers_reliability_test.go`
  - `reliability_integration_test.go`
  - `provider_validation_test.go`
  - `router_integration_test.go`

#### 🔄 **Workflow Tests** (`test-workflow`)
- **Purpose**: Validate complete agent lifecycle management
- **Duration**: ~2 minutes
- **Coverage**: Agent creation, updates, publishing, duplication, deletion
- **Files**: `agent_lifecycle_test.go`

#### ⚡ **Execution Tests** (`test-execution`)
- **Purpose**: Validate agent execution engine capabilities
- **Duration**: ~3 minutes
- **Coverage**: Single/concurrent execution, metadata collection, error handling
- **Files**: `execution_engine_test.go`

#### 🔒 **Security Tests** (`test-security`)
- **Purpose**: Validate multi-tenant isolation and access control
- **Duration**: ~1 minute
- **Coverage**: Space isolation, tenant segregation, permission validation
- **Files**: `space_management_test.go`

#### 📊 **Performance Tests** (`test-performance`)
- **Purpose**: Validate scalability and performance characteristics
- **Duration**: ~5 minutes (can be long-running)
- **Coverage**: Load testing, concurrency, resource usage, scalability limits
- **Files**: `performance_load_test.go`

#### 🎯 **End-to-End Tests** (`test-e2e`)
- **Purpose**: Validate complete system workflows and production readiness
- **Duration**: ~3 minutes
- **Coverage**: Full agent workflows, cross-feature integration, error recovery
- **Files**: `end_to_end_integration_test.go`

## Quick Start

### Prerequisites

```bash
# Required environment variables
export JWT_SECRET=test-secret-for-testing
export DB_PASSWORD=taspassword

# Ensure TAS-LLM-Router is running
make check-router
```

### Running Tests

```bash
# Run all tests (comprehensive)
make test-comprehensive

# Run specific categories
make test-unit           # Unit tests only (~30s)
make test-integration    # Integration tests (~5m)
make test-workflow       # Workflow tests (~2m)
make test-execution      # Execution tests (~3m)
make test-security       # Security tests (~1m)
make test-performance    # Performance tests (~5m)
make test-e2e           # End-to-end tests (~3m)

# Quick testing (skip long-running tests)
make test-quick

# Legacy reliability tests
make test-reliability
```

## Test Coverage Matrix

### Agent Capabilities Validation

| Capability | Unit | Integration | Workflow | Execution | Security | Performance | E2E |
|------------|------|-------------|----------|-----------|----------|-------------|-----|
| **Agent Creation** | ✅ | ✅ | ✅ | - | - | - | ✅ |
| **Configuration Validation** | ✅ | ✅ | ✅ | ✅ | - | - | ✅ |
| **LLM Provider Integration** | - | ✅ | - | ✅ | - | ✅ | ✅ |
| **Retry Logic** | ✅ | ✅ | - | ✅ | - | ✅ | ✅ |
| **Provider Fallback** | ✅ | ✅ | - | ✅ | - | ✅ | ✅ |
| **Execution Engine** | - | - | - | ✅ | - | ✅ | ✅ |
| **Metadata Tracking** | ✅ | ✅ | - | ✅ | - | - | ✅ |
| **Space Management** | - | - | ✅ | - | ✅ | - | ✅ |
| **Multi-tenant Isolation** | - | - | - | - | ✅ | - | ✅ |
| **Agent Publishing** | - | ✅ | ✅ | - | ✅ | - | ✅ |
| **Analytics & Monitoring** | - | ✅ | - | ✅ | - | - | ✅ |
| **Error Recovery** | ✅ | ✅ | - | ✅ | - | - | ✅ |
| **Performance & Scale** | - | - | - | - | - | ✅ | ✅ |

### Reliability Features Coverage

| Feature | Description | Test Coverage |
|---------|-------------|---------------|
| **Exponential Backoff** | Retry with exponential delay increase | Unit, Integration, Execution |
| **Linear Backoff** | Retry with linear delay increase | Unit, Integration, Execution |
| **Provider Fallback** | Automatic failover to healthy providers | Unit, Integration, Execution, Performance |
| **Cost Constraints** | Maximum cost increase limits for fallback | Unit, Integration, Performance |
| **Feature Matching** | Fallback providers must support same features | Unit, Integration |
| **Retry Attempts Limit** | Maximum retry attempts (1-5) | Unit, Integration, Execution |
| **Error Pattern Matching** | Specific error types that trigger retries | Unit, Integration, Execution |
| **Metadata Collection** | Complete execution tracking | Integration, Execution, E2E |
| **Analytics Calculation** | Reliability scores and metrics | Integration, E2E |

## Test Scenarios

### 1. Agent Creation & Management

**Scenario**: Complete agent lifecycle from creation to deletion

```bash
make test-workflow
```

**Validates**:
- ✅ Agent creation with valid/invalid configurations
- ✅ Template-based agent creation (high reliability, cost optimized, performance)
- ✅ Configuration updates and versioning
- ✅ Publishing workflow (draft → published → disabled)
- ✅ Space-based isolation (personal vs organization)
- ✅ Agent duplication with configuration inheritance
- ✅ Deletion and cleanup scenarios

### 2. Execution Engine Validation

**Scenario**: Comprehensive execution testing with various configurations

```bash
make test-execution
```

**Validates**:
- ✅ Single execution success with basic configuration
- ✅ Execution with retry configuration (exponential/linear backoff)
- ✅ Execution with fallback configuration
- ✅ Comprehensive metadata collection and validation
- ✅ Cost tracking and token usage monitoring
- ✅ Concurrent execution handling (5-20 simultaneous requests)
- ✅ Error handling (invalid configs, timeouts, large inputs)
- ✅ Performance characteristics and response time analysis

### 3. Multi-Provider Integration

**Scenario**: Validation across multiple LLM providers

```bash
make test-integration
```

**Validates**:
- ✅ OpenAI GPT-3.5 Turbo integration
- ✅ OpenAI GPT-4 integration  
- ✅ Anthropic Claude 3.5 Sonnet integration
- ✅ Anthropic Claude 3 Haiku integration
- ✅ Provider-specific feature validation
- ✅ Routing strategy testing (cost, performance, round-robin)
- ✅ Provider health monitoring

### 4. Security & Isolation Testing

**Scenario**: Multi-tenant security validation

```bash
make test-security
```

**Validates**:
- ✅ Personal space isolation between users
- ✅ Organization space access control
- ✅ Cross-space agent listing and visibility
- ✅ Execution history isolation
- ✅ Cross-tenant data segregation
- ✅ Permission-based access control
- ✅ Space member management

### 5. Performance & Scalability

**Scenario**: System performance under load

```bash
make test-performance
```

**Validates**:
- ✅ Single request baseline performance
- ✅ Concurrent load testing (10, 25, 50 requests)
- ✅ Sustained load testing (5 req/sec for 30 seconds)
- ✅ Performance with reliability features enabled
- ✅ Burst load handling
- ✅ Memory usage and resource cleanup
- ✅ Scalability limits identification

### 6. End-to-End Workflows

**Scenario**: Production-ready workflow validation

```bash
make test-e2e
```

**Validates**:
- ✅ Complete customer service agent workflow
- ✅ Cross-feature integration (retry + fallback + routing)
- ✅ Multi-provider routing scenarios
- ✅ Space and tenant integration
- ✅ Error recovery and resilience
- ✅ Production readiness validation

## Performance Benchmarks

### Expected Performance Characteristics

| Metric | Baseline | Acceptable | Excellent |
|--------|----------|------------|-----------|
| **Single Request Response Time** | < 10s | < 5s | < 3s |
| **Concurrent Success Rate (10)** | > 80% | > 90% | > 95% |
| **Concurrent Success Rate (25)** | > 70% | > 80% | > 90% |
| **Sustained Throughput** | > 3 req/sec | > 4 req/sec | > 5 req/sec |
| **Memory Usage (20 concurrent)** | < 100MB | < 50MB | < 25MB |
| **Retry Success Improvement** | +10% | +20% | +30% |
| **Fallback Success Rate** | > 85% | > 90% | > 95% |

### Load Testing Results

Performance tests provide detailed metrics:
- Response time distribution (min/avg/max)
- Success rates under different load levels
- Throughput measurements
- Resource utilization tracking
- Reliability feature effectiveness

## Troubleshooting

### Common Issues

#### 1. Router Not Available
```bash
# Check router status
make check-router

# Expected output:
✅ Router is running
```

#### 2. Environment Variables Missing
```bash
# Set required variables
export JWT_SECRET=test-secret-for-testing
export DB_PASSWORD=taspassword
```

#### 3. Database Connection Issues
```bash
# Check database connectivity
go run examples/test_reliability_framework.go
```

#### 4. Provider Authentication Issues
- Ensure API keys are configured in TAS-LLM-Router
- Check provider availability with `make test-providers-quick`

### Test Debugging

#### Run Individual Test Files
```bash
# Unit tests
go test -v ./test/reliability_test.go

# Integration tests
JWT_SECRET=test-secret-for-testing go test -v ./test/router_service_reliability_test.go

# With verbose output
go test -v -args -verbose ./test/execution_engine_test.go
```

#### Run Specific Test Functions
```bash
# Run specific test
go test -v ./test/agent_lifecycle_test.go -run TestAgentLifecycleComplete

# Run with timeout
go test -v ./test/performance_load_test.go -timeout 10m
```

#### Debug Performance Issues
```bash
# Run performance tests with profiling
go test -v ./test/performance_load_test.go -cpuprofile=cpu.prof -memprofile=mem.prof

# Short mode to skip long-running tests
go test -v ./test/performance_load_test.go -short
```

## Continuous Integration

### Recommended CI Pipeline

```yaml
# Example CI configuration
test_matrix:
  - name: "Unit & Integration"
    command: "make test-unit && make test-integration"
    timeout: "10m"
    
  - name: "Workflow & Execution"
    command: "make test-workflow && make test-execution"
    timeout: "10m"
    
  - name: "Security & E2E"
    command: "make test-security && make test-e2e"
    timeout: "10m"
    
  - name: "Performance (Quick)"
    command: "make test-performance -short"
    timeout: "5m"
```

### Production Readiness Checklist

- [ ] All unit tests pass (100%)
- [ ] All integration tests pass (≥95%)
- [ ] Workflow tests pass (100%)
- [ ] Execution tests pass (≥90%)
- [ ] Security tests pass (100%)
- [ ] Performance tests meet benchmarks
- [ ] End-to-end tests pass (≥95%)
- [ ] Router connectivity validated
- [ ] Database schema up to date
- [ ] All reliability features functional

## Advanced Testing

### Custom Test Scenarios

Create custom test scenarios by extending existing test files:

```go
// Example: Custom load test
func TestCustomLoadScenario(t *testing.T) {
    // Define custom agent configuration
    // Run specific load pattern
    // Validate custom metrics
}
```

### Mock Testing

For development without external dependencies:
- Mock router responses in test files
- Use test databases with Docker
- Simulate provider failures

### Integration with External Tools

- **Prometheus**: Metrics collection during testing
- **Grafana**: Performance visualization
- **K6**: Load testing integration
- **Docker**: Containerized test environments

## Reporting

### Test Output Format

The comprehensive test suite provides:
- **Category-based results** with pass/fail counts
- **Performance metrics** with timing analysis
- **Detailed failure information** with debugging hints
- **Success rate percentages** for each category
- **Overall readiness assessment**

### Example Output

```
🧪 TAS Agent Builder - Comprehensive Test Suite
===============================================

🏃 Running Unit Tests (1 tests)
==================================================
📋 Test 1/1: Reliability Models Unit Tests
   Category: unit
   Description: Tests retry/fallback config validation and model structures
   Expected duration: ~30s
   File: ./test/reliability_test.go
   ✅ PASSED (28s)

✅ Unit tests completed: 1/1 passed (100.0%)

📊 Comprehensive Test Results Summary
====================================
✅ Unit        : 1/1 passed (100.0%) - 28s
✅ Integration : 5/5 passed (100.0%) - 4m12s
✅ Workflow    : 1/1 passed (100.0%) - 1m45s
✅ Execution   : 1/1 passed (100.0%) - 2m31s
✅ Security    : 1/1 passed (100.0%) - 52s
✅ Performance : 1/1 passed (100.0%) - 4m18s
✅ E2e         : 1/1 passed (100.0%) - 2m44s
--------------------------------------------------
📈 Overall Results: 11/11 passed (100.0%) - 16m50s

⚡ Performance Analysis:
   Total execution time: 16m50s
   Average time per test: 1m32s
   Status: 🌟 Excellent - Ready for production

🎉 All tests passed! The system is ready for production.
```

This comprehensive test validation guide ensures that all TAS Agent Builder capabilities are thoroughly validated, from basic functionality to production-scale deployment.