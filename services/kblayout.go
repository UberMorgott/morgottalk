package services

import (
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// layoutToLang maps common keyboard layout codes to whisper language codes.
var layoutToLang = map[string]string{
	"us": "en", "gb": "en", "en": "en",
	"ru": "ru",
	"de": "de",
	"fr": "fr",
	"es": "es",
	"it": "it",
	"pt": "pt",
	"nl": "nl",
	"pl": "pl",
	"uk": "uk", "ua": "uk",
	"tr": "tr",
	"ar": "ar",
	"cs": "cs", "cz": "cs",
	"da": "da", "dk": "da",
	"fi": "fi",
	"el": "el", "gr": "el",
	"he": "he", "il": "he",
	"hi": "hi", "in": "hi",
	"hu": "hu",
	"id": "id",
	"ja": "ja", "jp": "ja",
	"ko": "ko", "kr": "ko",
	"ms": "ms",
	"no": "no",
	"ro": "ro",
	"sk": "sk",
	"sv": "sv", "se": "sv",
	"th": "th",
	"vi": "vi", "vn": "vi",
	"zh": "zh", "cn": "zh", "tw": "zh",
}

// detectKeyboardLanguage returns a whisper language code based on the current
// keyboard layout. Returns "" if detection fails.
func detectKeyboardLanguage() string {
	var layout string

	switch runtime.GOOS {
	case "linux":
		layout = detectLayoutLinux()
	case "darwin":
		layout = detectLayoutDarwin()
	case "windows":
		layout = detectLayoutWindows()
	}

	if layout == "" {
		return ""
	}

	// Normalize: take the first part before any variant (e.g. "us(intl)" → "us")
	layout = strings.ToLower(layout)
	if idx := strings.IndexAny(layout, "(-_"); idx > 0 {
		layout = layout[:idx]
	}
	layout = strings.TrimSpace(layout)

	if lang, ok := layoutToLang[layout]; ok {
		return lang
	}
	return ""
}

// detectLayoutLinux tries multiple methods to detect the current keyboard layout.
func detectLayoutLinux() string {
	// 1. KDE Plasma 6 via qdbus6: getLayout() returns index, getLayoutsList() returns layouts
	if layout := detectLayoutKDE(); layout != "" {
		return layout
	}

	// 2. xkb-switch — works on both X11 and some Wayland setups
	if out, err := exec.Command("xkb-switch").Output(); err == nil {
		s := strings.TrimSpace(string(out))
		if s != "" {
			return s
		}
	}

	// 3. setxkbmap — X11 only (unreliable on Wayland, always returns first layout)
	// Kept as last resort for X11 sessions.
	if out, err := exec.Command("setxkbmap", "-query").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "layout:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					layouts := strings.TrimSpace(parts[1])
					if idx := strings.Index(layouts, ","); idx > 0 {
						return layouts[:idx]
					}
					return layouts
				}
			}
		}
	}

	return ""
}

// detectLayoutKDE gets the current active layout from KDE Plasma via DBus.
// Uses getLayout() to get index and getLayoutsList() to get layout codes.
func detectLayoutKDE() string {
	// Get current layout index
	idxOut, err := exec.Command("qdbus6",
		"org.kde.keyboard", "/Layouts",
		"org.kde.KeyboardLayouts.getLayout").Output()
	if err != nil {
		return ""
	}

	idx, err := strconv.Atoi(strings.TrimSpace(string(idxOut)))
	if err != nil {
		return ""
	}

	// Get layouts list via dbus-send (qdbus6 can't display a(sss) without --literal)
	listOut, err := exec.Command("dbus-send", "--session",
		"--dest=org.kde.keyboard", "--print-reply",
		"/Layouts", "org.kde.KeyboardLayouts.getLayoutsList").Output()
	if err != nil {
		// Fallback: try qdbus6 --literal
		return detectLayoutKDELiteral(idx)
	}

	// Parse dbus-send output: extract layout codes from struct { string "us" ... }
	// Each struct has 3 strings; the first one is the layout code.
	layouts := parseDBusSendLayouts(string(listOut))
	if idx >= 0 && idx < len(layouts) {
		return layouts[idx]
	}

	return ""
}

// parseDBusSendLayouts extracts layout codes from dbus-send --print-reply output.
// Format: struct { string "us" string "" string "English (US)" }
var reDBusString = regexp.MustCompile(`string "([^"]*)"`)

func parseDBusSendLayouts(output string) []string {
	var layouts []string
	inStruct := false
	fieldIdx := 0

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "struct {" {
			inStruct = true
			fieldIdx = 0
			continue
		}
		if trimmed == "}" {
			inStruct = false
			continue
		}
		if inStruct {
			if m := reDBusString.FindStringSubmatch(trimmed); m != nil {
				if fieldIdx == 0 {
					// First string in struct is the layout code
					layouts = append(layouts, m[1])
				}
				fieldIdx++
			}
		}
	}
	return layouts
}

// detectLayoutKDELiteral fallback: parse qdbus6 --literal output.
// Format: [Argument: a(sss) {[Argument: (sss) "us", "", "English"], ...}]
func detectLayoutKDELiteral(idx int) string {
	out, err := exec.Command("qdbus6", "--literal",
		"org.kde.keyboard", "/Layouts",
		"org.kde.KeyboardLayouts.getLayoutsList").Output()
	if err != nil {
		return ""
	}

	// Extract quoted strings from each (sss) group
	// Pattern: [Argument: (sss) "us", "", "Английская (США)"]
	re := regexp.MustCompile(`\[Argument: \(sss\) "([^"]*)"`)
	matches := re.FindAllStringSubmatch(string(out), -1)

	if idx >= 0 && idx < len(matches) {
		return matches[idx][1]
	}
	return ""
}

// detectLayoutDarwin reads the current input source on macOS.
func detectLayoutDarwin() string {
	script := `tell application "System Events" to get name of current input source of keyboard preferences`
	if out, err := exec.Command("osascript", "-e", script).Output(); err == nil {
		s := strings.TrimSpace(string(out))
		// macOS returns names like "U.S.", "Russian", "German" etc.
		return macInputSourceToCode(s)
	}
	return ""
}

// detectLayoutWindows reads the currently active keyboard layout on Windows.
func detectLayoutWindows() string {
	// InputLanguage.CurrentInputLanguage returns the *active* layout (not just configured list)
	ps := `Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.InputLanguage]::CurrentInputLanguage.Culture.TwoLetterISOLanguageName`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", ps)
	hideWindow(cmd)
	if out, err := cmd.Output(); err == nil {
		s := strings.TrimSpace(string(out))
		if s != "" {
			return s
		}
	}
	return ""
}

// macInputSourceToCode converts macOS input source names to layout codes.
func macInputSourceToCode(name string) string {
	name = strings.ToLower(name)
	macNames := map[string]string{
		"u.s.": "us", "abc": "us", "british": "gb",
		"russian": "ru", "german": "de", "french": "fr",
		"spanish": "es", "italian": "it", "portuguese": "pt",
		"dutch": "nl", "polish": "pl", "ukrainian": "uk",
		"turkish": "tr", "arabic": "ar", "czech": "cs",
		"danish": "da", "finnish": "fi", "greek": "el",
		"hebrew": "he", "hindi": "hi", "hungarian": "hu",
		"japanese": "ja", "korean": "ko", "norwegian": "no",
		"romanian": "ro", "slovak": "sk", "swedish": "sv",
		"thai": "th", "vietnamese": "vi", "chinese": "zh",
	}
	for key, code := range macNames {
		if strings.Contains(name, key) {
			return code
		}
	}
	return ""
}
