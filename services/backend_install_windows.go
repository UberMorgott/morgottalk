//go:build windows

package services

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const cudaNetworkInstaller = "https://developer.download.nvidia.com/compute/cuda/13.1.1/network_installers/cuda_13.1.1_windows_network.exe"

// Only runtime components needed for whisper.cpp inference.
// cudart = CUDA Runtime, cublas = cuBLAS (matrix ops used by whisper.cpp).
const cudaComponents = "-s cudart_13.1 cublas_13.1"

func installBackend(id string) (string, error) {
	switch id {
	case "cuda":
		go installBackendAsync(id)
		return "installing", nil
	case "vulkan":
		go installBackendAsync(id)
		return "installing", nil
	case "rocm":
		return openURL("https://rocm.docs.amd.com/")
	default:
		return "", fmt.Errorf("backend %q is not available on Windows", id)
	}
}

// installBackendAsync handles the full async installation flow:
// 1. Install system runtime if needed (CUDA only)
// 2. Download the GPU backend DLL from GitHub Releases
func installBackendAsync(id string) {
	emit := func(stage, stageText string, pct float64, done bool, errMsg string) {
		emitBackendProgress(id, stage, stageText, pct, done, errMsg)
	}

	// Step 1: Install system runtime if needed.
	if id == "cuda" {
		det := detectGPU()
		if !det.CUDAAvailable {
			if err := installCUDARuntimeWindows(emit); err != nil {
				emit("", "", 0, true, err.Error())
				return
			}
		}
	}

	// Step 2: Download the backend DLL.
	emit("downloading", "", 0, false, "")
	if err := downloadBackendDLL(id); err != nil {
		emit("", "", 0, true, fmt.Sprintf("Backend download failed: %v", err))
		return
	}

	// Step 3: Hot-apply — flush engine caches and switch backend.
	if onBackendInstalled != nil {
		onBackendInstalled(id)
	}

	emit("", "", 100, true, "")
}

// installCUDARuntimeWindows downloads and silently installs CUDA runtime components.
func installCUDARuntimeWindows(emit func(stage, stageText string, pct float64, done bool, errMsg string)) error {
	installerPath := filepath.Join(os.TempDir(), "cuda_13.1.1_windows_network.exe")

	// Download network installer (~30 MB).
	err := downloadFileWithProgress(cudaNetworkInstaller, installerPath, func(pct float64) {
		emit("downloading_runtime", "", pct, false, "")
	})
	if err != nil {
		return fmt.Errorf("download CUDA installer: %w", err)
	}

	// Silent install with log monitoring.
	emit("installing_runtime", "", 0, false, "")

	logDone := make(chan struct{})
	go watchCUDAInstallerLog(logDone, func(text string) {
		emit("installing_runtime", text, 0, false, "")
	})

	// Escape single quotes for PowerShell single-quoted strings ('' = literal ').
	escapedPath := strings.ReplaceAll(installerPath, "'", "''")
	escapedArgs := strings.ReplaceAll(cudaComponents, "'", "''")
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		fmt.Sprintf(`Start-Process -FilePath '%s' -ArgumentList '%s' -Verb RunAs -Wait`,
			escapedPath, escapedArgs))
	hideWindow(cmd)
	err = cmd.Run()

	close(logDone)
	os.Remove(installerPath)

	if err != nil {
		return fmt.Errorf("CUDA install failed: %w", err)
	}

	refreshCUDAEnv()
	return nil
}

// watchCUDAInstallerLog tails the CUDA installer log file and reports stage changes.
// The NVIDIA installer writes logs to %TEMP% in various locations.
func watchCUDAInstallerLog(done <-chan struct{}, onStage func(text string)) {
	tempDir := os.TempDir()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var logPath string
	var lastSize int64
	var lastStage string

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			// Find log file if not found yet.
			if logPath == "" {
				logPath = findCUDALog(tempDir)
				if logPath == "" {
					continue
				}
			}

			// Check if file grew.
			info, err := os.Stat(logPath)
			if err != nil {
				continue
			}
			if info.Size() <= lastSize {
				continue
			}

			// Read new content.
			stage := parseCUDALogStage(logPath, lastSize)
			lastSize = info.Size()

			if stage != "" && stage != lastStage {
				lastStage = stage
				onStage(stage)
			}
		}
	}
}

// findCUDALog searches for the CUDA installer log file in common locations.
func findCUDALog(tempDir string) string {
	patterns := []string{
		filepath.Join(tempDir, "cuda_install*.log"),
		filepath.Join(tempDir, "CUDA_Install*.log"),
		filepath.Join(tempDir, "CUDA", "*.log"),
		filepath.Join(tempDir, "NVIDIA", "*.log"),
		filepath.Join(tempDir, "cuda*.log"),
	}

	var newest string
	var newestTime time.Time

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			if info, err := os.Stat(m); err == nil {
				if info.ModTime().After(newestTime) {
					newestTime = info.ModTime()
					newest = m
				}
			}
		}
	}
	return newest
}

// parseCUDALogStage reads the log from offset and extracts the last recognizable stage.
func parseCUDALogStage(logPath string, offset int64) string {
	f, err := os.Open(logPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	if offset > 0 {
		f.Seek(offset, io.SeekStart)
	}

	data, err := io.ReadAll(f)
	if err != nil || len(data) == 0 {
		return ""
	}

	// Scan lines bottom-up for the last meaningful stage.
	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)

		// Map known keywords to user-friendly descriptions.
		switch {
		case strings.Contains(lower, "compatibility") || strings.Contains(lower, "system check"):
			return "Checking compatibility... (this takes a while!)"
		case strings.Contains(lower, "extract"):
			return "Extracting..."
		case strings.Contains(lower, "download"):
			return "Downloading components..."
		case strings.Contains(lower, "install") && strings.Contains(lower, "cudart"):
			return "Installing CUDA Runtime..."
		case strings.Contains(lower, "install") && strings.Contains(lower, "cublas"):
			return "Installing cuBLAS..."
		case strings.Contains(lower, "install") && strings.Contains(lower, "driver"):
			return "Installing driver..."
		case strings.Contains(lower, "configur"):
			return "Configuring..."
		case strings.Contains(lower, "verif"):
			return "Verifying..."
		case strings.Contains(lower, "clean"):
			return "Cleaning up..."
		case strings.Contains(lower, "complete") || strings.Contains(lower, "success"):
			return "Finishing..."
		case strings.Contains(lower, "running package"):
			// "Running package: cublas_13.1" → extract package name
			if idx := strings.Index(lower, "running package"); idx != -1 {
				rest := strings.TrimSpace(line[idx+len("running package"):])
				rest = strings.TrimLeft(rest, ":= ")
				if rest != "" {
					return "Installing " + rest + "..."
				}
			}
			return "Installing package..."
		}
	}
	return ""
}

func downloadFileWithProgress(url, dest string, onProgress func(pct float64)) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 64*1024)
	var loaded int64
	var lastEmitPct float64

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := f.Write(buf[:n]); wErr != nil {
				return wErr
			}
			loaded += int64(n)

			if total > 0 {
				pct := float64(loaded) / float64(total) * 100
				if pct-lastEmitPct >= 1 || readErr == io.EOF {
					onProgress(pct)
					lastEmitPct = pct
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
	return nil
}

func openURL(url string) (string, error) {
	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start(); err != nil {
		return "", err
	}
	return "url", nil
}
