package services

// BackendInfo describes a compute backend and its availability.
type BackendInfo struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Compiled          bool   `json:"compiled"`        // binary includes this backend (build tag)
	SystemAvailable   bool   `json:"systemAvailable"` // hardware + runtime present
	CanInstall        bool   `json:"canInstall"`      // hardware exists but runtime missing
	InstallHint       string `json:"installHint"`     // e.g. "CUDA Toolkit"
	UnavailableReason string `json:"unavailableReason"` // "" | "no_hardware" | "no_driver" | "no_runtime" | "not_compiled"
	GPUDetected       string `json:"gpuDetected"`     // e.g. "NVIDIA RTX 5070 Ti", ""
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
	info := BackendInfo{
		ID: "cuda", Name: "CUDA",
		Compiled: hasCUDA,
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

	info.SystemAvailable = hasCUDA
	if !hasCUDA {
		info.UnavailableReason = "not_compiled"
	}

	return info
}

func vulkanBackend(det gpuDetection) BackendInfo {
	info := BackendInfo{
		ID: "vulkan", Name: "Vulkan",
		Compiled: hasVulkan,
	}

	if !det.VulkanAvailable {
		info.UnavailableReason = "no_runtime"
		info.CanInstall = true
		info.InstallHint = "Vulkan ICD Loader"
		return info
	}

	info.SystemAvailable = hasVulkan
	if !hasVulkan {
		info.UnavailableReason = "not_compiled"
	}

	return info
}

func metalBackend(det gpuDetection) BackendInfo {
	return BackendInfo{
		ID: "metal", Name: "Metal",
		Compiled: hasMetal, SystemAvailable: hasMetal,
		CanInstall: false,
	}
}

func rocmBackend(det gpuDetection) BackendInfo {
	info := BackendInfo{
		ID: "rocm", Name: "ROCm",
		Compiled: hasROCm,
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

	info.SystemAvailable = hasROCm
	if !hasROCm {
		info.UnavailableReason = "not_compiled"
	}

	return info
}

func openclBackend(det gpuDetection) BackendInfo {
	info := BackendInfo{
		ID: "opencl", Name: "OpenCL",
		Compiled: hasOpenCL,
	}

	if !det.OpenCLAvailable {
		info.UnavailableReason = "no_runtime"
		info.CanInstall = true
		info.InstallHint = "OpenCL Runtime"
		return info
	}

	info.SystemAvailable = hasOpenCL
	if !hasOpenCL {
		info.UnavailableReason = "not_compiled"
	}

	return info
}

// backendUseGPU returns whether a given backend should enable GPU acceleration.
func backendUseGPU(backend string) bool {
	return backend != "cpu"
}
