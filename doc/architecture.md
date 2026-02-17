# MorgoTTalk - Architecture

## Overview

MorgoTTalk — push-to-talk voice transcription desktop app. Hold a hotkey, speak, release — text is typed into the focused application. Fully offline via whisper.cpp, GPU-accelerated, cross-platform (Linux, macOS, Windows).

**Tech stack:** Go 1.25 + Wails v3 (alpha.67) + Svelte 4 + Tailwind CSS 4 + Vite 5.
whisper.cpp linked via CGO as git submodule at `third_party/whisper.cpp`.

## Directory Layout

```
morgottalk/
├── main.go                         # Entry point: services, window, tray, lifecycle
├── desktop_linux.go                # Linux .desktop file installation
├── desktop_other.go                # No-op for macOS/Windows
├── services/                       # All Go business logic
│   ├── preset.go                   # Central orchestrator (presets, recording lifecycle)
│   ├── settings.go                 # Global settings service (Wails-bound)
│   ├── models.go                   # Model catalog, HuggingFace download (Wails-bound)
│   ├── history.go                  # Transcription history (Wails-bound)
│   ├── whisper.go                  # CGO wrapper: whisper.cpp C API, inference
│   ├── audio.go                    # Microphone recording (malgo/miniaudio)
│   ├── hotkey.go                   # Global keyboard hooks (gohook)
│   ├── paste.go                    # Clipboard-based text insertion (dispatcher)
│   ├── paste_windows.go            # Windows pasting (PowerShell SendKeys)
│   ├── paste_nowin.go              # Linux/macOS pasting (ydotool/osascript)
│   ├── kblayout.go                 # Keyboard layout detection
│   ├── overlay.go                  # Recording overlay window
│   ├── cgo.go                      # CGO linker flags (static whisper.cpp + ggml)
│   ├── backend.go                  # GPU backend enumeration, DLL existence check
│   ├── backend_download.go         # Download GPU DLLs from GitHub Releases
│   ├── backend_detect_{platform}.go # GPU/runtime detection per platform
│   ├── backend_install_{platform}.go # 1-click GPU runtime installation per platform
│   └── cmdutil_{platform}.go       # Platform-specific command utilities
├── internal/
│   ├── config/
│   │   ├── config.go               # AppConfig, preset definitions, JSON persistence
│   │   └── history.go              # HistoryEntry, load/save/append/clear
│   └── i18n/
│       └── i18n.go                 # Go-side translations (tray menu, dialogs)
├── frontend/
│   ├── src/
│   │   ├── App.svelte              # Root: routes ?window= param
│   │   ├── main.ts                 # Vite entry
│   │   ├── lib/
│   │   │   └── i18n.ts             # Frontend translations (100+ keys, 9 languages)
│   │   ├── pages/
│   │   │   ├── MainPage.svelte     # Preset cards, recording controls
│   │   │   ├── HistoryPage.svelte  # Transcription history (separate window)
│   │   │   └── OverlayPage.svelte  # Vintage tube recording overlay
│   │   └── components/
│   │       ├── PresetCard.svelte   # Preset card with inline editing
│   │       ├── PresetEditor.svelte # Preset editor modal
│   │       ├── SettingsModal.svelte # Global settings UI
│   │       ├── ModelModal.svelte   # Model manager (download/delete)
│   │       ├── HotkeyCapture.svelte # Interactive hotkey capture
│   │       ├── ProgressBar.svelte  # Progress bar component
│   │       └── Toast.svelte        # Toast notifications
│   ├── bindings/                   # Auto-generated Wails bindings (do NOT edit)
│   └── package.json
├── third_party/
│   └── whisper.cpp/                # Git submodule
├── build/                          # Build resources and Taskfiles
├── doc/                            # This documentation
├── tools/                          # Build tools (Vulkan SDK headers, etc.)
├── release.bat                     # Windows production build script
├── Taskfile.yml                    # Task runner
├── go.mod / go.sum
├── CLAUDE.md                       # AI assistant instructions
└── README.md
```

## Data Flow

### Recording Lifecycle

```
User holds hotkey
    → HotkeyManager detects keydown
    → PresetService.startRecording()
        → AudioCapture.Start() (16kHz mono float32 PCM)
        → Overlay shows "recording" state

User releases hotkey
    → HotkeyManager detects keyup
    → PresetService.stopRecording()
        → AudioCapture.Stop() → PCM buffer
        → Overlay shows "processing" state
        → WhisperEngine.Transcribe(pcm) → text
        → paste.TypeText(text) → clipboard → Shift+Insert
        → Overlay hides
```

### Service Architecture

```
main.go
    ├── PresetService (orchestrator)
    │       ├── WhisperEngine (CGO → whisper.cpp)
    │       ├── AudioCapture (malgo → miniaudio)
    │       ├── HotkeyManager (gohook)
    │       └── paste (clipboard + platform input)
    ├── SettingsService (settings, microphones, backends)
    ├── ModelService (model catalog, HuggingFace download)
    └── HistoryService (transcription log)
```

### Frontend-Backend Communication

- **Wails bindings** — generated TypeScript wrappers for Go service methods
- **Events** — Go → Frontend push notifications:
  - `model:download:progress` — model download progress
  - `backend:install:progress` — GPU backend install progress
  - `preset:recording:state` — recording/processing state changes
  - `preset:transcription:result` — transcription result text

## Wails Service Binding

Services registered in `main.go`:
```go
Services: []application.Service{
    application.NewService(presetService),
    application.NewService(settingsService),
    application.NewService(historyService),
    application.NewService(modelService),
}
```

After changing Go service methods, regenerate bindings:
```bash
wails3 generate bindings
```

## Config System

Config stored as JSON at OS-standard path:
- Windows: `%APPDATA%/transcribation/config.json`
- Linux: `~/.config/transcribation/config.json`
- Portable mode: config file next to executable

History stored separately in `history.json` (same directory).

**Legacy note:** Go module path is `github.com/UberMorgott/transcribation` (legacy name). Binary and repo name is `morgottalk`.

## i18n (Two-Layer)

1. **Go-side** (`internal/i18n/i18n.go`) — tray menu items, native dialogs. 9 languages.
2. **Frontend** (`frontend/src/lib/i18n.ts`) — all UI strings, 100+ keys. 9 languages.

Supported: English, Russian, German, Spanish, French, Italian, Portuguese, Polish, Ukrainian.

## Dependencies

### Go
| Dependency | Purpose |
|---|---|
| `wails/v3 alpha.67` | Desktop GUI framework |
| `gen2brain/malgo` | Audio capture (miniaudio) |
| `robotn/gohook` | Global keyboard hooks |
| `google/uuid` | UUID generation |

### Frontend
| Dependency | Purpose |
|---|---|
| `@wailsio/runtime` | Wails frontend runtime |
| `sortablejs` | Drag-and-drop preset reordering |
| `svelte 4` | UI framework |
| `tailwindcss 4` | CSS framework |
| `vite 5` | Build tool |
