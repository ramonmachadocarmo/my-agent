//go:build windows

package cli

import (
	"errors"
	"os"
	"unsafe"
)

var procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")

const (
	enableProcessedInput       = 0x0001
	enableLineInput            = 0x0002
	enableEchoInput            = 0x0004
	enableVirtualTerminalInput = 0x0200
)

// makeRaw switches the console input to character-at-a-time mode with escape
// sequences for the arrow keys, and returns a function that restores it.
func makeRaw(f *os.File) (restore func(), err error) {
	h := f.Fd()
	var mode uint32
	if r, _, _ := procGetConsoleMode.Call(h, uintptr(unsafe.Pointer(&mode))); r == 0 {
		return nil, errors.New("input is not a console") // i18n:ok internal, the chat falls back to plain input
	}
	raw := mode&^(enableProcessedInput|enableLineInput|enableEchoInput) | enableVirtualTerminalInput
	if r, _, _ := procSetConsoleMode.Call(h, uintptr(raw)); r == 0 {
		return nil, errors.New("cannot switch the console to raw input") // i18n:ok internal, the chat falls back to plain input
	}
	return func() { procSetConsoleMode.Call(h, uintptr(mode)) }, nil
}

// termWidth returns the console width in columns, or 0 when unknown.
func termWidth(f *os.File) int {
	var info struct { // CONSOLE_SCREEN_BUFFER_INFO
		size, cursor [2]int16
		attributes   uint16
		window       [4]int16 // left, top, right, bottom
		maxSize      [2]int16
	}
	if r, _, _ := procGetConsoleScreenBufferInfo.Call(f.Fd(), uintptr(unsafe.Pointer(&info))); r == 0 {
		return 0
	}
	return int(info.window[2]-info.window[0]) + 1
}
