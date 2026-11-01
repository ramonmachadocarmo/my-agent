//go:build windows

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode     = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode     = kernel32.NewProc("SetConsoleMode")
	procSetConsoleOutputCP = kernel32.NewProc("SetConsoleOutputCP")
)

const enableVirtualTerminalProcessing = 0x0004

// enableVT turns on ANSI escape processing and UTF-8 output for the console
// (needed by Windows PowerShell 5.1 / conhost). It reports whether ANSI
// sequences will be understood.
func enableVT(f *os.File) bool {
	h := f.Fd()
	var mode uint32
	if r, _, _ := procGetConsoleMode.Call(h, uintptr(unsafe.Pointer(&mode))); r == 0 {
		return false
	}
	if mode&enableVirtualTerminalProcessing == 0 {
		if r, _, _ := procSetConsoleMode.Call(h, uintptr(mode|enableVirtualTerminalProcessing)); r == 0 {
			return false
		}
	}
	procSetConsoleOutputCP.Call(65001)
	return true
}
