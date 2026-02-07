//go:build linux

package services

import (
	"os"
	"os/exec"
	"strings"
)

func detectGPU() gpuDetection {
	det := gpuDetection{}

	// Detect NVIDIA GPU
	if _, err := os.Stat("/proc/driver/nvidia/version"); err == nil {
		det.HasNVIDIA = true
	} else if out, err := exec.Command("lspci").Output(); err == nil {
		lower := strings.ToLower(string(out))
		if strings.Contains(lower, "nvidia") {
			det.HasNVIDIA = true
		}
	}

	// Detect CUDA runtime
	if det.HasNVIDIA {
		det.CUDAAvailable = ldconfigHas("libcuda.so") || fileExists("/opt/cuda/lib64/libcudart.so")
	}

	// Detect Vulkan runtime
	det.VulkanAvailable = ldconfigHas("libvulkan.so") || fileExists("/usr/lib/libvulkan.so.1")

	// Detect AMD GPU
	if out, err := exec.Command("lspci").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			lower := strings.ToLower(line)
			if (strings.Contains(lower, "vga") || strings.Contains(lower, "display")) &&
				(strings.Contains(lower, "amd") || strings.Contains(lower, "radeon")) {
				det.HasAMD = true
				break
			}
		}
	}

	// Detect ROCm/HIP runtime
	if det.HasAMD {
		det.ROCmAvailable = ldconfigHas("libamdhip64.so") || fileExists("/opt/rocm/lib/libamdhip64.so")
	}

	// Detect OpenCL runtime
	det.OpenCLAvailable = ldconfigHas("libOpenCL.so")

	// Detect package manager
	det.PackageManager = detectPackageManager()

	return det
}

func ldconfigHas(lib string) bool {
	out, err := exec.Command("ldconfig", "-p").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), lib)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func detectPackageManager() string {
	if _, err := exec.LookPath("pacman"); err == nil {
		return "pacman"
	}
	if _, err := exec.LookPath("apt"); err == nil {
		return "apt"
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return "dnf"
	}
	if _, err := exec.LookPath("zypper"); err == nil {
		return "zypper"
	}
	return ""
}
