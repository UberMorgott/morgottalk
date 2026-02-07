//go:build linux

package main

import (
	"log"
	"os"
	"path/filepath"
)

// installDesktopEntry writes .desktop file and icon so that
// KDE/GNOME Wayland shows the correct taskbar icon.
// Runs on every launch to keep Exec path current.
func installDesktopEntry(icon []byte) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)

	// Write icon to app data dir (absolute path — no icon cache needed)
	dataDir := filepath.Join(home, ".local", "share", "morgottalk")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return
	}
	iconPath := filepath.Join(dataDir, "icon.png")
	if err := os.WriteFile(iconPath, icon, 0644); err != nil {
		return
	}

	// Write .desktop file with absolute icon path
	appDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return
	}

	desktop := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=MorgoTTalk\n" +
		"Exec=" + exe + "\n" +
		"Icon=" + iconPath + "\n" +
		"Categories=AudioVideo;Audio;Utility;\n" +
		"Terminal=false\n" +
		"StartupWMClass=morgottalk\n" +
		"Comment=Push-to-talk voice transcription\n"

	desktopPath := filepath.Join(appDir, "morgottalk.desktop")
	if err := os.WriteFile(desktopPath, []byte(desktop), 0644); err != nil {
		log.Printf("Failed to install .desktop file: %v", err)
	}
}
