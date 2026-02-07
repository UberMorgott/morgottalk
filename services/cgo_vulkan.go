//go:build vulkan

package services

// #cgo LDFLAGS: -L${SRCDIR}/../third_party/whisper.cpp/build_go/ggml/src/ggml-vulkan
// #cgo LDFLAGS: -lggml-vulkan -lvulkan
import "C"
