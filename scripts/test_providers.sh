#!/bin/bash

# TAS Agent Builder - Provider Validation Test Script
# Tests both OpenAI and Anthropic providers through TAS-LLM-Router

set -e

ROUTER_URL="${ROUTER_BASE_URL:-http://localhost:8080}"
SCRIPT_DIR="$(dirname "$0")"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "🚀 TAS Agent Builder - Provider Validation Test"
echo "=============================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if router is available
echo "1️⃣  Checking TAS-LLM-Router availability at $ROUTER_URL..."
if curl -s -f "$ROUTER_URL/health" > /dev/null 2>&1; then
    echo -e "   ${GREEN}✅ Router is available${NC}"
else
    echo -e "   ${RED}❌ Router is not available at $ROUTER_URL${NC}"
    echo ""
    echo "Please ensure TAS-LLM-Router is running:"
    echo "  cd /path/to/tas-llm-router"
    echo "  go run cmd/llm-router/main.go"
    echo ""
    exit 1
fi

# Check available providers
echo ""
echo "2️⃣  Checking available providers..."
PROVIDERS_RESPONSE=$(curl -s "$ROUTER_URL/v1/providers" 2>/dev/null || echo "")

if [ -z "$PROVIDERS_RESPONSE" ]; then
    echo -e "   ${YELLOW}⚠️  Could not fetch providers list${NC}"
else
    echo -e "   ${GREEN}✅ Providers endpoint accessible${NC}"
    # Try to parse and show provider count
    PROVIDER_COUNT=$(echo "$PROVIDERS_RESPONSE" | grep -o '"name"' | wc -l 2>/dev/null || echo "unknown")
    echo "   📊 Detected $PROVIDER_COUNT providers"
fi

# Check for API keys (via router health endpoints)
echo ""
echo "3️⃣  Checking provider health..."

# Check OpenAI
if curl -s -f "$ROUTER_URL/v1/health/openai" > /dev/null 2>&1; then
    echo -e "   ${GREEN}✅ OpenAI provider healthy${NC}"
    OPENAI_AVAILABLE=true
else
    echo -e "   ${YELLOW}⚠️  OpenAI provider not healthy (check API key)${NC}"
    OPENAI_AVAILABLE=false
fi

# Check Anthropic  
if curl -s -f "$ROUTER_URL/v1/health/anthropic" > /dev/null 2>&1; then
    echo -e "   ${GREEN}✅ Anthropic provider healthy${NC}" 
    ANTHROPIC_AVAILABLE=true
else
    echo -e "   ${YELLOW}⚠️  Anthropic provider not healthy (check API key)${NC}"
    ANTHROPIC_AVAILABLE=false
fi

if [ "$OPENAI_AVAILABLE" = false ] && [ "$ANTHROPIC_AVAILABLE" = false ]; then
    echo ""
    echo -e "${RED}❌ No providers are available. Please check API keys in router configuration.${NC}"
    echo ""
    echo "API keys should be configured in TAS-LLM-Router (not in this test)."
    echo "Please ensure the router is started with proper API key configuration."
    exit 1
fi

# Run the comprehensive provider tests
echo ""
echo "4️⃣  Running comprehensive provider validation tests..."
echo ""

cd "$PROJECT_DIR"

# Set environment for tests
export ROUTER_BASE_URL="$ROUTER_URL"

# Run the provider validation tests
echo -e "${BLUE}Running provider integration tests...${NC}"
if go test -v -timeout=5m ./test -run TestBothProvidersIntegration; then
    echo -e "${GREEN}✅ Provider integration tests passed!${NC}"
    INTEGRATION_PASSED=true
else
    echo -e "${RED}❌ Provider integration tests failed${NC}"
    INTEGRATION_PASSED=false
fi

echo ""
echo -e "${BLUE}Running provider-specific feature tests...${NC}"
if go test -v -timeout=3m ./test -run TestProviderSpecificFeatures; then
    echo -e "${GREEN}✅ Provider-specific tests passed!${NC}"
    FEATURES_PASSED=true
else
    echo -e "${YELLOW}⚠️  Some provider-specific tests failed (may be due to unavailable models)${NC}"
    FEATURES_PASSED=false
fi

echo ""
echo -e "${BLUE}Running routing strategy tests...${NC}"
if go test -v -timeout=3m ./test -run TestRoutingStrategies; then
    echo -e "${GREEN}✅ Routing strategy tests passed!${NC}"
    ROUTING_PASSED=true
else
    echo -e "${YELLOW}⚠️  Some routing strategy tests failed${NC}"
    ROUTING_PASSED=false
fi

# Summary
echo ""
echo "🏁 Test Summary"
echo "==============="
echo ""

if [ "$OPENAI_AVAILABLE" = true ]; then
    echo -e "   ${GREEN}✅ OpenAI${NC} - Provider healthy and accessible"
else
    echo -e "   ${RED}❌ OpenAI${NC} - Provider not available"
fi

if [ "$ANTHROPIC_AVAILABLE" = true ]; then
    echo -e "   ${GREEN}✅ Anthropic${NC} - Provider healthy and accessible"
else
    echo -e "   ${RED}❌ Anthropic${NC} - Provider not available"
fi

echo ""
echo "Test Results:"
if [ "$INTEGRATION_PASSED" = true ]; then
    echo -e "   ${GREEN}✅ Integration Tests${NC} - Both providers working"
else
    echo -e "   ${RED}❌ Integration Tests${NC} - Issues detected"
fi

if [ "$FEATURES_PASSED" = true ]; then
    echo -e "   ${GREEN}✅ Feature Tests${NC} - Provider-specific features working"
else
    echo -e "   ${YELLOW}⚠️  Feature Tests${NC} - Some features may not be available"
fi

if [ "$ROUTING_PASSED" = true ]; then
    echo -e "   ${GREEN}✅ Routing Tests${NC} - Intelligent routing working"
else
    echo -e "   ${YELLOW}⚠️  Routing Tests${NC} - Some routing strategies failed"
fi

echo ""

# Final verdict
if [ "$INTEGRATION_PASSED" = true ] && [ "$OPENAI_AVAILABLE" = true ] && [ "$ANTHROPIC_AVAILABLE" = true ]; then
    echo -e "${GREEN}🎉 SUCCESS: Both OpenAI and Anthropic are working perfectly through TAS-LLM-Router!${NC}"
    echo ""
    echo "🚀 Ready for agent development with multi-provider routing!"
    exit 0
elif [ "$INTEGRATION_PASSED" = true ]; then
    echo -e "${YELLOW}🎯 PARTIAL SUCCESS: At least one provider is working through TAS-LLM-Router${NC}"
    echo ""
    echo "💡 You can proceed with single-provider agent development"
    exit 0
else
    echo -e "${RED}❌ FAILURE: Provider integration not working properly${NC}"
    echo ""
    echo "🔧 Please check:"
    echo "   1. TAS-LLM-Router is running and configured correctly"
    echo "   2. API keys are set in router environment"
    echo "   3. Network connectivity between agent builder and router"
    exit 1
fi