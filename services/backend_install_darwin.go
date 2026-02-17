//go:build darwin

package services

import (
	"fmt"
	"os/exec"
)

func installBackend(id string) (string, error) {
	switch id {
	case "vulkan":
		go installBackendAsyncDarwin(id)
		return "installing", nil
	case "metal":
		// Metal is statically linked into the binary on macOS.
		return "installed", nil
	default:
		return "", fmt.Errorf("backend %q is not available on macOS", id)
	}
}

func installBackendAsyncDarwin(id string) {
	emit := func(stage, stageText string, pct float64, done bool, errMsg string) {
		emitBackendProgress(id, stage, stageText, pct, done, errMsg)
	}

	// Step 1: Install Vulkan runtime (MoltenVK) via Homebrew if available.
	if id == "vulkan" {
		if _, err := exec.LookPath("brew"); err == nil {
			emit("installing_runtime", "", 0, false, "")
			cmd := exec.Command("brew", "install", "molten-vk")
			if out, err := cmd.CombinedOutput(); err != nil {
				emit("", "", 0, true, fmt.Sprintf("brew install failed: %s\n%s", err, string(out)))
				return
			}
		}
	}

	// Step 2: Download the backend library (.dylib).
	emit("downloading", "", 0, false, "")
	if err := downloadBackendDLL(id); err != nil {
		emit("", "", 0, true, fmt.Sprintf("Backend download failed: %v", err))
		return
	}

	// Step 3: Hot-apply.
	if onBackendInstalled != nil {
		onBackendInstalled(id)
	}

	emit("", "", 100, true, "")
}
