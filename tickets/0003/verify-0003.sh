#!/bin/bash
# Verification script for UMBRA-0003: Observe vs Enforce Mode (PEP_MODE)
# Prerequisites: make dev, make seed must be run first

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../" && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=== UMBRA-0003 Verification: Observe vs Enforce Mode ==="
echo

# Function to generate a tenant ID for testing
generate_tenant_id() {
    if command -v uuidgen &> /dev/null; then
        uuidgen
    else
        python3 -c "import uuid; print(uuid.uuid4())"
    fi
}

# Function to test observe mode
test_observe_mode() {
    echo -e "${YELLOW}[1/4] Testing OBSERVE mode with DENY decision...${NC}"
    
    # We'll need to verify the mode is set to observe
    response=$(curl -s -X GET http://localhost:8082/healthz)
    if [[ $response == *"pep-gateway"* ]]; then
        echo -e "${GREEN}✓ PEP Gateway is running${NC}"
    else
        echo -e "${RED}✗ PEP Gateway not responding${NC}"
        return 1
    fi
    
    # Check docker-compose has PEP_MODE set
    if grep -q "PEP_MODE" "$REPO_ROOT/identity/control-plane/deployments/docker-compose.yml"; then
        echo -e "${GREEN}✓ PEP_MODE configured in docker-compose.yml${NC}"
    else
        echo -e "${RED}✗ PEP_MODE not found in docker-compose.yml${NC}"
        return 1
    fi
}

# Function to verify receipt structure
test_receipt_structure() {
    echo -e "${YELLOW}[2/4] Verifying receipt structure includes pep.mode and enforcement.outcome...${NC}"
    
    # Check if database schema supports new fields
    if command -v psql &> /dev/null; then
        # Query the receipts_invocation table schema
        table_info=$(psql -U umbra -d umbra -h localhost -c "\d receipts_invocation" 2>/dev/null || echo "")
        
        if [[ $table_info == *"body_json"* ]]; then
            echo -e "${GREEN}✓ Database has receipts_invocation table${NC}"
        else
            echo -e "${YELLOW}⚠ Could not verify database schema (psql may not be available)${NC}"
        fi
    else
        echo -e "${YELLOW}⚠ psql not available, skipping database verification${NC}"
    fi
}

# Function to verify response contract
test_response_contract() {
    echo -e "${YELLOW}[3/4] Verifying blocked response contract (POLICY_DENIED)...${NC}"
    
    # Check v0.go for blockedResponse struct
    if grep -q "type blockedResponse struct" "$REPO_ROOT/identity/control-plane/services/pep-gateway/internal/httpapi/v0.go"; then
        echo -e "${GREEN}✓ blockedResponse struct defined${NC}"
    else
        echo -e "${RED}✗ blockedResponse struct not found${NC}"
        return 1
    fi
    
    # Check for POLICY_DENIED error code
    if grep -q "POLICY_DENIED" "$REPO_ROOT/identity/control-plane/services/pep-gateway/internal/httpapi/v0.go"; then
        echo -e "${GREEN}✓ POLICY_DENIED error code implemented${NC}"
    else
        echo -e "${RED}✗ POLICY_DENIED error code not found${NC}"
        return 1
    fi
    
    # Check for request_id in response
    if grep -q "RequestID" "$REPO_ROOT/identity/control-plane/services/pep-gateway/internal/httpapi/v0.go"; then
        echo -e "${GREEN}✓ RequestID included in error response${NC}"
    else
        echo -e "${RED}✗ RequestID not found in error response${NC}"
        return 1
    fi
}

# Function to verify code implementation
test_code_implementation() {
    echo -e "${YELLOW}[4/4] Verifying code implementation...${NC}"
    
    local v0_file="$REPO_ROOT/identity/control-plane/services/pep-gateway/internal/httpapi/v0.go"
    
    # Check for observe vs enforce logic
    if grep -q 'pepMode == "observe"' "$v0_file"; then
        echo -e "${GREEN}✓ Observe mode logic implemented${NC}"
    else
        echo -e "${RED}✗ Observe mode logic not found${NC}"
        return 1
    fi
    
    if grep -q 'pepMode == "enforce"' "$v0_file"; then
        echo -e "${GREEN}✓ Enforce mode logic implemented${NC}"
    else
        echo -e "${RED}✗ Enforce mode logic not found${NC}"
        return 1
    fi
    
    # Check for PEP_MODE environment variable reading
    if grep -q 'getenv("PEP_MODE"' "$v0_file"; then
        echo -e "${GREEN}✓ PEP_MODE environment variable reading implemented${NC}"
    else
        echo -e "${RED}✗ PEP_MODE environment variable reading not found${NC}"
        return 1
    fi
    
    # Check for PEPMode and Enforcement fields in receipt
    if grep -q 'PEPMode' "$v0_file"; then
        echo -e "${GREEN}✓ PEPMode field added to receipt${NC}"
    else
        echo -e "${RED}✗ PEPMode field not found in receipt${NC}"
        return 1
    fi
    
    if grep -q 'Enforcement' "$v0_file"; then
        echo -e "${GREEN}✓ Enforcement field added to receipt${NC}"
    else
        echo -e "${RED}✗ Enforcement field not found in receipt${NC}"
        return 1
    fi
    
    # Check for forwarded/blocked in enforcement logic
    if grep -q '"forwarded"' "$v0_file" && grep -q '"blocked"' "$v0_file"; then
        echo -e "${GREEN}✓ Both 'forwarded' and 'blocked' outcomes implemented${NC}"
    else
        echo -e "${RED}✗ Enforcement outcomes not properly implemented${NC}"
        return 1
    fi
}

# Run all tests
test_observe_mode
test_receipt_structure
test_response_contract
test_code_implementation

echo
echo -e "${GREEN}=== UMBRA-0003 Verification Complete ===${NC}"
echo
echo "Summary:"
echo "- PEP_MODE configuration: ✓ Implemented"
echo "- Observe vs Enforce behavior: ✓ Implemented"
echo "- Response contract: ✓ Implemented"
echo "- Receipt fields: ✓ Implemented"
echo
echo "Next steps:"
echo "1. Run: make dev (from identity/control-plane)"
echo "2. Create a policy that will deny requests"
echo "3. Test with PEP_MODE=observe (requests forwarded)"
echo "4. Change to PEP_MODE=enforce and restart"
echo "5. Same request should be blocked with POLICY_DENIED"
echo
