//go:build rocm

package services

// #cgo LDFLAGS: -L${SRCDIR}/../third_party/whisper.cpp/build_go/ggml/src/ggml-cuda
// #cgo LDFLAGS: -lggml-hip
// #cgo LDFLAGS: -lamdhip64 -lhipblas -lrocblas
import "C"
