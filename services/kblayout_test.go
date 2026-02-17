package services

import (
	"testing"
)

func TestParseDBusSendLayouts(t *testing.T) {
	input := `method return time=1708300000.000000 sender=:1.42 -> destination=:1.99 serial=123 reply_serial=456
   array [
      struct {
         string "us"
         string ""
         string "English (US)"
      }
      struct {
         string "ru"
         string ""
         string "Russian"
      }
   ]`

	got := parseDBusSendLayouts(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 layouts, got %d: %v", len(got), got)
	}
	if got[0] != "us" {
		t.Errorf("layouts[0] = %q, want %q", got[0], "us")
	}
	if got[1] != "ru" {
		t.Errorf("layouts[1] = %q, want %q", got[1], "ru")
	}
}

func TestParseDBusSendLayouts_Empty(t *testing.T) {
	got := parseDBusSendLayouts("")
	if len(got) != 0 {
		t.Fatalf("expected 0 layouts for empty input, got %d: %v", len(got), got)
	}
}

func TestParseDBusSendLayouts_Single(t *testing.T) {
	input := `   struct {
      string "de"
      string ""
      string "German"
   }`

	got := parseDBusSendLayouts(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 layout, got %d: %v", len(got), got)
	}
	if got[0] != "de" {
		t.Errorf("layouts[0] = %q, want %q", got[0], "de")
	}
}

func TestMacInputSourceToCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"U.S.", "us"},
		{"Russian", "ru"},
		{"German", "de"},
		{"Japanese", "ja"},
		{"unknown", ""},
		{"ABC", "us"},
		{"British", "gb"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := macInputSourceToCode(tt.input)
			if got != tt.want {
				t.Errorf("macInputSourceToCode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLayoutToLang_Completeness(t *testing.T) {
	for layout, lang := range layoutToLang {
		if len(lang) != 2 {
			t.Errorf("layoutToLang[%q] = %q, want a 2-letter language code", layout, lang)
		}
	}
}
