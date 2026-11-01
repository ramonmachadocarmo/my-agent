package cli

import (
	"strings"
	"testing"
)

// fixed suggestions keyed by the word being typed
func fakeSuggest(line string) ([]suggestion, int) {
	ts := strings.LastIndexAny(line, " ") + 1
	tok := line[ts:]
	all := []suggestion{
		{insert: "/config", label: "/config", desc: "Configuração", space: true},
		{insert: "/connectors", label: "/connectors", desc: "Conectores", space: true},
		{insert: "/help", label: "/help", desc: "Ajuda"},
	}
	var out []suggestion
	if strings.HasPrefix(tok, "/") {
		for _, s := range all {
			if strings.HasPrefix(s.insert, tok) {
				out = append(out, s)
			}
		}
	}
	return out, len([]rune(line[:ts]))
}

func newTestEditor(hist ...string) *editor {
	return newEditor(style{}, "> ", 2, fakeSuggest, false, 80, hist)
}

func typeText(ed *editor, s string) {
	for _, r := range s {
		ed.handle(key{kind: kRune, r: r})
	}
}

func press(ed *editor, kinds ...keyKind) edStatus {
	var st edStatus
	for _, k := range kinds {
		st = ed.handle(key{kind: k})
	}
	return st
}

func TestEditorTypingShowsSuggestionsAndTabCompletes(t *testing.T) {
	ed := newTestEditor()
	typeText(ed, "/co")
	if len(ed.sugg) != 2 || !ed.open() {
		t.Fatalf("want config and connectors, got %+v", ed.sugg)
	}
	press(ed, kTab)
	if ed.text() != "/config " { // first suggestion, with a trailing space because it takes arguments
		t.Fatalf("text = %q", ed.text())
	}
	if ed.cur != len([]rune("/config ")) {
		t.Fatalf("cursor = %d", ed.cur)
	}
}

func TestEditorArrowsChooseAndEnterCompletesInsteadOfSubmitting(t *testing.T) {
	ed := newTestEditor()
	typeText(ed, "/co")
	press(ed, kDown) // choose /connectors
	if ed.sel != 1 || !ed.nav {
		t.Fatalf("sel=%d nav=%v", ed.sel, ed.nav)
	}
	if st := press(ed, kEnter); st != edContinue {
		t.Fatal("Enter on a highlighted suggestion must complete it, not submit")
	}
	if ed.text() != "/connectors " {
		t.Fatalf("text = %q", ed.text())
	}
	// now nothing is highlighted: Enter submits what is typed
	if st := press(ed, kEnter); st != edSubmit {
		t.Fatal("Enter should submit")
	}
}

func TestEditorEnterSubmitsTypedTextWhenNothingWasChosen(t *testing.T) {
	ed := newTestEditor()
	typeText(ed, "/he")
	if st := press(ed, kEnter); st != edSubmit || ed.text() != "/he" {
		t.Fatalf("status=%v text=%q", st, ed.text())
	}
}

func TestEditorSelectionWrapsAndEscHides(t *testing.T) {
	ed := newTestEditor()
	typeText(ed, "/co")
	press(ed, kUp) // wraps to the last one
	if ed.sel != 1 {
		t.Fatalf("sel=%d", ed.sel)
	}
	press(ed, kEsc)
	if ed.open() {
		t.Fatal("Esc should hide the list")
	}
	press(ed, kTab) // nothing to accept
	if ed.text() != "/co" {
		t.Fatalf("text = %q", ed.text())
	}
	typeText(ed, "n") // typing brings it back
	if !ed.open() {
		t.Fatal("typing should reopen suggestions")
	}
}

func TestEditorEditingKeys(t *testing.T) {
	ed := newTestEditor()
	typeText(ed, "hello world")
	press(ed, kBackspace)
	if ed.text() != "hello worl" {
		t.Fatalf("backspace: %q", ed.text())
	}
	press(ed, kLeft, kLeft, kDelete) // removes the 'r'
	if ed.text() != "hello wol" {
		t.Fatalf("delete: %q", ed.text())
	}
	press(ed, kHome)
	typeText(ed, ">> ")
	if ed.text() != ">> hello wol" || ed.cur != 3 {
		t.Fatalf("insert at start: %q cur=%d", ed.text(), ed.cur)
	}
	press(ed, kEnd, kCtrlW)
	if ed.text() != ">> hello " {
		t.Fatalf("ctrl-w: %q", ed.text())
	}
	press(ed, kHome, kCtrlK)
	if ed.text() != "" {
		t.Fatalf("ctrl-k: %q", ed.text())
	}
	typeText(ed, "abc")
	press(ed, kLeft, kCtrlU)
	if ed.text() != "c" || ed.cur != 0 {
		t.Fatalf("ctrl-u: %q cur=%d", ed.text(), ed.cur)
	}
}

func TestEditorUnicode(t *testing.T) {
	ed := newTestEditor()
	typeText(ed, "olá 漢")
	press(ed, kBackspace, kBackspace)
	if ed.text() != "olá" {
		t.Fatalf("%q", ed.text())
	}
}

func TestEditorCtrlCAndCtrlD(t *testing.T) {
	ed := newTestEditor()
	typeText(ed, "abc")
	if st := press(ed, kCtrlC); st != edContinue || ed.text() != "" {
		t.Fatalf("Ctrl+C with text clears the line: status=%v text=%q", st, ed.text())
	}
	if st := press(ed, kCtrlC); st != edEOF {
		t.Fatal("Ctrl+C on an empty line leaves")
	}
	ed = newTestEditor()
	typeText(ed, "ab")
	press(ed, kLeft)
	if st := press(ed, kCtrlD); st != edContinue || ed.text() != "a" {
		t.Fatalf("Ctrl+D with text deletes: status=%v text=%q", st, ed.text())
	}
	press(ed, kCtrlU, kEnd)
	ed.buf, ed.cur = nil, 0
	if st := press(ed, kCtrlD); st != edEOF {
		t.Fatal("Ctrl+D on an empty line leaves")
	}
}

func TestEditorHistory(t *testing.T) {
	ed := newTestEditor("first", "/second")
	typeText(ed, "draft")
	press(ed, kUp)
	if ed.text() != "/second" {
		t.Fatalf("up: %q", ed.text())
	}
	if ed.open() {
		t.Fatal("history entries must not pop the suggestion list open (arrows would stop browsing)")
	}
	press(ed, kUp)
	if ed.text() != "first" {
		t.Fatalf("up: %q", ed.text())
	}
	press(ed, kUp) // at the oldest: stays
	if ed.text() != "first" {
		t.Fatalf("up at the top: %q", ed.text())
	}
	press(ed, kDown, kDown)
	if ed.text() != "draft" {
		t.Fatalf("the unfinished line must come back: %q", ed.text())
	}
}

func TestEditorRenderDrawsPromptListAndCursor(t *testing.T) {
	ed := newTestEditor()
	typeText(ed, "/co")
	out := ed.render()
	for _, want := range []string{"\r> /co", "\x1b[J", "\r\n", "/config", "Configuração", "/connectors", "\x1b[2A", "\x1b[5C"} {
		if !strings.Contains(out, want) {
			t.Errorf("render is missing %q:\n%q", want, out)
		}
	}
	// cursor: prompt (2) + 3 typed characters = column 5
	if !strings.HasSuffix(out, "\r\x1b[5C") {
		t.Errorf("cursor not placed at column 5: %q", out)
	}
	press(ed, kEsc)
	if out := ed.render(); strings.Contains(out, "/connectors") || strings.Contains(out, "A") && strings.Contains(out, "\x1b[2A") {
		t.Errorf("hidden list must not be drawn: %q", out)
	}
	if got := ed.finish(); got != "\x1b[J\r\n" {
		t.Errorf("finish = %q", got)
	}
}

func TestEditorMarksTheSelection(t *testing.T) {
	ed := newEditor(style{on: true}, "> ", 2, fakeSuggest, false, 80, nil)
	typeText(ed, "/co")
	press(ed, kDown)
	lines := ed.suggestionLines()
	if len(lines) != 2 || !strings.Contains(lines[1], "❯") || strings.Contains(lines[0], "❯") {
		t.Fatalf("the ❯ marker must be on the second entry only: %q", lines)
	}
}

func TestEditorSuggestionListScrolls(t *testing.T) {
	many := func(string) ([]suggestion, int) {
		var out []suggestion
		for i := 0; i < 20; i++ {
			out = append(out, suggestion{insert: string(rune('a' + i)), label: string(rune('a' + i))})
		}
		return out, 0
	}
	ed := newEditor(style{}, "> ", 2, many, false, 80, nil)
	typeText(ed, "x")
	if n := len(ed.suggestionLines()); n != maxVisibleSuggestions+1 { // + the "1/20" counter
		t.Fatalf("visible lines = %d", n)
	}
	for i := 0; i < 12; i++ {
		press(ed, kDown)
	}
	lines := ed.suggestionLines()
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "m") || !strings.Contains(lines[len(lines)-1], "13/20") {
		t.Fatalf("window did not follow the selection:\n%s", joined)
	}
}

func TestEditorLongLineScrollsInsteadOfWrapping(t *testing.T) {
	ed := newEditor(style{}, "> ", 2, nil, false, 30, nil) // 27 columns for text
	typeText(ed, strings.Repeat("x", 100)+"END")
	shown, col := ed.visible()
	if len([]rune(shown)) > 27 || !strings.HasSuffix(shown, "END") {
		t.Fatalf("visible window %q (%d)", shown, len([]rune(shown)))
	}
	if col > 29 {
		t.Fatalf("cursor column %d is beyond the terminal width", col)
	}
	press(ed, kHome)
	shown, col = ed.visible()
	if !strings.HasPrefix(shown, "xxx") || col != 2 {
		t.Fatalf("after Home: %q col=%d", shown, col)
	}
}

func TestEditorMasksSecrets(t *testing.T) {
	ed := newEditor(style{}, "key: ", 5, fakeSuggest, true, 80, nil)
	typeText(ed, "/sup3r-secret")
	if ed.text() != "/sup3r-secret" {
		t.Fatalf("the value itself must be kept: %q", ed.text())
	}
	out := ed.render()
	if strings.Contains(out, "sup3r") || !strings.Contains(out, "*************") {
		t.Fatalf("secret leaked or not masked: %q", out)
	}
	if len(ed.sugg) != 0 {
		t.Fatal("no suggestions while typing a secret")
	}
}

func TestEditorAcceptReplacesTheWholeWordUnderTheCursor(t *testing.T) {
	ed := newTestEditor()
	typeText(ed, "/config x")
	ed.buf, ed.cur = []rune("/coXXX rest"), 3 // cursor in the middle of the first word
	ed.refresh()
	press(ed, kTab)
	if ed.text() != "/config  rest" && ed.text() != "/config rest" {
		t.Fatalf("leftover of the word must be replaced, got %q", ed.text())
	}
}

func TestEditorEnterRunsAFullyTypedWordEvenIfHighlighted(t *testing.T) {
	// regression (seen on a real console): "/help" typed in full, then ↑ moved the
	// highlight; Enter only appended a space instead of running the command
	ed := newTestEditor()
	typeText(ed, "/help")
	press(ed, kUp)
	if !ed.nav {
		t.Fatal("the highlight moved")
	}
	if st := press(ed, kEnter); st != edSubmit || ed.text() != "/help" {
		t.Fatalf("status=%v text=%q: nothing was left to complete, so Enter must submit", st, ed.text())
	}
	// but a partly typed word is still completed by Enter
	ed = newTestEditor()
	typeText(ed, "/he")
	press(ed, kDown)
	if st := press(ed, kEnter); st != edContinue || ed.text() != "/help" {
		t.Fatalf("status=%v text=%q", st, ed.text())
	}
}

func TestSubmittedLineIsDrawnEvenWhenItArrivedInOneRead(t *testing.T) {
	// regression (found by the scripted tests): a pasted or fast-typed line was
	// executed without ever being drawn, leaving an empty prompt on screen
	ce := interactive(t, "/model\r", "/exit\r")
	ce.runLoop(t)
	out := ce.out.String()
	if !strings.Contains(out, "you ❯ \x1b[0m\x1b[0m/model") {
		t.Fatalf("the submitted text must be on screen:\n%q", out)
	}
}
