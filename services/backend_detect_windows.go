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

	// CUDA runtime
	if det.HasNVIDIA {
		det.CUDAAvailable = os.Getenv("CUDA_PATH") != ""
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

	// OpenCL runtime
	det.OpenCLAvailable = fileExists(filepath.Join(sys32, "OpenCL.dll"))

	return det
}

func queryGPUNames() string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"Get-CimInstance Win32_VideoController | Select-Object -ExpandProperty Name").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
