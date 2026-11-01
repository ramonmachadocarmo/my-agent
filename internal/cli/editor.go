package cli

import (
	"fmt"
	"strings"
	"unicode"
)

type edStatus int

const (
	edContinue edStatus = iota
	edSubmit
	edEOF // Ctrl+D or Ctrl+C on an empty line: leave the chat
)

const maxVisibleSuggestions = 8

// editor is a one-line editor with an autocomplete list. It is pure state:
// handle applies a key, render returns the ANSI text that draws the current
// state. That keeps it testable without a terminal.
type editor struct {
	st      style
	prompt  string // may contain ANSI styling
	promptW int    // visible width of the prompt
	mask    bool   // echo '*' instead of the text (secrets)
	suggest suggestFunc
	width   int

	buf []rune
	cur int

	hist  []string
	hpos  int // position while browsing history; len(hist) when not browsing
	draft []rune

	sugg     []suggestion
	tokStart int
	sel      int
	nav      bool // the user moved the selection with the arrow keys
	hidden   bool // suggestions dismissed until the next edit
}

func newEditor(st style, prompt string, promptW int, suggest suggestFunc, mask bool, width int, hist []string) *editor {
	if width < 20 {
		width = 80
	}
	return &editor{st: st, prompt: prompt, promptW: promptW, suggest: suggest, mask: mask, width: width, hist: hist, hpos: len(hist)}
}

func (ed *editor) text() string { return string(ed.buf) }

func (ed *editor) open() bool { return !ed.hidden && len(ed.sugg) > 0 }

// token is the word being completed (from where the suggestions start up to
// the cursor).
func (ed *editor) token() string {
	if ed.tokStart < 0 || ed.tokStart > ed.cur || ed.cur > len(ed.buf) {
		return ""
	}
	return string(ed.buf[ed.tokStart:ed.cur])
}

// handle applies one key and reports whether the line is finished.
func (ed *editor) handle(k key) edStatus {
	switch k.kind {
	case kRune:
		ed.insert(k.r)
	case kEnter:
		// Enter on a highlighted suggestion completes it, unless the word is
		// already fully typed: then there is nothing to complete, so run it.
		if ed.open() && ed.nav && ed.sugg[ed.sel].insert != ed.token() {
			ed.accept()
			return edContinue
		}
		return edSubmit
	case kTab:
		if ed.open() {
			ed.accept()
		}
	case kUp:
		if ed.open() {
			ed.move(-1)
		} else {
			ed.histMove(-1)
		}
	case kDown:
		if ed.open() {
			ed.move(1)
		} else {
			ed.histMove(1)
		}
	case kLeft:
		if ed.cur > 0 {
			ed.cur--
			ed.refresh()
		}
	case kRight:
		if ed.cur < len(ed.buf) {
			ed.cur++
			ed.refresh()
		}
	case kHome:
		ed.cur = 0
		ed.refresh()
	case kEnd:
		ed.cur = len(ed.buf)
		ed.refresh()
	case kBackspace:
		if ed.cur > 0 {
			ed.buf = append(ed.buf[:ed.cur-1], ed.buf[ed.cur:]...)
			ed.cur--
			ed.refresh()
		}
	case kDelete:
		ed.deleteAt()
	case kCtrlU:
		ed.buf = append([]rune{}, ed.buf[ed.cur:]...)
		ed.cur = 0
		ed.refresh()
	case kCtrlK:
		ed.buf = ed.buf[:ed.cur]
		ed.refresh()
	case kCtrlW:
		i := ed.cur
		for i > 0 && unicode.IsSpace(ed.buf[i-1]) {
			i--
		}
		for i > 0 && !unicode.IsSpace(ed.buf[i-1]) {
			i--
		}
		ed.buf = append(ed.buf[:i], ed.buf[ed.cur:]...)
		ed.cur = i
		ed.refresh()
	case kCtrlC:
		if len(ed.buf) == 0 {
			return edEOF
		}
		ed.buf, ed.cur = nil, 0
		ed.refresh()
	case kCtrlD:
		if len(ed.buf) == 0 {
			return edEOF
		}
		ed.deleteAt()
	case kEsc:
		ed.hidden, ed.sugg = true, nil
	}
	return edContinue
}

func (ed *editor) insert(r rune) {
	ed.buf = append(ed.buf, 0)
	copy(ed.buf[ed.cur+1:], ed.buf[ed.cur:])
	ed.buf[ed.cur] = r
	ed.cur++
	ed.hpos = len(ed.hist)
	ed.refresh()
}

func (ed *editor) deleteAt() {
	if ed.cur < len(ed.buf) {
		ed.buf = append(ed.buf[:ed.cur], ed.buf[ed.cur+1:]...)
		ed.refresh()
	}
}

// refresh recomputes the suggestions for the word before the cursor.
func (ed *editor) refresh() {
	ed.sel, ed.nav, ed.hidden = 0, false, false
	if ed.mask || ed.suggest == nil {
		ed.sugg = nil
		return
	}
	ed.sugg, ed.tokStart = ed.suggest(string(ed.buf[:ed.cur]))
}

func (ed *editor) move(d int) {
	n := len(ed.sugg)
	ed.sel = (ed.sel + d + n) % n
	ed.nav = true
}

// accept replaces the word under the cursor with the selected suggestion.
func (ed *editor) accept() {
	s := ed.sugg[ed.sel]
	ins := []rune(s.insert)
	if s.space {
		ins = append(ins, ' ')
	}
	end := ed.cur
	for end < len(ed.buf) && !unicode.IsSpace(ed.buf[end]) {
		end++
	}
	nb := append([]rune{}, ed.buf[:ed.tokStart]...)
	nb = append(nb, ins...)
	nb = append(nb, ed.buf[end:]...)
	ed.buf, ed.cur = nb, ed.tokStart+len(ins)
	ed.refresh()
}

// histMove browses the lines sent before (Up/Down while no suggestion list
// is open). The unfinished line is kept as a draft.
func (ed *editor) histMove(d int) {
	if len(ed.hist) == 0 {
		return
	}
	if ed.hpos == len(ed.hist) {
		ed.draft = append([]rune{}, ed.buf...)
	}
	ed.hpos += d
	switch {
	case ed.hpos < 0:
		ed.hpos = 0
	case ed.hpos >= len(ed.hist):
		ed.hpos = len(ed.hist)
		ed.buf = append([]rune{}, ed.draft...)
	default:
		ed.buf = []rune(ed.hist[ed.hpos])
	}
	ed.cur = len(ed.buf)
	ed.sugg, ed.hidden = nil, true
}

// render returns the escape sequences that redraw the input line and the
// suggestion list, leaving the cursor at its place in the input line.
func (ed *editor) render() string {
	shown, col := ed.visible()
	var sb strings.Builder
	sb.WriteString("\r" + ed.prompt + shown + "\x1b[J") // \x1b[J also erases the old list
	lines := ed.suggestionLines()
	for _, l := range lines {
		sb.WriteString("\r\n" + l)
	}
	if len(lines) > 0 {
		fmt.Fprintf(&sb, "\x1b[%dA", len(lines))
	}
	sb.WriteString("\r")
	if col > 0 {
		fmt.Fprintf(&sb, "\x1b[%dC", col)
	}
	return sb.String()
}

// finish clears the suggestion list and moves to the next line.
func (ed *editor) finish() string { return "\x1b[J\r\n" }

// visible returns the part of the text that fits on one row and the cursor
// column. Long lines scroll horizontally instead of wrapping, because a
// wrapped line cannot be redrawn reliably with cursor movements.
func (ed *editor) visible() (string, int) {
	runes := ed.buf
	if ed.mask {
		runes = []rune(strings.Repeat("*", len(ed.buf)))
	}
	avail := max(ed.width-ed.promptW-1, 10)
	start := 0
	if len(runes) > avail {
		if ed.cur > avail-1 {
			start = ed.cur - (avail - 1)
		}
		runes = runes[start:min(len(runes), start+avail)]
	}
	return string(runes), ed.promptW + ed.cur - start
}

func (ed *editor) suggestionLines() []string {
	if !ed.open() {
		return nil
	}
	n := len(ed.sugg)
	from := 0
	if ed.sel >= maxVisibleSuggestions {
		from = ed.sel - maxVisibleSuggestions + 1
	}
	to := min(n, from+maxVisibleSuggestions)
	labelW := 0
	for _, s := range ed.sugg[from:to] {
		labelW = max(labelW, len([]rune(s.label)))
	}
	labelW = min(labelW, 30)
	room := max(ed.width-labelW-6, 0)
	var lines []string
	for i := from; i < to; i++ {
		s := ed.sugg[i]
		label := truncate(s.label, labelW)
		padded := label + strings.Repeat(" ", labelW-len([]rune(label)))
		desc := truncate(s.desc, room)
		if i == ed.sel {
			lines = append(lines, "  "+ed.st.cyan("❯ ")+ed.st.bold(padded)+"  "+ed.st.dim(desc))
		} else {
			lines = append(lines, "    "+padded+"  "+ed.st.dim(desc))
		}
	}
	if n > to-from {
		lines = append(lines, "    "+ed.st.dim(fmt.Sprintf("%d/%d", ed.sel+1, n)))
	}
	return lines
}

func truncate(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return string(r[:max(w, 0)])
	}
	return string(r[:w-1]) + "…"
}
