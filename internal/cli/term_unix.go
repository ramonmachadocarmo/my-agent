//go:build linux || darwin

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

// makeRaw switches the terminal to character-at-a-time mode without echo and
// returns a function that restores the previous mode.
func makeRaw(f *os.File) (restore func(), err error) {
	fd := f.Fd()
	var old syscall.Termios
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, ioctlGetTermios, uintptr(unsafe.Pointer(&old))); e != 0 {
		return nil, e
	}
	raw := old
	raw.Iflag &^= syscall.ICRNL | syscall.INLCR | syscall.IXON
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, ioctlSetTermios, uintptr(unsafe.Pointer(&raw))); e != 0 {
		return nil, e
	}
	return func() {
		syscall.Syscall(syscall.SYS_IOCTL, fd, ioctlSetTermios, uintptr(unsafe.Pointer(&old)))
	}, nil
}

// termWidth returns the terminal width in columns, or 0 when unknown.
func termWidth(f *os.File) int {
	var ws struct{ row, col, x, y uint16 }
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws))); e != 0 {
		return 0
	}
	return int(ws.col)
}
