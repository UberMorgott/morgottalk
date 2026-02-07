package services

// BackendInfo describes a compute backend and its availability.
type BackendInfo struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Compiled        bool   `json:"compiled"`        // binary includes this backend (build tag)
	SystemAvailable bool   `json:"systemAvailable"` // hardware + runtime present
	CanInstall      bool   `json:"canInstall"`      // hardware exists but runtime missing
	InstallHint     string `json:"installHint"`     // e.g. "CUDA Toolkit"
}

// gpuDetection holds the results of platform-specific GPU/runtime detection.
type gpuDetection struct {
	HasNVIDIA       bool
	CUDAAvailable   bool
	VulkanAvailable bool
	HasAMD          bool
	ROCmAvailable   bool
	OpenCLAvailable bool
	PackageManager  string // "pacman", "apt", "dnf", "zypper", ""
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
	available := hasCUDA && det.CUDAAvailable
	canInstall := det.HasNVIDIA && !det.CUDAAvailable
	hint := ""
	if canInstall {
		hint = "CUDA Toolkit"
	}
	return BackendInfo{
		ID: "cuda", Name: "CUDA",
		Compiled: hasCUDA, SystemAvailable: available,
		CanInstall: canInstall, InstallHint: hint,
	}
}

func vulkanBackend(det gpuDetection) BackendInfo {
	available := hasVulkan && det.VulkanAvailable
	canInstall := !det.VulkanAvailable
	hint := ""
	if canInstall {
		hint = "Vulkan ICD Loader"
	}
	return BackendInfo{
		ID: "vulkan", Name: "Vulkan",
		Compiled: hasVulkan, SystemAvailable: available,
		CanInstall: canInstall, InstallHint: hint,
	}
}

func metalBackend(det gpuDetection) BackendInfo {
	return BackendInfo{
		ID: "metal", Name: "Metal",
		Compiled: hasMetal, SystemAvailable: hasMetal,
		CanInstall: false,
	}
}

func rocmBackend(det gpuDetection) BackendInfo {
	available := hasROCm && det.ROCmAvailable
	canInstall := det.HasAMD && !det.ROCmAvailable
	hint := ""
	if canInstall {
		hint = "ROCm/HIP Runtime"
	}
	return BackendInfo{
		ID: "rocm", Name: "ROCm",
		Compiled: hasROCm, SystemAvailable: available,
		CanInstall: canInstall, InstallHint: hint,
	}
}

func openclBackend(det gpuDetection) BackendInfo {
	available := hasOpenCL && det.OpenCLAvailable
	canInstall := !det.OpenCLAvailable
	hint := ""
	if canInstall {
		hint = "OpenCL Runtime"
	}
	return BackendInfo{
		ID: "opencl", Name: "OpenCL",
		Compiled: hasOpenCL, SystemAvailable: available,
		CanInstall: canInstall, InstallHint: hint,
	}
}

// backendUseGPU returns whether a given backend should enable GPU acceleration.
func backendUseGPU(backend string) bool {
	return backend != "cpu"
}
