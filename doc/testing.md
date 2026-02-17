# Testing Guide

## Test Tiers

### Tier 1 — Always Runnable (no CGO, no hardware)

```bash
./scripts/test.sh
```

Or manually:
```bash
go test ./internal/... -v -count=1
cd frontend && npm run check
go run ./tools/check-i18n
```

**What's covered:**
- `internal/config` — DefaultPreset, DefaultAppConfig, migrateOldConfig (old→new format migration), AppConfig JSON roundtrip, history CRUD (append, delete, clear, max entries trim)
- `internal/i18n` — T() fallback chain (exact key, unknown language→English, missing key→key string), all backend translations present in all 9 languages
- Frontend TypeScript — all `.svelte` files type-checked via `svelte-check`
- Frontend i18n.ts — all 9 languages have identical key sets (via `tools/check-i18n`)

### Tier 2 — Dev Machine (CGO + built whisper.cpp)

```bash
./scripts/test-all.sh
```

Requires whisper.cpp to be built first:
```bash
cmake -S third_party/whisper.cpp -B third_party/whisper.cpp/build_static -DBUILD_SHARED_LIBS=OFF -DGGML_OPENMP=OFF
cmake --build third_party/whisper.cpp/build_static
```

**What's covered:**
- `services/kblayout.go` — parseDBusSendLayouts (dbus output parsing), macInputSourceToCode (macOS input source mapping), layoutToLang map completeness
- `services/backend.go` — backendUseGPU logic, cudaBackend/vulkanBackend with mock gpuDetection structs (no_hardware, no_runtime, etc.)

### What Is NOT Tested

| Component | Reason |
|-----------|--------|
| Microphone capture | Requires real hardware |
| Whisper transcription | Requires model file + CGO at runtime |
| Hotkey detection | Requires live X11/Wayland/Windows event loop |
| Text pasting | Requires ydotool/wtype/system utilities |
| GPU detection | Reads /proc/driver, lspci, system_profiler |
| Wails window/tray | Requires running Wails app instance |
| Model download | Network + disk I/O, flaky in CI |

## CI Gate

`.github/workflows/test.yml` runs Tier 1 tests on every push and pull request. It does NOT require whisper.cpp or CGO — keeps CI fast (~2 min).

## Tools

### `tools/check-i18n`

Validates frontend i18n.ts key consistency:
```bash
go run ./tools/check-i18n
go run ./tools/check-i18n --path frontend/src/lib/i18n.ts
```

Reports missing/extra keys per language vs English. Exit 1 if discrepancies found.

## Adding Tests

- **Pure Go functions** → add to the appropriate `_test.go` in `internal/`
- **Service pure logic** → add to `services/*_test.go` (note: CGO required to compile)
- **New i18n keys** → `tools/check-i18n` catches missing translations automatically
- **Hardware-dependent code** → don't unit test, verify manually
