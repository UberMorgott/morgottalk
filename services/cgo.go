package services

// CGO flags for static linking of whisper.cpp + ggml (CPU backend compiled in).
// GPU backends can optionally be loaded at runtime if DLLs are present.

// #cgo CFLAGS: -I${SRCDIR}/../third_party/whisper.cpp/include -I${SRCDIR}/../third_party/whisper.cpp/ggml/include
// #cgo LDFLAGS: -L${SRCDIR}/../third_party/whisper.cpp/build_static/src -L${SRCDIR}/../third_party/whisper.cpp/build_static/ggml/src
// #cgo LDFLAGS: -lwhisper -l:ggml.a -l:ggml-cpu.a -l:ggml-base.a -lm
// #cgo linux LDFLAGS: -lstdc++ -lpthread
// #cgo windows LDFLAGS: -lstdc++ -static-libgcc -static-libstdc++
// #cgo darwin LDFLAGS: -lc++ -framework Accelerate
import "C"
