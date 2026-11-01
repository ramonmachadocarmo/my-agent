package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"my-agent/internal/config"
	"my-agent/internal/i18n"
	"my-agent/internal/provider"
	"my-agent/internal/rag"
	"my-agent/internal/tools"
)

// Decision is the user's answer to a permission request.
type Decision int

const (
	Deny   Decision = iota
	Once            // allow this call
	Always          // allow this tool (or this command) for the rest of the session
)

// PermissionRequest asks the user whether a tool call may run.
type PermissionRequest struct {
	Tool    *tools.Tool
	Call    provider.ToolCall
	Subject string
}

// EventKind tells what happened during a run.
type EventKind int

const (
	EvText      EventKind = iota // a piece of the reply
	EvToolStart                  // a tool is about to run
	EvToolEnd                    // a tool finished (Result or Err)
	EvUsage                      // token usage of a model call
	EvStep                       // a model call finished; another one follows
)

// Event is one step of a run, reported to the UI.
type Event struct {
	Kind    EventKind
	Text    string
	Call    provider.ToolCall
	Subject string
	Result  tools.Result
	Err     error
	Denied  bool
	Usage   provider.Usage
}

// ErrDenied ends a run after the user refused a tool call.
var ErrDenied = i18n.Err("agent.err.denied")

// ErrMaxSteps ends a run that used all of its steps.
var ErrMaxSteps = i18n.Err("agent.err.max_steps")

// MaxContextChars caps the conversation sent to the model; older turns are
// dropped first (about 100k tokens).
var MaxContextChars = 400_000

// Runner is an agent with tools: it calls the model, runs the tools it asks
// for (after checking permissions) and repeats until the model answers.
type Runner struct {
	Agent    config.ResolvedAgent
	Provider provider.Provider
	Tools    *tools.Set
	Env      *tools.Env
	System   string     // complete system prompt (see BuildSystemPrompt)
	Index    *rag.Index // automatic retrieval before each message; nil = off

	// Approve is asked for tools whose permission is "ask". nil denies them.
	Approve func(ctx context.Context, req PermissionRequest) Decision
	// OnEvent receives the progress of a run. It may be nil.
	OnEvent func(Event)

	mu      sync.Mutex
	granted map[string]bool // "always" answers of this session
}

func (r *Runner) emit(e Event) {
	if r.OnEvent != nil {
		r.OnEvent(e)
	}
}

// Grants returns what the user allowed for the whole session, so that a
// rebuilt runner (another agent or model) keeps them.
func (r *Runner) Grants() map[string]bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]bool{}
	for k, v := range r.granted {
		out[k] = v
	}
	return out
}

// SetGrants restores the answers returned by Grants.
func (r *Runner) SetGrants(g map[string]bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.granted = g
}

func grantKey(tool, subject string) string {
	if tool == "bash" {
		// "always" for a command allows commands with the same program and
		// first argument (e.g. every "go test ...")
		f := strings.Fields(subject)
		if len(f) > 2 {
			f = f[:2]
		}
		return "bash:" + strings.Join(f, " ")
	}
	return tool
}

// Run sends input and works until the model gives a final answer. It returns
// the messages added to the conversation (the user's message, the model's
// turns and the tool results), even when it fails part way: they always form
// a valid history. query is searched in the knowledge base ("" = input).
func (r *Runner) Run(ctx context.Context, history []provider.Message, input, query string) ([]provider.Message, provider.Usage, error) {
	var total provider.Usage
	system := r.System
	if r.Index != nil && r.Agent.UseRAG {
		if query == "" {
			query = input
		}
		hits, err := r.Index.Search(ctx, query)
		if err != nil {
			return nil, total, i18n.Errorf("agent.err.rag_search", err)
		}
		system = BuildSystem(system, hits)
	}
	added := []provider.Message{{Role: "user", Content: input}}
	var specs []provider.Tool
	if r.Tools != nil {
		specs = r.Tools.Specs()
	}
	steps := r.Agent.MaxSteps
	if steps <= 0 {
		steps = 25
	}
	for step := 0; step < steps; step++ {
		msgs := FitHistory(append(append([]provider.Message{}, history...), added...), MaxContextChars)
		turn, err := provider.Complete(ctx, r.Provider, provider.ChatRequest{
			System:      system,
			Messages:    msgs,
			Temperature: r.Agent.Temperature,
			MaxTokens:   r.Agent.MaxTokens,
			Tools:       specs,
		}, func(d string) { r.emit(Event{Kind: EvText, Text: d}) })
		total.Input += turn.Usage.Input
		total.Output += turn.Usage.Output
		if turn.Usage != (provider.Usage{}) {
			r.emit(Event{Kind: EvUsage, Usage: turn.Usage})
		}
		if err != nil {
			if turn.Text != "" { // keep what was said before the interruption
				added = append(added, provider.Message{Role: "assistant", Content: turn.Text})
			}
			return added, total, err
		}
		if len(turn.ToolCalls) == 0 {
			if call, ok := textToolCall(turn.Text, r.Tools); ok {
				turn.Text, turn.ToolCalls = "", []provider.ToolCall{call}
			}
		}
		added = append(added, provider.Message{Role: "assistant", Content: turn.Text, ToolCalls: turn.ToolCalls})
		if len(turn.ToolCalls) == 0 {
			return added, total, nil
		}
		results, err := r.runCalls(ctx, turn.ToolCalls)
		added = append(added, results...)
		if err != nil {
			return added, total, err
		}
		r.emit(Event{Kind: EvStep})
	}
	return added, total, ErrMaxSteps
}

// runCalls runs the tool calls of one turn in order. It always returns one
// result per call; after a denial or an interruption the remaining calls are
// answered without running.
func (r *Runner) runCalls(ctx context.Context, calls []provider.ToolCall) ([]provider.Message, error) {
	var out []provider.Message
	var stop error
	for _, call := range calls {
		res := provider.Message{Role: "tool", ToolCallID: call.ID, Name: call.Name}
		if stop != nil {
			res.Content, res.IsError = "skipped: "+stop.Error(), true
			out = append(out, res)
			continue
		}
		t, ok := r.Tools.Get(call.Name)
		if !ok {
			res.Content, res.IsError = fmt.Sprintf("unknown tool %q", call.Name), true
			out = append(out, res)
			continue
		}
		subject := t.Describe(call.Args)
		if !r.permitted(ctx, t, call, subject) {
			r.emit(Event{Kind: EvToolEnd, Call: call, Subject: subject, Denied: true, Err: ErrDenied})
			res.Content, res.IsError = "the user denied this tool call; stop and ask the user how to proceed", true
			out = append(out, res)
			stop = ErrDenied
			continue
		}
		r.emit(Event{Kind: EvToolStart, Call: call, Subject: subject})
		result, err := runTool(ctx, t, r.Env, call.Args)
		r.emit(Event{Kind: EvToolEnd, Call: call, Subject: subject, Result: result, Err: err})
		switch {
		case ctx.Err() != nil:
			res.Content, res.IsError = "interrupted by the user", true
			stop = ctx.Err()
		case err != nil:
			res.Content, res.IsError = "error: "+err.Error(), true
		default:
			res.Content = result.Output
		}
		out = append(out, res)
	}
	return out, stop
}

func runTool(ctx context.Context, t *tools.Tool, env *tools.Env, raw json.RawMessage) (res tools.Result, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("tool %s crashed: %v", t.Name, p) // i18n:ok (read by the model)
		}
	}()
	var args map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return tools.Result{}, fmt.Errorf("invalid arguments (not a JSON object): %w", err) // i18n:ok (read by the model)
		}
	}
	if args == nil {
		args = map[string]any{}
	}
	return t.Run(ctx, env, args)
}

func (r *Runner) permitted(ctx context.Context, t *tools.Tool, call provider.ToolCall, subject string) bool {
	command := ""
	if t.Name == "bash" {
		command = subject
	}
	switch r.Agent.PermissionFor(t.Name, command) {
	case config.Allow:
		return true
	case config.Deny:
		return false
	}
	key := grantKey(t.Name, subject)
	r.mu.Lock()
	ok := r.granted[key]
	r.mu.Unlock()
	if ok {
		return true
	}
	if r.Approve == nil {
		return false
	}
	switch r.Approve(ctx, PermissionRequest{Tool: t, Call: call, Subject: subject}) {
	case Always:
		r.mu.Lock()
		if r.granted == nil {
			r.granted = map[string]bool{}
		}
		r.granted[key] = true
		r.mu.Unlock()
		return true
	case Once:
		return true
	}
	return false
}

// FitHistory drops the oldest turns until the conversation fits in max
// characters. It only cuts before a user message, so tool calls are never
// separated from their results; the last user turn is always kept.
func FitHistory(msgs []provider.Message, max int) []provider.Message {
	size := 0
	for _, m := range msgs {
		size += msgSize(m)
	}
	start := 0
	for size > max {
		next := -1
		for i := start + 1; i < len(msgs); i++ {
			if msgs[i].Role == "user" {
				next = i
				break
			}
		}
		if next < 0 {
			break
		}
		for _, m := range msgs[start:next] {
			size -= msgSize(m)
		}
		start = next
	}
	return msgs[start:]
}

func msgSize(m provider.Message) int {
	n := len(m.Content)
	for _, c := range m.ToolCalls {
		n += len(c.Args) + len(c.Name)
	}
	return n
}

// ProjectInstructions reads the project's instruction file (AGENTS.md, or
// CLAUDE.md) from dir, if any.
func ProjectInstructions(dir string) (name, text string) {
	for _, n := range []string{"AGENTS.md", "CLAUDE.md"} {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err == nil && len(b) > 0 {
			return n, tools.Truncate(string(b), 20<<10)
		}
	}
	return "", ""
}

// BuildSystemPrompt assembles the system prompt of an agent with tools: the
// agent's prompt and instructions, the environment and the project's
// instruction file.
func BuildSystemPrompt(a config.ResolvedAgent, env *tools.Env, set *tools.Set) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(a.SystemPrompt))
	// A custom system prompt replaces the one many local models ship with
	// ("You are Qwen..."); without an identity they tend to claim to be
	// another assistant. Say who they are and which model runs them.
	if id := identity(a); id != "" {
		sb.WriteString("\n\n" + id)
	}
	if set != nil && set.Len() > 0 {
		sb.WriteString("\n\nYou are working in the user's terminal as an agent with tools. " +
			"Use the tools to inspect and change things instead of guessing: read files before editing them, " +
			"prefer the edit tool for small changes, verify your work (build, tests) when you can, and keep answers short. " +
			"Paths may be relative to the working directory. When a task is done, summarize what changed.")
		shell := ""
		if len(env.Shell) > 0 {
			shell = filepath.Base(env.Shell[0])
		}
		fmt.Fprintf(&sb, "\n\n<env>\nWorking directory: %s\nPlatform: %s\nShell (bash tool): %s\nDate: %s\nTools: %s\n</env>", // i18n:ok (read by the model)
			env.Dir, runtime.GOOS, shell, time.Now().Format("2006-01-02"), strings.Join(set.Names(), ", "))
	}
	if a.Instructions != "" {
		sb.WriteString("\n\n" + a.Instructions)
	}
	if name, text := ProjectInstructions(env.Dir); text != "" {
		fmt.Fprintf(&sb, "\n\nProject instructions (%s):\n%s", name, text) // i18n:ok (read by the model)
	}
	return sb.String()
}

// textToolCall recognizes a tool call that a model wrote as text instead of
// using the tool-calling API, which small local models sometimes do: the
// whole reply is {"name": ..., "arguments": {...}}, possibly inside a code
// fence or <tool_call> tags, and names a tool the agent has.
func textToolCall(text string, set *tools.Set) (provider.ToolCall, bool) {
	s := strings.TrimSpace(text)
	s = strings.TrimSuffix(strings.TrimPrefix(s, "<tool_call>"), "</tool_call>")
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
		s = strings.TrimSpace(s)
	}
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return provider.ToolCall{}, false
	}
	var v struct {
		Name       string          `json:"name"`
		Arguments  json.RawMessage `json:"arguments"`
		Parameters json.RawMessage `json:"parameters"`
	}
	if json.Unmarshal([]byte(s), &v) != nil || v.Name == "" {
		return provider.ToolCall{}, false
	}
	if _, ok := set.Get(v.Name); !ok {
		return provider.ToolCall{}, false
	}
	args := v.Arguments
	if len(args) == 0 {
		args = v.Parameters
	}
	if len(args) == 0 || string(args) == "null" {
		args = json.RawMessage("{}")
	}
	return provider.ToolCall{ID: fmt.Sprintf("text_%d", time.Now().UnixNano()), Name: v.Name, Args: args}, true
}

// identity tells the model its name, agent, model and provider.
func identity(a config.ResolvedAgent) string {
	name := strings.TrimSpace(a.AgentConfig.Name)
	if name == "" {
		name = "my-agent"
	}
	s := fmt.Sprintf("Your name is %s, and you are its %s agent.", name, a.Name) // i18n:ok (read by the model)
	if a.Model != "" {
		s += fmt.Sprintf(" You run on the language model %s, served by %s.", a.Model, a.Provider) // i18n:ok (read by the model)
	}
	return s + " If asked who you are or which model you use, say so in plain words; never claim to be a different assistant, model or company."
}

// IsInterrupted reports whether err comes from a cancelled context.
func IsInterrupted(err error) bool {
	return errors.Is(err, context.Canceled)
}
