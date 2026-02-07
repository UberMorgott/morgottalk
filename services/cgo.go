package services

// CGO flags for linking whisper.cpp static libraries (base: CPU backend).
// GPU backend flags are in cgo_cuda.go, cgo_vulkan.go, cgo_metal.go.

// #cgo CFLAGS: -I${SRCDIR}/../third_party/whisper.cpp/include -I${SRCDIR}/../third_party/whisper.cpp/ggml/include
// #cgo LDFLAGS: -L${SRCDIR}/../third_party/whisper.cpp/build_go/src -L${SRCDIR}/../third_party/whisper.cpp/build_go/ggml/src
// #cgo LDFLAGS: -lwhisper -lggml -lggml-base -lggml-cpu -lm
// #cgo linux LDFLAGS: -lstdc++ -fopenmp
// #cgo windows LDFLAGS: -lstdc++ -fopenmp
// #cgo darwin LDFLAGS: -lc++
import "C"
