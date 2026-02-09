package services

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// pasteText inserts text into the currently focused input.
//
// Strategy (clipboard-based, like Espanso/Hyprvoice):
//  1. Save current clipboard contents
//  2. Write transcribed text to clipboard
//  3. Simulate Shift+Insert (universal paste — works in terminals, TUI, and GUI apps)
//  4. Restore original clipboard after a short delay
//
// Shift+Insert is the most universal paste shortcut on Linux:
//   - All terminal emulators (Konsole, Alacritty, Kitty, foot, etc.)
//   - All GUI apps (Firefox, Chrome, Kate, LibreOffice, etc.)
//   - TUI apps inside terminals (Claude Code, vim, etc.)
//   - On macOS/Windows: Cmd+V / Ctrl+V are universal, so no issue there.
func pasteText(text string) error {
	if text == "" {
		return nil
	}

	switch runtime.GOOS {
	case "linux":
		return pasteTextLinux(text)
	case "darwin":
		return pasteTextDarwin(text)
	case "windows":
		return pasteTextWindows(text)
	}
	return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
}

func pasteTextLinux(text string) error {
	// 1. Save current clipboard
	saved, hadClipboard := saveClipboardLinux()

	// 2. Write text to clipboard via wl-copy (Wayland) or xclip (X11)
	if err := writeClipboardLinux(text); err != nil {
		return fmt.Errorf("failed to write to clipboard: %w", err)
	}

	// Small delay to ensure clipboard is ready
	time.Sleep(30 * time.Millisecond)

	// 3. Simulate Shift+Insert (universal paste shortcut)
	if err := simulateShiftInsertLinux(); err != nil {
		return fmt.Errorf("failed to simulate paste: %w", err)
	}

	log.Printf("Text pasted via clipboard (%d chars)", len(text))

	// 4. Restore original clipboard after delay (in background)
	if hadClipboard {
		go func() {
			time.Sleep(500 * time.Millisecond)
			_ = writeClipboardLinux(saved)
		}()
	}

	return nil
}

func pasteTextDarwin(text string) error {
	// Save clipboard
	saved, hadClipboard := saveClipboardDarwin()

	// Write to clipboard via pbcopy
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pbcopy failed: %w", err)
	}

	time.Sleep(30 * time.Millisecond)

	// Simulate Cmd+V
	if err := exec.Command("osascript", "-e",
		`tell application "System Events" to keystroke "v" using command down`).Run(); err != nil {
		return fmt.Errorf("Cmd+V simulation failed: %w", err)
	}

	log.Printf("Text pasted via clipboard (%d chars)", len(text))

	if hadClipboard {
		go func() {
			time.Sleep(500 * time.Millisecond)
			cmd := exec.Command("pbcopy")
			cmd.Stdin = strings.NewReader(saved)
			_ = cmd.Run()
		}()
	}

	return nil
}

func pasteTextWindows(text string) error {
	// Save clipboard
	saved, hadClipboard := winClipRead()

	// Write to clipboard via Win32 API (instant, no process spawning)
	if err := winClipWrite(text); err != nil {
		return fmt.Errorf("clipboard write failed: %w", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Simulate Ctrl+V via SendInput (kernel-level, goes to focused window)
	if err := winSendCtrlV(); err != nil {
		// SendInput blocked (UIPI: target window is elevated).
		// Text is already in clipboard — user can Ctrl+V manually.
		log.Printf("SendInput blocked (target may be elevated), text left in clipboard: %v", err)
		return nil
	}

	log.Printf("Text pasted via clipboard (%d chars)", len(text))

	// Restore original clipboard after delay
	if hadClipboard {
		go func() {
			time.Sleep(500 * time.Millisecond)
			_ = winClipWrite(saved)
		}()
	}

	return nil
}

// --- Clipboard save/restore helpers ---

func saveClipboardLinux() (string, bool) {
	// Try wl-paste (Wayland)
	if out, err := exec.Command("wl-paste", "--no-newline").Output(); err == nil {
		return string(out), true
	}
	// Try xclip (X11)
	if out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output(); err == nil {
		return string(out), true
	}
	return "", false
}

func writeClipboardLinux(text string) error {
	// Try wl-copy (Wayland)
	if path, err := exec.LookPath("wl-copy"); err == nil {
		cmd := exec.Command(path)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	// Try xclip (X11)
	if path, err := exec.LookPath("xclip"); err == nil {
		cmd := exec.Command(path, "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no clipboard tool found (install wl-clipboard or xclip)")
}

func simulateShiftInsertLinux() error {
	// 1. ydotool — kernel-level uinput, works everywhere (Wayland, X11, TUI, terminals)
	//    Shift=42, Insert=110 (scancodes)
	if path, err := exec.LookPath("ydotool"); err == nil {
		if err := exec.Command(path, "key", "42:1", "110:1", "110:0", "42:0").Run(); err == nil {
			return nil
		}
	}
	// 2. wtype — Wayland virtual keyboard (works in GUI apps, may have issues in TUI)
	if path, err := exec.LookPath("wtype"); err == nil {
		if err := exec.Command(path, "-M", "shift", "-k", "Insert").Run(); err == nil {
			return nil
		}
	}
	// 3. xdotool — X11 fallback
	if path, err := exec.LookPath("xdotool"); err == nil {
		return exec.Command(path, "key", "--clearmodifiers", "shift+Insert").Run()
	}
	return fmt.Errorf("no key simulation tool found (install ydotool, wtype, or xdotool)")
}

func saveClipboardDarwin() (string, bool) {
	if out, err := exec.Command("pbpaste").Output(); err == nil {
		return string(out), true
	}
	return "", false
}

