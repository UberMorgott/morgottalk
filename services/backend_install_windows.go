//go:build windows

package services

import (
	"fmt"
	"os/exec"
)

func installBackend(id string) (string, error) {
	url := ""
	switch id {
	case "cuda":
		url = "https://developer.nvidia.com/cuda-downloads"
	case "vulkan":
		url = "https://vulkan.lunarg.com/sdk/home#windows"
	case "rocm":
		url = "https://rocm.docs.amd.com/"
	case "opencl":
		// OpenCL is typically bundled with GPU drivers on Windows
		return "", fmt.Errorf("OpenCL is bundled with your GPU driver; update your GPU driver to get OpenCL support")
	default:
		return "", fmt.Errorf("backend %q is not available on Windows", id)
	}

	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start(); err != nil {
		return "", err
	}
	return "url", nil
}
