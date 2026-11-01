package cli

import (
	"strings"
	"testing"

	"my-agent/internal/config"
)

// interactive returns a chat whose input is a scripted terminal session:
// each argument is one read from the keyboard (so "\x1b[A" is an arrow key
// arriving in one piece, like a real terminal delivers it). Styling is on,
// so the prompt, the suggestion list and the spinner are drawn.
func interactive(t *testing.T, reads ...string) *chatEnv {
	t.Helper()
	ce := newTestChat(t, "")
	ce.c.st = style{on: true}
	ce.c.term = &termInput{
		raw: func() (func(), error) { return func() {}, nil },
		out: ce.out,
		kr:  &keyReader{rd: chunks(reads...)},
		st:  ce.c.st,
	}
	return ce
}

// strokes splits typed text into one read per character, the way a terminal
// delivers keys as they are pressed (Tab and Enter included).
func strokes(s string) []string {
	var out []string
	for _, r := range s {
		out = append(out, string(r))
	}
	return out
}

func (ce *chatEnv) runLoop(t *testing.T) {
	t.Helper()
	if err := ce.c.loop(); err != nil {
		t.Fatalf("loop: %v\nstdout: %q\nstderr: %s", err, ce.out.String(), ce.err.String())
	}
}

func (ce *chatEnv) saved(t *testing.T) config.Config {
	t.Helper()
	c, err := config.Load(ce.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestInteractiveTabCompletionDrillsDownToAParameter(t *testing.T) {
	// "/co" Tab → "/config "; "ch" Tab → "chat "; "te" Tab → "temperature "; then the value
	ce := interactive(t, append(strokes("/co\tch\tte\t0.5\r"), "/exit\r")...) // one read per key, like real typing
	ce.runLoop(t)

	if got := ce.saved(t).Agent.Temperature; got != 0.5 {
		t.Fatalf("temperature = %v; the completed command did not run\nstdout: %q\nstderr: %s", got, ce.out.String(), ce.err.String())
	}
	out := ce.out.String()
	for _, want := range []string{
		"/config", "/connectors", // the list drawn after "/co"
		"provider", "temperature", // the list drawn after "/config chat "
		"❯",                       // the highlighted entry
		"\x1b[J",                  // list cleared/redrawn
		"agent.temperature = 0.5", // the command's own confirmation
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q", want)
		}
	}
	if ce.err.String() != "" {
		t.Errorf("errors: %s", ce.err.String())
	}
}

func TestInteractiveEnterOnAHighlightedSuggestionCompletesBeforeRunning(t *testing.T) {
	// "/mo" offers /models and /model; ↓ picks /model; the first Enter completes it, the second runs it
	ce := interactive(t, "/mo\x1b[B\r", "\r", "/exit\r")
	ce.runLoop(t)
	out := ce.out.String()
	if !strings.Contains(out, "ollama · llama3.2") {
		t.Fatalf("/model should have run:\n%q", out)
	}
	if strings.Contains(out, "Commands") || ce.err.String() != "" {
		t.Fatalf("the first Enter must only complete (not run /mo or /models): %q / %s", out, ce.err.String())
	}
}

func TestInteractiveEscHidesTheListAndEnterRunsWhatWasTyped(t *testing.T) {
	// "/he" is a unique prefix of /help
	ce := interactive(t, "/he\x1b", "\r", "/exit\r")
	ce.runLoop(t)
	if !strings.Contains(ce.out.String(), "Commands") {
		t.Fatalf("expected the help:\n%q", ce.out.String())
	}
}

func TestInteractiveHistoryWithArrowUp(t *testing.T) {
	ce := interactive(t, "primeira pergunta\r", "\x1b[A\r", "/exit\r")
	ce.runLoop(t)
	if len(ce.fp.all) != 2 {
		t.Fatalf("↑ then Enter must resend the previous line; %d requests", len(ce.fp.all))
	}
	last := ce.fp.all[1].Messages
	if got := last[len(last)-1].Content; got != "primeira pergunta" {
		t.Fatalf("resent %q", got)
	}
}

func TestInteractiveTypedAheadKeysAreNotLost(t *testing.T) {
	// everything arrives in ONE read (a paste, or fast typing): each Enter still ends a line
	ce := interactive(t, "olá\r/model\r/exit\r")
	ce.runLoop(t)
	out := ce.out.String()
	if len(ce.fp.all) != 1 || !strings.Contains(out, "echo: olá") || !strings.Contains(out, "ollama · llama3.2") {
		t.Fatalf("requests=%d\n%q", len(ce.fp.all), out)
	}
}

func TestInteractiveCtrlCClearsTheLineThenLeaves(t *testing.T) {
	ce := interactive(t, "texto que não deve ser enviado\x03", "\x03")
	ce.runLoop(t) // must return without error
	if len(ce.fp.all) != 0 {
		t.Fatalf("Ctrl+C discarded the line, yet %d request(s) were sent", len(ce.fp.all))
	}
}

func TestInteractiveCtrlDLeavesOnAnEmptyLine(t *testing.T) {
	ce := interactive(t, "\x04", "nunca lido\r")
	ce.runLoop(t)
	if len(ce.fp.all) != 0 {
		t.Fatal("the chat should have ended at Ctrl+D")
	}
}

func TestInteractiveEndOfInputEndsTheChat(t *testing.T) {
	ce := interactive(t, "oi\r") // then EOF
	ce.runLoop(t)
	if len(ce.fp.all) != 1 {
		t.Fatal("expected one message before the input ended")
	}
}

func TestInteractiveSecretIsMaskedAndKeptOutOfHistory(t *testing.T) {
	const s1, s2 = "S3CR3T-KEY-ONE", "SECOND-KEY-TWO"
	ce := interactive(t,
		"/config provider gemini api_key\r", s1+"\r",
		"\x1b[A\r", s2+"\r", // ↑ recalls the COMMAND (not the secret); it asks again
		"/exit\r",
	)
	ce.runLoop(t)
	out := ce.out.String()
	if strings.Contains(out, s1) || strings.Contains(out, s2) || strings.Contains(ce.err.String(), s1) {
		t.Fatalf("a secret was echoed:\n%q", out)
	}
	if !strings.Contains(out, strings.Repeat("*", len(s1))) {
		t.Fatalf("the typed characters should show as *:\n%q", out)
	}
	if got := ce.saved(t).Providers["gemini"].APIKey; got != s2 {
		t.Fatalf("saved key = %q, want the last one typed", got)
	}
	if strings.Count(out, "providers.gemini.api_key saved (hidden)") != 2 {
		t.Fatalf("expected two confirmations:\n%q", out)
	}
	for _, h := range ce.c.term.hist {
		if strings.Contains(h, "S3CR3T") || strings.Contains(h, "SECOND") {
			t.Fatalf("a secret entered the history: %q", ce.c.term.hist)
		}
	}
}

func TestInteractiveAtMentionCompletesAPathAndAttachesTheFile(t *testing.T) {
	inDir(t, t.TempDir())
	writeFile(t, "notas.txt", "A senha do cofre é ABACAXI-42.")
	ce := interactive(t, "qual é a senha em @no\t", "\r", "/exit\r")
	ce.runLoop(t)
	if len(ce.fp.all) != 1 {
		t.Fatalf("requests=%d\nstdout: %q", len(ce.fp.all), ce.out.String())
	}
	msgs := ce.fp.all[0].Messages
	sent := msgs[len(msgs)-1].Content
	if !strings.Contains(sent, "@notas.txt") || !strings.Contains(sent, "ABACAXI-42") || !strings.Contains(sent, "[Attached file: notas.txt]") {
		t.Fatalf("Tab should complete @no → @notas.txt and the file should be attached:\n%s", sent)
	}
	if !strings.Contains(ce.out.String(), "📎 attached: notas.txt") {
		t.Errorf("the user should see the attachment:\n%q", ce.out.String())
	}
}

func TestInteractiveStylingAroundReplies(t *testing.T) {
	ce := interactive(t, "oi\r", "/exit\r")
	ce.runLoop(t)
	out := ce.out.String()
	for _, want := range []string{" you ❯ ", "✦ agent", "echo: oi"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}

func TestInteractiveCommandErrorsAreShownAndTheSessionContinues(t *testing.T) {
	ce := interactive(t, "/config chat temperature 99\r", "/naoexiste\r", "ainda vivo\r", "/exit\r")
	ce.runLoop(t)
	if !strings.Contains(ce.err.String(), "agent.temperature") || !strings.Contains(ce.err.String(), "unknown command") {
		t.Fatalf("stderr: %q", ce.err.String())
	}
	if len(ce.fp.all) != 1 {
		t.Fatalf("the chat must go on after errors, %d requests", len(ce.fp.all))
	}
}
