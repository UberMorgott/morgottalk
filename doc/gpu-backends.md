# GPU Backend System

## Philosophy

One binary supports ALL GPU backends. CPU backend is statically linked into the binary. GPU backends are separate DLL/SO/DYLIB files loaded at runtime.

User flow: open Settings → click GPU backend → DLL auto-downloads from GitHub Releases → hot-loaded without restart.

## How It Works

### Static vs Dynamic

```
Binary (18 MB):
  ├── whisper.cpp (statically linked)
  ├── ggml core (statically linked)
  └── ggml-cpu (statically linked, all CPU variants)

Optional DLLs next to exe (downloaded on demand):
  ├── ggml-vulkan.dll     (~57 MB, any modern GPU)
  ├── ggml-cuda.dll        (NVIDIA GPUs)
  ├── ggml-rocm.dll        (AMD GPUs, Linux only)
  └── ggml-opencl.dll      (cross-vendor, Linux/macOS)
```

### Build Configuration

whisper.cpp built with static libraries (no shared DLLs needed at runtime for CPU):

```bash
cmake -S third_party/whisper.cpp -B third_party/whisper.cpp/build_static \
  -G "MinGW Makefiles" \
  -DBUILD_SHARED_LIBS=OFF \
  -DGGML_BACKEND_DL=OFF \
  -DGGML_NATIVE=OFF \
  -DGGML_CPU_ALL_VARIANTS=ON \
  -DGGML_OPENMP=OFF

cmake --build third_party/whisper.cpp/build_static -j$(nproc)
```

Output: `libwhisper.a`, `ggml.a`, `ggml-base.a`, `ggml-cpu.a`

CGO flags in `services/cgo.go`:
```go
// #cgo LDFLAGS: -L${SRCDIR}/../third_party/whisper.cpp/build_static/src
// #cgo LDFLAGS: -L${SRCDIR}/../third_party/whisper.cpp/build_static/ggml/src
// #cgo LDFLAGS: -lwhisper -l:ggml.a -l:ggml-cpu.a -l:ggml-base.a -lm
```

Note: `-l:ggml.a` syntax (with colon) is required because cmake produces `ggml.a` without `lib` prefix.

### GPU DLL Build (separate cmake, shared libs)

Example: Vulkan on Windows:
```bash
cmake -S third_party/whisper.cpp -B build_vulkan_dll \
  -G "MinGW Makefiles" \
  -DBUILD_SHARED_LIBS=ON \
  -DGGML_BACKEND_DL=ON \
  -DGGML_NATIVE=OFF \
  -DGGML_CPU_ALL_VARIANTS=ON \
  -DGGML_OPENMP=OFF \
  -DGGML_VULKAN=ON \
  -DVulkan_INCLUDE_DIR=tools/vulkan/include \
  -DVulkan_LIBRARY=tools/vulkan/lib/libvulkan.a \
  -DVulkan_GLSLC_EXECUTABLE=tools/vulkan/bin/glslc.exe

cmake --build build_vulkan_dll -j$(nproc)
```

Output: `build_vulkan_dll/bin/ggml-vulkan.dll` (~57 MB)

### GitHub Release Hosting

DLLs are uploaded to GitHub Releases at:
```
https://github.com/UberMorgott/morgottalk/releases/tag/gpu-v1
```

Naming convention: `ggml-{backend}-{os}-{arch}.{ext}`
- `ggml-vulkan-windows-amd64.dll`
- `ggml-vulkan-linux-amd64.so`
- `ggml-cuda-windows-amd64.dll`
- etc.

Important: Repository MUST be public for direct download URLs to work without authentication.

## Code Flow

### Backend Detection (`services/backend.go`)

```
GetAllBackends()
    → detectGPU()                    # Platform-specific GPU detection
    → For each backend:
        → backendDLLExists(name)     # Check if DLL file exists next to exe
        → Return BackendInfo{
            Compiled: hasDLL,        # true if DLL present
            SystemAvailable: ...,    # true if GPU runtime installed
            CanInstall: ...,         # true if we can auto-install
            UnavailableReason: ...,  # "no_hardware", "no_runtime", "not_compiled"
          }
```

### 1-Click Install Flow

```
Frontend: user clicks backend pill
    → handleBackendClick(b)
    → InstallBackend(b.id)          # Wails binding call

Go: installBackend(id)              # Platform-specific (build tags)
    → go installBackendAsync(id)    # Async goroutine

    Step 1: Install runtime (if needed)
        CUDA: download network installer → silent install
        Vulkan: already bundled with GPU drivers (usually)
        ROCm: package manager install (Linux)

    Step 2: Download DLL
        → downloadBackendDLL(id)
        → HTTP GET from GitHub Releases
        → Write to exe directory as ggml-{id}.{ext}
        → Progress events → frontend

    Step 3: Hot-load
        → loadBackendDLL(path)       # C: ggml_backend_load(path)
        → onBackendInstalled(id)     # Registered in main.go
            → presetService.FlushEngines()  # Close cached whisper contexts
            → config.Backend = id    # Auto-switch setting
            → config.Save()

Frontend: backend:install:progress event
    → Show progress ring on backend pill
    → On done: auto-switch localBackend, refresh backend list
    → No restart needed!
```

### Hot-Reload Mechanism

When a GPU DLL is downloaded:

1. **`loadBackendDLL(path)`** — calls `ggml_backend_load(path)` C API. Registers the backend in ggml's backend registry. Returns true if successful.

2. **`FlushEngines()`** — closes all cached `WhisperEngine` instances. They hold ggml contexts that reference the old backend set. Next transcription will create a new engine with the GPU backend available.

3. **Config auto-switch** — `config.Backend` set to new backend ID, saved to JSON.

4. **Frontend auto-switch** — `localBackend = d.backendId` in the event handler. Settings auto-save fires.

### Platform-Specific Details

| Platform | CUDA | Vulkan | Metal | ROCm | OpenCL |
|----------|------|--------|-------|------|--------|
| Windows | Network installer (silent) | DLL download only | N/A | Manual (URL) | Driver-bundled |
| Linux | Package manager (apt/dnf/pacman + NVIDIA repo) | Package manager + DLL | N/A | Package manager + DLL | Package manager + DLL |
| macOS | N/A | brew install MoltenVK + DLL | Statically linked | N/A | Driver-bundled |

### Backend State Machine

```
not_compiled (DLL missing, runtime present)
    → [user clicks] → installing
    → [download + hot-load] → compiled + systemAvailable

no_runtime (GPU present, runtime missing)
    → [user clicks] → installing
    → [install runtime + download DLL + hot-load] → compiled + systemAvailable

no_hardware (no compatible GPU detected)
    → [disabled, not clickable]
```

## Files Reference

| File | Purpose |
|------|---------|
| `services/cgo.go` | CGO linker flags (static whisper.cpp + ggml) |
| `services/whisper.go` | `loadGGMLBackends()`, `loadBackendDLL()`, whisper engine |
| `services/backend.go` | `GetAllBackends()`, `backendDLLExists()`, backend info structs |
| `services/backend_download.go` | `downloadBackendDLL()`, GitHub Release URLs, progress events |
| `services/backend_detect_{platform}.go` | GPU hardware/runtime detection |
| `services/backend_install_{platform}.go` | Runtime installation + DLL download orchestration |
| `services/preset.go` | `FlushEngines()` for hot-reload |
| `main.go` | `SetOnBackendInstalled()` callback registration |
| `frontend/.../SettingsModal.svelte` | Backend pills UI, install progress, auto-switch |

## Uploading New GPU DLLs

To add a new backend DLL to the release:

```bash
# Build the DLL (example: CUDA on Windows with full CUDA Toolkit)
cmake -S third_party/whisper.cpp -B build_cuda_dll \
  -G "MinGW Makefiles" \
  -DBUILD_SHARED_LIBS=ON -DGGML_BACKEND_DL=ON \
  -DGGML_NATIVE=OFF -DGGML_CPU_ALL_VARIANTS=ON \
  -DGGML_OPENMP=OFF -DGGML_CUDA=ON
cmake --build build_cuda_dll -j$(nproc)

# Rename to convention
cp build_cuda_dll/bin/ggml-cuda.dll ggml-cuda-windows-amd64.dll

# Upload to existing release
gh release upload gpu-v1 ggml-cuda-windows-amd64.dll --clobber
```

To bump version (e.g., after whisper.cpp update):
1. Update `backendReleaseTag` in `services/backend_download.go`
2. Create new release tag: `gh release create gpu-v2`
3. Upload all DLLs
