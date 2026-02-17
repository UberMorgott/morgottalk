package services

import (
	"bytes"
	"encoding/hex"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"unsafe"

	"github.com/emersion/go-autostart"
	"github.com/gen2brain/malgo"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/UberMorgott/transcribation/internal/config"
)

// MicrophoneInfo represents a capture device.
type MicrophoneInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

// LanguageInfo represents a supported transcription language.
type LanguageInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// GlobalSettings holds non-preset settings.
type GlobalSettings struct {
	MicrophoneID   string `json:"microphoneId"`
	ModelsDir      string `json:"modelsDir"`
	Theme          string `json:"theme"`
	UILang         string `json:"uiLang"`
	CloseAction    string `json:"closeAction"`
	AutoStart      bool   `json:"autoStart"`
	StartMinimized bool   `json:"startMinimized"`
	Backend        string `json:"backend"`
	OnboardingDone bool   `json:"onboardingDone"`
}

// onBackendChanged is called when the user changes the backend in Settings.
var onBackendChanged func()

// SetOnBackendChanged registers a callback invoked when the backend setting changes.
// Use to flush engine caches and reload config in PresetService.
func SetOnBackendChanged(fn func()) { onBackendChanged = fn }

// SettingsService provides global settings management to the frontend.
type SettingsService struct {
	models *ModelService
}

func NewSettingsService(models *ModelService) *SettingsService {
	return &SettingsService{models: models}
}

// GetGlobalSettings returns the global (non-preset) settings.
func (s *SettingsService) GetGlobalSettings() GlobalSettings {
	cfg, err := config.Load()
	if err != nil {
		slog.Warn("failed to load config", "err", err)
	}
	backend := cfg.Backend
	if backend == "" {
		backend = "auto"
	}
	return GlobalSettings{
		MicrophoneID:   cfg.MicrophoneID,
		ModelsDir:      cfg.ModelsDir,
		Theme:          cfg.Theme,
		UILang:         cfg.UILang,
		CloseAction:    cfg.CloseAction,
		AutoStart:      cfg.AutoStart,
		StartMinimized: cfg.StartMinimized,
		Backend:        backend,
		OnboardingDone: cfg.OnboardingDone,
	}
}

// SaveGlobalSettings saves the global settings.
func (s *SettingsService) SaveGlobalSettings(gs GlobalSettings) error {
	cfg, err := config.Load()
	if err != nil {
		slog.Warn("failed to load config", "err", err)
	}
	autoStartChanged := cfg.AutoStart != gs.AutoStart
	backendChanged := cfg.Backend != gs.Backend
	cfg.MicrophoneID = gs.MicrophoneID
	cfg.ModelsDir = gs.ModelsDir
	cfg.Theme = gs.Theme
	cfg.UILang = gs.UILang
	cfg.CloseAction = gs.CloseAction
	cfg.AutoStart = gs.AutoStart
	cfg.StartMinimized = gs.StartMinimized
	cfg.Backend = gs.Backend
	cfg.OnboardingDone = gs.OnboardingDone
	if err := config.Save(cfg); err != nil {
		return err
	}
	if backendChanged && onBackendChanged != nil {
		go onBackendChanged()
	}
	if autoStartChanged {
		a := autostartApp()
		if gs.AutoStart {
			if err := a.Enable(); err != nil {
				slog.Warn("failed to enable autostart", "err", err)
			}
		} else {
			if err := a.Disable(); err != nil {
				slog.Warn("failed to disable autostart", "err", err)
			}
		}
	}
	return nil
}

// autostartApp returns the autostart.App descriptor for this application.
func autostartApp() *autostart.App {
	exe, _ := os.Executable()
	exe, _ = filepath.EvalSymlinks(exe)
	return &autostart.App{
		Name:        "morgottalk",
		DisplayName: "MorgoTTalk",
		Exec:        []string{exe},
	}
}

// GetAllBackends returns all known compute backends with availability info.
func (s *SettingsService) GetAllBackends() []BackendInfo {
	return GetAllBackends()
}

// InstallBackend installs the runtime for the given backend.
// Returns "installed" if the package was installed directly,
// "url" if a download page was opened in the browser.
func (s *SettingsService) InstallBackend(id string) (string, error) {
	return installBackend(id)
}

// RestartApp launches a new instance of the application and quits the current one.
func (s *SettingsService) RestartApp() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	if err := cmd.Start(); err != nil {
		return err
	}
	if app := application.Get(); app != nil {
		app.Quit()
	}
	return nil
}

// PickModelsDir opens a native directory picker dialog.
func (s *SettingsService) PickModelsDir() (string, error) {
	app := application.Get()
	if app == nil {
		return "", nil
	}
	return app.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		SetTitle("Select Models Directory").
		PromptForSingleSelection()
}

// GetMicrophones returns available capture devices.
func (s *SettingsService) GetMicrophones() ([]MicrophoneInfo, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}
	defer ctx.Free()
	defer ctx.Uninit()

	devices, err := ctx.Devices(malgo.Capture)
	if err != nil {
		return nil, err
	}

	var result []MicrophoneInfo
	for _, d := range devices {
		idBytes := (*[256]byte)(unsafe.Pointer(d.ID.Pointer()))[:]
		trimmed := bytes.TrimRight(idBytes, "\x00")
		hexID := hex.EncodeToString(trimmed)

		result = append(result, MicrophoneInfo{
			ID:        hexID,
			Name:      d.Name(),
			IsDefault: d.IsDefault != 0,
		})
	}
	return result, nil
}

// SystemInfo provides diagnostic system information.
type SystemInfo struct {
	MicrophoneCount int           `json:"microphoneCount"`
	ModelsCount     int           `json:"modelsCount"`
	Backends        []BackendInfo `json:"backends"`
}

// GetSystemInfo returns diagnostic information about the system.
func (s *SettingsService) GetSystemInfo() SystemInfo {
	mics, _ := s.GetMicrophones()
	availableModels := s.models.GetAvailableModels()

	downloadedCount := 0
	for _, m := range availableModels {
		if m.Downloaded {
			downloadedCount++
		}
	}

	return SystemInfo{
		MicrophoneCount: len(mics),
		ModelsCount:     downloadedCount,
		Backends:        GetAllBackends(),
	}
}
