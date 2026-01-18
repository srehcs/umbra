#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
spec="$root_dir/docs/api/openapi.yaml"

grep -q "signature_alg" "$spec"
grep -q "signature_kid" "$spec"
grep -q "signature:" "$spec"
grep -q "signed_at" "$spec"

echo "Signature fields present in OpenAPI schema."
