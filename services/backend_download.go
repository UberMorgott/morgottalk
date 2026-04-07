package services

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var httpClient = &http.Client{Timeout: 5 * time.Minute}

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
// Supports HTTP Range resume if a partial .tmp file exists from a previous attempt.
func downloadBackendDLL(backendID string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find executable path: %w", err)
	}
	destDir := filepath.Dir(exe)
	destFile := filepath.Join(destDir, backendLibName(backendID))
	tmpFile := destFile + ".tmp"

	url := backendDownloadURL(backendID)

	// Resume support: check if a partial temp file exists.
	var resumeOffset int64
	if info, err := os.Stat(tmpFile); err == nil && info.Size() > 0 {
		resumeOffset = info.Size()
		log.Printf("Backend %s: found partial download (%d bytes), attempting resume", backendID, resumeOffset)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	if resumeOffset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeOffset))
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	var total int64
	var f *os.File

	switch resp.StatusCode {
	case http.StatusPartialContent:
		// Server supports Range — append to existing temp file.
		total = resumeOffset + resp.ContentLength
		f, err = os.OpenFile(tmpFile, os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("cannot open temp file for resume: %w", err)
		}
		log.Printf("Backend %s: resuming from %d / %d bytes", backendID, resumeOffset, total)

	case http.StatusOK:
		// Server does not support Range or resumeOffset was 0 — start fresh.
		if resumeOffset > 0 {
			log.Printf("Backend %s: server does not support Range, restarting download", backendID)
		}
		resumeOffset = 0
		total = resp.ContentLength
		f, err = os.Create(tmpFile)
		if err != nil {
			return fmt.Errorf("cannot create file: %w", err)
		}

	default:
		return fmt.Errorf("download failed: HTTP %d from %s", resp.StatusCode, url)
	}

	loaded := resumeOffset
	buf := make([]byte, 64*1024)
	var lastPct float64

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := f.Write(buf[:n]); wErr != nil {
				f.Close()
				os.Remove(tmpFile)
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
			f.Close()
			// Keep partial file for resume on network errors.
			log.Printf("Backend %s: download interrupted at %d bytes: %v", backendID, loaded, readErr)
			return readErr
		}
	}

	f.Close()

	if err := os.Rename(tmpFile, destFile); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("cannot place library: %w", err)
	}

	log.Printf("Backend %s: download complete (%d bytes)", backendID, loaded)

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
