package services

/*
#include <whisper.h>
#include <ggml-backend.h>
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

var backendsLoaded sync.Once

// loadGGMLBackends loads optional GPU backend DLLs from the executable directory.
// CPU backend is statically linked. GPU backends (Vulkan, CUDA, etc.) are optional DLLs.
// Safe to call multiple times (sync.Once). If no GPU DLLs are present, only CPU is used.
func loadGGMLBackends() {
	backendsLoaded.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			return
		}
		dir := filepath.Dir(exe)
		cDir := C.CString(dir)
		defer C.free(unsafe.Pointer(cDir))
		C.ggml_backend_load_all_from_path(cDir)
	})
}

// loadBackendDLL loads a single GPU backend DLL by path (e.g. after downloading it).
// Returns true if the backend was successfully loaded and registered.
func loadBackendDLL(dllPath string) bool {
	cPath := C.CString(dllPath)
	defer C.free(unsafe.Pointer(cPath))
	reg := C.ggml_backend_load(cPath)
	return reg != nil
}

// WhisperEngine wraps a whisper.cpp model context.
type WhisperEngine struct {
	ctx *C.struct_whisper_context
	mu  sync.Mutex
}

// NewWhisperEngine loads a GGML model file and returns an engine ready for transcription.
// backend: "auto", "cpu", "cuda", "vulkan", "metal".
func NewWhisperEngine(modelPath string, backend string) (*WhisperEngine, error) {
	loadGGMLBackends()

	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))

	useGPU := backendUseGPU(backend)
	params := C.whisper_context_default_params()
	params.use_gpu = C.bool(useGPU)
	// flash_attn disabled: padding calculation depends on GGML_USE_CUDA/METAL compile flags.
	params.flash_attn = C.bool(false)
	ctx := C.whisper_init_from_file_with_params(cPath, params)
	if ctx == nil {
		return nil, fmt.Errorf("failed to load whisper model: %s", modelPath)
	}

	return &WhisperEngine{ctx: ctx}, nil
}

// IsMultilingual returns true if the loaded model supports multiple languages.
func (w *WhisperEngine) IsMultilingual() bool {
	return C.whisper_is_multilingual(w.ctx) != 0
}

// WhisperLanguages returns all languages supported by whisper.cpp library.
// First entry is always {"auto", "Auto-detect"}.
func WhisperLanguages() []LanguageInfo {
	maxID := int(C.whisper_lang_max_id())
	langs := make([]LanguageInfo, 0, maxID+2)
	langs = append(langs, LanguageInfo{Code: "auto", Name: "Auto-detect"})
	for i := 0; i <= maxID; i++ {
		code := C.GoString(C.whisper_lang_str(C.int(i)))
		name := C.GoString(C.whisper_lang_str_full(C.int(i)))
		if code == "" {
			continue
		}
		// Capitalize first letter of the name
		if len(name) > 0 {
			name = strings.ToUpper(name[:1]) + name[1:]
		}
		langs = append(langs, LanguageInfo{Code: code, Name: name})
	}
	return langs
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
// onProgress is called after each chunk with (current, total) chunk indices (1-based).
func (w *WhisperEngine) TranscribeLong(samples []float32, lang string, translate bool, onProgress func(current, total int)) (string, error) {
	totalChunks := (len(samples) + chunkSamples - 1) / chunkSamples
	if totalChunks <= 1 {
		if onProgress != nil {
			onProgress(1, 1)
		}
		text, err := w.Transcribe(samples, lang, translate)
		if err != nil {
			return "", err
		}
		return cleanWhisperOutput(text), nil
	}

	var parts []string
	chunk := 0
	for i := 0; i < len(samples); i += chunkSamples {
		chunk++
		end := i + chunkSamples
		if end > len(samples) {
			end = len(samples)
		}
		if onProgress != nil {
			onProgress(chunk, totalChunks)
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

// Whisper outputs noise markers as [MUSIC], [музыка], [音楽], etc.
// In a push-to-talk tool, bracketed markers are never real speech — strip them all.
var whisperNoiseRe = regexp.MustCompile(`\[[^\[\]]+\]|\((?i:music|noise|silence|blank.?audio|laughter|applause)\)`)

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
