//go:build linux

package services

import (
	"fmt"
	"os/exec"
)

func installBackend(id string) (string, error) {
	pm := detectPackageManager()
	if pm == "" {
		return "", fmt.Errorf("no supported package manager found")
	}

	packages := backendPackages(pm, id)
	if len(packages) == 0 {
		return "", fmt.Errorf("no packages known for backend %q on %s", id, pm)
	}

	args := installArgs(pm, packages)
	cmd := exec.Command("pkexec", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("install failed: %s\n%s", err, string(out))
	}
	return "installed", nil
}

func backendPackages(pm, id string) []string {
	switch id {
	case "cuda":
		switch pm {
		case "pacman":
			return []string{"cuda"}
		case "apt":
			return []string{"nvidia-cuda-toolkit"}
		case "dnf":
			return []string{"cuda"}
		case "zypper":
			return []string{"cuda"}
		}
	case "vulkan":
		switch pm {
		case "pacman":
			return []string{"vulkan-icd-loader"}
		case "apt":
			return []string{"libvulkan1", "mesa-vulkan-drivers"}
		case "dnf":
			return []string{"vulkan-loader", "mesa-vulkan-drivers"}
		case "zypper":
			return []string{"libvulkan1", "Mesa-vulkan-drivers"}
		}
	case "rocm":
		switch pm {
		case "pacman":
			return []string{"rocm-hip-runtime"}
		case "apt":
			return []string{"rocm-hip-runtime"}
		case "dnf":
			return []string{"rocm-hip-runtime"}
		}
	case "opencl":
		switch pm {
		case "pacman":
			return []string{"ocl-icd"}
		case "apt":
			return []string{"ocl-icd-libopencl1"}
		case "dnf":
			return []string{"ocl-icd"}
		case "zypper":
			return []string{"ocl-icd"}
		}
	}
	return nil
}

func installArgs(pm string, packages []string) []string {
	switch pm {
	case "pacman":
		args := []string{"pacman", "-S", "--noconfirm"}
		return append(args, packages...)
	case "apt":
		args := []string{"apt", "install", "-y"}
		return append(args, packages...)
	case "dnf":
		args := []string{"dnf", "install", "-y"}
		return append(args, packages...)
	case "zypper":
		args := []string{"zypper", "install", "-y"}
		return append(args, packages...)
	}
	return nil
}
