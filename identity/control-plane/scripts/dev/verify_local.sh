#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "== Umbra local verify =="

command -v go >/dev/null 2>&1 || { echo "go not found"; exit 1; }
command -v node >/dev/null 2>&1 || { echo "node not found"; exit 1; }
command -v pnpm >/dev/null 2>&1 || { echo "pnpm not found (install with: npm i -g pnpm)"; exit 1; }

echo "Go:   $(go version)"
echo "Node: $(node -v)"
echo "pnpm: $(pnpm -v)"

cd "$ROOT_DIR"

echo "\n-- Go: fmt check"
# gofmt check: fail if any changes would be made
UNFORMATTED=$(gofmt -l . | grep -v '^ui/' || true)
if [[ -n "$UNFORMATTED" ]]; then
  echo "gofmt would change:";
  echo "$UNFORMATTED";
  echo "Run: make fmt";
  exit 1
fi

echo "\n-- Go: vet"
go vet ./...

echo "\n-- Go: test"
go test ./...

echo "\n-- UI: install (pnpm)"
cd "$ROOT_DIR/ui"
# prefer lockfile if present; otherwise generate it on first install
pnpm install

echo "\n-- UI: lint"
pnpm lint

echo "\n-- UI: build (Next.js)"
pnpm build

echo "\nOK: verify passed"
