package services

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// BackendInfo describes a compute backend and its availability.
type BackendInfo struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Compiled          bool   `json:"compiled"`          // backend DLL present next to exe
	SystemAvailable   bool   `json:"systemAvailable"`   // hardware + runtime present
	CanInstall        bool   `json:"canInstall"`        // hardware exists but runtime missing
	InstallHint       string `json:"installHint"`       // e.g. "CUDA Toolkit"
	UnavailableReason string `json:"unavailableReason"` // "" | "no_hardware" | "no_driver" | "no_runtime" | "not_compiled"
	GPUDetected       string `json:"gpuDetected"`       // e.g. "NVIDIA RTX 5070 Ti", ""
	Recommended       bool   `json:"recommended"`       // best backend for detected hardware
	DownloadSizeMB    int    `json:"downloadSizeMB"`    // approximate DLL download size, 0 = unknown
}

// gpuDetection holds the results of platform-specific GPU/runtime detection.
type gpuDetection struct {
	HasNVIDIA       bool
	NVIDIAModel     string // "NVIDIA RTX 5070 Ti", ""
	CUDAAvailable   bool
	VulkanAvailable bool
	HasAMD          bool
	AMDModel        string // "AMD Radeon RX 7900", ""
	ROCmAvailable   bool
	PackageManager  string // "pacman", "apt", "dnf", "zypper", ""
}

// backendDLLExists checks if a backend DLL/SO/dylib exists next to the executable.
func backendDLLExists(name string) bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	dir := filepath.Dir(exe)

	// Platform-specific library name patterns.
	var patterns []string
	switch runtime.GOOS {
	case "windows":
		patterns = []string{
			"ggml-" + name + ".dll",
			"ggml-" + name + "-*.dll", // e.g. ggml-cuda-sm75.dll
		}
	case "darwin":
		patterns = []string{
			"libggml-" + name + ".dylib",
		}
	default: // linux
		patterns = []string{
			"libggml-" + name + ".so",
		}
	}

	for _, p := range patterns {
		if strings.Contains(p, "*") {
			matches, _ := filepath.Glob(filepath.Join(dir, p))
			if len(matches) > 0 {
				return true
			}
		} else {
			if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
				return true
			}
		}
	}
	return false
}

// GetAllBackends returns ALL known backends with their availability status.
func GetAllBackends() []BackendInfo {
	det := detectGPU()
	backends := []BackendInfo{
		{ID: "auto", Name: "Auto", Compiled: true, SystemAvailable: true},
		{ID: "cpu", Name: "CPU", Compiled: true, SystemAvailable: true},
		cudaBackend(det),
		vulkanBackend(det),
		metalBackend(det),
	}

	// Mark recommended backend based on detected hardware.
	var recID string
	switch {
	case runtime.GOOS == "darwin":
		recID = "metal"
	case det.HasNVIDIA:
		recID = "cuda"
	case det.VulkanAvailable:
		recID = "vulkan"
	}
	for i := range backends {
		if backends[i].ID == recID {
			backends[i].Recommended = true
		}
		// Fill download sizes for backends that can be downloaded.
		if !backends[i].Compiled && backends[i].CanInstall {
			backends[i].DownloadSizeMB = backendDownloadSize(backends[i].ID)
		}
	}

	return backends
}

func cudaBackend(det gpuDetection) BackendInfo {
	hasDLL := backendDLLExists("cuda")
	info := BackendInfo{
		ID: "cuda", Name: "CUDA",
		Compiled: hasDLL,
	}

	if !det.HasNVIDIA {
		info.UnavailableReason = "no_hardware"
		return info
	}

	info.GPUDetected = det.NVIDIAModel

	if !det.CUDAAvailable {
		info.UnavailableReason = "no_runtime"
		info.CanInstall = true
		info.InstallHint = "CUDA Toolkit"
		return info
	}

	// Runtime is present on the system.
	if hasDLL {
		info.SystemAvailable = true
	} else {
		info.SystemAvailable = true
		info.UnavailableReason = "not_compiled"
		info.CanInstall = true
		info.InstallHint = "cuda_driver_525"
	}

	return info
}

func vulkanBackend(det gpuDetection) BackendInfo {
	hasDLL := backendDLLExists("vulkan")
	info := BackendInfo{
		ID: "vulkan", Name: "Vulkan",
		Compiled: hasDLL,
	}

	if !det.VulkanAvailable {
		info.UnavailableReason = "no_runtime"
		info.CanInstall = true
		info.InstallHint = "Vulkan ICD Loader"
		return info
	}

	// Runtime is present on the system.
	if hasDLL {
		info.SystemAvailable = true
	} else {
		info.SystemAvailable = true
		info.UnavailableReason = "not_compiled"
		info.CanInstall = true
	}

	return info
}

func metalBackend(det gpuDetection) BackendInfo {
	hasDLL := backendDLLExists("metal")
	available := runtime.GOOS == "darwin" && hasDLL
	return BackendInfo{
		ID: "metal", Name: "Metal",
		Compiled: available, SystemAvailable: available,
		CanInstall: false,
	}
}


func backendDownloadSize(id string) int {
	sizes := map[string]map[string]int{
		"cuda":   {"windows": 150, "linux": 200},
		"vulkan": {"windows": 57, "linux": 70},
		"metal":  {"darwin": 5},
	}
	if m, ok := sizes[id]; ok {
		if sz, ok := m[runtime.GOOS]; ok {
			return sz
		}
	}
	return 0
}

// backendUseGPU returns whether a given backend should enable GPU acceleration.
func backendUseGPU(backend string) bool {
	return backend != "cpu"
}
