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
	OpenCLAvailable bool
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
	return []BackendInfo{
		{ID: "auto", Name: "Auto", Compiled: true, SystemAvailable: true},
		{ID: "cpu", Name: "CPU", Compiled: true, SystemAvailable: true},
		cudaBackend(det),
		vulkanBackend(det),
		metalBackend(det),
		rocmBackend(det),
		openclBackend(det),
	}
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

func rocmBackend(det gpuDetection) BackendInfo {
	hasDLL := backendDLLExists("rocm")
	info := BackendInfo{
		ID: "rocm", Name: "ROCm",
		Compiled: hasDLL,
	}

	if !det.HasAMD {
		info.UnavailableReason = "no_hardware"
		return info
	}

	info.GPUDetected = det.AMDModel

	if !det.ROCmAvailable {
		info.UnavailableReason = "no_runtime"
		info.CanInstall = true
		info.InstallHint = "ROCm/HIP Runtime"
		return info
	}

	if hasDLL {
		info.SystemAvailable = true
	} else {
		info.SystemAvailable = true
		info.UnavailableReason = "not_compiled"
	}

	return info
}

func openclBackend(det gpuDetection) BackendInfo {
	hasDLL := backendDLLExists("opencl")
	info := BackendInfo{
		ID: "opencl", Name: "OpenCL",
		Compiled: hasDLL,
	}

	if !det.OpenCLAvailable {
		info.UnavailableReason = "no_runtime"
		info.CanInstall = true
		info.InstallHint = "OpenCL Runtime"
		return info
	}

	if hasDLL {
		info.SystemAvailable = true
	} else {
		info.SystemAvailable = true
		info.UnavailableReason = "not_compiled"
	}

	return info
}

// backendUseGPU returns whether a given backend should enable GPU acceleration.
func backendUseGPU(backend string) bool {
	return backend != "cpu"
}
