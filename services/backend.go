package services

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Minimum driver versions for GPU backends.
const (
	cudaMinNVIDIADriver   = "525.60"
	vulkanMinNVIDIADriver = "450.0"
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
	RuntimeInstalled  bool   `json:"runtimeInstalled"`  // true if system runtime (CUDA/Vulkan) is present
	DriverVersion     string `json:"driverVersion"`     // parsed driver version e.g. "560.81", "" if unknown
	DriverOK          bool   `json:"driverOK"`          // true if driver meets minimum requirements
}

// gpuInfo describes a single detected GPU.
type gpuInfo struct {
	Name          string // "NVIDIA GeForce RTX 4070"
	Vendor        string // "nvidia", "amd", "intel"
	DriverVersion string // parsed driver version e.g. "560.81", "" if unknown
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
	GPUs            []gpuInfo
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

	// Populate GPU info — prefer multi-GPU names, fallback to NVIDIAModel.
	if names := gpuNamesByVendor(det, "nvidia"); names != "" {
		info.GPUDetected = names
	} else {
		info.GPUDetected = det.NVIDIAModel
	}

	// Check driver version.
	driverVer := gpuDriverByVendor(det, "nvidia")
	info.DriverVersion = driverVer
	if driverVer != "" {
		if compareDriverVersion(driverVer, cudaMinNVIDIADriver) >= 0 {
			info.DriverOK = true
		} else {
			info.UnavailableReason = "no_driver"
			info.CanInstall = false
			info.InstallHint = "NVIDIA driver >= " + cudaMinNVIDIADriver
			return info
		}
	}

	if !det.CUDAAvailable {
		info.UnavailableReason = "no_runtime"
		info.CanInstall = true
		info.InstallHint = "CUDA Toolkit"
		return info
	}

	// Runtime is present on the system.
	info.RuntimeInstalled = true
	info.SystemAvailable = true
	if !hasDLL {
		info.UnavailableReason = "not_compiled"
		info.CanInstall = true
	}

	return info
}

func vulkanBackend(det gpuDetection) BackendInfo {
	hasDLL := backendDLLExists("vulkan")
	info := BackendInfo{
		ID: "vulkan", Name: "Vulkan",
		Compiled: hasDLL,
	}

	// Populate GPU info — Vulkan works with any vendor.
	var gpuNames []string
	for _, g := range det.GPUs {
		if g.Name != "" {
			gpuNames = append(gpuNames, g.Name)
		}
	}
	if len(gpuNames) > 0 {
		info.GPUDetected = strings.Join(gpuNames, ", ")
	}

	// Check driver version for NVIDIA (Vulkan needs modern driver).
	if det.HasNVIDIA {
		driverVer := gpuDriverByVendor(det, "nvidia")
		info.DriverVersion = driverVer
		if driverVer != "" {
			if compareDriverVersion(driverVer, vulkanMinNVIDIADriver) >= 0 {
				info.DriverOK = true
			} else {
				info.UnavailableReason = "no_driver"
				info.CanInstall = false
				info.InstallHint = "NVIDIA driver >= " + vulkanMinNVIDIADriver
				return info
			}
		}
	} else {
		// AMD/Intel — any modern driver supports Vulkan; mark OK if GPU detected.
		for _, g := range det.GPUs {
			if g.DriverVersion != "" {
				info.DriverVersion = g.DriverVersion
				info.DriverOK = true
				break
			}
		}
	}

	if !det.VulkanAvailable {
		info.UnavailableReason = "no_runtime"
		info.CanInstall = true
		info.InstallHint = "Vulkan ICD Loader"
		return info
	}

	// Runtime is present on the system.
	info.RuntimeInstalled = true
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


// compareDriverVersion compares two dot-separated version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareDriverVersion(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}
	for i := 0; i < maxLen; i++ {
		var va, vb int
		if i < len(partsA) {
			va, _ = strconv.Atoi(partsA[i])
		}
		if i < len(partsB) {
			vb, _ = strconv.Atoi(partsB[i])
		}
		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
	}
	return 0
}

// gpuDriverByVendor returns the driver version of the first GPU matching the given vendor.
func gpuDriverByVendor(det gpuDetection, vendor string) string {
	for _, g := range det.GPUs {
		if g.Vendor == vendor {
			return g.DriverVersion
		}
	}
	return ""
}

// gpuNamesByVendor returns a comma-separated list of GPU names for the given vendor.
func gpuNamesByVendor(det gpuDetection, vendor string) string {
	var names []string
	for _, g := range det.GPUs {
		if g.Vendor == vendor && g.Name != "" {
			names = append(names, g.Name)
		}
	}
	return strings.Join(names, ", ")
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
