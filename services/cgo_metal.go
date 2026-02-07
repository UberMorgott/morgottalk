//go:build darwin

package services

// #cgo LDFLAGS: -L${SRCDIR}/../third_party/whisper.cpp/build_go/ggml/src/ggml-metal
// #cgo LDFLAGS: -lggml-metal -framework Metal -framework Foundation -framework MetalPerformanceShaders
import "C"
