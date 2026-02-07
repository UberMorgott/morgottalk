package services

import (
	"encoding/hex"
	"fmt"
	"sync"
	"unsafe"

	"github.com/gen2brain/malgo"
)

const (
	sampleRate = 16000
	channels   = 1
)

// AudioCapture records audio from a microphone using malgo (miniaudio).
type AudioCapture struct {
	mu      sync.Mutex
	device  *malgo.Device
	ctx     *malgo.AllocatedContext
	samples []float32
	active  bool
	micID   string // hex-encoded DeviceID, empty = default
}

// NewAudioCapture creates a new audio capture instance.
func NewAudioCapture() (*AudioCapture, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("malgo init context: %w", err)
	}
	return &AudioCapture{ctx: ctx}, nil
}

// Start begins recording audio from the microphone.
func (a *AudioCapture) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.active {
		return nil
	}

	a.samples = a.samples[:0]

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatF32
	deviceConfig.Capture.Channels = channels
	deviceConfig.SampleRate = sampleRate

	// Set specific device if configured
	if a.micID != "" {
		if idBytes, err := hex.DecodeString(a.micID); err == nil {
			var devID malgo.DeviceID
			copy((*[unsafe.Sizeof(devID)]byte)(unsafe.Pointer(&devID))[:], idBytes)
			deviceConfig.Capture.DeviceID = devID.Pointer()
		}
	}

	onRecvFrames := func(outputSamples, inputSamples []byte, frameCount uint32) {
		a.mu.Lock()
		defer a.mu.Unlock()

		if !a.active {
			return
		}

		// Convert bytes to float32 slice (4 bytes per sample)
		count := int(frameCount) * channels
		if count*4 > len(inputSamples) {
			count = len(inputSamples) / 4
		}
		floats := unsafe.Slice((*float32)(unsafe.Pointer(&inputSamples[0])), count)
		a.samples = append(a.samples, floats...)
	}

	callbacks := malgo.DeviceCallbacks{
		Data: onRecvFrames,
	}

	device, err := malgo.InitDevice(a.ctx.Context, deviceConfig, callbacks)
	if err != nil {
		return fmt.Errorf("malgo init device: %w", err)
	}

	if err := device.Start(); err != nil {
		device.Uninit()
		return fmt.Errorf("malgo start device: %w", err)
	}

	a.device = device
	a.active = true
	return nil
}

// Stop ends recording and returns captured PCM samples (16 kHz, mono, float32).
func (a *AudioCapture) Stop() []float32 {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.active {
		return nil
	}

	a.active = false
	if a.device != nil {
		a.device.Stop()
		a.device.Uninit()
		a.device = nil
	}

	result := make([]float32, len(a.samples))
	copy(result, a.samples)
	a.samples = a.samples[:0]
	return result
}

// SetMicrophoneID sets the device to use for next recording.
func (a *AudioCapture) SetMicrophoneID(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.micID = id
}

// Close releases the malgo context.
func (a *AudioCapture) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.device != nil {
		a.device.Stop()
		a.device.Uninit()
		a.device = nil
	}
	if a.ctx != nil {
		_ = a.ctx.Uninit()
		a.ctx.Free()
		a.ctx = nil
	}
}
