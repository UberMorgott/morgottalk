//go:build windows

package services

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func detectGPU() gpuDetection {
	det := gpuDetection{}

	// Query video controllers via PowerShell
	gpuNames := queryGPUNames()
	lower := strings.ToLower(gpuNames)

	// NVIDIA GPU
	det.HasNVIDIA = strings.Contains(lower, "nvidia")

	// CUDA runtime: check env var, then file system.
	if det.HasNVIDIA {
		det.CUDAAvailable = detectCUDARuntime()
	}

	// Vulkan runtime (vulkan-1.dll in system32)
	sys32 := filepath.Join(os.Getenv("SystemRoot"), "System32")
	det.VulkanAvailable = fileExists(filepath.Join(sys32, "vulkan-1.dll"))

	// AMD GPU
	det.HasAMD = strings.Contains(lower, "amd") || strings.Contains(lower, "radeon")

	// ROCm/HIP runtime
	if det.HasAMD {
		det.ROCmAvailable = os.Getenv("HIP_PATH") != ""
	}


	return det
}

// detectCUDARuntime checks if CUDA runtime is installed by looking at:
// 1. CUDA_PATH env var (fast, works if process inherited it)
// 2. Known install path on disk (works immediately after install)
// 3. Registry via PowerShell (authoritative, works always)
func detectCUDARuntime() bool {
	// 1. Env var (inherited from parent process).
	if os.Getenv("CUDA_PATH") != "" {
		return true
	}

	// 2. Check default install location for CUDA DLLs.
	if cudaPath := findCUDAOnDisk(); cudaPath != "" {
		return true
	}

	// 3. Check registry (set by CUDA installer, visible immediately).
	if cudaPath := readCUDAPathFromRegistry(); cudaPath != "" {
		return true
	}

	return false
}

// findCUDAOnDisk scans the default NVIDIA CUDA install directory.
func findCUDAOnDisk() string {
	pgf := os.Getenv("ProgramFiles")
	if pgf == "" {
		return ""
	}
	base := filepath.Join(pgf, "NVIDIA GPU Computing Toolkit", "CUDA")
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	// Pick the latest version directory (e.g. v13.1).
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.IsDir() && strings.HasPrefix(e.Name(), "v") {
			binDir := filepath.Join(base, e.Name(), "bin")
			// Check for actual CUDA runtime DLL.
			matches, _ := filepath.Glob(filepath.Join(binDir, "cudart64_*.dll"))
			if len(matches) > 0 {
				return filepath.Join(base, e.Name())
			}
		}
	}
	return ""
}

// readCUDAPathFromRegistry reads CUDA_PATH from the system environment in the registry.
func readCUDAPathFromRegistry() string {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`(Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Environment' -Name CUDA_PATH -ErrorAction SilentlyContinue).CUDA_PATH`)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// refreshCUDAEnv updates the current process environment after CUDA installation.
// This allows the app to use CUDA without restart.
func refreshCUDAEnv() {
	if os.Getenv("CUDA_PATH") != "" {
		return // Already set.
	}

	// Try registry first, then disk scan.
	cudaPath := readCUDAPathFromRegistry()
	if cudaPath == "" {
		cudaPath = findCUDAOnDisk()
	}
	if cudaPath == "" {
		return
	}

	os.Setenv("CUDA_PATH", cudaPath)
	binDir := filepath.Join(cudaPath, "bin")
	if !strings.Contains(os.Getenv("PATH"), binDir) {
		os.Setenv("PATH", os.Getenv("PATH")+";"+binDir)
	}
}

func queryGPUNames() string {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"Get-CimInstance Win32_VideoController | Select-Object -ExpandProperty Name")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
