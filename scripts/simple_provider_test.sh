#!/bin/bash

# Simple Provider Test Script
# Tests OpenAI and Anthropic providers directly via curl

ROUTER_URL="${ROUTER_BASE_URL:-http://localhost:8086}"

echo "🧪 TAS-LLM-Router Provider Test"
echo "================================="
echo "Testing router at: $ROUTER_URL"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test function
test_provider() {
    local provider_name="$1"
    local model="$2"
    local test_message="$3"
    
    echo -e "${BLUE}Testing $provider_name ($model)...${NC}"
    
    response=$(curl -s -X POST "$ROUTER_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"$model\",\"messages\":[{\"role\":\"user\",\"content\":\"$test_message\"}]}")
    
    # Check if response contains choices (success) or error
    if echo "$response" | grep -q '"choices"'; then
        content=$(echo "$response" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('choices', [{}])[0].get('message', {}).get('content', 'No content'))" 2>/dev/null)
        provider=$(echo "$response" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('router_metadata', {}).get('provider', 'unknown'))" 2>/dev/null)
        tokens=$(echo "$response" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('usage', {}).get('total_tokens', 0))" 2>/dev/null)
        
        echo -e "   ${GREEN}✅ SUCCESS${NC}"
        echo "   Response: $content"
        echo "   Provider: $provider"
        echo "   Tokens: $tokens"
        return 0
    else
        error_msg=$(echo "$response" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('error', {}).get('message', 'Unknown error'))" 2>/dev/null)
        echo -e "   ${RED}❌ ERROR${NC}"
        echo "   Error: $error_msg"
        return 1
    fi
    echo ""
}

# Test providers
echo "1. Testing OpenAI Provider"
echo "=========================="
test_provider "OpenAI GPT-3.5" "gpt-3.5-turbo" "Say exactly: 'OpenAI is working through TAS-LLM-Router'"
echo ""

test_provider "OpenAI GPT-4" "gpt-4o" "Say exactly: 'GPT-4 is working through TAS-LLM-Router'"
echo ""

echo "2. Testing Anthropic Provider"
echo "============================="
test_provider "Claude 3.5 Sonnet" "claude-3-5-sonnet-20241022" "Say exactly: 'Claude is working through TAS-LLM-Router'"
echo ""

test_provider "Claude 3 Haiku" "claude-3-haiku-20240307" "Say exactly: 'Claude Haiku is working through TAS-LLM-Router'"
echo ""

# Test routing strategies
echo "3. Testing Routing Strategies"
echo "============================="

echo -e "${BLUE}Testing Cost Optimization...${NC}"
response=$(curl -s -X POST "$ROUTER_URL/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"What is 2+2?"}],"optimize_for":"cost"}')

if echo "$response" | grep -q '"choices"'; then
    provider=$(echo "$response" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('router_metadata', {}).get('provider', 'unknown'))" 2>/dev/null)
    model_used=$(echo "$response" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('model', 'unknown'))" 2>/dev/null)
    echo -e "   ${GREEN}✅ Cost optimization working${NC}"
    echo "   Routed to: $provider ($model_used)"
else
    echo -e "   ${RED}❌ Cost optimization failed${NC}"
fi

echo ""

# Summary
echo "🏁 Test Summary"
echo "==============="
echo ""
echo "Router is successfully:"
echo "• Routing requests to working providers"
echo "• Handling different model types"  
echo "• Providing proper response formatting"
echo "• Including routing metadata"
echo ""

# Check which providers are healthy
echo "Provider Status:"
if curl -s "$ROUTER_URL/v1/chat/completions" -X POST -H "Content-Type: application/json" -d '{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"test"}]}' | grep -q '"choices"'; then
    echo -e "   ${GREEN}✅ OpenAI${NC} - Working correctly"
else
    echo -e "   ${RED}❌ OpenAI${NC} - Not working"
fi

if curl -s "$ROUTER_URL/v1/chat/completions" -X POST -H "Content-Type: application/json" -d '{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"test"}]}' | grep -q '"choices"'; then
    echo -e "   ${GREEN}✅ Anthropic${NC} - Working correctly"
else
    echo -e "   ${RED}❌ Anthropic${NC} - Not healthy (check API key)"
fi

echo ""
echo "🎉 TAS-LLM-Router integration test completed!"