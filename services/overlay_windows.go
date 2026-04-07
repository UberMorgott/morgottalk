//go:build windows

package services

// procGetForegroundWindow and procSetForegroundWindow use `user32` from paste_windows.go.
var (
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

func saveForegroundWindow() uintptr {
	hwnd, _, _ := procGetForegroundWindow.Call()
	return hwnd
}

func restoreForegroundWindow(hwnd uintptr) {
	if hwnd != 0 {
		procSetForegroundWindow.Call(hwnd)
	}
}
