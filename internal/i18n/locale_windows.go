//go:build windows

package i18n

import (
	"syscall"
	"unsafe"
)

var procGetUserDefaultLocaleName = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")

// osLocale returns the user's Windows display locale, e.g. "pt-BR".
func osLocale() string {
	buf := make([]uint16, 85) // LOCALE_NAME_MAX_LENGTH
	n, _, _ := procGetUserDefaultLocaleName.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
