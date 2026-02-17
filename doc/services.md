# Services Reference

## Wails-Bound Services

These services are registered in `main.go` and their public methods are exposed to the frontend via auto-generated TypeScript bindings.

### PresetService (`services/preset.go`)

Central orchestrator. Manages presets (CRUD), coordinates recording lifecycle.

**Key methods:**
- `Init()` — initialize hotkeys, load config, set up engines
- `GetPresets()` — return all presets
- `CreatePreset(name)` — create new preset with defaults
- `UpdatePreset(preset)` — update preset settings (model, hotkey, language, etc.)
- `DeletePreset(id)` — delete preset
- `ReorderPresets(ids)` — reorder preset list
- `SetPresetEnabled(id, enabled)` — enable/disable preset (registers/unregisters hotkey)
- `FlushEngines()` — close all cached whisper engines (used after GPU backend install)
- `Shutdown()` — release all resources

**Internal components held by PresetService:**
- `engines map[string]*WhisperEngine` — cached whisper engines per model
- `hotkeys *HotkeyManager` — global keyboard hooks
- `audio *AudioCapture` — microphone recording

**Locking:** `s.mu` (sync.Mutex) protects shared state. Comments in code indicate which methods require lock held.

### SettingsService (`services/settings.go`)

Global settings management.

**Key methods:**
- `SaveGlobalSettings(settings)` — save all settings to config
- `InstallBackend(id) string` — install GPU backend (returns "installing", "installed", "url")
- `GetAllBackends() []BackendInfo` — enumerate available GPU backends
- `PickModelsDir() string` — open native directory picker
- `RestartApp()` — restart application
- `GetMicrophones()` — enumerate audio input devices via malgo

### ModelService (`services/models.go`)

Whisper model catalog and download management.

**Key methods:**
- `GetModels()` — return all available models with download status
- `DownloadModel(name)` — download model from HuggingFace (async, with progress events)
- `DeleteModel(name)` — delete downloaded model file
- `GetModelsDir() string` — current models directory path

**Events emitted:** `model:download:progress` with `{name, percent, done, error}`

### HistoryService (`services/history.go`)

Transcription history persistence.

**Key methods:**
- `GetHistory()` — return all history entries
- `ClearHistory()` — delete all entries
- `OpenHistoryWindow()` — open history in separate window

## Internal Components (Not Wails-Bound)

### WhisperEngine (`services/whisper.go`)

CGO wrapper around whisper.cpp C API.

**Key functions:**
- `NewWhisperEngine(modelPath, backend) *WhisperEngine` — load GGML model
- `engine.Transcribe(pcm []float32, lang string) (string, error)` — transcribe audio
- `engine.TranscribeLong(pcm, lang)` — chunks audio into 25s segments for long recordings
- `engine.Close()` — free C resources
- `loadGGMLBackends()` — one-time init: `ggml_backend_load_all_from_path(exeDir)`
- `loadBackendDLL(path) bool` — hot-load single GPU backend via `ggml_backend_load(path)`

**Important:** `flash_attn` is disabled due to padding bug with dynamic GPU backends.

### AudioCapture (`services/audio.go`)

Microphone recording via malgo (miniaudio wrapper).

- Records 16kHz mono float32 PCM
- Configurable device ID (or system default)
- Start/Stop API, returns PCM buffer

### HotkeyManager (`services/hotkey.go`)

Global keyboard hooks via gohook library.

- Event loop processes keydown/keyup events
- Matches key combinations to preset bindings
- Supports hold mode (record while held) and toggle mode (press to start/stop)
- Key capture mode for UI hotkey assignment

### Paste (`services/paste.go`, `paste_windows.go`, `paste_nowin.go`)

Clipboard-based text insertion into focused application.

Flow:
1. Save current clipboard content
2. Write transcription text to clipboard
3. Simulate Shift+Insert (or Ctrl+V) keystroke
4. Restore original clipboard

Platform implementations:
- **Windows:** PowerShell `[System.Windows.Forms.SendKeys]`
- **Linux:** ydotool (Wayland), xdotool (X11), or wtype
- **macOS:** osascript (AppleScript)

### Backend Download (`services/backend_download.go`)

Downloads pre-compiled GPU backend DLLs from GitHub Releases.

- `backendDownloadURL(id)` — constructs URL: `{base}/{tag}/ggml-{id}-{os}-{arch}.{ext}`
- `downloadBackendDLL(id)` — download with progress events, hot-load after completion
- `emitBackendProgress(...)` — sends `backend:install:progress` event to frontend
- `onBackendInstalled` callback — registered in main.go for cache flush + config switch

### Backend Detection (`services/backend_detect_{platform}.go`)

Platform-specific GPU and runtime detection.

Returns `gpuDetection` struct:
```go
type gpuDetection struct {
    HasNVIDIA, HasAMD    bool
    NVIDIAModel, AMDModel string
    CUDAAvailable        bool
    VulkanAvailable      bool
    ROCmAvailable        bool
    OpenCLAvailable      bool
    MetalAvailable       bool   // macOS only
}
```

Detection methods:
- **Windows:** WMI queries (Win32_VideoController), registry checks, DLL existence
- **Linux:** lspci, nvidia-smi, vulkaninfo, rocminfo, clinfo
- **macOS:** system_profiler, Metal framework check

### Backend Install (`services/backend_install_{platform}.go`)

One-click GPU runtime installation.

Flow per platform:
- **Windows CUDA:** Download NVIDIA network installer → silent install with component selection
- **Windows Vulkan:** Just download DLL (Vulkan runtime comes with GPU drivers)
- **Linux:** Detect package manager → install runtime packages → download DLL
- **macOS Metal:** Statically linked (nothing to install)
- **macOS Vulkan:** brew install MoltenVK → download DLL

## Event Protocol

### backend:install:progress

```typescript
{
  backendId: string,      // "vulkan", "cuda", etc.
  stage: string,          // "downloading", "downloading_runtime", "installing_runtime", "installing", ""
  stageText: string,      // Human-readable status (e.g., "Installing cuBLAS...")
  percent: number,        // 0-100 for download stages, 0 for install stages
  done: boolean,          // true when complete (success or error)
  error: string           // Error message if failed, empty on success
}
```

Stages flow:
1. `downloading_runtime` (CUDA only) — downloading CUDA installer with percent
2. `installing_runtime` (CUDA only) — installing with stageText updates
3. `downloading` — downloading GPU backend DLL with percent
4. done=true — complete (auto-switch backend, refresh list)
