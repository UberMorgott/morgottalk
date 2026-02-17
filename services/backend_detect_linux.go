//go:build linux

package services

import (
	"os"
	"os/exec"
	"strings"
)

func detectGPU() gpuDetection {
	det := gpuDetection{}

	// Get lspci output once
	lspciOut := ""
	if out, err := exec.Command("lspci").Output(); err == nil {
		lspciOut = string(out)
	}

	// Detect NVIDIA GPU
	if _, err := os.Stat("/proc/driver/nvidia/version"); err == nil {
		det.HasNVIDIA = true
	} else if strings.Contains(strings.ToLower(lspciOut), "nvidia") {
		det.HasNVIDIA = true
	}

	// Parse NVIDIA GPU model from lspci
	if det.HasNVIDIA {
		for _, line := range strings.Split(lspciOut, "\n") {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "nvidia") && (strings.Contains(lower, "vga") || strings.Contains(lower, "3d")) {
				det.NVIDIAModel = extractGPUModel(line)
				break
			}
		}
	}

	// Detect CUDA runtime
	if det.HasNVIDIA {
		det.CUDAAvailable = ldconfigHas("libcuda.so") || fileExists("/opt/cuda/lib64/libcudart.so")
	}

	// Detect Vulkan runtime
	det.VulkanAvailable = ldconfigHas("libvulkan.so") || fileExists("/usr/lib/libvulkan.so.1")

	// Detect AMD GPU and parse model
	for _, line := range strings.Split(lspciOut, "\n") {
		lower := strings.ToLower(line)
		if (strings.Contains(lower, "vga") || strings.Contains(lower, "display")) &&
			(strings.Contains(lower, "amd") || strings.Contains(lower, "radeon")) {
			det.HasAMD = true
			det.AMDModel = extractGPUModel(line)
			break
		}
	}

	// Detect ROCm/HIP runtime
	if det.HasAMD {
		det.ROCmAvailable = ldconfigHas("libamdhip64.so") || fileExists("/opt/rocm/lib/libamdhip64.so")
	}


	// Detect package manager
	det.PackageManager = detectPackageManager()

	return det
}

// extractGPUModel parses GPU model name from lspci output line
// e.g. "01:00.0 VGA compatible controller: NVIDIA Corporation: Device 2503 (rev a1)"
// returns "NVIDIA RTX 5070 Ti" or similar descriptive name
func extractGPUModel(lspciLine string) string {
	// Find the part after the colon
	parts := strings.Split(lspciLine, ": ")
	if len(parts) < 2 {
		return ""
	}

	desc := parts[len(parts)-1]

	// Handle NVIDIA cards
	if strings.Contains(strings.ToLower(desc), "nvidia") {
		// Try to extract brand name (RTX, GTX, GeForce, Tesla, etc)
		for _, brand := range []string{"RTX", "GTX", "GeForce", "Tesla", "Quadro", "A10", "A40", "L4", "L40"} {
			if idx := strings.Index(desc, brand); idx != -1 {
				// Extract from brand onwards, stop at the next device descriptor or end
				rest := desc[idx:]
				// Split on common delimiters
				for _, delim := range []string{"(", "["} {
					if idx := strings.Index(rest, delim); idx != -1 {
						rest = rest[:idx]
						break
					}
				}
				result := strings.TrimSpace(rest)
				if result != "" {
					return "NVIDIA " + result
				}
			}
		}
		// Fallback: extract numeric device ID
		if idx := strings.Index(desc, "Device"); idx != -1 {
			return "NVIDIA GPU"
		}
		return "NVIDIA"
	}

	// Handle AMD cards
	if strings.Contains(strings.ToLower(desc), "amd") || strings.Contains(strings.ToLower(desc), "radeon") {
		for _, brand := range []string{"Radeon", "RADEON", "RX", "Vega", "EPYC"} {
			if idx := strings.Index(desc, brand); idx != -1 {
				rest := desc[idx:]
				for _, delim := range []string{"(", "["} {
					if idx := strings.Index(rest, delim); idx != -1 {
						rest = rest[:idx]
						break
					}
				}
				result := strings.TrimSpace(rest)
				if result != "" {
					return "AMD " + result
				}
			}
		}
		return "AMD GPU"
	}

	return ""
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
