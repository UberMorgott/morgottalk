//go:build darwin

package services

import (
	"os"
	"os/exec"
	"strings"
)

func detectGPU() gpuDetection {
	det := gpuDetection{}

	// Metal is always available on macOS 10.11+
	// (handled by hasMetal build tag const)

	// NVIDIA/CUDA: Apple dropped NVIDIA support after macOS 10.14
	det.HasNVIDIA = false
	det.CUDAAvailable = false

	// Vulkan via MoltenVK
	det.VulkanAvailable = fileExists("/usr/local/lib/libvulkan.dylib") ||
		fileExists("/usr/local/lib/libMoltenVK.dylib")

	// AMD GPU (older Macs with AMD discrete GPUs)
	if out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output(); err == nil {
		lower := strings.ToLower(string(out))
		det.HasAMD = strings.Contains(lower, "amd") || strings.Contains(lower, "radeon")
	}

	// ROCm is not available on macOS
	det.ROCmAvailable = false

	// OpenCL is built into macOS (deprecated but available)
	det.OpenCLAvailable = true

	return det
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
