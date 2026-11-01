package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/i18n"
	"my-agent/internal/mcp"
)

type command struct {
	name     string
	args     string // argument hint; optional arguments are in [brackets]
	desc     string // catalog key
	run      func(m *model, args string) tea.Cmd
	complete func(m *model) []string // values offered for the argument
}

func (m *model) commands() []command {
	return []command{
		{name: "help", desc: "tui.cmd.help", run: (*model).cmdHelp},
		{name: "new", desc: "tui.cmd.new", run: func(m *model, _ string) tea.Cmd { m.newSession(); return nil }},
		{name: "sessions", desc: "tui.cmd.sessions", run: func(m *model, _ string) tea.Cmd { m.openSessionPicker(); return nil }},
		{name: "models", args: "[provider/model]", desc: "tui.cmd.models", run: (*model).cmdModels},
		{name: "agents", args: "[name]", desc: "tui.cmd.agents", run: (*model).cmdAgents, complete: func(m *model) []string { return m.ws.App.Cfg.AgentNames() }},
		{name: "connect", args: "[preset]", desc: "tui.cmd.connect", run: (*model).cmdConnect, complete: func(*model) []string { return config.PresetNames() }},
		{name: "keys", args: "[provider]", desc: "tui.cmd.keys", run: (*model).cmdKeys, complete: (*model).providerNames},
		{name: "tools", desc: "tui.cmd.tools", run: (*model).cmdTools},
		{name: "mcp", desc: "tui.cmd.mcp", run: (*model).cmdMCP},
		{name: "connectors", desc: "tui.cmd.connectors", run: (*model).cmdConnectors},
		{name: "sync", args: "<connector|all>", desc: "tui.cmd.sync", run: (*model).cmdSync, complete: (*model).connectorNames},
		{name: "rag", desc: "tui.cmd.rag", run: (*model).cmdRAG},
		{name: "ingest", args: "<path|url>", desc: "tui.cmd.ingest", run: (*model).cmdIngest},
		{name: "config", args: "[get KEY | set KEY VALUE]", desc: "tui.cmd.config", run: (*model).cmdConfig},
		{name: "reload", desc: "tui.cmd.reload", run: func(m *model, _ string) tea.Cmd { return m.reload() }},
		{name: "export", args: "[file]", desc: "tui.cmd.export", run: (*model).cmdExport},
		{name: "exit", desc: "tui.cmd.exit", run: func(m *model, _ string) tea.Cmd { return m.quit() }},
	}
}

func (m *model) providerNames() []string {
	out := make([]string, 0, len(m.ws.App.Cfg.Providers))
	for n := range m.ws.App.Cfg.Providers {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func (m *model) connectorNames() []string {
	out := []string{"all"}
	for _, c := range m.ws.App.Cfg.Connectors {
		out = append(out, c.Name)
	}
	return out
}

func (m *model) findCommand(name string) (command, bool) {
	var hits []command
	for _, c := range m.commands() {
		if c.name == name {
			return c, true
		}
		if strings.HasPrefix(c.name, name) {
			hits = append(hits, c)
		}
	}
	switch name {
	case "quit", "q":
		return m.findCommand("exit")
	case "agent":
		return m.findCommand("agents")
	case "model":
		return m.findCommand("models")
	case "clear":
		return m.findCommand("new")
	}
	if len(hits) == 1 {
		return hits[0], true
	}
	return command{}, false
}

func (m *model) commandTakesArgs(line string) bool {
	name := strings.TrimPrefix(strings.Fields(line + " ")[0], "/")
	c, ok := m.findCommand(name)
	return ok && c.args != "" && !strings.HasPrefix(c.args, "[")
}

func (m *model) commandSuggestions(v string) []suggestion {
	name, arg, hasArg := strings.Cut(strings.TrimPrefix(v, "/"), " ")
	var out []suggestion
	if !hasArg {
		for _, c := range m.commands() {
			if strings.HasPrefix(c.name, name) {
				out = append(out, suggestion{text: "/" + c.name, label: "/" + c.name + " " + c.args, desc: i18n.T(c.desc)})
			}
		}
		return out
	}
	c, ok := m.findCommand(name)
	if !ok || c.complete == nil || strings.Contains(arg, " ") {
		return nil
	}
	for _, val := range c.complete(m) {
		if strings.HasPrefix(val, arg) {
			out = append(out, suggestion{text: "/" + c.name + " " + val, label: val})
		}
	}
	return out
}

func (m *model) runCommand(line string) tea.Cmd {
	name, args, _ := strings.Cut(strings.TrimPrefix(line, "/"), " ")
	c, ok := m.findCommand(strings.ToLower(name))
	if !ok {
		m.addItem(&item{kind: kindError, text: i18n.T("tui.unknown_command", name)})
		return nil
	}
	return c.run(m, strings.TrimSpace(args))
}

func (m *model) info(text string) { m.addItem(&item{kind: kindInfo, text: text}) }

func (m *model) fail(err error) { m.addItem(&item{kind: kindError, text: err.Error()}) }

// background runs slow work (sync, ingest) without freezing the interface.
func (m *model) background(label string, fn func(ctx context.Context) (string, error)) tea.Cmd {
	if m.busy {
		m.setNotice(i18n.T("tui.busy"), true)
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.busy, m.busyText, m.cancel = true, label, cancel
	return func() tea.Msg {
		defer cancel()
		text, err := fn(ctx)
		return bgDoneMsg{text: text, err: err}
	}
}

func (m *model) cmdHelp(string) tea.Cmd {
	var sb strings.Builder
	sb.WriteString(i18n.T("tui.help_commands") + "\n")
	for _, c := range m.commands() {
		fmt.Fprintf(&sb, "  /%-12s %-24s %s\n", c.name, c.args, i18n.T(c.desc))
	}
	sb.WriteString("\n" + i18n.T("tui.help_keys") + "\n")
	for _, k := range [][2]string{
		{"enter", "tui.k.send"}, {"alt+enter, ctrl+j", "tui.k.newline"}, {"tab / shift+tab", "tui.k.agents"},
		{"ctrl+o", "tui.k.models"}, {"ctrl+p", "tui.k.commands"}, {"ctrl+s", "tui.k.sessions"}, {"ctrl+n", "tui.k.new"},
		{"↑ / ↓", "tui.k.history"}, {"pgup / pgdn", "tui.k.scroll"}, {"esc", "tui.k.stop"}, {"ctrl+c", "tui.k.quit"},
	} {
		fmt.Fprintf(&sb, "  %-20s %s\n", k[0], i18n.T(k[1]))
	}
	m.info(strings.TrimRight(sb.String(), "\n"))
	return nil
}

// ---- models ----

func (m *model) loadModels() tea.Cmd {
	m.setNotice(i18n.T("tui.loading_models"), false)
	a := m.ws.App
	ctx := m.ctx
	return func() tea.Msg { return modelsMsg{a.ModelChoices(ctx)} }
}

func (m *model) cmdModels(args string) tea.Cmd {
	if args == "" {
		return m.loadModels()
	}
	prov, mod, ok := strings.Cut(args, "/")
	if _, exists := m.ws.App.Cfg.Providers[prov]; !ok || !exists {
		prov, mod = m.ra.Provider, args
		if m.sel.Provider != "" {
			prov = m.sel.Provider
		}
	}
	m.selectModel(prov, mod)
	return nil
}

func (m *model) selectModel(prov, mod string) {
	m.sel.Provider, m.sel.Model = prov, mod
	m.setNotice(i18n.T("tui.model_selected", prov+"/"+mod), false)
}

func (m *model) openModelPicker(choices []app.ModelChoice) {
	m.notice = ""
	prov, mod := m.provModel()
	var items []pickItem
	for _, c := range choices {
		desc := ""
		if c.Current {
			desc = i18n.T("tui.default_model")
		}
		items = append(items, pickItem{title: c.Model, desc: desc, group: c.Provider, value: c.Provider + "/" + c.Model, current: c.Provider == prov && c.Model == mod})
	}
	m.dialog = newPicker(m.th, i18n.T("tui.pick_model"), items, func(m *model, it pickItem) tea.Cmd {
		p, md, _ := strings.Cut(it.value, "/")
		m.selectModel(p, md)
		return nil
	})
}

// ---- agents ----

func (m *model) cmdAgents(args string) tea.Cmd {
	if args != "" {
		m.switchAgent(args)
		return nil
	}
	var items []pickItem
	for _, n := range m.ws.App.Cfg.AgentNames() {
		ra, err := m.ws.App.Cfg.ResolveAgent(n)
		if err != nil {
			continue
		}
		desc := ra.Description
		if ra.Model != "" {
			desc += "  [" + ra.Provider + "/" + ra.Model + "]"
		}
		items = append(items, pickItem{title: n, desc: desc, value: n, current: n == m.ra.Name})
	}
	m.dialog = newPicker(m.th, i18n.T("tui.pick_agent"), items, func(m *model, it pickItem) tea.Cmd {
		m.switchAgent(it.value)
		return nil
	})
	return nil
}

// ---- sessions ----

func (m *model) openSessionPicker() {
	list, err := m.ws.Sessions.List(m.ws.Dir)
	if err != nil {
		m.fail(err)
		return
	}
	var items []pickItem
	for _, s := range list {
		group := i18n.T("tui.sessions_here")
		if s.Dir != m.ws.Dir {
			group = i18n.T("tui.sessions_other")
		}
		title := s.Title
		if title == "" {
			title = s.ID
		}
		desc := s.Updated.Format("2006-01-02 15:04") + " · " + s.Agent + " · " + i18n.N("tui.messages", s.Messages)
		if s.Dir != m.ws.Dir {
			desc += " · " + shortDir(s.Dir)
		}
		items = append(items, pickItem{title: title, desc: desc, group: group, value: s.ID, current: s.ID == m.sess.ID})
	}
	p := newPicker(m.th, i18n.T("tui.pick_session"), items, func(m *model, it pickItem) tea.Cmd {
		if m.busy {
			m.setNotice(i18n.T("tui.busy"), true)
			return nil
		}
		m.resume(it.value)
		return nil
	})
	p.onDelete = func(m *model, it pickItem) {
		if err := m.ws.Sessions.Delete(it.value); err != nil {
			m.setNotice(err.Error(), true)
		}
	}
	m.dialog = p
}

// ---- providers and keys ----

func (m *model) cmdConnect(args string) tea.Cmd {
	if args != "" {
		m.connectPreset(args)
		return nil
	}
	var items []pickItem
	for _, n := range config.PresetNames() {
		p := config.Presets[n]
		desc := p.BaseURL
		if _, ok := m.ws.App.Cfg.Providers[n]; ok {
			desc = i18n.T("tui.configured") + " · " + desc
		}
		items = append(items, pickItem{title: n, desc: desc, value: n})
	}
	m.dialog = newPicker(m.th, i18n.T("tui.pick_preset"), items, func(m *model, it pickItem) tea.Cmd {
		m.connectPreset(it.value)
		return nil
	})
	return nil
}

func (m *model) connectPreset(name string) {
	preset, ok := config.Presets[name]
	if !ok {
		m.fail(i18n.Errorf("tui.err.unknown_preset", name, strings.Join(config.PresetNames(), ", ")))
		return
	}
	if preset.KeyEnv == "" { // local servers need no key
		m.saveProviderKey(name, "")
		return
	}
	hint := i18n.T("tui.key_hint", preset.KeyEnv)
	m.prompt = newPrompt(m.th, i18n.T("tui.key_title", name), hint, true, func(m *model, v string) tea.Cmd {
		m.saveProviderKey(name, v)
		return nil
	})
}

// saveProviderKey adds the provider (from its preset) when missing and adds
// the key to it: the first key goes to api_key, the next ones to api_keys.
func (m *model) saveProviderKey(name, key string) {
	cfg := m.ws.App.Cfg
	pc, exists := cfg.Providers[name]
	if !exists {
		var ok bool
		if pc, ok = config.ProviderFromPreset(name); !ok {
			m.fail(i18n.Errorf("app.err.unknown_provider", name))
			return
		}
	}
	if key != "" {
		switch {
		case containsKey(pc, key):
		case pc.APIKey == "":
			pc.APIKey = key
		default:
			pc.APIKeys = append(pc.APIKeys, key)
		}
	}
	providers := map[string]config.ProviderConfig{}
	for k, v := range cfg.Providers {
		providers[k] = v
	}
	providers[name] = pc
	cfg.Providers = providers
	if err := cfg.Validate(); err != nil {
		m.fail(err)
		return
	}
	m.ws.App.Cfg = cfg
	if err := m.ws.App.Save(); err != nil {
		m.fail(err)
		return
	}
	n := len(pc.ResolveAPIKeys())
	m.info(i18n.T("tui.provider_saved", name, n))
	m.selectModel(name, pc.Model)
}

func containsKey(pc config.ProviderConfig, key string) bool {
	if pc.APIKey == key {
		return true
	}
	for _, k := range pc.APIKeys {
		if k == key {
			return true
		}
	}
	return false
}

func (m *model) cmdKeys(args string) tea.Cmd {
	if args != "" {
		if _, ok := m.ws.App.Cfg.Providers[args]; !ok {
			m.fail(i18n.Errorf("app.err.unknown_provider", args))
			return nil
		}
		m.prompt = newPrompt(m.th, i18n.T("tui.key_title", args), i18n.T("tui.key_add_hint"), true, func(m *model, v string) tea.Cmd {
			if v != "" {
				m.saveProviderKey(args, v)
			}
			return nil
		})
		return nil
	}
	var sb strings.Builder
	sb.WriteString(i18n.T("tui.keys_title") + "\n")
	for _, n := range m.providerNames() {
		pc := m.ws.App.Cfg.Providers[n]
		state := i18n.N("tui.keys_count", len(pc.ResolveAPIKeys()))
		if !pc.HasKey() {
			state = i18n.T("tui.keys_none")
		} else if len(pc.ResolveAPIKeys()) == 0 {
			state = i18n.T("tui.keys_local")
		}
		fmt.Fprintf(&sb, "  %-12s %-8s %-28s %s\n", n, pc.Type, pc.Model, state)
	}
	sb.WriteString(i18n.T("tui.keys_help"))
	m.info(sb.String())
	return nil
}

// ---- tools, MCP, connectors, RAG ----

func (m *model) cmdTools(string) tea.Cmd {
	prov, mod := m.provModel()
	r, ra, err := m.ws.Runner(app.Selection{Agent: m.ra.Name, Provider: prov, Model: mod})
	if err != nil {
		m.fail(err)
		return nil
	}
	var sb strings.Builder
	sb.WriteString(i18n.T("tui.tools_title", ra.Name) + "\n")
	for _, t := range r.Tools.List() {
		cmd := ""
		perm := ra.PermissionFor(t.Name, cmd)
		src := ""
		if t.Source != "builtin" {
			src = "[" + t.Source + "]"
		}
		fmt.Fprintf(&sb, "  %-28s %-6s %s\n", t.Name, perm, src)
	}
	if r.Tools.Len() == 0 {
		sb.WriteString("  " + i18n.T("tui.no_tools") + "\n")
	}
	m.info(strings.TrimRight(sb.String(), "\n"))
	return nil
}

func (m *model) cmdMCP(string) tea.Cmd {
	st := m.ws.MCP.Statuses()
	if len(st) == 0 {
		m.info(i18n.T("tui.mcp_none"))
		return nil
	}
	var sb strings.Builder
	sb.WriteString(i18n.T("tui.mcp_title") + "\n")
	for _, s := range st {
		var state string
		switch {
		case s.Disabled:
			state = i18n.T("tui.mcp_disabled")
		case s.Pending:
			state = i18n.T("tui.mcp_pending")
		case s.Err != nil:
			state = "✗ " + s.Err.Error()
		default:
			state = "✓ " + i18n.N("tui.mcp_tools", s.Tools) + "  " + s.Server
		}
		fmt.Fprintf(&sb, "  %-16s %-7s %s\n", s.Name, s.Type, state)
	}
	m.info(strings.TrimRight(sb.String(), "\n"))
	return nil
}

func (m *model) cmdConnectors(string) tea.Cmd {
	cs := m.ws.App.Cfg.Connectors
	if len(cs) == 0 {
		m.info(i18n.T("tui.connectors_none"))
		return nil
	}
	var sb strings.Builder
	sb.WriteString(i18n.T("tui.connectors_title") + "\n")
	for _, c := range cs {
		target := c.Path + c.URL
		switch {
		case c.Database != nil:
			mode := "ro"
			if c.Database.AllowWrite {
				mode = "rw"
			}
			target = c.Database.Driver + " (" + mode + ")"
		case c.Email != nil:
			target = c.Email.Username + " @ " + c.Email.IMAPHost + c.Email.SMTPHost
		}
		fmt.Fprintf(&sb, "  %-16s %-9s %s\n", c.Name, c.Type, target)
	}
	sb.WriteString(i18n.T("tui.connectors_help"))
	m.info(sb.String())
	return nil
}

func (m *model) cmdSync(args string) tea.Cmd {
	if args == "" {
		m.fail(i18n.Errorf("tui.err.usage", "/sync <connector|all>"))
		return nil
	}
	names := []string{args}
	if args == "all" {
		names = nil
		for _, c := range m.ws.App.Cfg.Connectors {
			names = append(names, c.Name)
		}
	}
	a := m.ws.App
	return m.background(i18n.T("tui.syncing", args), func(ctx context.Context) (string, error) {
		var sb strings.Builder
		for _, n := range names {
			st, err := a.SyncConnector(ctx, n)
			if err != nil {
				fmt.Fprintf(&sb, "✗ %s: %v\n", n, err)
				continue
			}
			sb.WriteString("✓ " + i18n.T("tui.synced", n, st.Documents, st.Chunks) + "\n")
		}
		return strings.TrimRight(sb.String(), "\n"), nil
	})
}

func (m *model) cmdRAG(string) tea.Cmd {
	ix, err := m.ws.App.Index()
	if err != nil {
		m.fail(err)
		return nil
	}
	src := ix.Store.Sources()
	var sb strings.Builder
	sb.WriteString(i18n.T("tui.rag_title", ix.Store.Len(), len(src), m.ws.App.Cfg.RAG.Embedder, m.ws.App.StorePath()) + "\n")
	for i, s := range src {
		if i == 15 {
			fmt.Fprintf(&sb, "  … %d\n", len(src)-15)
			break
		}
		fmt.Fprintf(&sb, "  %4d  %s\n", s.Chunks, s.Source)
	}
	on := i18n.T("tui.off")
	if m.ra.UseRAG {
		on = i18n.T("tui.on")
	}
	sb.WriteString(i18n.T("tui.rag_agent", m.ra.Name, on))
	m.info(sb.String())
	return nil
}

func (m *model) cmdIngest(args string) tea.Cmd {
	if args == "" {
		m.fail(i18n.Errorf("tui.err.usage", "/ingest <path|url>"))
		return nil
	}
	cn := config.ConnectorConfig{Name: args, Type: "url", URL: args}
	if !strings.HasPrefix(args, "http://") && !strings.HasPrefix(args, "https://") {
		p := args
		if !filepath.IsAbs(p) {
			p = filepath.Join(m.ws.Dir, p)
		}
		cn = config.ConnectorConfig{Name: args, Type: "file", Path: p, Recursive: true}
	}
	a := m.ws.App
	return m.background(i18n.T("tui.ingesting", args), func(ctx context.Context) (string, error) {
		st, err := a.IngestConnector(ctx, cn)
		if err != nil && st.Documents == 0 {
			return "", err
		}
		return "✓ " + i18n.T("tui.synced", args, st.Documents, st.Chunks), nil
	})
}

// ---- configuration ----

func (m *model) cmdConfig(args string) tea.Cmd {
	f := strings.Fields(args)
	switch {
	case len(f) == 0 || f[0] == "edit":
		return m.editConfig()
	case f[0] == "get" && len(f) == 2:
		v, err := config.Get(m.ws.App.Cfg.Masked(), f[1])
		if err != nil {
			m.fail(err)
			return nil
		}
		m.info(fmt.Sprintf("%s = %v", f[1], v))
	case f[0] == "set" && len(f) >= 3:
		rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(args), "set"))
		val := strings.TrimSpace(strings.TrimPrefix(rest, f[1]))
		cfg, err := config.Set(m.ws.App.Cfg, f[1], val)
		if err != nil {
			m.fail(err)
			return nil
		}
		m.ws.App.Cfg = cfg
		if err := m.ws.App.Save(); err != nil {
			m.fail(err)
			return nil
		}
		_ = m.resolveAgent()
		if f[1] == "language" { // applies at once, like --lang
			i18n.Set(i18n.Detect(cfg.Language))
			m.input.Placeholder = i18n.T("tui.placeholder")
		}
		m.info(i18n.T("tui.config_set", f[1]))
	case f[0] == "path":
		m.info(m.ws.App.Path)
	default:
		m.fail(i18n.Errorf("tui.err.usage", "/config [edit | get KEY | set KEY VALUE | path]"))
	}
	return nil
}

type editedMsg struct{ err error }

func (m *model) editConfig() tea.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
		if runtime.GOOS == "windows" {
			editor = "notepad"
		}
	}
	parts := strings.Fields(editor)
	c := exec.Command(parts[0], append(parts[1:], m.ws.App.Path)...)
	return tea.ExecProcess(c, func(err error) tea.Msg { return editedMsg{err} })
}

// reload reads the configuration again and restarts the MCP servers.
func (m *model) reload() tea.Cmd {
	a, err := app.Load(m.ws.App.Path)
	if err != nil {
		return func() tea.Msg { return reloadMsg{err} }
	}
	m.ws.App.Cfg = a.Cfg
	old := m.ws.MCP
	m.ws.MCP = mcp.NewManager()
	m.ws.MCP.Start(m.ctx, a.Cfg.MCP, m.ws.Version)
	go old.Close()
	if err := m.resolveAgent(); err != nil {
		m.sel = app.Selection{}
		_ = m.resolveAgent()
	}
	return func() tea.Msg { return reloadMsg{} }
}

// ---- export ----

var nonSlug = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func (m *model) cmdExport(args string) tea.Cmd {
	if len(m.sess.Messages) == 0 {
		m.fail(i18n.Err("tui.err.empty_session"))
		return nil
	}
	path := args
	if path == "" {
		slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(m.sess.Title), "-"), "-")
		if slug == "" {
			slug = "session"
		}
		path = fmt.Sprintf("%s-%s.md", slug, time.Now().Format("20060102-1504"))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.ws.Dir, path)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n", m.sess.Title)
	for _, msg := range m.sess.Messages {
		switch msg.Role {
		case "user":
			fmt.Fprintf(&sb, "## 🧑 %s\n\n%s\n\n", i18n.T("tui.export_user"), msg.Content)
		case "assistant":
			if strings.TrimSpace(msg.Content) != "" {
				fmt.Fprintf(&sb, "## ✦ %s\n\n%s\n\n", m.sess.Agent, msg.Content)
			}
			for _, c := range msg.ToolCalls {
				fmt.Fprintf(&sb, "> ⚙ `%s` %s\n\n", c.Name, string(c.Args))
			}
		}
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		m.fail(err)
		return nil
	}
	m.info(i18n.T("tui.exported", path))
	return nil
}

// ---- command palette ----

func (m *model) openCommandPalette() {
	var items []pickItem
	for _, c := range m.commands() {
		items = append(items, pickItem{title: "/" + c.name + " " + c.args, desc: i18n.T(c.desc), value: c.name})
	}
	m.dialog = newPicker(m.th, i18n.T("tui.pick_command"), items, func(m *model, it pickItem) tea.Cmd {
		c, _ := m.findCommand(it.value)
		if c.args != "" && !strings.HasPrefix(c.args, "[") {
			m.input.SetValue("/" + c.name + " ")
			m.input.CursorEnd()
			m.updateSuggestions()
			return nil
		}
		return c.run(m, "")
	})
}
