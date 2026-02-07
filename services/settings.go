package services

import (
	"bytes"
	"encoding/hex"
	"os"
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
	MicrophoneID string `json:"microphoneId"`
	ModelsDir    string `json:"modelsDir"`
	Theme        string `json:"theme"`
	UILang       string `json:"uiLang"`
	CloseAction  string `json:"closeAction"`
	AutoStart    bool   `json:"autoStart"`
	Backend      string `json:"backend"`
}

// SettingsService provides global settings management to the frontend.
type SettingsService struct{}

func NewSettingsService() *SettingsService {
	return &SettingsService{}
}

// GetGlobalSettings returns the global (non-preset) settings.
func (s *SettingsService) GetGlobalSettings() GlobalSettings {
	cfg, _ := config.Load()
	backend := cfg.Backend
	if backend == "" {
		backend = "auto"
	}
	return GlobalSettings{
		MicrophoneID: cfg.MicrophoneID,
		ModelsDir:    cfg.ModelsDir,
		Theme:        cfg.Theme,
		UILang:       cfg.UILang,
		CloseAction:  cfg.CloseAction,
		AutoStart:    cfg.AutoStart,
		Backend:      backend,
	}
}

// SaveGlobalSettings saves the global settings.
func (s *SettingsService) SaveGlobalSettings(gs GlobalSettings) error {
	cfg, _ := config.Load()
	autoStartChanged := cfg.AutoStart != gs.AutoStart
	cfg.MicrophoneID = gs.MicrophoneID
	cfg.ModelsDir = gs.ModelsDir
	cfg.Theme = gs.Theme
	cfg.UILang = gs.UILang
	cfg.CloseAction = gs.CloseAction
	cfg.AutoStart = gs.AutoStart
	cfg.Backend = gs.Backend
	if err := config.Save(cfg); err != nil {
		return err
	}
	if autoStartChanged {
		a := autostartApp()
		if gs.AutoStart {
			_ = a.Enable()
		} else {
			_ = a.Disable()
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
