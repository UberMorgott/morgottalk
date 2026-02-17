package config

import (
	"encoding/json"
	"testing"
)

func TestDefaultPreset(t *testing.T) {
	p := DefaultPreset()

	if p.ID == "" {
		t.Error("ID should be non-empty UUID")
	}
	if p.Name != "Default" {
		t.Errorf("Name = %q, want %q", p.Name, "Default")
	}
	if p.ModelName != "base-q5_1" {
		t.Errorf("ModelName = %q, want %q", p.ModelName, "base-q5_1")
	}
	if p.KeepModelLoaded {
		t.Error("KeepModelLoaded should be false")
	}
	if p.InputMode != "hold" {
		t.Errorf("InputMode = %q, want %q", p.InputMode, "hold")
	}
	if p.Hotkey != "" {
		t.Errorf("Hotkey = %q, want empty", p.Hotkey)
	}
	if p.Language != "auto" {
		t.Errorf("Language = %q, want %q", p.Language, "auto")
	}
	if p.UseKBLayout {
		t.Error("UseKBLayout should be false")
	}
	if !p.KeepHistory {
		t.Error("KeepHistory should be true")
	}
	if p.Enabled {
		t.Error("Enabled should be false")
	}

	// Two calls should produce different IDs
	p2 := DefaultPreset()
	if p.ID == p2.ID {
		t.Error("each DefaultPreset() call should generate a unique ID")
	}
}

func TestDefaultAppConfig(t *testing.T) {
	cfg := DefaultAppConfig()

	if cfg == nil {
		t.Fatal("DefaultAppConfig() returned nil")
	}
	if cfg.Backend != "auto" {
		t.Errorf("Backend = %q, want %q", cfg.Backend, "auto")
	}
	if cfg.Theme != "dark" {
		t.Errorf("Theme = %q, want %q", cfg.Theme, "dark")
	}
	if cfg.UILang != "en" {
		t.Errorf("UILang = %q, want %q", cfg.UILang, "en")
	}
	if cfg.Presets == nil {
		t.Fatal("Presets should be non-nil")
	}
	if len(cfg.Presets) != 0 {
		t.Errorf("Presets length = %d, want 0", len(cfg.Presets))
	}
	if cfg.OnboardingDone {
		t.Error("OnboardingDone should be false")
	}
	if cfg.MicrophoneID != "" {
		t.Errorf("MicrophoneID = %q, want empty", cfg.MicrophoneID)
	}
	if cfg.AutoStart {
		t.Error("AutoStart should be false")
	}
	if cfg.StartMinimized {
		t.Error("StartMinimized should be false")
	}
	if cfg.CloseAction != "" {
		t.Errorf("CloseAction = %q, want empty", cfg.CloseAction)
	}
}

func TestMigrateOldConfig(t *testing.T) {
	tests := []struct {
		name        string
		input       oldConfig
		wantHotkey  string
		wantEnabled bool
		wantLang    string
		wantMode    string
		wantModel   string
	}{
		{
			name: "full old config with mod+key",
			input: oldConfig{
				HotkeyMod:   "ctrl",
				HotkeyKey:   "f1",
				Language:     "en",
				RecordMode:   "toggle",
				ModelName:    "large-v3",
				MicrophoneID: "mic-1",
				ModelsDir:    "/tmp/models",
			},
			wantHotkey:  "ctrl+f1",
			wantEnabled: true,
			wantLang:    "en",
			wantMode:    "toggle",
			wantModel:   "large-v3",
		},
		{
			name: "key only, no mod",
			input: oldConfig{
				HotkeyKey: "f1",
				Language:  "ru",
			},
			wantHotkey:  "f1",
			wantEnabled: true,
			wantLang:    "ru",
			wantMode:    "hold",
			wantModel:   "base-q5_1",
		},
		{
			name:        "no hotkey at all",
			input:       oldConfig{Language: "de"},
			wantHotkey:  "",
			wantEnabled: false,
			wantLang:    "de",
			wantMode:    "hold",
			wantModel:   "base-q5_1",
		},
		{
			name:        "empty language defaults to auto",
			input:       oldConfig{},
			wantHotkey:  "",
			wantEnabled: false,
			wantLang:    "auto",
			wantMode:    "hold",
			wantModel:   "base-q5_1",
		},
		{
			name:        "empty recordMode defaults to hold",
			input:       oldConfig{RecordMode: ""},
			wantHotkey:  "",
			wantEnabled: false,
			wantLang:    "auto",
			wantMode:    "hold",
			wantModel:   "base-q5_1",
		},
		{
			name:        "empty modelName defaults to base-q5_1",
			input:       oldConfig{ModelName: ""},
			wantHotkey:  "",
			wantEnabled: false,
			wantLang:    "auto",
			wantMode:    "hold",
			wantModel:   "base-q5_1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.input)
			if err != nil {
				t.Fatalf("marshal input: %v", err)
			}

			cfg := migrateOldConfig(data)
			if cfg == nil {
				t.Fatal("migrateOldConfig returned nil")
			}
			if len(cfg.Presets) != 1 {
				t.Fatalf("Presets length = %d, want 1", len(cfg.Presets))
			}

			p := cfg.Presets[0]
			if p.Hotkey != tc.wantHotkey {
				t.Errorf("Hotkey = %q, want %q", p.Hotkey, tc.wantHotkey)
			}
			if p.Enabled != tc.wantEnabled {
				t.Errorf("Enabled = %v, want %v", p.Enabled, tc.wantEnabled)
			}
			if p.Language != tc.wantLang {
				t.Errorf("Language = %q, want %q", p.Language, tc.wantLang)
			}
			if p.InputMode != tc.wantMode {
				t.Errorf("InputMode = %q, want %q", p.InputMode, tc.wantMode)
			}
			if p.ModelName != tc.wantModel {
				t.Errorf("ModelName = %q, want %q", p.ModelName, tc.wantModel)
			}
			if p.ID == "" {
				t.Error("preset ID should be non-empty")
			}
			if p.Name != "Default" {
				t.Errorf("preset Name = %q, want %q", p.Name, "Default")
			}
			if !p.KeepHistory {
				t.Error("KeepHistory should be true")
			}

			// Verify parent config fields are preserved
			if cfg.MicrophoneID != tc.input.MicrophoneID {
				t.Errorf("MicrophoneID = %q, want %q", cfg.MicrophoneID, tc.input.MicrophoneID)
			}
			if cfg.ModelsDir != tc.input.ModelsDir {
				t.Errorf("ModelsDir = %q, want %q", cfg.ModelsDir, tc.input.ModelsDir)
			}
		})
	}
}

func TestMigrateOldConfig_MalformedJSON(t *testing.T) {
	result := migrateOldConfig([]byte(`{invalid json`))
	if result != nil {
		t.Error("migrateOldConfig should return nil for malformed JSON")
	}

	result = migrateOldConfig([]byte(`not json at all`))
	if result != nil {
		t.Error("migrateOldConfig should return nil for non-JSON input")
	}
}

func TestAppConfigJSONRoundtrip(t *testing.T) {
	original := &AppConfig{
		MicrophoneID:   "mic-42",
		ModelsDir:      "/home/user/models",
		Theme:          "light",
		UILang:         "ru",
		CloseAction:    "tray",
		AutoStart:      true,
		StartMinimized: true,
		Backend:        "cuda",
		OnboardingDone: true,
		Presets: []Preset{
			{
				ID:              "test-id-1",
				Name:            "Work",
				ModelName:       "large-v3",
				KeepModelLoaded: true,
				InputMode:       "toggle",
				Hotkey:          "ctrl+shift+f1",
				Language:        "en",
				UseKBLayout:     true,
				KeepHistory:     false,
				Enabled:         true,
			},
			{
				ID:        "test-id-2",
				Name:      "Quick",
				ModelName: "base-q5_1",
				InputMode: "hold",
				Language:  "auto",
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	restored := &AppConfig{}
	if err := json.Unmarshal(data, restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Compare top-level fields
	if restored.MicrophoneID != original.MicrophoneID {
		t.Errorf("MicrophoneID = %q, want %q", restored.MicrophoneID, original.MicrophoneID)
	}
	if restored.ModelsDir != original.ModelsDir {
		t.Errorf("ModelsDir = %q, want %q", restored.ModelsDir, original.ModelsDir)
	}
	if restored.Theme != original.Theme {
		t.Errorf("Theme = %q, want %q", restored.Theme, original.Theme)
	}
	if restored.UILang != original.UILang {
		t.Errorf("UILang = %q, want %q", restored.UILang, original.UILang)
	}
	if restored.CloseAction != original.CloseAction {
		t.Errorf("CloseAction = %q, want %q", restored.CloseAction, original.CloseAction)
	}
	if restored.AutoStart != original.AutoStart {
		t.Errorf("AutoStart = %v, want %v", restored.AutoStart, original.AutoStart)
	}
	if restored.StartMinimized != original.StartMinimized {
		t.Errorf("StartMinimized = %v, want %v", restored.StartMinimized, original.StartMinimized)
	}
	if restored.Backend != original.Backend {
		t.Errorf("Backend = %q, want %q", restored.Backend, original.Backend)
	}
	if restored.OnboardingDone != original.OnboardingDone {
		t.Errorf("OnboardingDone = %v, want %v", restored.OnboardingDone, original.OnboardingDone)
	}

	// Compare presets
	if len(restored.Presets) != len(original.Presets) {
		t.Fatalf("Presets length = %d, want %d", len(restored.Presets), len(original.Presets))
	}
	for i := range original.Presets {
		op := original.Presets[i]
		rp := restored.Presets[i]
		if rp.ID != op.ID {
			t.Errorf("Preset[%d].ID = %q, want %q", i, rp.ID, op.ID)
		}
		if rp.Name != op.Name {
			t.Errorf("Preset[%d].Name = %q, want %q", i, rp.Name, op.Name)
		}
		if rp.ModelName != op.ModelName {
			t.Errorf("Preset[%d].ModelName = %q, want %q", i, rp.ModelName, op.ModelName)
		}
		if rp.KeepModelLoaded != op.KeepModelLoaded {
			t.Errorf("Preset[%d].KeepModelLoaded = %v, want %v", i, rp.KeepModelLoaded, op.KeepModelLoaded)
		}
		if rp.InputMode != op.InputMode {
			t.Errorf("Preset[%d].InputMode = %q, want %q", i, rp.InputMode, op.InputMode)
		}
		if rp.Hotkey != op.Hotkey {
			t.Errorf("Preset[%d].Hotkey = %q, want %q", i, rp.Hotkey, op.Hotkey)
		}
		if rp.Language != op.Language {
			t.Errorf("Preset[%d].Language = %q, want %q", i, rp.Language, op.Language)
		}
		if rp.UseKBLayout != op.UseKBLayout {
			t.Errorf("Preset[%d].UseKBLayout = %v, want %v", i, rp.UseKBLayout, op.UseKBLayout)
		}
		if rp.KeepHistory != op.KeepHistory {
			t.Errorf("Preset[%d].KeepHistory = %v, want %v", i, rp.KeepHistory, op.KeepHistory)
		}
		if rp.Enabled != op.Enabled {
			t.Errorf("Preset[%d].Enabled = %v, want %v", i, rp.Enabled, op.Enabled)
		}
	}
}
