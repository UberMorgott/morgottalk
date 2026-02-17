package i18n

import (
	"fmt"
	"testing"
)

func TestT_ExistingKey(t *testing.T) {
	tests := []struct {
		lang, key, want string
	}{
		{"en", "tray_show", "Show"},
		{"en", "tray_quit", "Quit"},
		{"ru", "tray_quit", "Выход"},
		{"ru", "tray_show", "Показать"},
		{"de", "tray_show", "Anzeigen"},
		{"ja", "tray_quit", "終了"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s", tt.lang, tt.key), func(t *testing.T) {
			got := T(tt.lang, tt.key)
			if got != tt.want {
				t.Errorf("T(%q, %q) = %q, want %q", tt.lang, tt.key, got, tt.want)
			}
		})
	}
}

func TestT_FallbackToEnglish(t *testing.T) {
	tests := []struct {
		lang, key, want string
	}{
		{"xx", "tray_show", "Show"},
		{"xx", "close_quit", "Quit"},
		{"zz", "tray_history", "History"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s", tt.lang, tt.key), func(t *testing.T) {
			got := T(tt.lang, tt.key)
			if got != tt.want {
				t.Errorf("T(%q, %q) = %q, want %q (expected English fallback)", tt.lang, tt.key, got, tt.want)
			}
		})
	}
}

func TestT_MissingKey(t *testing.T) {
	keys := []string{"nonexistent_key_xyz", "no_such_key", ""}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			got := T("en", key)
			if got != key {
				t.Errorf("T(%q, %q) = %q, want the key itself returned", "en", key, got)
			}
			// Also verify missing key with unknown language returns the key.
			got = T("xx", key)
			if got != key {
				t.Errorf("T(%q, %q) = %q, want the key itself returned", "xx", key, got)
			}
		})
	}
}

func TestAllLanguagesHaveAllKeys(t *testing.T) {
	enKeys := translations["en"]
	if len(enKeys) == 0 {
		t.Fatal("English translations are empty")
	}

	for lang, langKeys := range translations {
		if lang == "en" {
			continue
		}
		for key := range enKeys {
			if _, ok := langKeys[key]; !ok {
				t.Errorf("language %q is missing key %q", lang, key)
			}
		}
		// Check for extra keys not present in English (likely typos).
		for key := range langKeys {
			if _, ok := enKeys[key]; !ok {
				t.Errorf("language %q has extra key %q not present in English", lang, key)
			}
		}
	}
}
