# Build Guide

## Prerequisites

- Go 1.25+
- Node.js 18+
- GCC (MinGW on Windows, gcc/clang on Linux/macOS)
- cmake
- Wails v3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

### Platform-specific

- **Windows:** MinGW (w64devkit recommended), WebView2 runtime (usually pre-installed)
- **Linux:** `webkit2gtk-4.1`, `gtk3` dev packages
- **macOS:** Xcode Command Line Tools

## Step 1: Clone with Submodules

```bash
git clone --recurse-submodules https://github.com/UberMorgott/morgottalk.git
cd morgottalk
```

## Step 2: Build whisper.cpp (Static, CPU-only)

This builds whisper.cpp as static libraries linked into the Go binary. CPU backend is included, GPU backends are optional DLLs downloaded on demand.

### Windows (MinGW)

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

### Linux

```bash
cmake -S third_party/whisper.cpp -B third_party/whisper.cpp/build_static \
  -DBUILD_SHARED_LIBS=OFF \
  -DGGML_BACKEND_DL=OFF \
  -DGGML_NATIVE=OFF \
  -DGGML_CPU_ALL_VARIANTS=ON

cmake --build third_party/whisper.cpp/build_static -j$(nproc)
```

### macOS

```bash
cmake -S third_party/whisper.cpp -B third_party/whisper.cpp/build_static \
  -DBUILD_SHARED_LIBS=OFF \
  -DGGML_BACKEND_DL=OFF \
  -DGGML_NATIVE=OFF \
  -DGGML_CPU_ALL_VARIANTS=ON \
  -DGGML_BLAS=ON -DGGML_BLAS_VENDOR=Apple

cmake --build third_party/whisper.cpp/build_static -j$(nproc)
```

Output: `third_party/whisper.cpp/build_static/src/libwhisper.a` + `ggml/src/ggml*.a`

## Step 3: Build Frontend

```bash
cd frontend && npm install && npm run build && cd ..
```

## Step 4: Generate Wails Bindings

Only needed after changing Go service method signatures:

```bash
wails3 generate bindings
```

## Step 5: Build Binary

### Debug

```bash
CGO_ENABLED=1 go build -o morgottalk .
# Windows: morgottalk.exe
```

### Production (Windows)

```bash
CGO_ENABLED=1 go build -ldflags="-s -w -H windowsgui" -o morgottalk.exe .
```

Result: ~18 MB single executable. No DLLs needed for CPU transcription.

### Using release.bat (Windows)

```bash
release.bat
```

Creates `release/` directory with production binary.

## Dev Mode (Hot Reload)

```bash
wails3 dev -config ./build/config.yml -port 9245
```

Or via Taskfile:
```bash
task dev
```

## Building GPU Backend DLLs

GPU backend DLLs are built separately and uploaded to GitHub Releases. Users download them on demand via the Settings UI.

See [gpu-backends.md](gpu-backends.md) for detailed instructions.

### Quick: Vulkan on Windows

```bash
cmake -S third_party/whisper.cpp -B build_vulkan \
  -G "MinGW Makefiles" \
  -DBUILD_SHARED_LIBS=ON -DGGML_BACKEND_DL=ON \
  -DGGML_NATIVE=OFF -DGGML_CPU_ALL_VARIANTS=ON \
  -DGGML_OPENMP=OFF -DGGML_VULKAN=ON \
  -DVulkan_INCLUDE_DIR=tools/vulkan/include \
  -DVulkan_LIBRARY=tools/vulkan/lib/libvulkan.a \
  -DVulkan_GLSLC_EXECUTABLE=tools/vulkan/bin/glslc.exe

cmake --build build_vulkan -j$(nproc)
# Result: build_vulkan/bin/ggml-vulkan.dll (~57 MB)

# Upload to release
cp build_vulkan/bin/ggml-vulkan.dll ggml-vulkan-windows-amd64.dll
gh release upload gpu-v1 ggml-vulkan-windows-amd64.dll --clobber
```

### Quick: CUDA on Windows (requires full CUDA Toolkit with nvcc)

```bash
cmake -S third_party/whisper.cpp -B build_cuda \
  -G "MinGW Makefiles" \
  -DBUILD_SHARED_LIBS=ON -DGGML_BACKEND_DL=ON \
  -DGGML_NATIVE=OFF -DGGML_CPU_ALL_VARIANTS=ON \
  -DGGML_OPENMP=OFF -DGGML_CUDA=ON

cmake --build build_cuda -j$(nproc)
# Result: build_cuda/bin/ggml-cuda.dll

cp build_cuda/bin/ggml-cuda.dll ggml-cuda-windows-amd64.dll
gh release upload gpu-v1 ggml-cuda-windows-amd64.dll --clobber
```

## Key Notes

- **No build tags needed.** GPU backends are loaded dynamically from DLLs at runtime.
- **MinGW, not MSVC.** Windows CI uses `-G "MinGW Makefiles"`, not Visual Studio.
- **OpenMP disabled on Windows.** `-DGGML_OPENMP=OFF` — avoids libgomp dependency.
- **Static C++ runtime on Windows.** `-static-libgcc -static-libstdc++` in cgo.go.
- **`-l:ggml.a` syntax.** cmake produces `ggml.a` (no `lib` prefix), so standard `-lggml` doesn't find it.
- **Vulkan SDK headers** are vendored in `tools/vulkan/` for Windows builds.
