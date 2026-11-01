package mcp

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"

	"my-agent/internal/config"
	"my-agent/internal/tools"
)

// Status describes one configured server.
type Status struct {
	Name     string
	Type     string
	Disabled bool
	Pending  bool
	Err      error
	Tools    int
	Server   string
}

// Manager owns the connections to every configured server.
type Manager struct {
	mu      sync.Mutex
	clients map[string]*Client
	status  map[string]Status
	wg      sync.WaitGroup
}

func NewManager() *Manager {
	return &Manager{clients: map[string]*Client{}, status: map[string]Status{}}
}

// Start connects to every enabled server in the background. Tools become
// available as each server answers; Wait blocks until all have.
func (m *Manager) Start(ctx context.Context, cfgs map[string]config.MCPConfig, version string) {
	for name, cfg := range cfgs {
		st := Status{Name: name, Type: cfg.Type, Disabled: !cfg.IsEnabled(), Pending: cfg.IsEnabled()}
		m.mu.Lock()
		m.status[name] = st
		m.mu.Unlock()
		if st.Disabled {
			continue
		}
		m.wg.Add(1)
		go func(name string, cfg config.MCPConfig) {
			defer m.wg.Done()
			c, err := Connect(ctx, name, cfg, version)
			m.mu.Lock()
			defer m.mu.Unlock()
			st := m.status[name]
			st.Pending, st.Err = false, err
			if err == nil {
				m.clients[name] = c
				st.Tools, st.Server = len(c.Tools), c.Server
			}
			m.status[name] = st
		}(name, cfg)
	}
}

// Wait blocks until every server has connected or failed.
func (m *Manager) Wait() { m.wg.Wait() }

// Statuses returns the state of every server, sorted by name.
func (m *Manager) Statuses() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Status, 0, len(m.status))
	for _, s := range m.status {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

var unsafe = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// ToolName is the name under which a server's tool is offered to the model:
// mcp_<server>_<tool>, limited to the characters and length every provider
// accepts.
func ToolName(server, tool string) string {
	n := "mcp_" + unsafe.ReplaceAllString(server, "_") + "_" + unsafe.ReplaceAllString(tool, "_")
	if len(n) > 64 {
		n = n[:64]
	}
	return n
}

// Tools returns the tools of every connected server.
func (m *Manager) Tools() []*tools.Tool {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.clients))
	for n := range m.clients {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []*tools.Tool
	for _, n := range names {
		c := m.clients[n]
		for _, ti := range c.Tools {
			c, ti := c, ti
			params := ti.InputSchema
			if params == nil {
				params = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			delete(params, "$schema")
			out = append(out, &tools.Tool{
				Name:        ToolName(c.Name, ti.Name),
				Description: strings.TrimSpace("[" + c.Name + "] " + ti.Description),
				Params:      params,
				Source:      c.Name,
				Subject: func(args map[string]any) string {
					return c.Name + "/" + ti.Name
				},
				Run: func(ctx context.Context, _ *tools.Env, args map[string]any) (tools.Result, error) {
					text, isErr, err := c.Call(ctx, ti.Name, args)
					if err != nil {
						return tools.Result{}, err
					}
					if isErr {
						return tools.Result{}, toolError(text)
					}
					return tools.Result{Output: tools.Truncate(text, 100<<10)}, nil
				},
			})
		}
	}
	return out
}

type toolError string

func (e toolError) Error() string { return string(e) }

// Close disconnects every server.
func (m *Manager) Close() {
	m.wg.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clients {
		c.Close()
	}
	m.clients = map[string]*Client{}
}
