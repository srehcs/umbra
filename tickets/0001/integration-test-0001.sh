#!/bin/bash

# Integration test script for UMBRA-0001 Policy Validation
# Tests the verification steps from the ticket

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"
TENANT_ID="${TENANT_ID:-550e8400-e29b-41d4-a716-446655440000}"

echo "=========================================="
echo "UMBRA-0001 Integration Test"
echo "=========================================="
echo ""
echo "Base URL: $BASE_URL"
echo "Tenant ID: $TENANT_ID"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

test_count=0
pass_count=0
fail_count=0

run_test() {
    local test_name="$1"
    local expected_code="$2"
    local response
    local actual_code

    ((test_count++))
    
    echo -n "Test $test_count: $test_name ... "
}

assert_http_code() {
    local expected="$1"
    local actual="$2"
    local response="$3"
    
    if [ "$actual" = "$expected" ]; then
        echo -e "${GREEN}PASS${NC}"
        ((pass_count++))
    else
        echo -e "${RED}FAIL${NC} (expected $expected, got $actual)"
        echo "Response: $response"
        ((fail_count++))
    fi
}

# Test 1: Create malformed policy JSON (should return 400 with POLICY_INVALID)
echo -n "Test 1: Create malformed policy JSON ... "
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/policies" \
  -H "Content-Type: application/json" \
  -H "x-umbra-tenant-id: $TENANT_ID" \
  -d '{
    "name": "bad-policy",
    "policy": {
      "version": 1,
      "mode": "invalid_mode",
      "rules": [],
      "default": "deny"
    }
  }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [ "$HTTP_CODE" = "400" ] && echo "$BODY" | grep -q "POLICY_INVALID"; then
    echo -e "${GREEN}PASS${NC}"
    ((pass_count++))
else
    echo -e "${RED}FAIL${NC} (expected 400 with POLICY_INVALID)"
    echo "HTTP Code: $HTTP_CODE"
    echo "Response: $BODY"
    ((fail_count++))
fi

# Test 2: Create valid policy (should return 201 with policy_hash)
echo -n "Test 2: Create valid policy ... "
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/policies" \
  -H "Content-Type: application/json" \
  -H "x-umbra-tenant-id: $TENANT_ID" \
  -d '{
    "name": "good-policy",
    "policy": {
      "version": 1,
      "mode": "abac_v0",
      "rules": [
        {
          "effect": "allow",
          "roles_any": ["admin"],
          "methods_any": ["GET"]
        }
      ],
      "default": "deny"
    }
  }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [ "$HTTP_CODE" = "201" ] && echo "$BODY" | grep -q "policy_hash"; then
    POLICY_ID=$(echo "$BODY" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
    POLICY_HASH=$(echo "$BODY" | grep -o '"policy_hash":"[^"]*' | head -1 | cut -d'"' -f4)
    echo -e "${GREEN}PASS${NC}"
    echo "  Policy ID: $POLICY_ID"
    echo "  Policy Hash: ${POLICY_HASH:0:16}..."
    ((pass_count++))
else
    echo -e "${RED}FAIL${NC} (expected 201 with policy_hash)"
    echo "HTTP Code: $HTTP_CODE"
    echo "Response: $BODY"
    ((fail_count++))
fi

# Test 3: Verify error format contains field-level issues
echo -n "Test 3: Verify error format ... "
RESPONSE=$(curl -s -X POST "$BASE_URL/v1/policies" \
  -H "Content-Type: application/json" \
  -H "x-umbra-tenant-id: $TENANT_ID" \
  -d '{
    "name": "invalid-policy",
    "policy": {
      "version": 1,
      "rules": [],
      "default": "deny"
    }
  }')

if echo "$RESPONSE" | grep -q '"path"' && echo "$RESPONSE" | grep -q '"message"'; then
    echo -e "${GREEN}PASS${NC}"
    ((pass_count++))
else
    echo -e "${RED}FAIL${NC} (expected field-level errors)"
    echo "Response: $RESPONSE"
    ((fail_count++))
fi

# Test 4: Verify hash consistency (same policy same hash)
echo -n "Test 4: Verify hash consistency ... "
POLICY_JSON='{
  "version": 1,
  "mode": "abac_v0",
  "rules": [
    {
      "effect": "allow",
      "roles_any": ["user"],
      "methods_any": ["GET"]
    }
  ],
  "default": "deny"
}'

RESPONSE1=$(curl -s -X POST "$BASE_URL/v1/policies" \
  -H "Content-Type: application/json" \
  -H "x-umbra-tenant-id: $TENANT_ID" \
  -d "{\"name\": \"policy-v1\", \"policy\": $POLICY_JSON}")

RESPONSE2=$(curl -s -X POST "$BASE_URL/v1/policies" \
  -H "Content-Type: application/json" \
  -H "x-umbra-tenant-id: $TENANT_ID" \
  -d "{\"name\": \"policy-v2\", \"policy\": $POLICY_JSON}")

HASH1=$(echo "$RESPONSE1" | grep -o '"policy_hash":"[^"]*' | cut -d'"' -f4)
HASH2=$(echo "$RESPONSE2" | grep -o '"policy_hash":"[^"]*' | cut -d'"' -f4)

if [ "$HASH1" = "$HASH2" ]; then
    echo -e "${GREEN}PASS${NC}"
    echo "  Hash: ${HASH1:0:16}..."
    ((pass_count++))
else
    echo -e "${RED}FAIL${NC} (hashes don't match)"
    echo "  Hash 1: $HASH1"
    echo "  Hash 2: $HASH2"
    ((fail_count++))
fi

# Test 5: Policy size limit
echo -n "Test 5: Policy size limit validation ... "
# Create a policy that exceeds size limit (this will be a mock test without actual large payload)
# In a real test, you'd create a policy > 10MB which the validator would reject

echo -e "${YELLOW}SKIP${NC} (requires >10MB payload, skipped)"

echo ""
echo "=========================================="
echo "Test Results"
echo "=========================================="
echo "Total: $test_count tests"
echo -e "Passed: ${GREEN}$pass_count${NC}"
if [ $fail_count -gt 0 ]; then
    echo -e "Failed: ${RED}$fail_count${NC}"
else
    echo -e "Failed: ${GREEN}0${NC}"
fi
echo "=========================================="
echo ""

if [ $fail_count -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi
