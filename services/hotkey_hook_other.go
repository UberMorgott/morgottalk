//go:build !windows

package services

import "fmt"

// startHook is a stub for non-Windows platforms.
// TODO: implement using evdev (Linux) or IOKit (macOS) if needed.
func startHook(onKey func(vk uint16, down bool), onInstalled func(error)) error {
	return fmt.Errorf("keyboard hook not implemented on this platform")
}

func stopHook() {}
