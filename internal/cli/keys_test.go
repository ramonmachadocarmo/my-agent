package cli

import (
	"bytes"
	"io"
	"testing"
)

func kinds(keys []key) []keyKind {
	out := make([]keyKind, len(keys))
	for i, k := range keys {
		out[i] = k.kind
	}
	return out
}

func sameKinds(a, b []keyKind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestParseKeys(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []keyKind
	}{
		{"arrows (CSI)", "\x1b[A\x1b[B\x1b[C\x1b[D", []keyKind{kUp, kDown, kRight, kLeft}},
		{"arrows (SS3)", "\x1bOA\x1bOD", []keyKind{kUp, kLeft}},
		{"home/end", "\x1b[H\x1b[F\x1b[1~\x1b[4~\x01\x05", []keyKind{kHome, kEnd, kHome, kEnd, kHome, kEnd}},
		{"delete", "\x1b[3~", []keyKind{kDelete}},
		{"modifiers are ignored", "\x1b[1;5C", []keyKind{kRight}},
		{"enter variants", "\r\n\r\n", []keyKind{kEnter, kEnter}}, // CRLF counts once
		{"backspace variants", "\x7f\x08", []keyKind{kBackspace, kBackspace}},
		{"tab and controls", "\t\x03\x04\x15\x17\x0b\x0c", []keyKind{kTab, kCtrlC, kCtrlD, kCtrlU, kCtrlW, kCtrlK, kCtrlL}},
		{"lone escape", "\x1b", []keyKind{kEsc}},
		{"alt+key is dropped", "\x1bx", nil},
		{"unknown csi is dropped", "\x1b[99z", nil},
		{"other control bytes are dropped", "\x00\x02", nil},
	}
	for _, tc := range cases {
		keys, rest := parseKeys([]byte(tc.in))
		if len(rest) != 0 || !sameKinds(kinds(keys), tc.want) {
			t.Errorf("%s: got %v rest=%q, want %v", tc.name, kinds(keys), rest, tc.want)
		}
	}
}

func TestParseKeysText(t *testing.T) {
	keys, rest := parseKeys([]byte("aé漢🙂"))
	var got []rune
	for _, k := range keys {
		if k.kind != kRune {
			t.Fatalf("unexpected key %v", k)
		}
		got = append(got, k.r)
	}
	if string(got) != "aé漢🙂" || len(rest) != 0 {
		t.Fatalf("got %q rest=%q", string(got), rest)
	}
}

func TestParseKeysKeepsIncompleteSequences(t *testing.T) {
	// half of "é" (0xC3 0xA9) and half of an arrow key are kept for the next read
	keys, rest := parseKeys([]byte("a\xc3"))
	if len(keys) != 1 || string(rest) != "\xc3" {
		t.Fatalf("utf-8: keys=%v rest=%q", keys, rest)
	}
	keys, rest = parseKeys(append(rest, 0xa9))
	if len(keys) != 1 || keys[0].r != 'é' || len(rest) != 0 {
		t.Fatalf("utf-8 completed: keys=%v rest=%q", keys, rest)
	}
	keys, rest = parseKeys([]byte("x\x1b["))
	if len(keys) != 1 || string(rest) != "\x1b[" {
		t.Fatalf("csi: keys=%v rest=%q", keys, rest)
	}
}

// chunks returns a reader that hands out one prepared chunk per Read.
func chunks(parts ...string) *chunkReader { return &chunkReader{parts: parts} }

type chunkReader struct{ parts []string }

func (c *chunkReader) Read(p []byte) (int, error) {
	if len(c.parts) == 0 {
		return 0, io.EOF
	}
	n := copy(p, c.parts[0])
	c.parts[0] = c.parts[0][n:]
	if c.parts[0] == "" {
		c.parts = c.parts[1:]
	}
	return n, nil
}

func TestKeyReaderAcrossSplitReads(t *testing.T) {
	// a multi-byte character split between two reads must still assemble;
	// escape sequences arrive whole
	kr := &keyReader{rd: chunks("\xc3", "\xa9", "\x1b[A\r")}
	var got []key
	for len(got) < 3 {
		keys, err := kr.next()
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, keys...)
	}
	if got[0].kind != kRune || got[0].r != 'é' || got[1].kind != kUp || got[2].kind != kEnter {
		t.Fatalf("got %+v", got)
	}
	if _, err := kr.next(); err == nil {
		t.Fatal("expected EOF at the end of the stream")
	}
}

func TestKeyReaderUnread(t *testing.T) {
	kr := &keyReader{rd: bytes.NewReader(nil)}
	kr.unread([]key{{kind: kTab}})
	kr.unread([]key{{kind: kUp}}) // goes in front
	keys, err := kr.next()
	if err != nil || !sameKinds(kinds(keys), []keyKind{kUp, kTab}) {
		t.Fatalf("%v %v", kinds(keys), err)
	}
}
