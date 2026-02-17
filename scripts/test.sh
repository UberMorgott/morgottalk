#!/usr/bin/env bash
# Run all tests that don't require CGO or hardware.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== [1/3] Go unit tests (internal/) ==="
cd "$ROOT"
go test ./internal/... -v -count=1

echo ""
echo "=== [2/3] Frontend type check ==="
cd "$ROOT/frontend"
npm run check

echo ""
echo "=== [3/3] i18n key coverage ==="
cd "$ROOT"
go run ./tools/check-i18n

echo ""
echo "All checks passed."
