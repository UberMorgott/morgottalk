# MorgoTTalk

Push-to-talk voice transcription for your desktop. One portable binary, fully offline, GPU accelerated.

Press a hotkey, speak, release — your speech appears as text in any application. No cloud, no API keys, no internet required.

## Features

- **Push-to-talk** — hold or toggle a global hotkey to record, text is typed automatically on release
- **Preset system** — multiple profiles with different models, hotkeys, languages, and input modes
- **GPU acceleration** — CUDA (NVIDIA), Vulkan, Metal (macOS), ROCm (AMD), OpenCL — auto-detected with one-click install
- **Offline** — everything runs locally via [whisper.cpp](https://github.com/ggerganov/whisper.cpp), no data leaves your machine
- **Cross-platform** — Linux, macOS, Windows
- **Text output** — types directly into any focused app, or pastes via clipboard
- **Auto language** — detects your keyboard layout and selects transcription language automatically
- **Model manager** — download whisper models from HuggingFace directly in the app
- **History** — searchable log of all transcriptions
- **9 UI languages** — English, Russian, German, Spanish, French, Chinese, Japanese, Portuguese, Korean
- **System tray** — minimize to tray, keep listening in the background
- **Dark & light themes**

## Quick Start

### Linux

1. Install dependencies:
   ```bash
   # Arch / CachyOS
   sudo pacman -S webkit2gtk-4.1 gtk3

   # Ubuntu / Debian
   sudo apt install libwebkit2gtk-4.1-dev libgtk-3-dev

   # Fedora
   sudo dnf install webkit2gtk4.1-devel gtk3-devel
   ```

2. Download the binary from [Releases](https://github.com/UberMorgott/morgottalk/releases), make it executable and run:
   ```bash
   chmod +x morgottalk-linux-amd64
   ./morgottalk-linux-amd64
   ```

3. On first launch, open Settings and download a model (recommended: `base-q5_1` for fast, `large-v3-turbo-q8_0` for accuracy).

4. Create a preset, set a hotkey, enable it — done.

### macOS / Windows

Pre-built binaries will be available in future releases. For now, build from source (see below).

## GPU Acceleration

MorgoTTalk automatically detects available GPU backends. Open **Settings** to see which backends are available on your system:

| Backend | Hardware | Status |
|---------|----------|--------|
| **CPU** | Any | Always available |
| **CUDA** | NVIDIA GPU | Requires [CUDA Toolkit](https://developer.nvidia.com/cuda-downloads) |
| **Vulkan** | Any modern GPU | Requires Vulkan runtime |
| **Metal** | Apple GPU | Built into macOS |
| **ROCm** | AMD GPU | Requires [ROCm runtime](https://rocm.docs.amd.com/) |
| **OpenCL** | Any GPU | Requires OpenCL runtime |

Grey backends can be installed with one click from the Settings panel.

> **Note**: The pre-built Linux binary includes CUDA support. For other GPU backends, build from source with the appropriate tags.

## Build from Source

### Prerequisites

- Go 1.22+
- Node.js 18+
- GCC / Clang (C/C++ compiler)
- [Wails v3](https://v3.wails.io/) CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- cmake
- **Linux**: `webkit2gtk-4.1`, `gtk3` development packages
- **macOS**: Xcode Command Line Tools
- **Windows**: MSVC or MinGW, WebView2 runtime

### Build

```bash
# 1. Clone with submodules
git clone --recurse-submodules https://github.com/UberMorgott/morgottalk.git
cd morgottalk

# 2. Build whisper.cpp (one-time)
cmake -S third_party/whisper.cpp -B third_party/whisper.cpp/build_go \
  -DBUILD_SHARED_LIBS=OFF
cmake --build third_party/whisper.cpp/build_go

# For NVIDIA GPU acceleration, add:
#   -DGGML_CUDA=ON -DCMAKE_CUDA_ARCHITECTURES="60;70;75;80;86;89;90;100;120"
# For Vulkan: -DGGML_VULKAN=ON

# 3. Build frontend
cd frontend && npm install && npm run build && cd ..

# 4. Generate bindings
wails3 generate bindings

# 5. Build binary
CGO_ENABLED=1 go build -o morgottalk .

# With GPU tags:
# CGO_ENABLED=1 go build -tags cuda -o morgottalk .
# CGO_ENABLED=1 go build -tags vulkan -o morgottalk .
# CGO_ENABLED=1 go build -tags "cuda vulkan" -o morgottalk .
```

## How It Works

1. You assign a global hotkey to a preset (e.g., `Ctrl`, `Ctrl+Shift+F1`, etc.)
2. **Hold mode**: hold the hotkey to record, release to transcribe
3. **Toggle mode**: press once to start, press again to stop
4. Audio is captured at 16kHz mono, chunked into 25-second segments
5. whisper.cpp transcribes each chunk (on GPU if available)
6. Result is typed into the currently focused application via system text input

## Text Input Methods

| Platform | Method |
|----------|--------|
| Linux (Wayland) | ydotool, wtype (fallback) |
| Linux (X11) | xdotool |
| macOS | osascript (AppleScript) |
| Windows | SendKeys (PowerShell) |

If direct typing fails, text is copied to clipboard automatically.

## License

MIT
