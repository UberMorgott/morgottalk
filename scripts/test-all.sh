#!/usr/bin/env bash
# Run all tests including service-level (requires CGO + built whisper.cpp).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Run Tier 1 first
"$ROOT/scripts/test.sh"

echo ""
echo "=== [4/4] Service tests (CGO) ==="
cd "$ROOT"
CGO_ENABLED=1 go test ./services/... -v -count=1 -run "^Test(ParseDBusSend|MacInputSource|LayoutToLang|BackendUseGPU|CudaBackend|VulkanBackend)"

echo ""
echo "All tests passed (including service tests)."
