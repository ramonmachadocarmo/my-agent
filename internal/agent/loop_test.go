package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"my-agent/internal/config"
	"my-agent/internal/provider"
	"my-agent/internal/tools"
)

// scripted replays a list of turns and records the requests it got.
type scripted struct {
	turns []provider.Turn
	reqs  []provider.ChatRequest
}

func (s *scripted) Name() string                                               { return "scripted" }
func (s *scripted) Chat(context.Context, provider.ChatRequest) (string, error) { return "", nil }
func (s *scripted) Embed(context.Context, []string) ([][]float32, error)       { return nil, nil }
func (s *scripted) Complete(_ context.Context, req provider.ChatRequest, onDelta func(string)) (provider.Turn, error) {
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

func call(id, name, args string) provider.ToolCall {
	return provider.ToolCall{ID: id, Name: name, Args: json.RawMessage(args)}
}

func echoTool(name string, ran *[]string) *tools.Tool {
	return &tools.Tool{
		Name:    name,
		Subject: func(a map[string]any) string { s, _ := a["command"].(string); return s },
		Run: func(_ context.Context, _ *tools.Env, a map[string]any) (tools.Result, error) {
			*ran = append(*ran, name)
			return tools.Result{Output: name + " ok"}, nil
		},
	}
}

func agentWith(perm map[string]string) config.ResolvedAgent {
	cfg := config.Default()
	cfg.Permission = perm
	ra, _ := cfg.ResolveAgent("build")
	return ra
}

func TestRunnerLoopsUntilFinalAnswer(t *testing.T) {
	var ran []string
	p := &scripted{turns: []provider.Turn{
		{Text: "checking", ToolCalls: []provider.ToolCall{call("1", "read", `{}`), call("2", "ls", `{}`)}},
		{Text: "all good"},
	}}
	r := &Runner{Agent: agentWith(nil), Provider: p, Tools: tools.NewSet(echoTool("read", &ran), echoTool("ls", &ran))}
	var events []EventKind
	r.OnEvent = func(e Event) { events = append(events, e.Kind) }
	added, _, err := r.Run(context.Background(), nil, "hi", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ran, ",") != "read,ls" {
		t.Fatalf("ran %v", ran)
	}
	// user, assistant(with calls), 2 tool results, final assistant
	if len(added) != 5 || added[2].Role != "tool" || added[2].ToolCallID != "1" || added[4].Content != "all good" {
		t.Fatalf("added: %+v", added)
	}
	if len(p.reqs) != 2 || len(p.reqs[1].Messages) != 4 {
		t.Fatalf("second request should carry the tool results: %+v", p.reqs)
	}
	if len(p.reqs[0].Tools) != 2 {
		t.Fatalf("tools not sent: %+v", p.reqs[0].Tools)
	}
	_ = events
}

func TestRunnerAsksPermissionAndRemembersAlways(t *testing.T) {
	var ran []string
	p := &scripted{turns: []provider.Turn{
		{ToolCalls: []provider.ToolCall{call("1", "bash", `{"command":"go test ./..."}`)}},
		{ToolCalls: []provider.ToolCall{call("2", "bash", `{"command":"go test ./internal/..."}`)}},
		{Text: "tests pass"},
	}}
	asked := 0
	r := &Runner{Agent: agentWith(nil), Provider: p, Tools: tools.NewSet(echoTool("bash", &ran))}
	r.Approve = func(_ context.Context, req PermissionRequest) Decision {
		asked++
		if req.Subject != "go test ./..." {
			t.Errorf("subject %q", req.Subject)
		}
		return Always
	}
	if _, _, err := r.Run(context.Background(), nil, "test it", ""); err != nil {
		t.Fatal(err)
	}
	if asked != 1 || len(ran) != 2 {
		t.Fatalf("asked %d times, ran %v", asked, ran)
	}
}

func TestRunnerStopsWhenDenied(t *testing.T) {
	var ran []string
	p := &scripted{turns: []provider.Turn{
		{ToolCalls: []provider.ToolCall{call("1", "write", `{}`), call("2", "read", `{}`)}},
	}}
	r := &Runner{Agent: agentWith(nil), Provider: p, Tools: tools.NewSet(echoTool("write", &ran), echoTool("read", &ran))}
	r.Approve = func(context.Context, PermissionRequest) Decision { return Deny }
	added, _, err := r.Run(context.Background(), nil, "go", "")
	if !errors.Is(err, ErrDenied) {
		t.Fatalf("err = %v", err)
	}
	if len(ran) != 0 || len(p.reqs) != 1 {
		t.Fatalf("ran %v after a denial; %d model calls", ran, len(p.reqs))
	}
	// every call still gets a result, so the history stays valid
	if len(added) != 4 || !added[2].IsError || !added[3].IsError {
		t.Fatalf("added: %+v", added)
	}
}

func TestRunnerHonoursDenyAndAllowRules(t *testing.T) {
	var ran []string
	p := &scripted{turns: []provider.Turn{
		{ToolCalls: []provider.ToolCall{call("1", "bash", `{"command":"git status"}`)}},
		{ToolCalls: []provider.ToolCall{call("2", "bash", `{"command":"rm -rf /"}`)}},
	}}
	r := &Runner{Agent: agentWith(map[string]string{"bash:git status*": "allow", "bash:rm *": "deny"}), Provider: p, Tools: tools.NewSet(echoTool("bash", &ran))}
	r.Approve = func(context.Context, PermissionRequest) Decision { t.Error("should not ask"); return Deny }
	_, _, err := r.Run(context.Background(), nil, "x", "")
	if !errors.Is(err, ErrDenied) || len(ran) != 1 {
		t.Fatalf("err=%v ran=%v", err, ran)
	}
}

func TestRunnerMaxSteps(t *testing.T) {
	var ran []string
	var turns []provider.Turn
	for i := 0; i < 5; i++ {
		turns = append(turns, provider.Turn{ToolCalls: []provider.ToolCall{call("x", "read", `{}`)}})
	}
	ra := agentWith(nil)
	ra.MaxSteps = 3
	r := &Runner{Agent: ra, Provider: &scripted{turns: turns}, Tools: tools.NewSet(echoTool("read", &ran))}
	if _, _, err := r.Run(context.Background(), nil, "x", ""); !errors.Is(err, ErrMaxSteps) || len(ran) != 3 {
		t.Fatalf("err=%v ran=%d", err, len(ran))
	}
}

func TestUnknownToolIsReportedToTheModel(t *testing.T) {
	p := &scripted{turns: []provider.Turn{{ToolCalls: []provider.ToolCall{call("1", "nope", `{}`)}}, {Text: "sorry"}}}
	r := &Runner{Agent: agentWith(nil), Provider: p, Tools: tools.NewSet()}
	added, _, err := r.Run(context.Background(), nil, "x", "")
	if err != nil || !added[2].IsError || !strings.Contains(added[2].Content, "unknown tool") {
		t.Fatalf("%v %+v", err, added)
	}
}

func TestFitHistoryCutsAtUserTurns(t *testing.T) {
	big := strings.Repeat("x", 100)
	msgs := []provider.Message{
		{Role: "user", Content: big},
		{Role: "assistant", ToolCalls: []provider.ToolCall{call("1", "read", `{}`)}},
		{Role: "tool", ToolCallID: "1", Content: big},
		{Role: "user", Content: "latest"},
	}
	got := FitHistory(msgs, 50)
	if len(got) != 1 || got[0].Content != "latest" {
		t.Fatalf("got %+v", got)
	}
	if len(FitHistory(msgs, 10_000)) != 4 {
		t.Fatal("history that fits must be kept whole")
	}
}

func TestToolCallWrittenAsTextIsRecognized(t *testing.T) {
	var ran []string
	set := tools.NewSet(echoTool("read", &ran))
	for _, text := range []string{
		`{"name": "read", "arguments": {"path": "a.txt"}}`,
		"```json\n{\"name\":\"read\",\"arguments\":{\"path\":\"a.txt\"}}\n```",
		`<tool_call>{"name":"read","parameters":{"path":"a.txt"}}</tool_call>`,
	} {
		c, ok := textToolCall(text, set)
		if !ok || c.Name != "read" || !strings.Contains(string(c.Args), "a.txt") {
			t.Errorf("%q: %+v %v", text, c, ok)
		}
	}
	for _, text := range []string{
		`{"name": "my-agent", "model": "qwen"}`, // not a tool
		`Use {"name":"read"} to read files`,     // prose around it
		`the answer is 42`,
	} {
		if _, ok := textToolCall(text, set); ok {
			t.Errorf("%q must stay text", text)
		}
	}
	// in a run, the text call is executed and the reply continues
	p := &scripted{turns: []provider.Turn{{Text: `{"name": "read", "arguments": {}}`}, {Text: "done reading"}}}
	r := &Runner{Agent: agentWith(nil), Provider: p, Tools: set}
	added, _, err := r.Run(context.Background(), nil, "x", "")
	if err != nil || len(ran) != 1 || added[len(added)-1].Content != "done reading" || added[1].Content != "" {
		t.Fatalf("%v ran=%v %+v", err, ran, added)
	}
}

func TestSystemPromptStatesIdentity(t *testing.T) {
	ra := agentWith(nil)
	ra.Model, ra.Provider = "qwen2.5:14b", "ollama"
	s := BuildSystemPrompt(ra, &tools.Env{Dir: t.TempDir()}, nil)
	if !strings.Contains(s, "qwen2.5:14b") || !strings.Contains(s, "my-agent") || !strings.Contains(s, "never claim") {
		t.Fatalf("%s", s)
	}
}
