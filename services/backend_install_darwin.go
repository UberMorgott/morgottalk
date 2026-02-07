//go:build darwin

package services

import (
	"fmt"
	"os/exec"
)

func installBackend(id string) (string, error) {
	switch id {
	case "vulkan":
		// Try Homebrew first, fall back to opening download URL
		if _, err := exec.LookPath("brew"); err == nil {
			cmd := exec.Command("brew", "install", "molten-vk")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("brew install failed: %s\n%s", err, string(out))
			}
			return "installed", nil
		}
		if err := exec.Command("open", "https://vulkan.lunarg.com/sdk/home#mac").Run(); err != nil {
			return "", err
		}
		return "url", nil
	case "opencl":
		return "installed", nil
	case "metal":
		return "installed", nil
	default:
		return "", fmt.Errorf("backend %q is not available on macOS", id)
	}
}
