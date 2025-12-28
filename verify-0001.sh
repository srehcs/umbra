#!/bin/bash
# Verification script for UMBRA-0001 policy validation

set -e

echo "=========================================="
echo "UMBRA-0001 Policy Validation Verification"
echo "=========================================="
echo ""

echo "1. Testing policy validator package..."
cd identity/control-plane
go test -v ./packages/go/policy/ || exit 1
echo "✓ Policy validator tests passed"
echo ""

echo "2. Compiling controlplane-api..."
go build ./services/controlplane-api/cmd/server/ || exit 1
echo "✓ API server compiled successfully"
echo ""

echo "3. Testing HTTP API endpoints..."
go test -v ./services/controlplane-api/internal/httpapi/ || exit 1
echo "✓ HTTP API tests passed"
echo ""

echo "4. Running lint on modified files..."
go vet ./packages/go/policy/ ./services/controlplane-api/internal/httpapi/ || exit 1
echo "✓ No lint issues found"
echo ""

echo "=========================================="
echo "All verification tests passed!"
echo "=========================================="
echo ""
echo "Key changes:"
echo "  - packages/go/policy/validator.go: Policy validation logic"
echo "  - packages/go/policy/validator_test.go: Comprehensive unit tests"
echo "  - services/controlplane-api/internal/httpapi/v0.go: API integration"
echo "  - services/controlplane-api/internal/httpapi/v0_test.go: API tests"
echo "  - docs/policy-validation.md: Validation documentation"
echo ""
