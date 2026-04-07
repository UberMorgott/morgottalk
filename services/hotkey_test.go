package services

import (
	"sort"
	"testing"
)

func TestParseHotkeyStr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantLen int // expected number of keys (if no error)
	}{
		{"single key", "a", false, 1},
		{"ctrl+a", "ctrl+a", false, 2},
		{"ctrl+shift+a", "ctrl+shift+a", false, 3},
		{"f1", "f1", false, 1},
		{"ctrl+f1", "ctrl+f1", false, 2},
		{"space", "space", false, 1},
		{"alt+enter", "alt+enter", false, 2},

		// Case insensitivity
		{"uppercase", "CTRL+A", false, 2},
		{"mixed case", "Ctrl+Shift+F1", false, 3},

		// Aliases
		{"escape alias", "escape", false, 1},
		{"cmd alias", "cmd+a", false, 2},
		{"meta alias", "meta+a", false, 2},
		{"win alias", "win+a", false, 2},
		{"option alias", "option+a", false, 2},

		// Whitespace handling
		{"with spaces", " ctrl + a ", false, 2},
		{"leading spaces", "  ctrl+a", false, 2},

		// Errors
		{"empty string", "", true, 0},
		{"unknown key", "ctrl+unknownkey", true, 0},
		{"spaces only", "   ", true, 0},
		{"trailing plus", "ctrl+", true, 0},   // empty part after split
		{"leading plus", "+a", true, 0},        // empty part after split
		{"duplicate modifier", "ctrl+ctrl", false, 2}, // parses as two identical keycodes
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, err := parseHotkeyStr(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHotkeyStr(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(keys) != tt.wantLen {
				t.Errorf("parseHotkeyStr(%q) returned %d keys, want %d", tt.input, len(keys), tt.wantLen)
			}
			if !tt.wantErr {
				// Keys must be sorted
				sorted := sort.SliceIsSorted(keys, func(i, j int) bool { return keys[i] < keys[j] })
				if !sorted {
					t.Errorf("parseHotkeyStr(%q) keys not sorted: %v", tt.input, keys)
				}
			}
		})
	}
}

func TestKeysToString(t *testing.T) {
	tests := []struct {
		name string
		keys []uint16
		want string
	}{
		{"single key a", []uint16{0x41}, "a"},
		{"ctrl+a", []uint16{0xA2, 0x41}, "ctrl+a"},
		{"reversed order", []uint16{0x41, 0xA2}, "ctrl+a"}, // modifiers first
		{"shift+ctrl+a", []uint16{0xA0, 0xA2, 0x41}, "ctrl+shift+a"},
		{"f1", []uint16{0x70}, "f1"},
		{"empty", []uint16{}, ""},
		{"unknown keycode", []uint16{0xFFFF}, ""}, // unknown code skipped
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keysToString(tt.keys)
			if got != tt.want {
				t.Errorf("keysToString(%v) = %q, want %q", tt.keys, got, tt.want)
			}
		})
	}
}

func TestParseKeysRoundTrip(t *testing.T) {
	// Parse → keysToString → parse again should yield same keycodes.
	combos := []string{
		"ctrl+a",
		"ctrl+shift+f1",
		"alt+enter",
		"shift+space",
		"f12",
		"rctrl+rshift+delete",
	}

	for _, combo := range combos {
		t.Run(combo, func(t *testing.T) {
			keys1, err := parseHotkeyStr(combo)
			if err != nil {
				t.Fatalf("parseHotkeyStr(%q) failed: %v", combo, err)
			}

			str := keysToString(keys1)
			if str == "" {
				t.Fatalf("keysToString(%v) returned empty string", keys1)
			}

			keys2, err := parseHotkeyStr(str)
			if err != nil {
				t.Fatalf("parseHotkeyStr(%q) round-trip failed: %v", str, err)
			}

			// Sort both for comparison
			sort.Slice(keys1, func(i, j int) bool { return keys1[i] < keys1[j] })
			sort.Slice(keys2, func(i, j int) bool { return keys2[i] < keys2[j] })

			if len(keys1) != len(keys2) {
				t.Fatalf("round-trip key count mismatch: %v vs %v", keys1, keys2)
			}
			for i := range keys1 {
				if keys1[i] != keys2[i] {
					t.Errorf("round-trip mismatch at %d: %d vs %d", i, keys1[i], keys2[i])
				}
			}
		})
	}
}

func TestParseAliasRoundTrip(t *testing.T) {
	// "escape" is an alias for VK 0x1B, whose canonical name is "esc".
	keys1, err := parseHotkeyStr("escape")
	if err != nil {
		t.Fatalf("parseHotkeyStr(%q) failed: %v", "escape", err)
	}

	str := keysToString(keys1)
	if str != "esc" {
		t.Fatalf("keysToString(%v) = %q, want %q", keys1, str, "esc")
	}

	keys2, err := parseHotkeyStr(str)
	if err != nil {
		t.Fatalf("parseHotkeyStr(%q) failed: %v", str, err)
	}

	if len(keys1) != len(keys2) || keys1[0] != keys2[0] {
		t.Errorf("alias round-trip mismatch: escape→%v, esc→%v", keys1, keys2)
	}
}

func TestMatchBinding(t *testing.T) {
	tests := []struct {
		name        string
		bindingKeys []uint16
		pressedKeys map[uint16]bool
		want        bool
	}{
		{
			"exact match single",
			[]uint16{0x41}, // a
			map[uint16]bool{0x41: true},
			true,
		},
		{
			"exact match combo",
			[]uint16{0xA2, 0x41}, // ctrl+a
			map[uint16]bool{0xA2: true, 0x41: true},
			true,
		},
		{
			"superset pressed",
			[]uint16{0xA2, 0x41}, // ctrl+a
			map[uint16]bool{0xA2: true, 0x41: true, 0xA0: true}, // ctrl+a+shift
			true,
		},
		{
			"missing key",
			[]uint16{0xA2, 0x41}, // ctrl+a
			map[uint16]bool{0xA2: true},
			false,
		},
		{
			"no keys pressed",
			[]uint16{0xA2, 0x41},
			map[uint16]bool{},
			false,
		},
		{
			"empty binding",
			[]uint16{},
			map[uint16]bool{0x41: true},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchBinding(tt.bindingKeys, tt.pressedKeys)
			if got != tt.want {
				t.Errorf("matchBinding(%v, %v) = %v, want %v", tt.bindingKeys, tt.pressedKeys, got, tt.want)
			}
		})
	}
}

func TestIsModifier(t *testing.T) {
	tests := []struct {
		name string
		kc   uint16
		want bool
	}{
		{"left ctrl", 0xA2, true},
		{"right ctrl", 0xA3, true},
		{"left shift", 0xA0, true},
		{"right shift", 0xA1, true},
		{"left alt", 0xA4, true},
		{"right alt", 0xA5, true},
		{"left super", 0x5B, true},
		{"right super", 0x5C, true},
		{"a key", 0x41, false},
		{"f1 key", 0x70, false},
		{"space", 0x20, false},
		{"zero", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isModifier(tt.kc)
			if got != tt.want {
				t.Errorf("isModifier(%d) = %v, want %v", tt.kc, got, tt.want)
			}
		})
	}
}
