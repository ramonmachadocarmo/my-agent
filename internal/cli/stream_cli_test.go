package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/provider"
)

type streamFake struct {
	*fakeProvider
	fail   bool
	deltas int
}

func (s *streamFake) ChatStream(_ context.Context, r provider.ChatRequest, onDelta func(string)) (string, error) {
	s.last = r
	pieces := []string{"Hello", " ", "world", "!"}
	if s.fail {
		pieces = pieces[:2]
	}
	for _, p := range pieces {
		s.deltas++
		onDelta(p)
	}
	if s.fail {
		return "Hello ", errors.New("connection reset")
	}
	return strings.Join(pieces, ""), nil
}

// setupStream installs a provider that streams its reply in pieces.
func setupStream(t *testing.T) (string, *streamFake) {
	t.Helper()
	cfg, fp := setup(t)
	sf := &streamFake{fakeProvider: fp}
	orig := loadApp
	loadApp = func(path string) (*app.App, error) {
		a, err := orig(path)
		if err == nil {
			a.NewProvider = func(string, config.ProviderConfig) (provider.Provider, error) { return sf, nil }
		}
		return a, err
	}
	t.Cleanup(func() { loadApp = orig })
	return cfg, sf
}

func TestAskStreamsReply(t *testing.T) {
	cfg, sf := setupStream(t)
	mustRun(t, "", "-config", cfg, "init")
	out := mustRun(t, "", "-config", cfg, "ask", "--no-rag", "hi")
	if out != "Hello world!\n" || sf.deltas != 4 {
		t.Fatalf("out=%q deltas=%d", out, sf.deltas)
	}
}

func TestChatStreamsReplyAndKeepsHistory(t *testing.T) {
	cfg, sf := setupStream(t)
	mustRun(t, "", "-config", cfg, "init")
	out := mustRun(t, "one\ntwo\n", "-config", cfg, "chat", "--no-rag")
	if strings.Count(out, "agent> Hello world!") != 2 || sf.deltas != 8 {
		t.Fatalf("deltas=%d\n%s", sf.deltas, out)
	}
	if len(sf.last.Messages) != 3 { // one, reply, two
		t.Fatalf("history not kept across streamed turns: %d messages", len(sf.last.Messages))
	}
}

func TestChatStreamErrorMidReply(t *testing.T) {
	cfg, sf := setupStream(t)
	sf.fail = true
	mustRun(t, "", "-config", cfg, "init")
	var o, e strings.Builder
	code := runTo(&o, &e, "one\ntwo\n", "-config", cfg, "chat", "--no-rag")
	if code != 0 {
		t.Fatalf("a failed turn must not end the chat, exit=%d", code)
	}
	if !strings.Contains(o.String(), "agent> Hello ") || !strings.Contains(e.String(), "connection reset") {
		t.Fatalf("stdout=%q stderr=%q", o.String(), e.String())
	}
	if len(sf.last.Messages) != 1 { // failed turn 'one' must not be in the history of 'two'
		t.Fatalf("failed turn leaked into history: %d", len(sf.last.Messages))
	}
}

func TestAskStreamErrorKeepsPartialAndFails(t *testing.T) {
	cfg, sf := setupStream(t)
	sf.fail = true
	mustRun(t, "", "-config", cfg, "init")
	code, out, errOut := run(t, "", "-config", cfg, "ask", "--no-rag", "hi")
	if code != 1 || !strings.HasPrefix(out, "Hello ") || !strings.Contains(errOut, "connection reset") {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
}

func runTo(o, e *strings.Builder, stdin string, args ...string) int {
	return Run(args, strings.NewReader(stdin), o, e)
}
