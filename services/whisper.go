package services

/*
#include <whisper.h>
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

// WhisperEngine wraps a whisper.cpp model context.
type WhisperEngine struct {
	ctx *C.struct_whisper_context
	mu  sync.Mutex
}

// NewWhisperEngine loads a GGML model file and returns an engine ready for transcription.
// backend: "auto", "cpu", "cuda", "vulkan", "metal".
func NewWhisperEngine(modelPath string, backend string) (*WhisperEngine, error) {
	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))

	useGPU := backendUseGPU(backend)
	params := C.whisper_context_default_params()
	params.use_gpu = C.bool(useGPU)
	params.flash_attn = C.bool(useGPU)
	ctx := C.whisper_init_from_file_with_params(cPath, params)
	if ctx == nil {
		return nil, fmt.Errorf("failed to load whisper model: %s", modelPath)
	}

	return &WhisperEngine{ctx: ctx}, nil
}

// Transcribe runs speech-to-text on float32 PCM samples (16 kHz, mono).
// lang: language code ("en", "ru", "auto"), translate: translate to English.
func (w *WhisperEngine) Transcribe(samples []float32, lang string, translate bool) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.ctx == nil {
		return "", fmt.Errorf("whisper engine not initialized")
	}
	if len(samples) == 0 {
		return "", nil
	}

	params := C.whisper_full_default_params(C.WHISPER_SAMPLING_GREEDY)
	params.print_progress = C.bool(false)
	params.print_special = C.bool(false)
	params.print_realtime = C.bool(false)
	params.print_timestamps = C.bool(false)
	params.single_segment = C.bool(false)
	params.no_context = C.bool(true)

	nThreads := runtime.NumCPU()
	if nThreads > 8 {
		nThreads = 8
	}
	params.n_threads = C.int(nThreads)

	if translate {
		params.translate = C.bool(true)
	}

	if lang != "" && lang != "auto" {
		cLang := C.CString(lang)
		defer C.free(unsafe.Pointer(cLang))
		params.language = cLang
	} else {
		cAuto := C.CString("auto")
		defer C.free(unsafe.Pointer(cAuto))
		params.language = cAuto
	}

	ret := C.whisper_full(w.ctx, params, (*C.float)(unsafe.Pointer(&samples[0])), C.int(len(samples)))
	if ret != 0 {
		return "", fmt.Errorf("whisper_full failed with code %d", int(ret))
	}

	nSegments := int(C.whisper_full_n_segments(w.ctx))
	var result string
	for i := 0; i < nSegments; i++ {
		text := C.GoString(C.whisper_full_get_segment_text(w.ctx, C.int(i)))
		result += text
	}

	return result, nil
}

const chunkSeconds = 25
const chunkSamples = chunkSeconds * 16000

// TranscribeLong splits long audio into chunks for reliable transcription.
func (w *WhisperEngine) TranscribeLong(samples []float32, lang string, translate bool) (string, error) {
	if len(samples) <= chunkSamples {
		text, err := w.Transcribe(samples, lang, translate)
		if err != nil {
			return "", err
		}
		return cleanWhisperOutput(text), nil
	}

	var parts []string
	for i := 0; i < len(samples); i += chunkSamples {
		end := i + chunkSamples
		if end > len(samples) {
			end = len(samples)
		}
		text, err := w.Transcribe(samples[i:end], lang, translate)
		if err != nil {
			continue
		}
		text = cleanWhisperOutput(text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " "), nil
}

var whisperNoiseRe = regexp.MustCompile(`(?i)\[(BLANK_AUDIO|MUSIC|NOISE|SILENCE)\]|\((?:music|noise|silence|blank audio)\)`)

// cleanWhisperOutput removes whisper noise markers but keeps all real text.
func cleanWhisperOutput(text string) string {
	text = whisperNoiseRe.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)
	return text
}

// Close frees the whisper context.
func (w *WhisperEngine) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.ctx != nil {
		C.whisper_free(w.ctx)
		w.ctx = nil
	}
}
