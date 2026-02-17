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
	Name       string `json:"name"`
	FileName   string `json:"fileName"`
	Size       string `json:"size"`
	SizeBytes  int64  `json:"sizeBytes"`
	Downloaded bool   `json:"downloaded"`
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
	Name      string
	SizeBytes int64
	SizeLabel string
}

var catalog = []modelCatalogEntry{
	{"tiny", 77_700_000, "78 MB"},
	{"tiny-q5_1", 47_500_000, "48 MB"},
	{"tiny-q8_0", 42_200_000, "42 MB"},
	{"tiny.en", 77_700_000, "78 MB"},
	{"tiny.en-q5_1", 47_500_000, "48 MB"},
	{"tiny.en-q8_0", 42_200_000, "42 MB"},
	{"base", 147_500_000, "148 MB"},
	{"base-q5_1", 57_400_000, "57 MB"},
	{"base-q8_0", 78_200_000, "78 MB"},
	{"base.en", 147_500_000, "148 MB"},
	{"base.en-q5_1", 57_400_000, "57 MB"},
	{"base.en-q8_0", 78_200_000, "78 MB"},
	{"small", 488_000_000, "488 MB"},
	{"small-q5_1", 190_000_000, "190 MB"},
	{"small-q8_0", 259_000_000, "259 MB"},
	{"small.en", 488_000_000, "488 MB"},
	{"small.en-q5_1", 190_000_000, "190 MB"},
	{"small.en-q8_0", 259_000_000, "259 MB"},
	{"medium", 1_533_000_000, "1.5 GB"},
	{"medium-q5_0", 539_000_000, "539 MB"},
	{"medium-q8_0", 812_000_000, "812 MB"},
	{"medium.en", 1_533_000_000, "1.5 GB"},
	{"medium.en-q5_0", 539_000_000, "539 MB"},
	{"medium.en-q8_0", 812_000_000, "812 MB"},
	{"large-v3", 3_094_000_000, "3.1 GB"},
	{"large-v3-q5_0", 1_080_000_000, "1.1 GB"},
	{"large-v3-turbo", 1_623_000_000, "1.6 GB"},
	{"large-v3-turbo-q5_0", 574_000_000, "574 MB"},
	{"large-v3-turbo-q8_0", 862_000_000, "862 MB"},
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
			Name:       c.Name,
			FileName:   fileName,
			Size:       c.SizeLabel,
			SizeBytes:  c.SizeBytes,
			Downloaded: downloaded,
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

	go s.downloadWorker(ctx, name)
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

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		emit(DownloadProgress{ModelName: name, Done: true, Error: err.Error()})
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		emit(DownloadProgress{ModelName: name, Done: true, Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		emit(DownloadProgress{ModelName: name, Done: true, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)})
		return
	}

	total := resp.ContentLength

	f, err := os.Create(tmpPath)
	if err != nil {
		emit(DownloadProgress{ModelName: name, Done: true, Error: err.Error()})
		return
	}

	var loaded int64
	buf := make([]byte, 64*1024)
	lastEmit := int64(0)

	for {
		select {
		case <-ctx.Done():
			f.Close()
			os.Remove(tmpPath)
			emit(DownloadProgress{ModelName: name, Done: true, Error: "cancelled"})
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
			os.Remove(tmpPath)
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

	log.Printf("Model downloaded: %s", destPath)
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
