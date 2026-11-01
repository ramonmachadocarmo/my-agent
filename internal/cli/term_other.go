//go:build !windows && !linux && !darwin

package cli

import (
	"errors"
	"os"
)

// Raw terminal mode is not implemented here: the chat uses plain line reads.
func makeRaw(*os.File) (func(), error) {
	return nil, errors.New("raw mode unsupported") // i18n:ok internal, never shown
}

func termWidth(*os.File) int { return 0 }
