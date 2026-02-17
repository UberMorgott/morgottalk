package services

import (
	"testing"
)

func TestBackendUseGPU(t *testing.T) {
	tests := []struct {
		backend string
		want    bool
	}{
		{"cpu", false},
		{"cuda", true},
		{"auto", true},
		{"vulkan", true},
		{"metal", true},
		{"rocm", true},
		{"opencl", true},
	}

	for _, tt := range tests {
		t.Run(tt.backend, func(t *testing.T) {
			got := backendUseGPU(tt.backend)
			if got != tt.want {
				t.Errorf("backendUseGPU(%q) = %v, want %v", tt.backend, got, tt.want)
			}
		})
	}
}

func TestCudaBackend_NoNVIDIA(t *testing.T) {
	det := gpuDetection{HasNVIDIA: false}
	info := cudaBackend(det)

	if info.ID != "cuda" {
		t.Errorf("ID = %q, want %q", info.ID, "cuda")
	}
	if info.UnavailableReason != "no_hardware" {
		t.Errorf("UnavailableReason = %q, want %q", info.UnavailableReason, "no_hardware")
	}
	if info.GPUDetected != "" {
		t.Errorf("GPUDetected = %q, want empty", info.GPUDetected)
	}
	if info.CanInstall {
		t.Error("CanInstall = true, want false (no NVIDIA hardware)")
	}
}

func TestCudaBackend_NVIDIANoCUDA(t *testing.T) {
	det := gpuDetection{
		HasNVIDIA:     true,
		NVIDIAModel:   "NVIDIA RTX 5070 Ti",
		CUDAAvailable: false,
	}
	info := cudaBackend(det)

	if info.UnavailableReason != "no_runtime" {
		t.Errorf("UnavailableReason = %q, want %q", info.UnavailableReason, "no_runtime")
	}
	if !info.CanInstall {
		t.Error("CanInstall = false, want true (CUDA runtime installable)")
	}
	if info.GPUDetected != "NVIDIA RTX 5070 Ti" {
		t.Errorf("GPUDetected = %q, want %q", info.GPUDetected, "NVIDIA RTX 5070 Ti")
	}
	if info.InstallHint != "CUDA Toolkit" {
		t.Errorf("InstallHint = %q, want %q", info.InstallHint, "CUDA Toolkit")
	}
}

func TestVulkanBackend_NoVulkan(t *testing.T) {
	det := gpuDetection{VulkanAvailable: false}
	info := vulkanBackend(det)

	if info.ID != "vulkan" {
		t.Errorf("ID = %q, want %q", info.ID, "vulkan")
	}
	if info.UnavailableReason != "no_runtime" {
		t.Errorf("UnavailableReason = %q, want %q", info.UnavailableReason, "no_runtime")
	}
	if !info.CanInstall {
		t.Error("CanInstall = false, want true (Vulkan installable)")
	}
	if info.InstallHint != "Vulkan ICD Loader" {
		t.Errorf("InstallHint = %q, want %q", info.InstallHint, "Vulkan ICD Loader")
	}
}
