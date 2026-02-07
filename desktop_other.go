//go:build !linux

package main

// installDesktopEntry is a no-op on macOS and Windows.
// macOS uses the .app bundle icon (Info.plist + icons.icns).
// Windows uses the embedded .ico resource (.syso).
func installDesktopEntry(_ []byte) {}
