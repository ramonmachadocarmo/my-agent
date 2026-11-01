package cli

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestStyleOffIsPassThrough(t *testing.T) {
	st := style{}
	if got := st.bold(st.cyan("x")); got != "x" {
		t.Fatalf("got %q", got)
	}
	md := "# T\n- a **b** `c`\n```go\nx\n```"
	if got := st.renderMarkdown(md); got != md {
		t.Fatalf("markdown must be untouched when styling is off: %q", got)
	}
	if newStyle(&strings.Builder{}).on {
		t.Fatal("non-terminal writers must not be styled")
	}
}

func TestRenderMarkdown(t *testing.T) {
	st := style{on: true}
	in := "# Title\nSome **bold** and `code` text\n- item one\n  * nested\n```go\nfmt.Println(1)\n```\nafter"
	out := st.renderMarkdown(in)
	plain := ansi.ReplaceAllString(out, "")
	for _, want := range []string{"Title", "Some bold and code text", "• item one", "• nested", "┌─ go", "│ fmt.Println(1)", "└─", "after"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}
	for _, leaked := range []string{"**", "`", "# "} {
		if strings.Contains(plain, leaked) {
			t.Errorf("markup %q leaked in:\n%s", leaked, plain)
		}
	}
	if !strings.Contains(out, "\x1b[1mbold") {
		t.Errorf("bold not styled: %q", out)
	}
	// text inside a code fence must not be interpreted
	code := ansi.ReplaceAllString(st.renderMarkdown("```\n**not bold** `x`\n```"), "")
	if !strings.Contains(code, "**not bold** `x`") {
		t.Errorf("code block content was altered: %q", code)
	}
}

func TestBoxLinesHaveEqualWidth(t *testing.T) {
	st := style{on: true}
	box := st.box([][2]string{{"✦ agent", st.bold("✦ agent")}, {"a much longer second line", "a much longer second line"}})
	lines := strings.Split(box, "\n")
	if len(lines) != 4 {
		t.Fatalf("want 4 lines, got %d", len(lines))
	}
	w := utf8.RuneCountInString(ansi.ReplaceAllString(lines[0], ""))
	for _, l := range lines {
		if got := utf8.RuneCountInString(ansi.ReplaceAllString(l, "")); got != w {
			t.Errorf("uneven box line (%d != %d): %q", got, w, l)
		}
	}
}

func TestSpinnerNoopWhenPlain(t *testing.T) {
	var out strings.Builder
	e := &env{out: &out}
	stop := e.spin(style{}, "x")
	stop()
	if out.Len() != 0 {
		t.Fatalf("plain spinner wrote %q", out.String())
	}
}

func TestSpinnerStops(t *testing.T) {
	var out strings.Builder
	e := &env{out: &out}
	stop := e.spin(style{on: true}, "thinking")
	stop() // must return, and clear the line
	if !strings.HasSuffix(out.String(), "\r\x1b[2K") {
		t.Fatalf("spinner did not clear its line: %q", out.String())
	}
}
