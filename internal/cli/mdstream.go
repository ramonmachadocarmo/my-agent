package cli

import "strings"

// mdStream renders a small markdown subset (headings, bullets, **bold**,
// `inline code`, fenced code blocks) incrementally, so text can be styled
// while it is still arriving. Chunks may split anywhere, even inside "**".
//
// When styling is off it is a pass-through.
type mdStream struct {
	st     style
	indent string // printed after every newline

	inFence   bool
	fenceLine bool   // consuming a ``` line up to its newline
	fenceInfo string // text after the opening ```
	bold      bool
	code      bool
	heading   bool
	star      bool   // a single '*' waiting to learn if it is "**"
	lineStart bool   // current line not classified yet
	head      string // unclassified characters at the start of a line
}

func newMDStream(st style, indent string) *mdStream {
	return &mdStream{st: st, indent: indent, lineStart: true}
}

const (
	escBold    = "\x1b[1m"
	escUnbold  = "\x1b[22m"
	escHeading = "\x1b[1;35m"
	escCode    = "\x1b[33m"
	escReset   = "\x1b[0m"
)

// Write consumes a chunk and returns the text to print for it.
func (m *mdStream) Write(s string) string {
	if !m.st.on {
		return s
	}
	var out strings.Builder
	for _, r := range s {
		switch {
		case m.fenceLine:
			if r == '\n' {
				m.endFenceLine(&out)
			} else {
				m.fenceInfo += string(r)
			}
		case m.lineStart:
			m.atLineStart(&out, r)
		default:
			m.inline(&out, r)
		}
	}
	return out.String()
}

// Flush ends the stream, closing any open styling.
func (m *mdStream) Flush() string {
	if !m.st.on {
		return ""
	}
	var out strings.Builder
	m.fenceLine = false
	if m.lineStart {
		m.flushHead(&out)
	}
	m.flushStar(&out)
	out.WriteString(escReset)
	m.bold, m.code, m.heading, m.inFence = false, false, false, false
	return out.String()
}

func (m *mdStream) newline(out *strings.Builder) {
	m.flushStar(out)
	out.WriteString(escReset + "\n" + m.indent)
	m.bold, m.code, m.heading = false, false, false
	m.lineStart, m.head = true, ""
}

func (m *mdStream) flushStar(out *strings.Builder) {
	if m.star {
		out.WriteByte('*')
		m.star = false
	}
}

func (m *mdStream) endFenceLine(out *strings.Builder) {
	if m.inFence {
		out.WriteString(m.st.dim("  └─"))
	} else {
		out.WriteString(m.st.dim("  ┌─ " + strings.TrimSpace(m.fenceInfo)))
	}
	m.inFence = !m.inFence
	m.fenceLine, m.fenceInfo = false, ""
	m.newline(out)
}

// beginFenceContent starts a code line inside a fenced block.
func (m *mdStream) beginFenceContent(out *strings.Builder) {
	out.WriteString(m.st.dim("  │ ") + escCode)
	m.lineStart = false
}

// flushHead emits buffered line-start characters that never became markup.
func (m *mdStream) flushHead(out *strings.Builder) {
	h := m.head
	m.head, m.lineStart = "", false
	if h == "" {
		return
	}
	if m.inFence {
		m.beginFenceContent(out)
		out.WriteString(h)
		return
	}
	for _, c := range h {
		m.inline(out, c)
	}
}

func (m *mdStream) atLineStart(out *strings.Builder, r rune) {
	if r == '\n' {
		m.flushHead(out)
		m.newline(out)
		return
	}
	if m.head == "" && r == ' ' { // indentation passes straight through
		out.WriteRune(r)
		return
	}
	m.head += string(r)
	h := m.head
	switch {
	case h != "```" && strings.HasPrefix("```", h):
		// could still become a fence
	case h == "```":
		m.head, m.fenceLine, m.fenceInfo = "", true, ""
	case m.inFence:
		m.head = ""
		m.beginFenceContent(out)
		out.WriteString(h)
	case strings.Trim(h, "#") == "":
		// only #'s so far: might be a heading
	case h[0] == '#' && h[len(h)-1] == ' ' && strings.Trim(h[:len(h)-1], "#") == "":
		out.WriteString(escHeading)
		m.heading, m.head, m.lineStart = true, "", false
	case h == "-" || h == "*":
		// might be a bullet
	case h == "- " || h == "* ":
		out.WriteString(m.st.cyan("•") + " ")
		m.head, m.lineStart = "", false
	default:
		m.flushHead(out)
	}
}

func (m *mdStream) inline(out *strings.Builder, r rune) {
	switch {
	case r == '\n':
		m.newline(out)
	case m.inFence:
		out.WriteRune(r)
	case m.code:
		if r == '`' {
			m.code = false
			out.WriteString(escReset + m.restore())
		} else {
			out.WriteRune(r)
		}
	case r == '`':
		m.flushStar(out)
		m.code = true
		out.WriteString(escCode)
	case r == '*':
		if m.star {
			m.star = false
			m.bold = !m.bold
			if m.bold {
				out.WriteString(escBold)
			} else {
				out.WriteString(escUnbold)
			}
		} else {
			m.star = true
		}
	default:
		m.flushStar(out)
		out.WriteRune(r)
	}
}

// restore re-applies line-level styles after an inline reset.
func (m *mdStream) restore() string {
	switch {
	case m.heading:
		return escHeading
	case m.bold:
		return escBold
	}
	return ""
}
