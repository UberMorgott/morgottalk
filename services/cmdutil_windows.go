//go:build windows

package services

import (
	"os/exec"
	"syscall"
)

// hideWindow prevents a console window from flashing when running external commands.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
