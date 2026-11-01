package tui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"my-agent/internal/agent"
	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/i18n"
	"my-agent/internal/provider"
)

// scripted replays turns: the first asks to read a file, the second answers.
type scripted struct {
	mu    sync.Mutex
	turns []provider.Turn
	reqs  []provider.ChatRequest
}

func (s *scripted) Name() string                                               { return "fake" }
func (s *scripted) Chat(context.Context, provider.ChatRequest) (string, error) { return "", nil }
func (s *scripted) Embed(context.Context, []string) ([][]float32, error)       { return nil, nil }
func (s *scripted) Complete(_ context.Context, req provider.ChatRequest, onDelta func(string)) (provider.Turn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reqs = append(s.reqs, req)
	if len(s.turns) == 0 {
		return provider.Turn{Text: "done"}, nil
	}
	t := s.turns[0]
	s.turns = s.turns[1:]
	if onDelta != nil && t.Text != "" {
		onDelta(t.Text)
	}
	return t, nil
}

func setup(t *testing.T, fake *scripted) (*model, string) {
	t.Helper()
	i18n.Set(i18n.EN)
	dir := t.TempDir()
	t.Setenv("MY_AGENT_DATA", filepath.Join(dir, "data"))
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("the secret word is swordfish\n"), 0o644)
	cfgPath := filepath.Join(dir, "my-agent.json")
	cfg := config.Default()
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}
	a, err := app.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	a.NewProvider = func(string, config.ProviderConfig) (provider.Provider, error) { return fake, nil }
	ws := a.Workspace(context.Background(), dir, "test")
	t.Cleanup(ws.Close)
	m := newModel(context.Background(), ws, newTheme(true), Options{})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m, dir
}

// drive runs a command and feeds every message it produces back into the
// model, the way tea.Program does, answering permission requests with
// answer.
func drive(t *testing.T, m *model, answer string) {
	t.Helper()
	msgs := make(chan tea.Msg, 100)
	m.post = func(msg tea.Msg) { msgs <- msg }
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	var queue []tea.Cmd
	if cmd != nil {
		queue = append(queue, cmd)
	}
	for steps := 0; steps < 200; steps++ {
		if len(queue) > 0 {
			c := queue[0]
			queue = queue[1:]
			go func() {
				if msg := c(); msg != nil {
					msgs <- msg
				}
			}()
		}
		msg := <-msgs
		switch msg := msg.(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
			continue
		case permMsg:
			m.Update(msg)
			_, c := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(answer)})
			if c != nil {
				queue = append(queue, c)
			}
			continue
		case doneMsg:
			m.Update(msg)
			return
		}
		if _, c := m.Update(msg); c != nil {
			// ignore timers (blink, spinner, tick): they never finish
		}
	}
	t.Fatal("the run did not finish")
}

func typeText(m *model, s string) {
	m.input.SetValue(s)
	m.updateSuggestions()
}

func plain(s string) string { return ansi.Strip(s) }

func TestWelcomeScreen(t *testing.T) {
	m, _ := setup(t, &scripted{})
	v := plain(m.View())
	for _, want := range []string{"build", "ctrl+p", "command palette", "BUILD", "ollama/"} {
		if !strings.Contains(v, want) {
			t.Errorf("welcome screen lacks %q:\n%s", want, v)
		}
	}
}

func TestConversationWithToolAndSession(t *testing.T) {
	fake := &scripted{turns: []provider.Turn{
		{Text: "Let me read it.", ToolCalls: []provider.ToolCall{{ID: "1", Name: "read", Args: json.RawMessage(`{"path":"notes.txt"}`)}}},
		{Text: "The word is **swordfish**.", Usage: provider.Usage{Input: 10, Output: 5}},
	}}
	m, _ := setup(t, fake)
	typeText(m, "what is the secret word?")
	drive(t, m, "y")

	v := plain(m.transcript())
	for _, want := range []string{"what is the secret word?", "read", "notes.txt", "swordfish", "build · ollama/"} {
		if !strings.Contains(v, want) {
			t.Errorf("transcript lacks %q:\n%s", want, v)
		}
	}
	if len(fake.reqs) != 2 || !strings.Contains(fake.reqs[1].Messages[2].Content, "swordfish") {
		t.Fatalf("the tool result did not reach the model: %+v", fake.reqs)
	}
	if len(m.sess.Messages) != 4 || m.sess.Title != "what is the secret word?" {
		t.Fatalf("session: %+v", m.sess)
	}
	list, _ := m.ws.Sessions.List("")
	if len(list) != 1 || list[0].Messages != 4 {
		t.Fatalf("session was not saved: %+v", list)
	}
	// resuming rebuilds the same transcript
	m2, _ := setup(t, &scripted{})
	m2.ws.Sessions = m.ws.Sessions
	m2.resume(list[0].ID)
	if !strings.Contains(plain(m2.transcript()), "swordfish") {
		t.Fatal("resumed transcript lacks the answer")
	}
}

func TestPermissionDenied(t *testing.T) {
	fake := &scripted{turns: []provider.Turn{
		{ToolCalls: []provider.ToolCall{{ID: "1", Name: "write", Args: json.RawMessage(`{"path":"x.txt","content":"hi"}`)}}},
	}}
	m, dir := setup(t, fake)
	typeText(m, "create x.txt")
	drive(t, m, "n")
	if _, err := os.Stat(filepath.Join(dir, "x.txt")); err == nil {
		t.Fatal("the file was written although permission was denied")
	}
	if !strings.Contains(plain(m.transcript()), "denied") {
		t.Fatalf("transcript:\n%s", plain(m.transcript()))
	}
}

func TestPermissionAllowedWritesFile(t *testing.T) {
	fake := &scripted{turns: []provider.Turn{
		{ToolCalls: []provider.ToolCall{{ID: "1", Name: "write", Args: json.RawMessage(`{"path":"x.txt","content":"hi"}`)}}},
		{Text: "created"},
	}}
	m, dir := setup(t, fake)
	typeText(m, "create x.txt")
	drive(t, m, "a")
	if b, err := os.ReadFile(filepath.Join(dir, "x.txt")); err != nil || string(b) != "hi" {
		t.Fatalf("file: %q %v", b, err)
	}
	if !m.grants["write"] {
		t.Fatalf("'always' was not remembered: %v", m.grants)
	}
}

func TestAgentCyclingAndCommands(t *testing.T) {
	m, _ := setup(t, &scripted{})
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.ra.Name != "chat" {
		t.Fatalf("tab should move to the next agent, got %s", m.ra.Name)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.ra.Name != "build" {
		t.Fatalf("shift+tab should go back, got %s", m.ra.Name)
	}
	typeText(m, "/age")
	if len(m.sugg) != 1 || m.sugg[0].text != "/agents" {
		t.Fatalf("suggestions: %+v", m.sugg)
	}
	typeText(m, "/agents pl")
	if len(m.sugg) != 1 || m.sugg[0].text != "/agents plan" {
		t.Fatalf("value suggestions: %+v", m.sugg)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.ra.Name != "plan" {
		t.Fatalf("enter on a complete suggestion should run it, agent=%s", m.ra.Name)
	}
	typeText(m, "/tools")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	out := plain(m.transcript())
	if !strings.Contains(out, "Tools of agent plan") || strings.Contains(out, "write ") {
		t.Fatalf("plan agent tools:\n%s", out)
	}
	typeText(m, "/help")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(plain(m.transcript()), "/sessions") {
		t.Fatal("help lacks commands")
	}
	typeText(m, "/nope")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(plain(m.transcript()), "unknown command /nope") {
		t.Fatal("unknown command not reported")
	}
}

func TestConnectAddsProviderAndKeys(t *testing.T) {
	m, _ := setup(t, &scripted{})
	m.runCommand("/connect openrouter")
	if m.prompt == nil {
		t.Fatal("connect should ask for the key")
	}
	m.prompt.in.SetValue("sk-one")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.runCommand("/keys openrouter")
	m.prompt.in.SetValue("sk-two")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	saved, err := config.Load(m.ws.App.Path)
	if err != nil {
		t.Fatal(err)
	}
	pc := saved.Providers["openrouter"]
	if pc.Type != "openai" || pc.APIKey != "sk-one" || len(pc.APIKeys) != 1 || pc.APIKeys[0] != "sk-two" {
		t.Fatalf("provider: %+v", pc)
	}
	if p, _ := m.provModel(); p != "openrouter" {
		t.Fatalf("the new provider should be selected, got %s", p)
	}
	if strings.Contains(plain(m.View()), "sk-") {
		t.Fatal("a key is visible on screen")
	}
}

func TestMentionsAttachFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha"), 0o644)
	full, att, warns := expandMentions(dir, "look at @a.txt and @missing.txt, thanks @bob")
	if len(att) != 1 || !strings.Contains(full, "alpha") || len(warns) != 1 {
		t.Fatalf("%q %v %v", full, att, warns)
	}
	if s := fileSuggestions(dir, "a"); len(s) != 1 || s[0].text != "a.txt" {
		t.Fatalf("suggestions: %+v", s)
	}
}

var _ = agent.Once
