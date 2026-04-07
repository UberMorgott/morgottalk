package services

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
)

// HotkeyManager manages global hotkey registrations using a platform keyboard hook.
// Single event loop processes both hotkey matching and key capture.
type HotkeyManager struct {
	mu        sync.Mutex
	active    map[string]*hotkeyBinding // presetID → binding
	onPress   func(presetID string)
	onRelease func(presetID string)

	// Event loop
	running bool
	stop    chan struct{}

	// Hook status callback
	onHookStatus func(ok bool, msg string)

	// Key capture (for UI)
	capturing   bool
	captureCh   chan string
	captureKeys map[uint16]bool // modifiers accumulated during capture
}

type hotkeyBinding struct {
	keys    []uint16 // sorted VK codes
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

// SetOnHookStatus sets a callback for reporting hook installation status.
func (m *HotkeyManager) SetOnHookStatus(fn func(bool, string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onHookStatus = fn
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

	// stopHook posts WM_QUIT — non-blocking
	stopHook()
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

// keyEvent is sent from the hook callback to the processing goroutine.
type keyEvent struct {
	vk   uint16
	down bool
}

// eventLoop runs the keyboard hook and processes key events.
func (m *HotkeyManager) eventLoop() {
	keyCh := make(chan keyEvent, 256)
	pressedKeys := make(map[uint16]bool)

	// onKey is called from the hook thread — must return fast.
	// Sends to a buffered channel (non-blocking) to avoid holding up the hook.
	onKey := func(vk uint16, down bool) {
		select {
		case keyCh <- keyEvent{vk, down}:
		default:
			// Channel full — drop event (shouldn't happen with 256 buffer)
		}
	}

	onInstalled := func(err error) {
		if err != nil {
			log.Printf("ERROR: keyboard hook failed: %v", err)
			m.mu.Lock()
			cb := m.onHookStatus
			m.mu.Unlock()
			if cb != nil {
				cb(false, err.Error())
			}
		} else {
			log.Println("HotkeyManager: keyboard hook installed successfully")
			m.mu.Lock()
			cb := m.onHookStatus
			m.mu.Unlock()
			if cb != nil {
				cb(true, "")
			}
		}
	}

	// Process key events on a separate goroutine
	go func() {
		for {
			select {
			case <-m.stop:
				return
			case ev := <-keyCh:
				if ev.down {
					pressedKeys[ev.vk] = true
					m.handleKeyDown(ev.vk, pressedKeys)
				} else {
					delete(pressedKeys, ev.vk)
					m.handleKeyUp(ev.vk, pressedKeys)
				}
			}
		}
	}()

	// startHook blocks in the message pump until stopHook() is called
	if err := startHook(onKey, onInstalled); err != nil {
		// Already reported via onInstalled callback
		log.Printf("HotkeyManager: startHook returned: %v", err)
	}
	log.Println("HotkeyManager: event loop ended")
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
	if kc == vkEscape {
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
		if pk != vkEscape {
			keys = append(keys, pk)
		}
	}
	m.finishCapture(keys)
}

// captureKeyUp handles key releases during capture mode.
// Finalizes modifier-only captures when all modifiers are released.
func (m *HotkeyManager) captureKeyUp(kc uint16, pressedKeys map[uint16]bool) {
	if len(m.captureKeys) == 0 || !isModifier(kc) {
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

// --- VK code constants and maps ---

const vkEscape = 0x1B

// Windows Virtual Key codes for modifiers.
var modifierVKCodes = map[uint16]bool{
	0xA0: true, // VK_LSHIFT
	0xA1: true, // VK_RSHIFT
	0xA2: true, // VK_LCONTROL
	0xA3: true, // VK_RCONTROL
	0xA4: true, // VK_LMENU (left alt)
	0xA5: true, // VK_RMENU (right alt)
	0x5B: true, // VK_LWIN
	0x5C: true, // VK_RWIN
}

func isModifier(kc uint16) bool {
	return modifierVKCodes[kc]
}

// vkToName maps Windows Virtual Key codes to display names.
var vkToName = map[uint16]string{
	// Modifiers (left/right specific — the hook gives us these)
	0xA2: "ctrl",
	0xA3: "rctrl",
	0xA0: "shift",
	0xA1: "rshift",
	0xA4: "alt",
	0xA5: "ralt",
	0x5B: "super",
	0x5C: "rsuper",
	// Letters (VK_A .. VK_Z = 0x41..0x5A)
	0x41: "a", 0x42: "b", 0x43: "c", 0x44: "d", 0x45: "e", 0x46: "f", 0x47: "g",
	0x48: "h", 0x49: "i", 0x4A: "j", 0x4B: "k", 0x4C: "l", 0x4D: "m", 0x4E: "n",
	0x4F: "o", 0x50: "p", 0x51: "q", 0x52: "r", 0x53: "s", 0x54: "t", 0x55: "u",
	0x56: "v", 0x57: "w", 0x58: "x", 0x59: "y", 0x5A: "z",
	// Numbers (VK_0..VK_9 = 0x30..0x39)
	0x31: "1", 0x32: "2", 0x33: "3", 0x34: "4", 0x35: "5",
	0x36: "6", 0x37: "7", 0x38: "8", 0x39: "9", 0x30: "0",
	// Function keys (VK_F1..VK_F12 = 0x70..0x7B)
	0x70: "f1", 0x71: "f2", 0x72: "f3", 0x73: "f4", 0x74: "f5", 0x75: "f6",
	0x76: "f7", 0x77: "f8", 0x78: "f9", 0x79: "f10", 0x7A: "f11", 0x7B: "f12",
	// Special
	0x1B: "esc",
	0x08: "backspace",
	0x09: "tab",
	0x0D: "enter",
	0x20: "space",
	// Arrows
	0x26: "up",
	0x28: "down",
	0x25: "left",
	0x27: "right",
	// Navigation
	0x24: "home",
	0x23: "end",
	0x21: "pageup",
	0x22: "pagedown",
	0x2D: "insert",
	0x2E: "delete",
	// Misc
	0x14: "capslock",
	0x2C: "printscreen",
	0x13: "pause",
	// Numpad
	0x67: "num7", 0x68: "num8", 0x69: "num9",
	0x64: "num4", 0x65: "num5", 0x66: "num6",
	0x61: "num1", 0x62: "num2", 0x63: "num3",
	0x60: "num0",
	0x6D: "num-", 0x6B: "num+", 0x6A: "num*",
	0x6F: "num/", 0x0E: "numenter", // Note: numpad enter sends VK_RETURN (0x0D) normally; this maps the extended key
	// Symbols (OEM keys — US layout VK codes)
	0xBD: "-",  // VK_OEM_MINUS
	0xBB: "=",  // VK_OEM_PLUS (the = key)
	0xDB: "[",  // VK_OEM_4
	0xDD: "]",  // VK_OEM_6
	0xDC: "\\", // VK_OEM_5
	0xBA: ";",  // VK_OEM_1
	0xDE: "'",  // VK_OEM_7
	0xBC: ",",  // VK_OEM_COMMA
	0xBE: ".",  // VK_OEM_PERIOD
	0xBF: "/",  // VK_OEM_2
	0xC0: "`",  // VK_OEM_3
}

// nameToVK is the reverse map, built at init.
var nameToVK map[string]uint16

func init() {
	nameToVK = make(map[string]uint16, len(vkToName)+10)
	for vk, name := range vkToName {
		nameToVK[name] = vk
	}
	// Aliases for compatibility
	nameToVK["escape"] = 0x1B
	nameToVK["return"] = 0x0D
	nameToVK["del"] = 0x2E
	nameToVK["control"] = 0xA2
	nameToVK["cmd"] = 0x5B
	nameToVK["command"] = 0x5B
	nameToVK["meta"] = 0x5B
	nameToVK["win"] = 0x5B
	nameToVK["option"] = 0xA4
}

// parseHotkeyStr parses "ctrl+shift+a" into sorted VK codes.
func parseHotkeyStr(s string) ([]uint16, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return nil, fmt.Errorf("empty hotkey string")
	}

	parts := strings.Split(s, "+")
	keys := make([]uint16, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		kc, ok := nameToVK[p]
		if !ok {
			return nil, fmt.Errorf("unknown key: %q", p)
		}
		keys = append(keys, kc)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys, nil
}

// modifierOrder defines display order: ctrl, shift, alt, super.
var modifierOrder = map[uint16]int{
	0xA2: 0, 0xA3: 1, // ctrl, rctrl
	0xA0: 2, 0xA1: 3, // shift, rshift
	0xA4: 4, 0xA5: 5, // alt, ralt
	0x5B: 6, 0x5C: 7, // super, rsuper
}

// keysToString converts VK codes to a display string like "ctrl+shift+a".
// Modifiers are placed first (in ctrl/shift/alt/super order); then regular keys, sorted.
func keysToString(keys []uint16) string {
	sort.Slice(keys, func(i, j int) bool {
		mi, mj := isModifier(keys[i]), isModifier(keys[j])
		if mi != mj {
			return mi // modifiers first
		}
		if mi && mj {
			return modifierOrder[keys[i]] < modifierOrder[keys[j]]
		}
		return keys[i] < keys[j]
	})

	parts := make([]string, 0, len(keys))
	for _, kc := range keys {
		if name, ok := vkToName[kc]; ok {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, "+")
}
