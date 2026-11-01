package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// style adds ANSI colors when writing to a real terminal. When output is
// piped, redirected, or NO_COLOR is set, every method returns text unchanged
// and the chat falls back to plain output.
type style struct{ on bool }

func newStyle(out io.Writer) style {
	f, ok := out.(*os.File)
	if !ok || os.Getenv("NO_COLOR") != "" {
		return style{}
	}
	if fi, err := f.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return style{}
	}
	return style{on: enableVT(f)}
}

func (s style) wrap(code, t string) string {
	if !s.on || t == "" {
		return t
	}
	return "\x1b[" + code + "m" + t + "\x1b[0m"
}

func (s style) bold(t string) string    { return s.wrap("1", t) }
func (s style) dim(t string) string     { return s.wrap("2", t) }
func (s style) cyan(t string) string    { return s.wrap("36", t) }
func (s style) green(t string) string   { return s.wrap("32", t) }
func (s style) yellow(t string) string  { return s.wrap("33", t) }
func (s style) magenta(t string) string { return s.wrap("35", t) }
func (s style) red(t string) string     { return s.wrap("31", t) }

// box draws a rounded box around lines. Each line has a plain form (used to
// measure width) and a styled form (what is printed).
func (s style) box(lines [][2]string) string {
	width := 0
	for _, l := range lines {
		width = max(width, utf8.RuneCountInString(l[0]))
	}
	var sb strings.Builder
	sb.WriteString(s.cyan("╭"+strings.Repeat("─", width+2)+"╮") + "\n")
	for _, l := range lines {
		pad := strings.Repeat(" ", width-utf8.RuneCountInString(l[0]))
		sb.WriteString(s.cyan("│") + " " + l[1] + pad + " " + s.cyan("│") + "\n")
	}
	sb.WriteString(s.cyan("╰" + strings.Repeat("─", width+2) + "╯"))
	return sb.String()
}

// renderMarkdown renders a complete text with the streaming renderer.
func (s style) renderMarkdown(text string) string {
	m := newMDStream(s, "")
	return m.Write(strings.TrimRight(text, "\n")) + m.Flush()
}

// spin shows an animated "working" indicator until the returned stop
// function is called. It does nothing when styling is off.
func (e *env) spin(st style, label string) (stop func()) {
	if !st.on {
		return func() {}
	}
	quit, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		tick := time.NewTicker(80 * time.Millisecond)
		defer tick.Stop()
		for i := 0; ; i++ {
			fmt.Fprintf(e.out, "\r %s %s", st.cyan(frames[i%len(frames)]), st.dim(label))
			select {
			case <-quit:
				fmt.Fprint(e.out, "\r\x1b[2K")
				return
			case <-tick.C:
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(quit); <-done }) }
}
