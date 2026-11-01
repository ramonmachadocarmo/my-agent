//go:build !windows

package cli

import "os"

// enableVT is a no-op: Unix terminals understand ANSI sequences already.
func enableVT(*os.File) bool { return true }
