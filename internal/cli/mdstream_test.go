package cli

import (
	"strings"
	"testing"
)

const sample = "# Title\n" +
	"Some **bold** and `code` text, 2 * 3 = 6.\n" +
	"- item **one**\n" +
	"  * nested item\n" +
	"```go\n" +
	"if a < b { fmt.Println(\"**not bold**\") }\n" +
	"```\n" +
	"## Sub heading\n" +
	"-not a bullet, #not heading\n" +
	"last line without newline"

func renderChunks(text string, size int) string {
	m := newMDStream(style{on: true}, "")
	var sb strings.Builder
	for i := 0; i < len(text); {
		end := min(i+size, len(text))
		for end < len(text) && text[end]&0xC0 == 0x80 { // do not split a UTF-8 rune
			end++
		}
		sb.WriteString(m.Write(text[i:end]))
		i = end
	}
	sb.WriteString(m.Flush())
	return sb.String()
}

func TestMDStreamIsChunkInvariant(t *testing.T) {
	want := renderChunks(sample, len(sample))
	for _, size := range []int{1, 2, 3, 5, 7, 16} {
		if got := renderChunks(sample, size); got != want {
			t.Errorf("chunk size %d differs:\nwant %q\n got %q", size, want, got)
		}
	}
}

func TestMDStreamRendering(t *testing.T) {
	plain := ansi.ReplaceAllString(renderChunks(sample, 1), "")
	for _, want := range []string{
		"Title\n",
		"Some bold and code text, 2 * 3 = 6.",
		"• item one",
		"  • nested item",
		"┌─ go",
		`│ if a < b { fmt.Println("**not bold**") }`,
		"└─",
		"Sub heading",
		"-not a bullet, #not heading",
		"last line without newline",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}
	for _, leaked := range []string{"# Title", "## Sub", "```", "`code`"} {
		if strings.Contains(plain, leaked) {
			t.Errorf("markup %q leaked in:\n%s", leaked, plain)
		}
	}
}

func TestMDStreamStylesAndResets(t *testing.T) {
	out := renderChunks("plain **bold** `code`\nnext", 3)
	if !strings.Contains(out, escBold+"bold"+escUnbold) || !strings.Contains(out, escCode+"code"+escReset) {
		t.Errorf("styles missing: %q", out)
	}
	if !strings.HasSuffix(out, escReset) {
		t.Errorf("stream must end with a reset: %q", out)
	}
	// unterminated bold must not leak past the end of the line
	out = renderChunks("**never closed\nnormal", 2)
	if idx := strings.Index(out, "\nnormal"); idx < 0 || !strings.Contains(out[:idx], escReset) {
		t.Errorf("unterminated bold leaked across lines: %q", out)
	}
}

func TestMDStreamIndentAndPlain(t *testing.T) {
	m := newMDStream(style{on: true}, "   ")
	got := ansi.ReplaceAllString(m.Write("a\nb\n")+m.Flush(), "")
	if got != "a\n   b\n   " {
		t.Errorf("indent: %q", got)
	}
	p := newMDStream(style{}, "   ")
	if got := p.Write("**x**\n") + p.Flush(); got != "**x**\n" {
		t.Errorf("plain mode must pass text through untouched: %q", got)
	}
}
