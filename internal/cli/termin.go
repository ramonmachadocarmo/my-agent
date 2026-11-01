package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// termInput reads lines from a real terminal with the line editor. Raw mode
// is only on while a line is being read, so commands and replies run with the
// normal terminal (Ctrl+C then works as a signal).
type termInput struct {
	raw  func() (restore func(), err error) // switches the terminal to raw mode
	out  io.Writer
	kr   *keyReader
	st   style
	hist []string
}

// newTermInput returns nil when the input is not a terminal that supports
// raw mode (a pipe, mintty...): the chat then falls back to plain line reads.
func newTermInput(in io.Reader, out io.Writer, rd *bufio.Reader, st style) *termInput {
	f, ok := in.(*os.File)
	if !ok {
		return nil
	}
	raw := func() (func(), error) { return makeRaw(f) }
	restore, err := raw() // probe
	if err != nil {
		return nil
	}
	restore()
	return &termInput{raw: raw, out: out, kr: &keyReader{rd: rd}, st: st}
}

func (t *termInput) width() int {
	if f, ok := t.out.(*os.File); ok {
		if w := termWidth(f); w > 0 {
			return w
		}
	}
	return 80
}

// readLine edits one line. It returns io.EOF when the user leaves with
// Ctrl+D, or Ctrl+C on an empty line.
func (t *termInput) readLine(prompt string, promptW int, suggest suggestFunc, secret bool) (string, error) {
	restore, err := t.raw()
	if err != nil {
		return "", err
	}
	defer restore()

	hist := t.hist
	if secret {
		hist = nil
	}
	ed := newEditor(t.st, prompt, promptW, suggest, secret, t.width(), hist)
	fmt.Fprint(t.out, ed.render())
	for {
		keys, err := t.kr.next()
		if err != nil {
			fmt.Fprint(t.out, ed.finish())
			return "", err
		}
		for i, k := range keys {
			switch ed.handle(k) {
			case edSubmit:
				t.kr.unread(keys[i+1:]) // typed-ahead keys belong to the next line
				// Draw the final text (without the suggestion list) before moving
				// on: when the whole line arrives in one read (a paste, fast
				// typing) nothing was drawn for it yet.
				ed.sugg = nil
				fmt.Fprint(t.out, ed.render(), ed.finish())
				line := ed.text()
				if !secret && strings.TrimSpace(line) != "" && (len(t.hist) == 0 || t.hist[len(t.hist)-1] != line) {
					t.hist = append(t.hist, line)
					if len(t.hist) > 200 {
						t.hist = t.hist[1:]
					}
				}
				return line, nil
			case edEOF:
				fmt.Fprint(t.out, ed.finish())
				return "", io.EOF
			}
		}
		fmt.Fprint(t.out, ed.render())
	}
}
