package app

import (
	"context"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"my-agent/internal/agent"
	"my-agent/internal/config"
	"my-agent/internal/connectors"
	"my-agent/internal/mcp"
	"my-agent/internal/provider"
	"my-agent/internal/rag"
	"my-agent/internal/session"
	"my-agent/internal/tools"
)

// Workspace is the environment of the interactive agent: the configuration,
// the working directory, the MCP servers and the saved sessions.
type Workspace struct {
	App      *App
	Dir      string
	MCP      *mcp.Manager
	Sessions session.Store
	Version  string
}

// Workspace starts the MCP servers in the background and returns the
// environment for dir.
func (a *App) Workspace(ctx context.Context, dir, version string) *Workspace {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	w := &Workspace{App: a, Dir: dir, MCP: mcp.NewManager(), Sessions: session.Store{Dir: filepath.Join(config.DataDir(), "sessions")}, Version: version}
	w.MCP.Start(ctx, a.Cfg.MCP, version)
	return w
}

// Close stops the MCP servers and closes the databases.
func (w *Workspace) Close() {
	w.MCP.Close()
	connectors.CloseDBs()
}

// ProviderFor builds a provider, optionally with another model.
func (a *App) ProviderFor(name, model string) (provider.Provider, error) {
	if name == "" {
		name = a.Cfg.Agent.Provider
	}
	pc, ok := a.Cfg.Providers[name]
	if !ok {
		p, err := a.Provider(name) // reports the unknown provider
		return p, err
	}
	if model != "" {
		pc.Model = model
	}
	return a.NewProvider(name, pc)
}

// index opens the knowledge base when it has content.
func (a *App) nonEmptyIndex() *rag.Index {
	ix, err := a.Index()
	if err != nil || ix.Store.Len() == 0 {
		return nil
	}
	return ix
}

// Tools returns the tools an agent may use and their environment.
func (w *Workspace) Tools(ra config.ResolvedAgent) (*tools.Set, *tools.Env) {
	env := &tools.Env{Dir: w.Dir, Shell: w.App.Cfg.ShellCommand()}
	if ix := w.App.nonEmptyIndex(); ix != nil {
		env.Index = func() (*rag.Index, error) { return ix, nil }
	}
	all := tools.NewSet(tools.Builtins(env)...)
	all.Add(tools.ConnectorTools(w.App.Cfg.Connectors)...)
	all.Add(w.MCP.Tools()...)
	// a denied tool is not offered at all, except bash: "bash:<command>"
	// rules may still allow some commands, and each call is checked anyway
	set := all.Filter(func(t *tools.Tool) bool {
		return ra.Enabled(t.Name) && (t.Name == "bash" || ra.PermissionFor(t.Name, "") != config.Deny)
	})
	return set, env
}

// Selection is what the user picked in the interface; empty fields use the
// agent's own settings.
type Selection struct {
	Agent, Provider, Model string
}

// Runner builds the runner of an agent.
func (w *Workspace) Runner(sel Selection) (*agent.Runner, config.ResolvedAgent, error) {
	ra, err := w.App.Cfg.ResolveAgent(sel.Agent)
	if err != nil {
		return nil, ra, err
	}
	if sel.Provider != "" {
		ra.Provider, ra.Model = sel.Provider, sel.Model
	} else if sel.Model != "" {
		ra.Model = sel.Model
	}
	if ra.Model == "" {
		ra.Model = w.App.Cfg.Providers[ra.Provider].Model
	}
	p, err := w.App.ProviderFor(ra.Provider, ra.Model)
	if err != nil {
		return nil, ra, err
	}
	set, env := w.Tools(ra)
	r := &agent.Runner{Agent: ra, Provider: p, Tools: set, Env: env, System: agent.BuildSystemPrompt(ra, env, set)}
	if ra.UseRAG {
		r.Index = w.App.nonEmptyIndex()
	}
	return r, ra, nil
}

// ModelChoice is a provider/model pair offered in the model picker.
type ModelChoice struct {
	Provider, Model string
	Current         bool // the provider's configured model
}

// ModelChoices lists, for every provider, its configured model, its
// "models" list and whatever the provider reports (each provider is asked
// with a short timeout, in parallel).
func (a *App) ModelChoices(ctx context.Context) []ModelChoice {
	names := make([]string, 0, len(a.Cfg.Providers))
	for n := range a.Cfg.Providers {
		names = append(names, n)
	}
	sort.Strings(names)
	listed := make([][]string, len(names))
	var wg sync.WaitGroup
	for i, n := range names {
		wg.Add(1)
		go func(i int, n string) {
			defer wg.Done()
			p, err := a.Provider(n)
			if err != nil {
				return
			}
			if l, ok := p.(provider.ModelLister); ok {
				c, cancel := context.WithTimeout(ctx, 6*time.Second)
				defer cancel()
				if ms, err := l.Models(c); err == nil {
					listed[i] = ms
				}
			}
		}(i, n)
	}
	wg.Wait()
	var out []ModelChoice
	for i, n := range names {
		pc := a.Cfg.Providers[n]
		seen := map[string]bool{}
		add := func(m string, cur bool) {
			if m != "" && !seen[m] {
				seen[m] = true
				out = append(out, ModelChoice{Provider: n, Model: m, Current: cur})
			}
		}
		add(pc.Model, true)
		for _, m := range pc.Models {
			add(m, false)
		}
		for _, m := range listed[i] {
			add(m, false)
		}
	}
	return out
}
