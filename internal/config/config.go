package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// Config holds the user's application settings.
type Config struct {
	ModelName    string `json:"modelName"`
	ModelsDir    string `json:"modelsDir"`
	Language     string `json:"language"`
	Translate    bool   `json:"translate"`
	HotkeyMod   string `json:"hotkeyMod"`
	HotkeyKey    string `json:"hotkeyKey"`
	MicrophoneID string `json:"microphoneId"`
	AutoStart    bool   `json:"autoStart"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		ModelName:  "base-q5_1",
		Language:   "auto",
		Translate:  false,
		HotkeyMod:  "ctrl+shift",
		HotkeyKey:  "space",
		AutoStart:  false,
	}
}

// configDir returns the OS-appropriate config directory.
func configDir() (string, error) {
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

// Load reads config from disk, returning defaults if not found.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultConfig(), nil
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultConfig(), err
	}
	return cfg, nil
}

// Save writes config to disk.
func Save(cfg *Config) error {
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
