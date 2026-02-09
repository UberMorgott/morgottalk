package services

// CGO flags for linking whisper.cpp shared libraries with dynamic backend loading.
// GPU backends are loaded at runtime as DLLs/SOs via ggml_backend_load_all().

// #cgo CFLAGS: -I${SRCDIR}/../third_party/whisper.cpp/include -I${SRCDIR}/../third_party/whisper.cpp/ggml/include
// #cgo LDFLAGS: -L${SRCDIR}/../third_party/whisper.cpp/build_go/src -L${SRCDIR}/../third_party/whisper.cpp/build_go/ggml/src
// #cgo LDFLAGS: -lwhisper -lggml -lggml-base -lm
// #cgo linux LDFLAGS: -lstdc++ -Wl,-rpath,'$ORIGIN'
// #cgo windows LDFLAGS: -lstdc++ -static-libgcc -static-libstdc++
// #cgo darwin LDFLAGS: -lc++ -Wl,-rpath,@executable_path
import "C"
