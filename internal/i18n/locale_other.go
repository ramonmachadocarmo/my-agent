//go:build !windows

package i18n

// osLocale has nothing beyond the environment variables on Unix systems.
func osLocale() string { return "" }
