//go:build cuda

package services

// #cgo LDFLAGS: -L${SRCDIR}/../third_party/whisper.cpp/build_go/ggml/src/ggml-cuda
// #cgo LDFLAGS: -lggml-cuda
// #cgo LDFLAGS: -L/opt/cuda/lib64 -lcudart -lcublas -lcublasLt -lcuda
import "C"
