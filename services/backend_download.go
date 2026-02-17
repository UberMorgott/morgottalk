package services

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	// Base URL for pre-compiled GPU backend libraries hosted on GitHub Releases.
	// Each release tag contains platform-specific files: ggml-{backend}-{os}-{arch}.{ext}
	backendReleaseBase = "https://github.com/UberMorgott/morgottalk/releases/download"
	backendReleaseTag  = "gpu-v1"
)

// backendLibName returns the expected library filename for a backend on the current platform.
// Must match what ggml_backend_load_all_from_path() scans for.
func backendLibName(id string) string {
	switch runtime.GOOS {
	case "windows":
		return "ggml-" + id + ".dll"
	case "darwin":
		return "libggml-" + id + ".dylib"
	default:
		return "libggml-" + id + ".so"
	}
}

// backendDownloadURL returns the full GitHub Release URL for a backend library.
func backendDownloadURL(id string) string {
	var ext string
	switch runtime.GOOS {
	case "windows":
		ext = "dll"
	case "darwin":
		ext = "dylib"
	default:
		ext = "so"
	}
	filename := fmt.Sprintf("ggml-%s-%s-%s.%s", id, runtime.GOOS, runtime.GOARCH, ext)
	return fmt.Sprintf("%s/%s/%s", backendReleaseBase, backendReleaseTag, filename)
}

// emitBackendProgress sends a backend:install:progress event to the frontend.
func emitBackendProgress(backendID, stage, stageText string, pct float64, done bool, errMsg string) {
	if app := application.Get(); app != nil {
		app.Event.Emit("backend:install:progress", map[string]any{
			"backendId": backendID,
			"stage":     stage,
			"stageText": stageText,
			"percent":   pct,
			"done":      done,
			"error":     errMsg,
		})
	}
}

// downloadBackendDLL downloads a GPU backend library from GitHub Releases
// and places it next to the executable. Reports progress via events.
func downloadBackendDLL(backendID string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find executable path: %w", err)
	}
	destDir := filepath.Dir(exe)
	destFile := filepath.Join(destDir, backendLibName(backendID))
	tmpFile := destFile + ".tmp"

	url := backendDownloadURL(backendID)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d from %s", resp.StatusCode, url)
	}

	f, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}
	defer func() {
		f.Close()
		os.Remove(tmpFile)
	}()

	total := resp.ContentLength
	buf := make([]byte, 64*1024)
	var loaded int64
	var lastPct float64

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := f.Write(buf[:n]); wErr != nil {
				return wErr
			}
			loaded += int64(n)
			if total > 0 {
				pct := float64(loaded) / float64(total) * 100
				if pct-lastPct >= 1 || readErr == io.EOF {
					emitBackendProgress(backendID, "downloading", "", pct, false, "")
					lastPct = pct
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	f.Close()

	if err := os.Rename(tmpFile, destFile); err != nil {
		return fmt.Errorf("cannot place library: %w", err)
	}

	// Hot-load the backend into ggml so it's available immediately.
	if loadBackendDLL(destFile) {
		log.Printf("GPU backend %q loaded from %s", backendID, destFile)
	} else {
		log.Printf("GPU backend %q downloaded but failed to load from %s", backendID, destFile)
	}

	return nil
}

// onBackendInstalled is called after a backend DLL is downloaded and loaded.
// It flushes whisper engine caches so they recreate with GPU, and auto-switches the backend setting.
var onBackendInstalled func(backendID string)

// SetOnBackendInstalled registers a callback for post-install actions (cache flush, setting switch).
func SetOnBackendInstalled(fn func(backendID string)) {
	onBackendInstalled = fn
}
