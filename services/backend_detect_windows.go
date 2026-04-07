//go:build windows

package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func detectGPU() gpuDetection {
	det := gpuDetection{}

	// Query all video controllers with names and driver versions.
	det.GPUs = queryAllGPUs()

	// Populate legacy fields from GPU list.
	for _, g := range det.GPUs {
		switch g.Vendor {
		case "nvidia":
			det.HasNVIDIA = true
			if det.NVIDIAModel == "" {
				det.NVIDIAModel = g.Name
			}
		case "amd":
			det.HasAMD = true
			if det.AMDModel == "" {
				det.AMDModel = g.Name
			}
		}
	}

	// CUDA runtime: check env var, then file system.
	if det.HasNVIDIA {
		det.CUDAAvailable = detectCUDARuntime()
	}

	// Vulkan runtime (vulkan-1.dll in system32)
	sys32 := filepath.Join(os.Getenv("SystemRoot"), "System32")
	det.VulkanAvailable = fileExists(filepath.Join(sys32, "vulkan-1.dll"))

	// ROCm/HIP runtime
	if det.HasAMD {
		det.ROCmAvailable = os.Getenv("HIP_PATH") != ""
	}

	return det
}

// queryAllGPUs queries Win32_VideoController for all GPUs and their driver versions.
func queryAllGPUs() []gpuInfo {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"Get-CimInstance Win32_VideoController | ForEach-Object { $_.Name + '|' + $_.DriverVersion }")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var gpus []gpuInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		name := strings.TrimSpace(parts[0])
		rawDriver := ""
		if len(parts) > 1 {
			rawDriver = strings.TrimSpace(parts[1])
		}

		vendor := classifyGPUVendor(name)
		driverVer := ""
		switch vendor {
		case "nvidia":
			driverVer = parseNVIDIADriverVersion(rawDriver)
		default:
			driverVer = rawDriver // AMD/Intel: use WMI version as-is
		}

		gpus = append(gpus, gpuInfo{
			Name:          name,
			Vendor:        vendor,
			DriverVersion: driverVer,
		})
	}

	// For NVIDIA, try nvidia-smi for a cleaner version (overrides WMI parse).
	if smiVer := queryNVIDIASMI(); smiVer != "" {
		for i := range gpus {
			if gpus[i].Vendor == "nvidia" {
				gpus[i].DriverVersion = smiVer
			}
		}
	}

	return gpus
}

// classifyGPUVendor returns "nvidia", "amd", "intel", or "unknown" based on GPU name.
func classifyGPUVendor(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "nvidia"):
		return "nvidia"
	case strings.Contains(lower, "amd") || strings.Contains(lower, "radeon"):
		return "amd"
	case strings.Contains(lower, "intel"):
		return "intel"
	default:
		return "unknown"
	}
}

// parseNVIDIADriverVersion converts WMI driver version (e.g. "32.0.15.6081")
// to NVIDIA-style version (e.g. "560.81").
// WMI format: AA.BB.CC.DDEE where NVIDIA version = (CC-40)*100 + DD . EE
// More precisely: last 5 digits DDDEE → DDD.EE where DDD = first 3 digits, EE = last 2.
func parseNVIDIADriverVersion(wmiVer string) string {
	if wmiVer == "" {
		return ""
	}
	// Remove dots to get raw digits, then take last 5.
	digits := strings.ReplaceAll(wmiVer, ".", "")
	if len(digits) < 5 {
		return wmiVer // Can't parse, return as-is.
	}
	last5 := digits[len(digits)-5:]
	major, err1 := strconv.Atoi(last5[:3])
	minor, err2 := strconv.Atoi(last5[3:])
	if err1 != nil || err2 != nil {
		return wmiVer
	}
	return fmt.Sprintf("%d.%02d", major, minor)
}

// queryNVIDIASMI tries to get the NVIDIA driver version from nvidia-smi.
func queryNVIDIASMI() string {
	// Check common locations.
	paths := []string{
		"nvidia-smi", // In PATH
		`C:\Program Files\NVIDIA Corporation\NVSMI\nvidia-smi.exe`,
		`C:\Windows\System32\nvidia-smi.exe`,
	}
	for _, p := range paths {
		cmd := exec.Command(p, "--query-gpu=driver_version", "--format=csv,noheader")
		hideWindow(cmd)
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		// nvidia-smi may return multiple lines for multi-GPU — take first.
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				return line
			}
		}
	}
	return ""
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
