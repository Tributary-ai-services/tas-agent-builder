BINARY_NAME=agent-builder
ROUTER_URL=http://localhost:8080

.PHONY: help build test test-router test-providers test-providers-quick example-router clean deps

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

deps: ## Install dependencies
	go mod download
	go mod tidy

build: ## Build the application
	go build -o $(BINARY_NAME) cmd/main.go

test: ## Run all tests
	go test -v ./...

test-comprehensive: ## Run comprehensive test suite with all categories
	@echo "Running comprehensive test suite..."
	JWT_SECRET=test-secret-for-testing DB_PASSWORD=taspassword go run scripts/run_comprehensive_tests.go

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	JWT_SECRET=test-secret-for-testing DB_PASSWORD=taspassword go run scripts/run_comprehensive_tests.go -category unit

test-integration: ## Run integration tests only
	@echo "Running integration tests..."
	JWT_SECRET=test-secret-for-testing DB_PASSWORD=taspassword go run scripts/run_comprehensive_tests.go -category integration

test-workflow: ## Run workflow tests (agent lifecycle)
	@echo "Running workflow tests..."
	JWT_SECRET=test-secret-for-testing DB_PASSWORD=taspassword go run scripts/run_comprehensive_tests.go -category workflow

test-execution: ## Run execution engine tests
	@echo "Running execution engine tests..."
	JWT_SECRET=test-secret-for-testing DB_PASSWORD=taspassword go run scripts/run_comprehensive_tests.go -category execution

test-security: ## Run security and isolation tests
	@echo "Running security tests..."
	JWT_SECRET=test-secret-for-testing DB_PASSWORD=taspassword go run scripts/run_comprehensive_tests.go -category security

test-performance: ## Run performance and load tests
	@echo "Running performance tests..."
	JWT_SECRET=test-secret-for-testing DB_PASSWORD=taspassword go run scripts/run_comprehensive_tests.go -category performance

test-e2e: ## Run end-to-end integration tests
	@echo "Running end-to-end tests..."
	JWT_SECRET=test-secret-for-testing DB_PASSWORD=taspassword go run scripts/run_comprehensive_tests.go -category e2e

test-quick: ## Run tests in short mode (skip long-running tests)
	@echo "Running quick test suite..."
	JWT_SECRET=test-secret-for-testing DB_PASSWORD=taspassword go run scripts/run_comprehensive_tests.go -short

test-router: ## Test TAS-LLM-Router integration
	@echo "Testing router integration..."
	ROUTER_BASE_URL=$(ROUTER_URL) go test -v ./test -run TestRouter

test-providers: ## Comprehensive test of both OpenAI and Anthropic providers
	@echo "Testing both OpenAI and Anthropic providers..."
	./scripts/test_providers.sh

test-providers-quick: ## Quick provider validation test
	@echo "Quick provider validation..."
	ROUTER_BASE_URL=$(ROUTER_URL) go test -v ./test -run TestBothProvidersIntegration

test-reliability: ## Run original reliability test suite
	@echo "Running reliability test suite..."
	JWT_SECRET=test-secret-for-testing DB_PASSWORD=taspassword go run scripts/run_reliability_tests.go

example-router: ## Run router integration example
	@echo "Running router integration example..."
	ROUTER_BASE_URL=$(ROUTER_URL) go run examples/router_example.go

check-router: ## Check if TAS-LLM-Router is running
	@echo "Checking if TAS-LLM-Router is available at $(ROUTER_URL)..."
	@curl -s -f $(ROUTER_URL)/health > /dev/null && echo "✅ Router is running" || echo "❌ Router is not available"

db-migrate-up: ## Run database migrations
	./database/migrate.sh up

db-migrate-down: ## Rollback database migrations
	./database/migrate.sh down

db-status: ## Show migration status
	./database/migrate.sh status

clean: ## Clean build artifacts
	go clean
	rm -f $(BINARY_NAME)

fmt: ## Format code
	go fmt ./...

lint: ## Run linter
	golangci-lint run

dev: ## Run in development mode
	go run cmd/main.go

.DEFAULT_GOAL := help