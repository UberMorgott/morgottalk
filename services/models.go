package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/UberMorgott/transcribation/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const baseURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/"

// ModelInfo describes a whisper model.
type ModelInfo struct {
	Name        string `json:"name"`
	FileName    string `json:"fileName"`
	Size        string `json:"size"`
	SizeBytes   int64  `json:"sizeBytes"`
	Downloaded  bool   `json:"downloaded"`
	Description string `json:"description"`
	Languages   int    `json:"languages"`
	Speed       int    `json:"speed"`
	Quality     int    `json:"quality"`
	EnglishOnly bool   `json:"englishOnly"`
	Translation bool   `json:"translation"`
	Category    string `json:"category"` // "fast"/"balanced"/"quality"/""
}

// DownloadProgress is emitted as a Wails event during model download.
type DownloadProgress struct {
	ModelName   string  `json:"modelName"`
	BytesLoaded int64   `json:"bytesLoaded"`
	BytesTotal  int64   `json:"bytesTotal"`
	Percent     float64 `json:"percent"`
	Done        bool    `json:"done"`
	Error       string  `json:"error,omitempty"`
}

type modelCatalogEntry struct {
	Name        string
	SizeBytes   int64
	SizeLabel   string
	Family      string // "tiny", "base", "small", "medium", "large-v3", "large-v3-turbo"
	Quantized   string // "", "q5_0", "q5_1", "q8_0"
	EnglishOnly bool
	Languages   int // number of supported languages (99 for multilingual, 1 for .en)
	Speed       int // 1-5 rating (5 = fastest)
	Quality     int // 1-5 rating (5 = best)
	Translation bool
	Category    string // "fast", "balanced", "quality" — for onboarding
}

var catalog = []modelCatalogEntry{
	// tiny family: Speed 5, Quality 1
	{Name: "tiny", SizeBytes: 77_700_000, SizeLabel: "78 MB", Family: "tiny", Speed: 5, Quality: 1, Languages: 99, Translation: true},
	{Name: "tiny-q5_1", SizeBytes: 47_500_000, SizeLabel: "48 MB", Family: "tiny", Quantized: "q5_1", Speed: 5, Quality: 1, Languages: 99, Translation: true, Category: "fast"},
	{Name: "tiny-q8_0", SizeBytes: 42_200_000, SizeLabel: "42 MB", Family: "tiny", Quantized: "q8_0", Speed: 5, Quality: 1, Languages: 99, Translation: true},
	{Name: "tiny.en", SizeBytes: 77_700_000, SizeLabel: "78 MB", Family: "tiny", EnglishOnly: true, Speed: 5, Quality: 1, Languages: 1},
	{Name: "tiny.en-q5_1", SizeBytes: 47_500_000, SizeLabel: "48 MB", Family: "tiny", Quantized: "q5_1", EnglishOnly: true, Speed: 5, Quality: 1, Languages: 1},
	{Name: "tiny.en-q8_0", SizeBytes: 42_200_000, SizeLabel: "42 MB", Family: "tiny", Quantized: "q8_0", EnglishOnly: true, Speed: 5, Quality: 1, Languages: 1},
	// base family: Speed 4, Quality 2
	{Name: "base", SizeBytes: 147_500_000, SizeLabel: "148 MB", Family: "base", Speed: 4, Quality: 2, Languages: 99, Translation: true},
	{Name: "base-q5_1", SizeBytes: 57_400_000, SizeLabel: "57 MB", Family: "base", Quantized: "q5_1", Speed: 4, Quality: 2, Languages: 99, Translation: true, Category: "balanced"},
	{Name: "base-q8_0", SizeBytes: 78_200_000, SizeLabel: "78 MB", Family: "base", Quantized: "q8_0", Speed: 4, Quality: 2, Languages: 99, Translation: true},
	{Name: "base.en", SizeBytes: 147_500_000, SizeLabel: "148 MB", Family: "base", EnglishOnly: true, Speed: 4, Quality: 2, Languages: 1},
	{Name: "base.en-q5_1", SizeBytes: 57_400_000, SizeLabel: "57 MB", Family: "base", Quantized: "q5_1", EnglishOnly: true, Speed: 4, Quality: 2, Languages: 1},
	{Name: "base.en-q8_0", SizeBytes: 78_200_000, SizeLabel: "78 MB", Family: "base", Quantized: "q8_0", EnglishOnly: true, Speed: 4, Quality: 2, Languages: 1},
	// small family: Speed 3, Quality 3
	{Name: "small", SizeBytes: 488_000_000, SizeLabel: "488 MB", Family: "small", Speed: 3, Quality: 3, Languages: 99, Translation: true},
	{Name: "small-q5_1", SizeBytes: 190_000_000, SizeLabel: "190 MB", Family: "small", Quantized: "q5_1", Speed: 3, Quality: 3, Languages: 99, Translation: true},
	{Name: "small-q8_0", SizeBytes: 259_000_000, SizeLabel: "259 MB", Family: "small", Quantized: "q8_0", Speed: 3, Quality: 3, Languages: 99, Translation: true},
	{Name: "small.en", SizeBytes: 488_000_000, SizeLabel: "488 MB", Family: "small", EnglishOnly: true, Speed: 3, Quality: 3, Languages: 1},
	{Name: "small.en-q5_1", SizeBytes: 190_000_000, SizeLabel: "190 MB", Family: "small", Quantized: "q5_1", EnglishOnly: true, Speed: 3, Quality: 3, Languages: 1},
	{Name: "small.en-q8_0", SizeBytes: 259_000_000, SizeLabel: "259 MB", Family: "small", Quantized: "q8_0", EnglishOnly: true, Speed: 3, Quality: 3, Languages: 1},
	// medium family: Speed 2, Quality 4
	{Name: "medium", SizeBytes: 1_533_000_000, SizeLabel: "1.5 GB", Family: "medium", Speed: 2, Quality: 4, Languages: 99, Translation: true},
	{Name: "medium-q5_0", SizeBytes: 539_000_000, SizeLabel: "539 MB", Family: "medium", Quantized: "q5_0", Speed: 2, Quality: 4, Languages: 99, Translation: true},
	{Name: "medium-q8_0", SizeBytes: 812_000_000, SizeLabel: "812 MB", Family: "medium", Quantized: "q8_0", Speed: 2, Quality: 4, Languages: 99, Translation: true},
	{Name: "medium.en", SizeBytes: 1_533_000_000, SizeLabel: "1.5 GB", Family: "medium", EnglishOnly: true, Speed: 2, Quality: 4, Languages: 1},
	{Name: "medium.en-q5_0", SizeBytes: 539_000_000, SizeLabel: "539 MB", Family: "medium", Quantized: "q5_0", EnglishOnly: true, Speed: 2, Quality: 4, Languages: 1},
	{Name: "medium.en-q8_0", SizeBytes: 812_000_000, SizeLabel: "812 MB", Family: "medium", Quantized: "q8_0", EnglishOnly: true, Speed: 2, Quality: 4, Languages: 1},
	// large-v3: Speed 1, Quality 5
	{Name: "large-v3", SizeBytes: 3_094_000_000, SizeLabel: "3.1 GB", Family: "large-v3", Speed: 1, Quality: 5, Languages: 99, Translation: true},
	{Name: "large-v3-q5_0", SizeBytes: 1_080_000_000, SizeLabel: "1.1 GB", Family: "large-v3", Quantized: "q5_0", Speed: 1, Quality: 5, Languages: 99, Translation: true},
	// large-v3-turbo: Speed 3, Quality 5
	{Name: "large-v3-turbo", SizeBytes: 1_623_000_000, SizeLabel: "1.6 GB", Family: "large-v3-turbo", Speed: 3, Quality: 5, Languages: 99, Translation: true},
	{Name: "large-v3-turbo-q5_0", SizeBytes: 574_000_000, SizeLabel: "574 MB", Family: "large-v3-turbo", Quantized: "q5_0", Speed: 3, Quality: 5, Languages: 99, Translation: true, Category: "quality"},
	{Name: "large-v3-turbo-q8_0", SizeBytes: 862_000_000, SizeLabel: "862 MB", Family: "large-v3-turbo", Quantized: "q8_0", Speed: 3, Quality: 5, Languages: 99, Translation: true},
}

// modelDescription generates a human-readable description from catalog metadata.
func modelDescription(e modelCatalogEntry) string {
	stars := func(n int) string {
		s := ""
		for i := 0; i < 5; i++ {
			if i < n {
				s += "\u2605" // ★
			} else {
				s += "\u2606" // ☆
			}
		}
		return s
	}

	desc := fmt.Sprintf("Speed: %s | Quality: %s", stars(e.Speed), stars(e.Quality))

	if e.EnglishOnly {
		desc += " | English only"
	} else {
		desc += fmt.Sprintf(" | %d languages", e.Languages)
	}

	if e.Translation {
		desc += " | Translation \u2713"
	}

	// Use case hint based on family
	switch e.Family {
	case "tiny":
		desc += " \u2014 Fastest option, works on CPU"
	case "base":
		desc += " \u2014 Good balance for simple dictation"
	case "small":
		desc += " \u2014 Balanced speed and quality"
	case "medium":
		desc += " \u2014 High quality, slower"
	case "large-v3":
		desc += " \u2014 Best quality, slowest"
	case "large-v3-turbo":
		desc += " \u2014 Near-best quality, 4x faster than large"
	}

	if e.Quantized != "" {
		desc += fmt.Sprintf(" (%s quantized)", e.Quantized)
	}

	return desc
}

func isValidModelName(name string) bool {
	for _, c := range catalog {
		if c.Name == name {
			return true
		}
	}
	return false
}

// ModelService manages whisper model files.
type ModelService struct {
	mu          sync.Mutex
	downloading map[string]context.CancelFunc
}

func NewModelService() *ModelService {
	return &ModelService{
		downloading: make(map[string]context.CancelFunc),
	}
}

// GetAvailableModels returns the full catalog with download status.
func (s *ModelService) GetAvailableModels() []ModelInfo {
	dir := s.ResolveModelsDir()

	var models []ModelInfo
	for _, c := range catalog {
		fileName := "ggml-" + c.Name + ".bin"
		downloaded := false
		if info, err := os.Stat(filepath.Join(dir, fileName)); err == nil && info.Size() > 0 {
			downloaded = true
		}
		models = append(models, ModelInfo{
			Name:        c.Name,
			FileName:    fileName,
			Size:        c.SizeLabel,
			SizeBytes:   c.SizeBytes,
			Downloaded:  downloaded,
			Description: modelDescription(c),
			Languages:   c.Languages,
			Speed:       c.Speed,
			Quality:     c.Quality,
			EnglishOnly: c.EnglishOnly,
			Translation: c.Translation,
			Category:    c.Category,
		})
	}
	return models
}

// GetModelsDir returns the current models directory path.
func (s *ModelService) GetModelsDir() string {
	return s.ResolveModelsDir()
}

// ResolveModelsDir determines the models directory.
func (s *ModelService) ResolveModelsDir() string {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("failed to load config: %v", err)
		cfg = &config.AppConfig{}
	}
	if cfg.ModelsDir != "" {
		os.MkdirAll(cfg.ModelsDir, 0o755)
		return cfg.ModelsDir
	}

	// Default: directory of the executable + /models
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Join(filepath.Dir(exe), "models")
		if err := os.MkdirAll(dir, 0o755); err == nil {
			return dir
		}
	}

	// Fallback: XDG data dir
	return xdgModelsDir()
}

func xdgModelsDir() string {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
	case "darwin":
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "Library", "Application Support")
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".local", "share")
		}
	}
	dir := filepath.Join(base, "transcribation", "models")
	os.MkdirAll(dir, 0o755)
	return dir
}

// DownloadModel downloads a model from HuggingFace with progress events.
func (s *ModelService) DownloadModel(name string) error {
	if !isValidModelName(name) {
		return fmt.Errorf("unknown model name: %s", name)
	}
	s.mu.Lock()
	if _, exists := s.downloading[name]; exists {
		s.mu.Unlock()
		return fmt.Errorf("already downloading %s", name)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.downloading[name] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("recovered panic in downloadWorker: %v", r)
			}
		}()
		s.downloadWorker(ctx, name)
	}()
	return nil
}

func (s *ModelService) downloadWorker(ctx context.Context, name string) {
	defer func() {
		s.mu.Lock()
		delete(s.downloading, name)
		s.mu.Unlock()
	}()

	fileName := "ggml-" + name + ".bin"
	url := baseURL + fileName
	dir := s.ResolveModelsDir()
	destPath := filepath.Join(dir, fileName)
	tmpPath := destPath + ".tmp"

	emit := func(p DownloadProgress) {
		app := application.Get()
		if app != nil {
			app.Event.Emit("model:download:progress", p)
		}
	}

	// Resume support: check if a partial temp file exists.
	var resumeOffset int64
	if info, err := os.Stat(tmpPath); err == nil && info.Size() > 0 {
		resumeOffset = info.Size()
		log.Printf("Model %s: found partial download (%d bytes), attempting resume", name, resumeOffset)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		emit(DownloadProgress{ModelName: name, Done: true, Error: err.Error()})
		return
	}

	if resumeOffset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeOffset))
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		emit(DownloadProgress{ModelName: name, Done: true, Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	var total int64
	var f *os.File

	switch resp.StatusCode {
	case http.StatusPartialContent:
		// Server supports Range — append to existing temp file.
		total = resumeOffset + resp.ContentLength
		f, err = os.OpenFile(tmpPath, os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			emit(DownloadProgress{ModelName: name, Done: true, Error: err.Error()})
			return
		}
		log.Printf("Model %s: resuming from %d / %d bytes", name, resumeOffset, total)

	case http.StatusOK:
		// Server does not support Range or resumeOffset was 0 — start fresh.
		if resumeOffset > 0 {
			log.Printf("Model %s: server does not support Range, restarting download", name)
		}
		resumeOffset = 0
		total = resp.ContentLength
		f, err = os.Create(tmpPath)
		if err != nil {
			emit(DownloadProgress{ModelName: name, Done: true, Error: err.Error()})
			return
		}

	default:
		emit(DownloadProgress{ModelName: name, Done: true, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)})
		return
	}

	loaded := resumeOffset
	buf := make([]byte, 64*1024)
	lastEmit := int64(0)

	// Emit initial progress immediately so the frontend knows the download started.
	emit(DownloadProgress{
		ModelName:   name,
		BytesLoaded: loaded,
		BytesTotal:  total,
		Percent:     0,
	})

	for {
		select {
		case <-ctx.Done():
			f.Close()
			// Keep the temp file for future resume (don't remove).
			emit(DownloadProgress{ModelName: name, Done: true, Error: "cancelled"})
			log.Printf("Model %s: download cancelled, %d bytes saved for resume", name, loaded)
			return
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				f.Close()
				os.Remove(tmpPath)
				emit(DownloadProgress{ModelName: name, Done: true, Error: writeErr.Error()})
				return
			}
			loaded += int64(n)

			// Emit progress every ~500KB
			if loaded-lastEmit > 500*1024 || readErr == io.EOF {
				pct := float64(0)
				if total > 0 {
					pct = float64(loaded) / float64(total) * 100
				}
				emit(DownloadProgress{
					ModelName:   name,
					BytesLoaded: loaded,
					BytesTotal:  total,
					Percent:     pct,
				})
				lastEmit = loaded
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			// Keep partial file for resume on network errors.
			log.Printf("Model %s: download interrupted at %d bytes: %v", name, loaded, readErr)
			emit(DownloadProgress{ModelName: name, Done: true, Error: readErr.Error()})
			return
		}
	}

	f.Close()

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		emit(DownloadProgress{ModelName: name, Done: true, Error: err.Error()})
		return
	}

	log.Printf("Model downloaded: %s (%d bytes)", destPath, loaded)
	emit(DownloadProgress{
		ModelName:   name,
		BytesLoaded: loaded,
		BytesTotal:  total,
		Percent:     100,
		Done:        true,
	})
}

// CancelDownload cancels an in-progress download.
func (s *ModelService) CancelDownload(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cancel, ok := s.downloading[name]; ok {
		cancel()
	}
}

// DeleteModel removes a downloaded model file.
func (s *ModelService) DeleteModel(name string) error {
	if !isValidModelName(name) {
		return fmt.Errorf("unknown model name: %s", name)
	}
	dir := s.ResolveModelsDir()
	fileName := "ggml-" + name + ".bin"
	path := filepath.Join(dir, fileName)
	return os.Remove(path)
}

// SetModelsDir changes the models directory and optionally moves existing models.
func (s *ModelService) SetModelsDir(newDir string, moveModels bool) error {
	oldDir := s.ResolveModelsDir()

	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return fmt.Errorf("cannot create directory: %w", err)
	}

	if moveModels && oldDir != newDir {
		entries, err := os.ReadDir(oldDir)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".bin") {
					src := filepath.Join(oldDir, e.Name())
					dst := filepath.Join(newDir, e.Name())
					if err := os.Rename(src, dst); err != nil {
						log.Printf("Failed to move %s: %v", e.Name(), err)
					}
				}
			}
		}
	}

	cfg, err := config.Load()
	if err != nil {
		log.Printf("failed to load config: %v", err)
		cfg = &config.AppConfig{}
	}
	cfg.ModelsDir = newDir
	return config.Save(cfg)
}
