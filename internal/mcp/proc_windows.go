package mcp

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps console servers from opening a window of their own.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
