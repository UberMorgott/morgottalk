package services

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	hook "github.com/robotn/gohook"
)

// HotkeyManager manages global hotkey registrations using gohook.
// Single event loop processes both hotkey matching and key capture.
type HotkeyManager struct {
	mu        sync.Mutex
	active    map[string]*hotkeyBinding // presetID → binding
	onPress   func(presetID string)
	onRelease func(presetID string)

	// Event loop
	running bool
	stop    chan struct{}

	// Key capture (for UI)
	capturing   bool
	captureCh   chan string
	captureKeys map[uint16]bool // modifiers accumulated during capture
}

type hotkeyBinding struct {
	keys    []uint16 // sorted keycodes
	mode    string   // "hold" | "toggle"
	pressed bool     // currently matched
}

func NewHotkeyManager(onPress, onRelease func(presetID string)) *HotkeyManager {
	return &HotkeyManager{
		active:    make(map[string]*hotkeyBinding),
		onPress:   onPress,
		onRelease: onRelease,
	}
}

// Start begins the global keyboard event loop.
func (m *HotkeyManager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stop = make(chan struct{})
	m.mu.Unlock()

	go m.eventLoop()
	log.Println("HotkeyManager: event loop started")
}

// Stop terminates the event loop and cancels any pending capture.
func (m *HotkeyManager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stop)

	if m.capturing {
		m.capturing = false
		m.captureKeys = nil
		if m.captureCh != nil {
			m.captureCh <- ""
		}
	}
	m.mu.Unlock()

	// hook.End() may block on some platforms — don't hold mutex
	go hook.End()
	log.Println("HotkeyManager: stopped")
}

// Register adds a hotkey binding for a preset.
func (m *HotkeyManager) Register(presetID, hotkeyStr, mode string) error {
	keys, err := parseHotkeyStr(hotkeyStr)
	if err != nil {
		return fmt.Errorf("parse hotkey %q: %w", hotkeyStr, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.active[presetID] = &hotkeyBinding{keys: keys, mode: mode}
	log.Printf("Hotkey registered: %q for preset %s (mode=%s)", hotkeyStr, presetID, mode)
	return nil
}

// Unregister removes a hotkey for a preset.
func (m *HotkeyManager) Unregister(presetID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.active, presetID)
	log.Printf("Hotkey unregistered for preset %s", presetID)
}

// UnregisterAll removes all hotkey bindings.
func (m *HotkeyManager) UnregisterAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = make(map[string]*hotkeyBinding)
}

// CaptureHotkey blocks until the user presses a key/combo and returns it as a string.
// Returns "" if cancelled (Escape or CancelCapture).
func (m *HotkeyManager) CaptureHotkey() string {
	ch := make(chan string, 1)

	m.mu.Lock()
	m.capturing = true
	m.captureCh = ch
	m.captureKeys = nil
	m.mu.Unlock()

	result := <-ch
	return result
}

// CancelCapture cancels an in-progress key capture.
func (m *HotkeyManager) CancelCapture() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.capturing {
		m.capturing = false
		m.captureKeys = nil
		if m.captureCh != nil {
			m.captureCh <- ""
		}
	}
}

// eventLoop is the main event processing goroutine.
func (m *HotkeyManager) eventLoop() {
	evChan := hook.Start()
	pressedKeys := make(map[uint16]bool)

	for {
		select {
		case <-m.stop:
			return
		case ev, ok := <-evChan:
			if !ok {
				return
			}
			switch ev.Kind {
			case hook.KeyDown:
				kc := ev.Keycode
				pressedKeys[kc] = true
				m.handleKeyDown(kc, pressedKeys)

			case hook.KeyUp:
				kc := ev.Keycode
				delete(pressedKeys, kc)
				m.handleKeyUp(kc, pressedKeys)
			}
		}
	}
}

func (m *HotkeyManager) handleKeyDown(kc uint16, pressedKeys map[uint16]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.capturing {
		m.captureKeyDown(kc, pressedKeys)
		return
	}

	for id, b := range m.active {
		if !b.pressed && matchBinding(b.keys, pressedKeys) {
			b.pressed = true
			if m.onPress != nil {
				go m.onPress(id)
			}
		}
	}
}

func (m *HotkeyManager) handleKeyUp(kc uint16, pressedKeys map[uint16]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.capturing {
		m.captureKeyUp(kc, pressedKeys)
		return
	}

	for id, b := range m.active {
		if b.pressed && !matchBinding(b.keys, pressedKeys) {
			b.pressed = false
			if m.onRelease != nil {
				go m.onRelease(id)
			}
		}
	}
}

// captureKeyDown handles key presses during capture mode.
func (m *HotkeyManager) captureKeyDown(kc uint16, pressedKeys map[uint16]bool) {
	// Escape cancels capture
	if kc == kcEscape {
		m.capturing = false
		m.captureKeys = nil
		m.captureCh <- ""
		return
	}

	if isModifier(kc) {
		// Accumulate modifiers for modifier-only capture
		if m.captureKeys == nil {
			m.captureKeys = make(map[uint16]bool)
		}
		m.captureKeys[kc] = true
		return
	}

	// Non-modifier pressed → finalize with all currently held keys
	keys := make([]uint16, 0, len(pressedKeys))
	for pk := range pressedKeys {
		if pk != kcEscape {
			keys = append(keys, pk)
		}
	}
	m.finishCapture(keys)
}

// captureKeyUp handles key releases during capture mode.
// Finalizes modifier-only captures when all modifiers are released.
func (m *HotkeyManager) captureKeyUp(kc uint16, pressedKeys map[uint16]bool) {
	if m.captureKeys == nil || len(m.captureKeys) == 0 || !isModifier(kc) {
		return
	}

	// kc already removed from pressedKeys by caller.
	// Check if any accumulated modifier is still physically held.
	for ck := range m.captureKeys {
		if pressedKeys[ck] {
			return // other modifiers still held
		}
	}

	// All capture modifiers released → finalize
	keys := make([]uint16, 0, len(m.captureKeys))
	for ck := range m.captureKeys {
		keys = append(keys, ck)
	}
	m.finishCapture(keys)
}

func (m *HotkeyManager) finishCapture(keys []uint16) {
	hotkeyStr := keysToString(keys)
	m.capturing = false
	m.captureKeys = nil
	m.captureCh <- hotkeyStr
}

// matchBinding returns true if all binding keys are currently pressed.
func matchBinding(bindingKeys []uint16, pressedKeys map[uint16]bool) bool {
	if len(bindingKeys) == 0 {
		return false
	}
	for _, kc := range bindingKeys {
		if !pressedKeys[kc] {
			return false
		}
	}
	return true
}

// --- Keycode maps ---

const kcEscape = 1

var modifierKeycodes = map[uint16]bool{
	29:   true, // ctrl (left)
	3613: true, // rctrl
	42:   true, // shift (left)
	54:   true, // rshift
	56:   true, // alt (left)
	3640: true, // ralt
	3675: true, // super/cmd (left)
	3676: true, // rsuper/rcmd
}

func isModifier(kc uint16) bool {
	return modifierKeycodes[kc]
}

// keycodeToName maps libuiohook virtual keycodes to display names.
var keycodeToName = map[uint16]string{
	// Modifiers
	29:   "ctrl",
	3613: "rctrl",
	42:   "shift",
	54:   "rshift",
	56:   "alt",
	3640: "ralt",
	3675: "super",
	3676: "rsuper",
	// Letters
	30: "a", 48: "b", 46: "c", 32: "d", 18: "e", 33: "f", 34: "g",
	35: "h", 23: "i", 36: "j", 37: "k", 38: "l", 50: "m", 49: "n",
	24: "o", 25: "p", 16: "q", 19: "r", 31: "s", 20: "t", 22: "u",
	47: "v", 17: "w", 45: "x", 21: "y", 44: "z",
	// Numbers
	2: "1", 3: "2", 4: "3", 5: "4", 6: "5",
	7: "6", 8: "7", 9: "8", 10: "9", 11: "0",
	// Function keys
	59: "f1", 60: "f2", 61: "f3", 62: "f4", 63: "f5", 64: "f6",
	65: "f7", 66: "f8", 67: "f9", 68: "f10", 69: "f11", 70: "f12",
	// Special
	1:  "esc",
	14: "backspace",
	15: "tab",
	28: "enter",
	57: "space",
	// Arrows
	57416: "up",
	57424: "down",
	57419: "left",
	57421: "right",
	// Navigation
	57415: "home",
	57423: "end",
	57417: "pageup",
	57425: "pagedown",
	57426: "insert",
	57427: "delete",
	// Misc
	58:   "capslock",
	3639: "printscreen",
	3653: "pause",
	// Numpad
	71: "num7", 72: "num8", 73: "num9",
	75: "num4", 76: "num5", 77: "num6",
	79: "num1", 80: "num2", 81: "num3",
	82: "num0",
	74: "num-", 78: "num+", 55: "num*",
	3637: "num/", 3612: "numenter",
	// Symbols
	12: "-", 13: "=",
	26: "[", 27: "]", 43: "\\",
	39: ";", 40: "'",
	51: ",", 52: ".", 53: "/",
	41: "`",
}

// nameToKeycode is the reverse map, built at init.
var nameToKeycode map[string]uint16

func init() {
	nameToKeycode = make(map[string]uint16, len(keycodeToName)+10)
	for kc, name := range keycodeToName {
		nameToKeycode[name] = kc
	}
	// Aliases for compatibility
	nameToKeycode["escape"] = 1
	nameToKeycode["return"] = 28
	nameToKeycode["del"] = 57427
	nameToKeycode["control"] = 29
	nameToKeycode["cmd"] = 3675
	nameToKeycode["command"] = 3675
	nameToKeycode["meta"] = 3675
	nameToKeycode["win"] = 3675
	nameToKeycode["option"] = 56
}

// parseHotkeyStr parses "ctrl+shift+a" into sorted keycodes.
func parseHotkeyStr(s string) ([]uint16, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return nil, fmt.Errorf("empty hotkey string")
	}

	parts := strings.Split(s, "+")
	keys := make([]uint16, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		kc, ok := nameToKeycode[p]
		if !ok {
			return nil, fmt.Errorf("unknown key: %q", p)
		}
		keys = append(keys, kc)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys, nil
}

// keysToString converts keycodes to a display string like "ctrl+shift+a".
// Modifiers are placed first, sorted; then regular keys, sorted.
func keysToString(keys []uint16) string {
	sort.Slice(keys, func(i, j int) bool {
		mi, mj := isModifier(keys[i]), isModifier(keys[j])
		if mi != mj {
			return mi // modifiers first
		}
		return keys[i] < keys[j]
	})

	parts := make([]string, 0, len(keys))
	for _, kc := range keys {
		if name, ok := keycodeToName[kc]; ok {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, "+")
}
