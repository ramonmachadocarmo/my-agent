package cli

import (
	"io"
	"unicode/utf8"
)

type keyKind int

const (
	kRune keyKind = iota
	kEnter
	kBackspace
	kDelete
	kTab
	kUp
	kDown
	kLeft
	kRight
	kHome
	kEnd
	kCtrlC
	kCtrlD
	kCtrlU
	kCtrlW
	kCtrlK
	kCtrlL
	kEsc
)

type key struct {
	kind keyKind
	r    rune // for kRune
}

// parseKeys decodes as many keys as possible from b. rest is an incomplete
// trailing sequence (part of an escape sequence or of a UTF-8 character) to
// be completed by the next read.
//
// A lone ESC at the very end of the input is the Esc key. Terminals deliver
// an arrow key's "ESC [ A" in one piece, so this only misreads a sequence that
// a slow link splits exactly after the ESC byte; telling the two apart would
// need a timer and a concurrent reader, which would fight with the menus that
// also read from the same input.
func parseKeys(b []byte) (keys []key, rest []byte) {
	for len(b) > 0 {
		c := b[0]
		switch {
		case c == 0x1b:
			if len(b) == 1 {
				keys = append(keys, key{kind: kEsc})
				b = b[1:]
				continue
			}
			switch b[1] {
			case '[': // CSI: ESC [ params final
				i := 2
				for i < len(b) && b[i] >= 0x20 && b[i] <= 0x3f {
					i++
				}
				if i >= len(b) {
					return keys, b
				}
				keys = appendCSI(keys, string(b[2:i]), b[i])
				b = b[i+1:]
			case 'O': // SS3: ESC O final (some terminals send arrows this way)
				if len(b) < 3 {
					return keys, b
				}
				keys = appendCSI(keys, "", b[2])
				b = b[3:]
			default: // Alt+key and the like: ignored
				b = b[2:]
			}
		case c == '\r':
			keys = append(keys, key{kind: kEnter})
			b = b[1:]
			if len(b) > 0 && b[0] == '\n' {
				b = b[1:]
			}
		case c == '\n':
			keys = append(keys, key{kind: kEnter})
			b = b[1:]
		case c == 0x7f || c == 0x08:
			keys, b = append(keys, key{kind: kBackspace}), b[1:]
		case c == 0x09:
			keys, b = append(keys, key{kind: kTab}), b[1:]
		case c == 0x01:
			keys, b = append(keys, key{kind: kHome}), b[1:]
		case c == 0x05:
			keys, b = append(keys, key{kind: kEnd}), b[1:]
		case c == 0x03:
			keys, b = append(keys, key{kind: kCtrlC}), b[1:]
		case c == 0x04:
			keys, b = append(keys, key{kind: kCtrlD}), b[1:]
		case c == 0x0b:
			keys, b = append(keys, key{kind: kCtrlK}), b[1:]
		case c == 0x0c:
			keys, b = append(keys, key{kind: kCtrlL}), b[1:]
		case c == 0x15:
			keys, b = append(keys, key{kind: kCtrlU}), b[1:]
		case c == 0x17:
			keys, b = append(keys, key{kind: kCtrlW}), b[1:]
		case c < 0x20:
			b = b[1:] // other control characters
		default:
			r, size := utf8.DecodeRune(b)
			if r == utf8.RuneError && size <= 1 && !utf8.FullRune(b) {
				return keys, b // half of a multi-byte character
			}
			keys = append(keys, key{kind: kRune, r: r})
			b = b[size:]
		}
	}
	return keys, nil
}

func appendCSI(keys []key, params string, final byte) []key {
	switch final {
	case 'A':
		return append(keys, key{kind: kUp})
	case 'B':
		return append(keys, key{kind: kDown})
	case 'C':
		return append(keys, key{kind: kRight})
	case 'D':
		return append(keys, key{kind: kLeft})
	case 'H':
		return append(keys, key{kind: kHome})
	case 'F':
		return append(keys, key{kind: kEnd})
	case '~':
		switch params {
		case "1", "7":
			return append(keys, key{kind: kHome})
		case "4", "8":
			return append(keys, key{kind: kEnd})
		case "3":
			return append(keys, key{kind: kDelete})
		}
	}
	return keys
}

// keyReader turns a byte stream from the terminal into keys.
type keyReader struct {
	rd      io.Reader
	pending []byte
	queue   []key
	buf     [256]byte
}

// next blocks until at least one key is available.
func (kr *keyReader) next() ([]key, error) {
	for {
		if len(kr.queue) > 0 {
			keys := kr.queue
			kr.queue = nil
			return keys, nil
		}
		if len(kr.pending) > 0 {
			keys, rest := parseKeys(kr.pending)
			kr.pending = append(kr.pending[:0], rest...)
			if len(keys) > 0 {
				return keys, nil
			}
		}
		n, err := kr.rd.Read(kr.buf[:])
		if n > 0 {
			kr.pending = append(kr.pending, kr.buf[:n]...)
		}
		if err != nil && n == 0 {
			return nil, err
		}
	}
}

// unread puts keys back, to be returned by the next call to next.
func (kr *keyReader) unread(keys []key) {
	kr.queue = append(append([]key{}, keys...), kr.queue...)
}
