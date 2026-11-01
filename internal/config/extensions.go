package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"my-agent/internal/i18n"
)

// TypeOpenAI is any server that speaks the OpenAI Chat Completions API:
// OpenAI itself, OpenRouter, Groq, DeepSeek, Mistral, LM Studio, vLLM...
const TypeOpenAI = "openai"

// Permission actions.
const (
	Allow = "allow"
	Ask   = "ask"
	Deny  = "deny"
)

// Built-in agent names.
const (
	AgentBuild = "build"
	AgentPlan  = "plan"
	AgentChat  = "chat"
)

// AgentDef is a named agent: its own prompt, model, tools and permissions.
// Every field is optional; unset fields fall back to the built-in agent of
// the same name and then to the "agent" section.
type AgentDef struct {
	Description  string            `json:"description,omitempty"`
	SystemPrompt string            `json:"system_prompt,omitempty"`
	Instructions string            `json:"instructions,omitempty"` // appended to the system prompt
	Provider     string            `json:"provider,omitempty"`
	Model        string            `json:"model,omitempty"`
	Temperature  *float64          `json:"temperature,omitempty"`
	MaxTokens    int               `json:"max_tokens,omitempty"`
	UseRAG       *bool             `json:"use_rag,omitempty"`
	MaxSteps     int               `json:"max_steps,omitempty"`
	Tools        map[string]bool   `json:"tools,omitempty"`      // tool name or glob → enabled
	Permission   map[string]string `json:"permission,omitempty"` // tool name or glob → allow|ask|deny
	Color        string            `json:"color,omitempty"`      // TUI badge color, e.g. "#7aa2f7"
	Disabled     bool              `json:"disabled,omitempty"`
}

// MCPConfig is a Model Context Protocol server whose tools are offered to
// the agents. "local" servers are started as a subprocess (stdio); "remote"
// ones are reached over Streamable HTTP.
type MCPConfig struct {
	Type           string            `json:"type"` // local | remote
	Command        []string          `json:"command,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	URL            string            `json:"url,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Enabled        *bool             `json:"enabled,omitempty"` // default true
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
}

// IsEnabled reports whether the server should be started.
func (m MCPConfig) IsEnabled() bool { return m.Enabled == nil || *m.Enabled }

// DatabaseConn configures a "database" connector.
type DatabaseConn struct {
	Driver     string `json:"driver"` // sqlite | postgres | mysql
	DSN        string `json:"dsn,omitempty"`
	DSNEnv     string `json:"dsn_env,omitempty"`
	AllowWrite bool   `json:"allow_write,omitempty"` // default: read-only
	Query      string `json:"query,omitempty"`       // rows ingested by "sync"
	MaxRows    int    `json:"max_rows,omitempty"`
}

// ResolveDSN returns the literal DSN or the value of the env var.
func (d DatabaseConn) ResolveDSN() string {
	if d.DSN != "" {
		return d.DSN
	}
	if d.DSNEnv != "" {
		return os.Getenv(d.DSNEnv)
	}
	return ""
}

// EmailConn configures an "email" connector (IMAP to read, SMTP to send).
type EmailConn struct {
	IMAPHost    string `json:"imap_host,omitempty"`
	IMAPPort    int    `json:"imap_port,omitempty"` // default 993 (TLS)
	SMTPHost    string `json:"smtp_host,omitempty"`
	SMTPPort    int    `json:"smtp_port,omitempty"` // default 587 (STARTTLS)
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
	PasswordEnv string `json:"password_env,omitempty"`
	From        string `json:"from,omitempty"`
	Folder      string `json:"folder,omitempty"` // default INBOX
	Limit       int    `json:"limit,omitempty"`  // messages ingested by "sync" (default 50)
}

// ResolvePassword returns the literal password or the value of the env var.
func (e EmailConn) ResolvePassword() string {
	if e.Password != "" {
		return e.Password
	}
	if e.PasswordEnv != "" {
		return os.Getenv(e.PasswordEnv)
	}
	return ""
}

// ResolveAPIKeys returns every usable key, in order: api_key, the entries of
// api_keys ("env:NAME" reads a variable) and the variable in api_key_env.
// Providers rotate through them when one is rejected or rate limited.
func (p ProviderConfig) ResolveAPIKeys() []string {
	var out []string
	seen := map[string]bool{}
	add := func(k string) {
		k = strings.TrimSpace(k)
		if k != "" && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	add(p.APIKey)
	for _, k := range p.APIKeys {
		if name, ok := strings.CutPrefix(k, "env:"); ok {
			add(os.Getenv(name))
		} else {
			add(k)
		}
	}
	if p.APIKeyEnv != "" && ValidEnvName(p.APIKeyEnv) {
		add(os.Getenv(p.APIKeyEnv))
	}
	return out
}

// HasKey reports whether the provider has at least one key (or needs none).
func (p ProviderConfig) HasKey() bool {
	return len(p.ResolveAPIKeys()) > 0 || p.Type == TypeOllama || (p.Type == TypeOpenAI && isLocalURL(p.BaseURL))
}

func isLocalURL(u string) bool {
	return strings.Contains(u, "://localhost") || strings.Contains(u, "://127.0.0.1") || strings.Contains(u, "://[::1]")
}

// Preset is a ready-made provider definition, used by "providers add".
type Preset struct {
	Type, BaseURL, Model, KeyEnv, EmbeddingModel string
}

// Presets are well-known providers. Every OpenAI-compatible one uses the
// "openai" type with its own base URL.
var Presets = map[string]Preset{
	"claude":     {TypeClaude, "https://api.anthropic.com", "claude-sonnet-5", "ANTHROPIC_API_KEY", ""},
	"gemini":     {TypeGemini, "https://generativelanguage.googleapis.com", "gemini-2.5-flash", "GEMINI_API_KEY", "gemini-embedding-001"},
	"ollama":     {TypeOllama, "http://localhost:11434", "llama3.2", "", "nomic-embed-text"},
	"openai":     {TypeOpenAI, "https://api.openai.com/v1", "gpt-5-mini", "OPENAI_API_KEY", "text-embedding-3-small"},
	"openrouter": {TypeOpenAI, "https://openrouter.ai/api/v1", "openrouter/auto", "OPENROUTER_API_KEY", ""},
	"groq":       {TypeOpenAI, "https://api.groq.com/openai/v1", "llama-3.3-70b-versatile", "GROQ_API_KEY", ""},
	"deepseek":   {TypeOpenAI, "https://api.deepseek.com/v1", "deepseek-chat", "DEEPSEEK_API_KEY", ""},
	"mistral":    {TypeOpenAI, "https://api.mistral.ai/v1", "mistral-large-latest", "MISTRAL_API_KEY", "mistral-embed"},
	"xai":        {TypeOpenAI, "https://api.x.ai/v1", "grok-4", "XAI_API_KEY", ""},
	"together":   {TypeOpenAI, "https://api.together.xyz/v1", "meta-llama/Llama-3.3-70B-Instruct-Turbo", "TOGETHER_API_KEY", ""},
	"lmstudio":   {TypeOpenAI, "http://localhost:1234/v1", "local-model", "", ""},
}

// PresetNames lists the presets alphabetically.
func PresetNames() []string {
	out := make([]string, 0, len(Presets))
	for n := range Presets {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ProviderFromPreset builds a provider configuration from a preset.
func ProviderFromPreset(name string) (ProviderConfig, bool) {
	p, ok := Presets[name]
	if !ok {
		return ProviderConfig{}, false
	}
	pc := ProviderConfig{Type: p.Type, Model: p.Model, BaseURL: p.BaseURL, APIKeyEnv: p.KeyEnv, EmbeddingModel: p.EmbeddingModel, TimeoutSeconds: 120}
	if p.Type == TypeOllama {
		pc.TimeoutSeconds, pc.KeepAlive = 300, "30m"
	}
	return pc, true
}

// BuiltinAgents are always available; the "agents" section can override
// any of their fields or add new agents.
func BuiltinAgents() map[string]AgentDef {
	return map[string]AgentDef{
		AgentBuild: {
			Description: "builtin.build",
			Color:       "#7aa2f7",
		},
		AgentPlan: {
			Description: "builtin.plan",
			Instructions: "You are in PLAN mode. You cannot modify files, run commands or send anything. " +
				"Investigate with the read-only tools, then answer with a clear, numbered plan of the changes you propose, " +
				"naming the files involved. Ask the user to switch to the build agent to carry it out.",
			Tools: map[string]bool{"write": false, "edit": false, "bash": false, "db_exec": false, "email_send": false},
			Color: "#e0af68",
		},
		AgentChat: {
			Description: "builtin.chat",
			Tools:       map[string]bool{"*": false, "rag_search": true},
			Color:       "#9ece6a",
		},
	}
}

// DefaultPermission is used for tools that no rule matches.
var DefaultPermission = map[string]string{
	"*":          Allow,
	"write":      Ask,
	"edit":       Ask,
	"bash":       Ask,
	"db_exec":    Ask,
	"email_send": Ask,
	"mcp_*":      Ask,
}

// AgentNames returns the enabled agents: the built-in ones plus those in
// the config, sorted with the default agent first.
func (c Config) AgentNames() []string {
	set := map[string]bool{}
	for n := range BuiltinAgents() {
		set[n] = true
	}
	for n, a := range c.Agents {
		set[n] = !a.Disabled
	}
	var out []string
	for n, ok := range set {
		if ok {
			out = append(out, n)
		}
	}
	def := c.DefaultAgentName()
	sort.Slice(out, func(i, j int) bool {
		if (out[i] == def) != (out[j] == def) {
			return out[i] == def
		}
		return out[i] < out[j]
	})
	return out
}

// DefaultAgentName is default_agent, or "build".
func (c Config) DefaultAgentName() string {
	if c.DefaultAgent != "" {
		return c.DefaultAgent
	}
	return AgentBuild
}

// ResolvedAgent is an agent with every inherited value filled in.
type ResolvedAgent struct {
	Name        string
	Description string
	AgentConfig
	Model        string
	Instructions string
	MaxSteps     int
	Tools        map[string]bool
	Permission   map[string]string
	Color        string
}

// ResolveAgent merges an agent definition with the built-in one of the same
// name and the "agent" section.
func (c Config) ResolveAgent(name string) (ResolvedAgent, error) {
	if name == "" {
		name = c.DefaultAgentName()
	}
	b, isBuiltin := BuiltinAgents()[name]
	u, isUser := c.Agents[name]
	if (!isBuiltin && !isUser) || (isUser && u.Disabled) {
		return ResolvedAgent{}, i18n.Errorf("config.err.unknown_agent", name)
	}
	r := ResolvedAgent{Name: name, AgentConfig: c.Agent, MaxSteps: 25, Tools: map[string]bool{}, Permission: map[string]string{}}
	for k, v := range DefaultPermission {
		r.Permission[k] = v
	}
	for k, v := range c.Permission {
		r.Permission[k] = v
	}
	for _, d := range []AgentDef{b, u} {
		if d.Description != "" {
			r.Description = d.Description
		}
		if d.SystemPrompt != "" {
			r.SystemPrompt = d.SystemPrompt
		}
		if d.Instructions != "" {
			r.Instructions = d.Instructions
		}
		if d.Provider != "" {
			r.Provider = d.Provider
		}
		if d.Model != "" {
			r.Model = d.Model
		}
		if d.Temperature != nil {
			r.Temperature = *d.Temperature
		}
		if d.MaxTokens > 0 {
			r.MaxTokens = d.MaxTokens
		}
		if d.UseRAG != nil {
			r.UseRAG = *d.UseRAG
		}
		if d.MaxSteps > 0 {
			r.MaxSteps = d.MaxSteps
		}
		if d.Color != "" {
			r.Color = d.Color
		}
		for k, v := range d.Tools {
			r.Tools[k] = v
		}
		for k, v := range d.Permission {
			r.Permission[k] = v
		}
	}
	if strings.HasPrefix(r.Description, "builtin.") {
		r.Description = i18n.T("config.agent." + r.Description)
	}
	return r, nil
}

// Enabled reports whether an agent may use a tool. The most specific rule
// wins: an exact name beats a glob, and a longer glob beats a shorter one.
func (r ResolvedAgent) Enabled(tool string) bool {
	v, ok := matchRule(r.Tools, tool)
	return !ok || v
}

// PermissionFor returns allow, ask or deny for a tool call. For "bash",
// rules such as "bash:git status*" match the command line.
func (r ResolvedAgent) PermissionFor(tool, command string) string {
	if tool == "bash" && command != "" {
		best, bestLen := "", -1
		for k, v := range r.Permission {
			pat, ok := strings.CutPrefix(k, "bash:")
			if ok && globMatch(pat, command) && len(pat) > bestLen {
				best, bestLen = v, len(pat)
			}
		}
		if bestLen >= 0 {
			return best
		}
	}
	if v, ok := matchRule(r.Permission, tool); ok {
		return v
	}
	return Allow
}

func matchRule[V any](rules map[string]V, name string) (V, bool) {
	if v, ok := rules[name]; ok {
		return v, true
	}
	var best V
	bestLen := -1
	for k, v := range rules {
		if strings.Contains(k, ":") {
			continue
		}
		if globMatch(k, name) && len(k) > bestLen {
			best, bestLen = v, len(k)
		}
	}
	return best, bestLen >= 0
}

// globMatch matches "*" (any run of characters) and "?" (one character).
func globMatch(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	ok, err := filepath.Match(pattern, s)
	if err == nil && ok {
		return true
	}
	// filepath.Match stops "*" at separators; commands contain "/" freely.
	if !strings.Contains(pattern, "*") {
		return false
	}
	parts := strings.Split(pattern, "*")
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for i, p := range parts[1:] {
		if i == len(parts)-2 {
			return strings.HasSuffix(s, p)
		}
		idx := strings.Index(s, p)
		if idx < 0 {
			return false
		}
		s = s[idx+len(p):]
	}
	return true
}

// Shell returns the command used by the bash tool: the "shell" setting, or
// PowerShell on Windows and /bin/sh elsewhere.
func (c Config) ShellCommand() []string {
	if s := strings.TrimSpace(c.Shell); s != "" {
		return strings.Fields(s)
	}
	if runtime.GOOS == "windows" {
		return []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command"}
	}
	return []string{"/bin/sh", "-c"}
}

func validPermission(v string) bool { return v == Allow || v == Ask || v == Deny }

// Optional scalar fields, which are left out of the file while unset. Their
// zero value tells Set the field's type.
var (
	optionalTop    = map[string]any{"default_agent": "", "shell": "", "theme": ""}
	optionalAgents = map[string]any{
		"description": "", "system_prompt": "", "instructions": "", "provider": "", "model": "", "color": "",
		"temperature": 0.0, "max_tokens": 0.0, "max_steps": 0.0, "use_rag": false, "disabled": false,
	}
)

// addOptional lets Set reach an optional field that is not in the file yet:
// "theme", or "agents.<name>.<field>" (creating the agent entry). Only the
// requested field is added, so no other setting changes meaning.
func addOptional(m map[string]any, parts []string) {
	switch {
	case len(parts) == 1:
		if zero, ok := optionalTop[parts[0]]; ok {
			if _, has := m[parts[0]]; !has {
				m[parts[0]] = zero
			}
		}
	case len(parts) == 3 && parts[0] == "agents":
		zero, ok := optionalAgents[parts[2]]
		if !ok || parts[1] == "" {
			return
		}
		agents, _ := m["agents"].(map[string]any)
		if agents == nil {
			agents = map[string]any{}
			m["agents"] = agents
		}
		agent, _ := agents[parts[1]].(map[string]any)
		if agent == nil {
			agent = map[string]any{}
			agents[parts[1]] = agent
		}
		if _, has := agent[parts[2]]; !has {
			agent[parts[2]] = zero
		}
	}
}

// validateExtensions checks agents, permissions, MCP servers and the new
// connector types.
func (c Config) validateExtensions(add func(key string, a ...any)) {
	for k, v := range c.Permission {
		if !validPermission(v) {
			add("config.err.permission", "permission."+k, v)
		}
	}
	names := make([]string, 0, len(c.Agents))
	for n := range c.Agents {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		a := c.Agents[n]
		if a.Provider != "" {
			if _, ok := c.Providers[a.Provider]; !ok {
				add("config.err.agent_provider_of", n, a.Provider)
			}
		}
		if a.Temperature != nil && (*a.Temperature < 0 || *a.Temperature > 2) {
			add("config.err.between", "agents."+n+".temperature", 0, 2)
		}
		for k, v := range a.Permission {
			if !validPermission(v) {
				add("config.err.permission", "agents."+n+".permission."+k, v)
			}
		}
	}
	if c.DefaultAgent != "" {
		if _, err := c.ResolveAgent(c.DefaultAgent); err != nil {
			add("config.err.unknown_agent", c.DefaultAgent)
		}
	}
	mcps := make([]string, 0, len(c.MCP))
	for n := range c.MCP {
		mcps = append(mcps, n)
	}
	sort.Strings(mcps)
	for _, n := range mcps {
		m := c.MCP[n]
		switch m.Type {
		case "local":
			if len(m.Command) == 0 {
				add("config.err.required", "mcp."+n+".command")
			}
		case "remote":
			if !strings.HasPrefix(m.URL, "http://") && !strings.HasPrefix(m.URL, "https://") {
				add("config.err.mcp_url", n)
			}
		default:
			add("config.err.mcp_type", n, m.Type)
		}
	}
	switch c.Theme {
	case "", "auto", "dark", "light":
	default:
		add("config.err.theme", c.Theme)
	}
}

func (c Config) validateConnector(i int, cn ConnectorConfig, add func(key string, a ...any)) bool {
	switch cn.Type {
	case "database":
		d := cn.Database
		if d == nil {
			add("config.err.required", fmt.Sprintf("connectors[%d].database", i))
			return true
		}
		switch d.Driver {
		case "sqlite", "postgres", "mysql":
		default:
			add("config.err.db_driver", i, d.Driver)
		}
		if d.DSN == "" && d.DSNEnv == "" {
			add("config.err.required", fmt.Sprintf("connectors[%d].database.dsn", i))
		}
		return true
	case "email":
		e := cn.Email
		if e == nil {
			add("config.err.required", fmt.Sprintf("connectors[%d].email", i))
			return true
		}
		if e.IMAPHost == "" && e.SMTPHost == "" {
			add("config.err.required", fmt.Sprintf("connectors[%d].email.imap_host", i))
		}
		if e.Username == "" {
			add("config.err.required", fmt.Sprintf("connectors[%d].email.username", i))
		}
		return true
	}
	return false
}

// maskExtensions hides secrets of the new sections in a copy of c.
func (c Config) maskExtensions(out *Config) {
	maskMap := func(m map[string]string) map[string]string {
		if m == nil {
			return nil
		}
		cp := make(map[string]string, len(m))
		for k, v := range m {
			if v != "" {
				v = MaskedKey
			}
			cp[k] = v
		}
		return cp
	}
	for k, p := range out.Providers {
		keys := make([]string, len(p.APIKeys))
		for i, key := range p.APIKeys {
			if strings.HasPrefix(key, "env:") {
				keys[i] = key
			} else {
				keys[i] = MaskedKey
			}
		}
		if p.APIKeys != nil {
			p.APIKeys = keys
		}
		p.Headers = maskMap(p.Headers)
		out.Providers[k] = p
	}
	if c.MCP != nil {
		out.MCP = make(map[string]MCPConfig, len(c.MCP))
		for k, m := range c.MCP {
			m.Environment = maskMap(m.Environment)
			m.Headers = maskMap(m.Headers)
			out.MCP[k] = m
		}
	}
	out.Connectors = make([]ConnectorConfig, len(c.Connectors))
	for i, cn := range c.Connectors {
		if cn.Database != nil {
			d := *cn.Database
			if d.DSN != "" {
				d.DSN = MaskedKey
			}
			cn.Database = &d
		}
		if cn.Email != nil {
			e := *cn.Email
			if e.Password != "" {
				e.Password = MaskedKey
			}
			cn.Email = &e
		}
		out.Connectors[i] = cn
	}
}

// GlobalDir is the per-user configuration directory:
// %APPDATA%\my-agent on Windows, ~/.config/my-agent elsewhere.
func GlobalDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "my-agent")
	}
	return ".my-agent"
}

// GlobalPath is the per-user configuration file.
func GlobalPath() string { return filepath.Join(GlobalDir(), "my-agent.json") }

// DataDir holds sessions and other state: %LOCALAPPDATA%\my-agent on
// Windows, $XDG_DATA_HOME/my-agent or ~/.local/share/my-agent elsewhere.
func DataDir() string {
	if d := os.Getenv("MY_AGENT_DATA"); d != "" {
		return d
	}
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "my-agent")
		}
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "my-agent")
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".local", "share", "my-agent")
	}
	return filepath.Join(GlobalDir(), "data")
}
