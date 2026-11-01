package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"my-agent/internal/agent"
	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/i18n"
	"my-agent/internal/provider"
	"my-agent/internal/session"
	"my-agent/internal/tui"
)

// tuiOptions are the flags that apply when my-agent runs without a command.
type tuiOptions struct {
	agent, model, session string
	cont                  bool
}

// splitModel turns "provider/model" into its parts; a bare model keeps the
// provider empty.
func splitModel(a *app.App, s string) (prov, model string) {
	if p, m, ok := strings.Cut(s, "/"); ok {
		if _, exists := a.Cfg.Providers[p]; exists {
			return p, m
		}
	}
	return "", s
}

// ensureConfig creates the per-user configuration on the first run, when no
// configuration was given or found.
func (e *env) ensureConfig(explicit bool) error {
	if _, err := os.Stat(e.cfgPath); err == nil || explicit {
		return nil
	}
	e.cfgPath = config.GlobalPath()
	if _, err := os.Stat(e.cfgPath); err == nil {
		return nil
	}
	cfg := config.Default()
	cfg.Agent.SystemPrompt = config.DefaultPrompt(i18n.Current())
	if e.langFlag != "" {
		cfg.Language = i18n.Canonical(e.langFlag)
	}
	// start with a model that is really installed, when Ollama is running
	if m := installedOllamaModel(cfg.Providers[config.TypeOllama]); m != "" {
		pc := cfg.Providers[config.TypeOllama]
		pc.Model = m
		cfg.Providers[config.TypeOllama] = pc
		fmt.Fprintln(e.err, i18n.T("cli.tui.ollama_model", m))
	}
	if err := cfg.Save(e.cfgPath); err != nil {
		return err
	}
	fmt.Fprintln(e.err, i18n.T("cli.tui.created_config", e.cfgPath))
	return nil
}

// installedOllamaModel returns the configured model when it is installed,
// else the first installed chat model, or "" when Ollama does not answer.
func installedOllamaModel(pc config.ProviderConfig) string {
	p, err := provider.New(config.TypeOllama, pc)
	if err != nil {
		return ""
	}
	mm, ok := p.(provider.ModelManager)
	if !ok {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	models, err := mm.ListModels(ctx)
	if err != nil || len(models) == 0 || provider.ModelInstalled(models, pc.Model) {
		return ""
	}
	for _, m := range models {
		if !strings.Contains(strings.ToLower(m.Name), "embed") {
			return m.Name
		}
	}
	return ""
}

// cmdTUI opens the full-screen interface.
func (e *env) cmdTUI(ctx context.Context, o tuiOptions, explicitConfig bool, prompt string) error {
	if !isTerminal(e.in) || !isTerminal(e.out) {
		return i18n.Err("cli.tui.no_terminal")
	}
	if err := e.ensureConfig(explicitConfig); err != nil {
		return err
	}
	a, err := e.app()
	if err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	ws := a.Workspace(ctx, dir, Version)
	defer ws.Close()
	opts := tui.Options{Agent: o.agent, Session: o.session, Prompt: prompt}
	if o.cont && opts.Session == "" {
		opts.Session = "last"
	}
	if o.model != "" {
		opts.Provider, opts.Model = splitModel(a, o.model)
	}
	return tui.Run(ctx, ws, opts)
}

func isTerminal(f any) bool {
	file, ok := f.(*os.File)
	if !ok {
		return false
	}
	fi, err := file.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// cmdRun runs the agent with tools once, without the interface: the reply
// goes to stdout and tool activity to stderr. Tools that need permission
// are denied unless --yes is given.
func (e *env) cmdRun(ctx context.Context, args []string) error {
	fs := e.flags("run")
	agentName := fs.String("agent", "", i18n.T("cli.flag.agent"))
	model := fs.String("model", "", i18n.T("cli.flag.model"))
	yes := fs.Bool("yes", false, i18n.T("cli.flag.yes"))
	if err := parse(fs, args); err != nil {
		return err
	}
	msg := joinArgs(fs.Args())
	if msg == "" {
		return usageSyntax("run [--agent A] [--model P/M] [--yes] MESSAGE...")
	}
	a, err := e.app()
	if err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	ws := a.Workspace(ctx, dir, Version)
	defer ws.Close()
	ws.MCP.Wait()
	sel := app.Selection{Agent: *agentName}
	if *model != "" {
		sel.Provider, sel.Model = splitModel(a, *model)
	}
	r, _, err := ws.Runner(sel)
	if err != nil {
		return err
	}
	st := newStyle(e.err)
	r.OnEvent = func(ev agent.Event) {
		switch ev.Kind {
		case agent.EvText:
			fmt.Fprint(e.out, ev.Text)
		case agent.EvToolStart:
			fmt.Fprintf(e.err, "%s\n", st.dim("⚙ "+ev.Call.Name+" "+oneLine(ev.Subject)))
		case agent.EvToolEnd:
			if ev.Err != nil {
				fmt.Fprintf(e.err, "%s\n", st.red("✗ "+ev.Call.Name+": "+oneLine(ev.Err.Error())))
			}
		case agent.EvStep:
			fmt.Fprintln(e.out)
		}
	}
	r.Approve = func(_ context.Context, req agent.PermissionRequest) agent.Decision {
		if *yes {
			return agent.Always
		}
		fmt.Fprintln(e.err, st.yellow(i18n.T("cli.run.denied", req.Tool.Name)))
		return agent.Deny
	}
	_, _, err = r.Run(ctx, nil, msg, "")
	fmt.Fprintln(e.out)
	return err
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 120 {
		s = s[:117] + "..."
	}
	return s
}

// cmdAgents lists the agents.
func (e *env) cmdAgents(args []string) error {
	if len(args) > 0 && args[0] != "list" {
		return usageSyntax("agents [list]")
	}
	a, err := e.app()
	if err != nil {
		return err
	}
	for _, n := range a.Cfg.AgentNames() {
		ra, err := a.Cfg.ResolveAgent(n)
		if err != nil {
			continue
		}
		mark := " "
		if n == a.Cfg.DefaultAgentName() {
			mark = "*"
		}
		model := ra.Provider
		if ra.Model != "" {
			model += "/" + ra.Model
		}
		fmt.Fprintf(e.out, "%s %-10s %-28s %s\n", mark, n, model, ra.Description)
	}
	return nil
}

// cmdMCP connects to the MCP servers and lists their tools.
func (e *env) cmdMCP(ctx context.Context, args []string) error {
	if len(args) > 0 && args[0] != "list" {
		return usageSyntax("mcp [list]")
	}
	a, err := e.app()
	if err != nil {
		return err
	}
	if len(a.Cfg.MCP) == 0 {
		fmt.Fprintln(e.out, i18n.T("tui.mcp_none"))
		return nil
	}
	ws := a.Workspace(ctx, ".", Version)
	defer ws.Close()
	ws.MCP.Wait()
	byServer := map[string][]string{}
	for _, t := range ws.MCP.Tools() {
		byServer[t.Source] = append(byServer[t.Source], t.Name)
	}
	failed := 0
	for _, s := range ws.MCP.Statuses() {
		switch {
		case s.Disabled:
			fmt.Fprintf(e.out, "-    %-16s %s\n", s.Name, i18n.T("tui.mcp_disabled"))
		case s.Err != nil:
			failed++
			fmt.Fprintf(e.out, "FAIL %-16s %v\n", s.Name, s.Err)
		default:
			fmt.Fprintf(e.out, "OK   %-16s %s  %s\n", s.Name, i18n.N("tui.mcp_tools", s.Tools), s.Server)
			for _, t := range byServer[s.Name] {
				fmt.Fprintf(e.out, "       %s\n", t)
			}
		}
	}
	if failed > 0 {
		return i18n.Errorf("cli.mcp.failed", failed)
	}
	return nil
}

// cmdSessions lists or removes saved sessions.
func (e *env) cmdSessions(args []string) error {
	store := session.Store{Dir: filepath.Join(config.DataDir(), "sessions")}
	if len(args) == 0 || args[0] == "list" {
		dir, _ := os.Getwd()
		list, err := store.List(dir)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Fprintln(e.out, i18n.T("cli.sessions.none"))
			return nil
		}
		for _, s := range list {
			fmt.Fprintf(e.out, "%s  %s  %-8s %4d  %s\n", s.ID, s.Updated.Format("2006-01-02 15:04"), s.Agent, s.Messages, s.Title)
		}
		return nil
	}
	if args[0] == "remove" && len(args) > 1 {
		for _, id := range args[1:] {
			if err := store.Delete(id); err != nil {
				return err
			}
		}
		return nil
	}
	return usageSyntax("sessions [list | remove ID...]")
}

// providersAdd adds a provider from a preset (and its key).
func (e *env) providersAdd(a *app.App, args []string) error {
	fs := e.flags("providers add")
	preset := fs.String("preset", "", i18n.T("cli.flag.preset"))
	key := fs.String("key", "", i18n.T("cli.flag.key"))
	model := fs.String("model", "", i18n.T("cli.flag.model"))
	baseURL := fs.String("base-url", "", i18n.T("cli.flag.base_url"))
	// the name may come before the flags ("add groq --key ...") or after them
	name := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args = args[0], args[1:]
	}
	if err := parse(fs, args); err != nil {
		return err
	}
	if name == "" && fs.NArg() == 1 {
		name = fs.Arg(0)
	} else if fs.NArg() != 0 {
		name = ""
	}
	if name == "" {
		return usageSyntax("providers add NAME [--preset P] [--key K] [--model M] [--base-url URL]   (" + strings.Join(config.PresetNames(), ", ") + ")")
	}
	p := *preset
	if p == "" {
		p = name
	}
	pc, exists := a.Cfg.Providers[name]
	if !exists {
		var ok bool
		if pc, ok = config.ProviderFromPreset(p); !ok {
			return i18n.Errorf("tui.err.unknown_preset", p, strings.Join(config.PresetNames(), ", "))
		}
	}
	if *model != "" {
		pc.Model = *model
	}
	if *baseURL != "" {
		pc.BaseURL = *baseURL
	}
	if k := strings.TrimSpace(*key); k != "" {
		switch {
		case pc.APIKey == "" || pc.APIKey == k:
			pc.APIKey = k
		default:
			pc.APIKeys = append(pc.APIKeys, k)
		}
	}
	cfg := a.Cfg
	providers := map[string]config.ProviderConfig{}
	for k, v := range cfg.Providers {
		providers[k] = v
	}
	providers[name] = pc
	cfg.Providers = providers
	if err := cfg.Validate(); err != nil {
		return err
	}
	a.Cfg = cfg
	if err := a.Save(); err != nil {
		return err
	}
	fmt.Fprintln(e.out, i18n.T("tui.provider_saved", name, len(pc.ResolveAPIKeys())))
	return nil
}
