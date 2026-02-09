package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"github.com/google/uuid"
)

// Preset holds settings for a single transcription preset.
type Preset struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ModelName       string `json:"modelName"`
	KeepModelLoaded bool   `json:"keepModelLoaded"`
	InputMode       string `json:"inputMode"` // "hold" | "toggle"
	Hotkey          string `json:"hotkey"`     // "ctrl+shift+f1"
	Language        string `json:"language"`   // "auto", "en", "ru"...
	UseKBLayout     bool   `json:"useKBLayout"`
	KeepHistory     bool   `json:"keepHistory"`
	Enabled         bool   `json:"enabled"`
}

// AppConfig holds the global application settings and presets.
type AppConfig struct {
	MicrophoneID string   `json:"microphoneId"`
	ModelsDir    string   `json:"modelsDir"`
	Theme        string   `json:"theme"`       // "dark" | "light"
	UILang       string   `json:"uiLang"`      // "en" | "ru"
	CloseAction  string   `json:"closeAction"` // "" = ask, "tray", "quit"
	AutoStart      bool     `json:"autoStart"`
	StartMinimized bool     `json:"startMinimized"`
	Backend        string   `json:"backend"` // "auto", "cpu", "cuda", "vulkan", "metal", "rocm", "opencl"
	Presets      []Preset `json:"presets"`
}

// DefaultPreset returns a sensible default preset.
func DefaultPreset() Preset {
	return Preset{
		ID:              uuid.New().String(),
		Name:            "Default",
		ModelName:       "base-q5_1",
		KeepModelLoaded: false,
		InputMode:       "hold",
		Hotkey:          "",
		Language:        "auto",
		UseKBLayout:     false,
		KeepHistory:     true,
		Enabled:         false,
	}
}

// DefaultAppConfig returns defaults with one preset.
func DefaultAppConfig() *AppConfig {
	return &AppConfig{
		Theme:   "dark",
		UILang:  "en",
		Backend: "auto",
		Presets: []Preset{DefaultPreset()},
	}
}

// ConfigDir exports the config directory path for use by other packages.
func ConfigDir() (string, error) {
	return configDir()
}

// configDir returns the config directory.
// Priority: directory of the executable (portable), fallback to OS-standard.
func configDir() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		testFile := filepath.Join(dir, ".transcribation_write_test")
		if f, err := os.Create(testFile); err == nil {
			f.Close()
			os.Remove(testFile)
			return dir, nil
		}
	}
	return osConfigDir()
}

func osConfigDir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
	case "darwin":
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "Library", "Application Support")
	default:
		base = os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".config")
		}
	}
	dir := filepath.Join(base, "transcribation")
	return dir, os.MkdirAll(dir, 0o755)
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// oldConfig is the legacy flat config format for migration.
type oldConfig struct {
	ModelName    string `json:"modelName"`
	ModelsDir    string `json:"modelsDir"`
	Language     string `json:"language"`
	Translate    bool   `json:"translate"`
	HotkeyMod   string `json:"hotkeyMod"`
	HotkeyKey    string `json:"hotkeyKey"`
	MicrophoneID string `json:"microphoneId"`
	AutoStart    bool   `json:"autoStart"`
	RecordMode   string `json:"recordMode"`
	OutputMode   string `json:"outputMode"`
}

// migrateOldConfig converts old flat config to the new AppConfig format.
func migrateOldConfig(data []byte) *AppConfig {
	var old oldConfig
	if err := json.Unmarshal(data, &old); err != nil {
		return nil
	}

	hotkey := ""
	if old.HotkeyMod != "" && old.HotkeyKey != "" {
		hotkey = old.HotkeyMod + "+" + old.HotkeyKey
	} else if old.HotkeyKey != "" {
		hotkey = old.HotkeyKey
	}

	lang := old.Language
	if lang == "" {
		lang = "auto"
	}

	mode := old.RecordMode
	if mode == "" {
		mode = "hold"
	}

	modelName := old.ModelName
	if modelName == "" {
		modelName = "base-q5_1"
	}

	preset := Preset{
		ID:              uuid.New().String(),
		Name:            "Default",
		ModelName:       modelName,
		KeepModelLoaded: false,
		InputMode:       mode,
		Hotkey:          hotkey,
		Language:        lang,
		UseKBLayout:     false,
		KeepHistory:     true,
		Enabled:         false,
	}

	return &AppConfig{
		MicrophoneID: old.MicrophoneID,
		ModelsDir:    old.ModelsDir,
		Presets:      []Preset{preset},
	}
}

// Load reads config from disk. Migrates old format if detected.
func Load() (*AppConfig, error) {
	path, err := configPath()
	if err != nil {
		return DefaultAppConfig(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultAppConfig(), nil
	}

	// Try new format first
	cfg := &AppConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultAppConfig(), err
	}

	// Detect old format: has presets field → new format; no presets → old format
	if cfg.Presets == nil {
		// Try migration from old flat config
		if migrated := migrateOldConfig(data); migrated != nil {
			// Save migrated config
			_ = Save(migrated)
			return migrated, nil
		}
		return DefaultAppConfig(), nil
	}

	return cfg, nil
}

// Save writes config to disk.
func Save(cfg *AppConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
