//go:build linux

package services

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func installBackend(id string) (string, error) {
	pm := detectPackageManager()
	if pm == "" {
		return "", fmt.Errorf("no supported package manager found")
	}

	// CUDA uses NVIDIA official repo (except Arch which has up-to-date packages).
	if id == "cuda" && pm != "pacman" {
		return installCUDALinux(pm)
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

// installCUDALinux adds NVIDIA's official repo and installs cuda-toolkit meta-package.
// See https://docs.nvidia.com/cuda/cuda-installation-guide-linux/#meta-packages
func installCUDALinux(pm string) (string, error) {
	distroID, version := detectDistro()
	slug := nvidiaRepoSlug(distroID, version)
	if slug == "" {
		return "", fmt.Errorf("unsupported distro for NVIDIA CUDA repo: %s %s", distroID, version)
	}

	var err error
	switch pm {
	case "apt":
		err = installCUDADebian(slug)
	case "dnf":
		err = installCUDAFedora(distroID, slug)
	case "zypper":
		err = installCUDAOpenSUSE(slug)
	default:
		return "", fmt.Errorf("CUDA install not supported for package manager %q", pm)
	}
	if err != nil {
		return "", err
	}
	return "installed", nil
}

// detectDistro parses /etc/os-release to determine the distro ID and version.
func detectDistro() (id, version string) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}
	return id, version
}

// nvidiaRepoSlug maps distro ID + version to the NVIDIA repo path component.
// e.g. "ubuntu", "24.04" → "ubuntu2404/x86_64"
func nvidiaRepoSlug(distroID, version string) string {
	ver := strings.ReplaceAll(version, ".", "")

	switch distroID {
	case "ubuntu":
		switch version {
		case "24.04", "22.04", "20.04":
			return "ubuntu" + ver + "/x86_64"
		}
	case "debian":
		switch version {
		case "12", "11":
			return "debian" + ver + "/x86_64"
		}
	case "fedora":
		if ver != "" {
			return "fedora" + ver + "/x86_64"
		}
	case "rhel", "centos", "rocky", "almalinux", "ol":
		major := strings.Split(version, ".")[0]
		if major != "" {
			return "rhel" + major + "/x86_64"
		}
	case "opensuse-leap":
		major := strings.Split(version, ".")[0]
		if major != "" {
			return "opensuse" + major + "/x86_64"
		}
	}
	return ""
}

const nvidiaRepoBase = "https://developer.download.nvidia.com/compute/cuda/repos/"

// installCUDADebian adds NVIDIA keyring and installs cuda-toolkit on Debian/Ubuntu.
func installCUDADebian(slug string) error {
	keyringURL := nvidiaRepoBase + slug + "/cuda-keyring_1.1-1_all.deb"
	keyringPath := "/tmp/cuda-keyring.deb"

	if err := downloadFile(keyringURL, keyringPath); err != nil {
		return fmt.Errorf("download keyring: %w", err)
	}
	defer os.Remove(keyringPath)

	// Install keyring, update, install cuda-toolkit — all elevated via pkexec.
	// Uses bash -c to chain commands under a single elevation prompt.
	cmd := exec.Command("pkexec", "bash", "-c",
		fmt.Sprintf("dpkg -i %s && apt-get update && apt-get install -y cuda-toolkit", keyringPath))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install failed: %s\n%s", err, string(out))
	}
	return nil
}

// installCUDAFedora adds NVIDIA repo and installs cuda-toolkit on Fedora/RHEL.
func installCUDAFedora(distroID, slug string) error {
	repoURL := nvidiaRepoBase + slug + "/cuda-" + distroID + ".repo"

	cmd := exec.Command("pkexec", "bash", "-c",
		fmt.Sprintf("dnf config-manager --add-repo %s && dnf install -y cuda-toolkit", repoURL))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install failed: %s\n%s", err, string(out))
	}
	return nil
}

// installCUDAOpenSUSE adds NVIDIA repo and installs cuda-toolkit on openSUSE.
func installCUDAOpenSUSE(slug string) error {
	repoURL := nvidiaRepoBase + slug + "/"

	cmd := exec.Command("pkexec", "bash", "-c",
		fmt.Sprintf("zypper addrepo --refresh %s cuda-repo && zypper --gpg-auto-import-keys refresh && zypper install -y cuda-toolkit", repoURL))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install failed: %s\n%s", err, string(out))
	}
	return nil
}

// downloadFile downloads a URL to a local path.
func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func backendPackages(pm, id string) []string {
	switch id {
	case "cuda":
		switch pm {
		case "pacman":
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
