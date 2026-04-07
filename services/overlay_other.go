//go:build !windows

package services

func saveForegroundWindow() uintptr { return 0 }
func restoreForegroundWindow(hwnd uintptr) {}
