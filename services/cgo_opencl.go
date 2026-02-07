//go:build opencl

package services

// #cgo LDFLAGS: -L${SRCDIR}/../third_party/whisper.cpp/build_go/ggml/src/ggml-opencl
// #cgo LDFLAGS: -lggml-opencl -lOpenCL
import "C"
