// Package tools holds the functions the agent can call: file access, shell,
// search, web and knowledge base, plus the database, e-mail and MCP tools
// built from the configured connectors.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"my-agent/internal/provider"
	"my-agent/internal/rag"
)

// Result is what a tool returns: Output goes back to the model; Display, when
// set, is shown to the user instead (a diff, for instance).
type Result struct {
	Output  string
	Display string
}

// Tool is one callable function.
type Tool struct {
	Name        string
	Description string
	Params      map[string]any // JSON Schema of the arguments
	// Subject is the short "what" of a call, shown in the UI and matched by
	// permission rules (the command for bash, the path for files...).
	Subject func(args map[string]any) string
	Run     func(ctx context.Context, env *Env, args map[string]any) (Result, error)
	Source  string // "builtin", a connector name or an MCP server name
}

// Spec is the definition sent to the model.
func (t *Tool) Spec() provider.Tool {
	return provider.Tool{Name: t.Name, Description: t.Description, Parameters: t.Params}
}

// Describe returns the call's subject, or "" when the tool has none.
func (t *Tool) Describe(raw json.RawMessage) string {
	if t.Subject == nil {
		return ""
	}
	return t.Subject(Args(raw))
}

// Env is what tools need from the outside world.
type Env struct {
	Dir   string   // working directory; relative paths start here
	Shell []string // command prefix of the bash tool, e.g. ["/bin/sh","-c"]
	// Index opens the knowledge base (nil: rag_search is not offered).
	Index func() (*rag.Index, error)
}

// Abs resolves p against the working directory.
func (e *Env) Abs(p string) string {
	p = expandHome(strings.TrimSpace(p))
	if p == "" {
		return e.Dir
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(e.Dir, p)
}

// Rel shows p relative to the working directory when it is inside it.
func (e *Env) Rel(p string) string {
	if r, err := filepath.Rel(e.Dir, p); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return p
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[1:])
		}
	}
	return p
}

// Set is an ordered collection of tools.
type Set struct {
	list []*Tool
	byN  map[string]*Tool
}

func NewSet(ts ...*Tool) *Set {
	s := &Set{byN: map[string]*Tool{}}
	s.Add(ts...)
	return s
}

// Add appends tools; a later tool with the same name replaces the earlier one.
func (s *Set) Add(ts ...*Tool) {
	for _, t := range ts {
		if old, ok := s.byN[t.Name]; ok {
			*old = *t
			continue
		}
		s.byN[t.Name] = t
		s.list = append(s.list, t)
	}
}

func (s *Set) Get(name string) (*Tool, bool) {
	if s == nil {
		return nil, false
	}
	t, ok := s.byN[name]
	return t, ok
}

func (s *Set) List() []*Tool {
	if s == nil {
		return nil
	}
	return s.list
}

func (s *Set) Len() int { return len(s.List()) }

// Filter keeps the tools for which keep returns true.
func (s *Set) Filter(keep func(*Tool) bool) *Set {
	out := NewSet()
	for _, t := range s.List() {
		if keep(t) {
			out.Add(t)
		}
	}
	return out
}

// Specs returns the definitions sent to the model.
func (s *Set) Specs() []provider.Tool {
	out := make([]provider.Tool, 0, s.Len())
	for _, t := range s.List() {
		out = append(out, t.Spec())
	}
	return out
}

// Names returns the tool names, sorted.
func (s *Set) Names() []string {
	out := make([]string, 0, s.Len())
	for _, t := range s.List() {
		out = append(out, t.Name)
	}
	sort.Strings(out)
	return out
}

// Args decodes a call's arguments; invalid JSON gives an empty map.
func Args(raw json.RawMessage) map[string]any {
	m := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

func str(args map[string]any, k string) string {
	switch v := args[k].(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func num(args map[string]any, k string, def int) int {
	switch v := args[k].(type) {
	case float64:
		return int(v)
	case string:
		var n int
		if _, err := fmt.Sscan(v, &n); err == nil {
			return n
		}
	}
	return def
}

func boolean(args map[string]any, k string) bool {
	switch v := args[k].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

// schema builds an object schema. props maps a name to "type:description";
// names ending in "?" are optional.
func schema(props ...string) map[string]any {
	p := map[string]any{}
	var req []string
	for i := 0; i+1 < len(props); i += 2 {
		name, spec := props[i], props[i+1]
		optional := strings.HasSuffix(name, "?")
		name = strings.TrimSuffix(name, "?")
		typ, desc, _ := strings.Cut(spec, ":")
		p[name] = map[string]any{"type": typ, "description": desc}
		if !optional {
			req = append(req, name)
		}
	}
	s := map[string]any{"type": "object", "properties": p}
	if len(req) > 0 {
		s["required"] = req
	}
	return s
}

// Truncate keeps at most n bytes, cutting at a line boundary and saying how
// much was left out.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := s[:n]
	if i := strings.LastIndexByte(cut, '\n'); i > n/2 {
		cut = cut[:i]
	}
	return cut + fmt.Sprintf("\n... [truncated: %d more bytes]", len(s)-len(cut))
}
