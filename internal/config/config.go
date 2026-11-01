// Package config defines the agent configuration file and helpers to
// load, save, validate and edit it (including dotted-key get/set).
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"my-agent/internal/i18n"
)

// MaskedKey is shown in place of secrets when a config is displayed.
const MaskedKey = "********"

// EnvConfigPath is the environment variable that overrides the config path.
const EnvConfigPath = "MY_AGENT_CONFIG"

// Provider types supported by the agent.
const (
	TypeClaude = "claude"
	TypeGemini = "gemini"
	TypeOllama = "ollama"
)

// LocalEmbedder is the reserved embedder name for the offline hashing embedder.
const LocalEmbedder = "local"

type Config struct {
	// Language of the messages: "auto" (detect it), "en", "pt-BR" or "es".
	Language   string                    `json:"language"`
	Agent      AgentConfig               `json:"agent"`
	Providers  map[string]ProviderConfig `json:"providers"`
	RAG        RAGConfig                 `json:"rag"`
	Connectors []ConnectorConfig         `json:"connectors"`
	Server     ServerConfig              `json:"server"`

	// Agents, tools and connectors of the interactive coding agent (TUI).
	DefaultAgent string               `json:"default_agent,omitempty"`
	Agents       map[string]AgentDef  `json:"agents,omitempty"`
	Permission   map[string]string    `json:"permission,omitempty"` // tool glob → allow|ask|deny
	MCP          map[string]MCPConfig `json:"mcp,omitempty"`
	Shell        string               `json:"shell,omitempty"` // command of the bash tool, e.g. "pwsh -Command"
	Theme        string               `json:"theme,omitempty"` // auto | dark | light
}

type AgentConfig struct {
	Name         string  `json:"name"`
	SystemPrompt string  `json:"system_prompt"`
	Provider     string  `json:"provider"`
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens"`
	UseRAG       bool    `json:"use_rag"`
}

type ProviderConfig struct {
	Type           string `json:"type"`
	Model          string `json:"model"`
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	APIKeyEnv      string `json:"api_key_env"`
	EmbeddingModel string `json:"embedding_model"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	// Ollama only. KeepAlive is how long the model stays loaded after a
	// request ("30m", "1h", seconds, or -1 for forever; empty = 30m).
	// NumCtx caps the context window (0 = Ollama default); smaller values use
	// less GPU memory, which keeps big models fully on the GPU.
	KeepAlive string `json:"keep_alive"`
	NumCtx    int    `json:"num_ctx"`
	// Extra keys, tried in turn when one is rejected or rate limited.
	// "env:NAME" reads the key from a variable.
	APIKeys []string          `json:"api_keys,omitempty"`
	Headers map[string]string `json:"headers,omitempty"` // extra HTTP headers
	Models  []string          `json:"models,omitempty"`  // models offered in the model picker
}

// ValidKeepAlive reports whether s is accepted as a keep_alive value.
func ValidKeepAlive(s string) bool {
	if s == "" {
		return true
	}
	if _, err := strconv.Atoi(s); err == nil {
		return true
	}
	_, err := time.ParseDuration(s)
	return err == nil
}

type RAGConfig struct {
	StorePath    string  `json:"store_path"`
	Embedder     string  `json:"embedder"` // "local" or a provider name
	ChunkSize    int     `json:"chunk_size"`
	ChunkOverlap int     `json:"chunk_overlap"`
	TopK         int     `json:"top_k"`
	MinScore     float64 `json:"min_score"`
}

type ConnectorConfig struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"` // file | url | database | email
	Path      string   `json:"path"`
	URL       string   `json:"url"`
	Include   []string `json:"include"`
	Recursive bool     `json:"recursive"`

	Database *DatabaseConn `json:"database,omitempty"`
	Email    *EmailConn    `json:"email,omitempty"`
}

type ServerConfig struct {
	Addr string `json:"addr"`
}

// Default returns a working configuration (Ollama + offline embeddings).
func Default() Config {
	return Config{
		Language: i18n.Auto,
		Agent: AgentConfig{
			Name:         "my-agent",
			SystemPrompt: "You are a helpful assistant. Answer concisely and say when you do not know.",
			Provider:     TypeOllama,
			Temperature:  0.2,
			MaxTokens:    1024,
			UseRAG:       true,
		},
		Providers: map[string]ProviderConfig{
			TypeClaude: {Type: TypeClaude, Model: "claude-sonnet-5", BaseURL: "https://api.anthropic.com", APIKeyEnv: "ANTHROPIC_API_KEY", TimeoutSeconds: 120},
			TypeGemini: {Type: TypeGemini, Model: "gemini-2.5-flash", BaseURL: "https://generativelanguage.googleapis.com", APIKeyEnv: "GEMINI_API_KEY", EmbeddingModel: "gemini-embedding-001", TimeoutSeconds: 120},
			TypeOllama: {Type: TypeOllama, Model: "llama3.2", BaseURL: "http://localhost:11434", EmbeddingModel: "nomic-embed-text", TimeoutSeconds: 300, KeepAlive: "30m"},
		},
		RAG: RAGConfig{
			StorePath:    "data/rag-store.json",
			Embedder:     LocalEmbedder,
			ChunkSize:    800,
			ChunkOverlap: 100,
			TopK:         4,
			MinScore:     0.05,
		},
		Connectors: []ConnectorConfig{},
		Server:     ServerConfig{Addr: "127.0.0.1:8080"},
	}
}

// DefaultPrompt is the agent's default system prompt in a language.
func DefaultPrompt(l i18n.Lang) string { return i18n.TL(l, "config.default_prompt") }

// PeekLanguage reads only the "language" setting of a config file, without
// validating it, so the language can be chosen before anything is printed.
// It returns "" when the file is missing or unreadable.
func PeekLanguage(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var v struct {
		Language string `json:"language"`
	}
	if json.Unmarshal(data, &v) != nil {
		return ""
	}
	return v.Language
}

// DefaultPath returns $MY_AGENT_CONFIG, else ./my-agent.json when it exists,
// else the per-user file when it exists, else ./my-agent.json.
func DefaultPath() string {
	if p := os.Getenv(EnvConfigPath); p != "" {
		return p
	}
	if _, err := os.Stat("my-agent.json"); err == nil {
		return "my-agent.json"
	}
	if _, err := os.Stat(GlobalPath()); err == nil {
		return GlobalPath()
	}
	return "my-agent.json"
}

// ResolveAPIKey returns the first usable key (see ResolveAPIKeys).
func (p ProviderConfig) ResolveAPIKey() string {
	if keys := p.ResolveAPIKeys(); len(keys) > 0 {
		return keys[0]
	}
	return ""
}

// Load reads and validates a config file.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(data)
}

// Parse decodes (strictly) and validates config JSON.
func Parse(data []byte) (Config, error) {
	var c Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Config{}, i18n.Errorf("config.err.invalid", err)
	}
	if i18n.Valid(c.Language) {
		c.Language = i18n.Canonical(c.Language)
	}
	if c.Connectors == nil {
		c.Connectors = []ConnectorConfig{}
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Save writes the config as indented JSON (file mode 0600: it may hold keys).
func (c Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

var envNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidEnvName reports whether s can be an environment variable name (empty
// means "no variable"). A secret pasted into api_key_env fails this check.
func ValidEnvName(s string) bool { return s == "" || envNameRE.MatchString(s) }

// Masked returns a copy safe to display: API keys are hidden. A value in
// api_key_env that is not a variable name is most likely a key pasted in the
// wrong field, so it is hidden too.
func (c Config) Masked() Config {
	out := c
	out.Providers = make(map[string]ProviderConfig, len(c.Providers))
	for k, p := range c.Providers {
		if p.APIKey != "" {
			p.APIKey = MaskedKey
		}
		if !ValidEnvName(p.APIKeyEnv) {
			p.APIKeyEnv = MaskedKey
		}
		out.Providers[k] = p
	}
	c.maskExtensions(&out)
	return out
}

// Warnings lists likely mistakes that do not make the config invalid. The
// messages never include the offending value, which may be a secret.
func (c Config) Warnings() []string {
	names := make([]string, 0, len(c.Providers))
	for n := range c.Providers {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []string
	for _, n := range names {
		if !ValidEnvName(c.Providers[n].APIKeyEnv) {
			out = append(out, i18n.T("config.warn.api_key_env", n))
		}
	}
	return out
}

// Validate checks the whole config and reports all problems at once.
func (c Config) Validate() error {
	var errs []error
	add := func(key string, a ...any) { errs = append(errs, i18n.Errorf(key, a...)) }

	if !i18n.Valid(c.Language) {
		add("config.err.language", c.Language)
	}
	if len(c.Providers) == 0 {
		add("config.err.providers_required")
	}
	names := make([]string, 0, len(c.Providers))
	for n := range c.Providers {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := c.Providers[n]
		path := "providers." + n
		switch p.Type {
		case TypeClaude, TypeGemini, TypeOllama, TypeOpenAI:
		default:
			add("config.err.provider_type", n, p.Type)
		}
		if p.Model == "" {
			add("config.err.required", path+".model")
		}
		if p.BaseURL == "" {
			add("config.err.required", path+".base_url")
		}
		if p.TimeoutSeconds < 0 {
			add("config.err.non_negative", path+".timeout_seconds")
		}
		if !ValidKeepAlive(p.KeepAlive) {
			add("config.err.keep_alive", n, p.KeepAlive)
		}
		if p.NumCtx < 0 {
			add("config.err.non_negative", path+".num_ctx")
		}
	}
	if _, ok := c.Providers[c.Agent.Provider]; !ok {
		add("config.err.agent_provider", c.Agent.Provider)
	}
	if c.Agent.Temperature < 0 || c.Agent.Temperature > 2 {
		add("config.err.between", "agent.temperature", 0, 2)
	}
	if c.Agent.MaxTokens <= 0 {
		add("config.err.positive", "agent.max_tokens")
	}

	r := c.RAG
	if r.StorePath == "" {
		add("config.err.required", "rag.store_path")
	}
	if r.Embedder != LocalEmbedder {
		if _, ok := c.Providers[r.Embedder]; !ok {
			add("config.err.embedder", LocalEmbedder, r.Embedder)
		}
	}
	if r.ChunkSize <= 0 {
		add("config.err.positive", "rag.chunk_size")
	}
	if r.ChunkOverlap < 0 || r.ChunkOverlap >= max(r.ChunkSize, 1) {
		add("config.err.overlap")
	}
	if r.TopK <= 0 {
		add("config.err.positive", "rag.top_k")
	}
	if r.MinScore < -1 || r.MinScore > 1 {
		add("config.err.between", "rag.min_score", -1, 1)
	}

	seen := map[string]bool{}
	for i, cn := range c.Connectors {
		if cn.Name == "" {
			add("config.err.required", fmt.Sprintf("connectors[%d].name", i))
		} else if seen[cn.Name] {
			add("config.err.conn_dup", i, cn.Name)
		}
		seen[cn.Name] = true
		if c.validateConnector(i, cn, add) {
			continue
		}
		switch cn.Type {
		case "file":
			if cn.Path == "" {
				add("config.err.conn_path", i)
			}
		case "url":
			if !strings.HasPrefix(cn.URL, "http://") && !strings.HasPrefix(cn.URL, "https://") {
				add("config.err.conn_url", i)
			}
		default:
			add("config.err.conn_type", i, cn.Type)
		}
	}
	if c.Server.Addr == "" {
		add("config.err.required", "server.addr")
	}
	c.validateExtensions(add)
	return errors.Join(errs...)
}

// FindConnector returns the connector with the given name.
func (c Config) FindConnector(name string) (ConnectorConfig, bool) {
	for _, cn := range c.Connectors {
		if cn.Name == name {
			return cn, true
		}
	}
	return ConnectorConfig{}, false
}

func toMap(c Config) (map[string]any, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(b, &m)
}

// Get returns the value at a dotted key such as "rag.top_k".
func Get(c Config, key string) (any, error) {
	m, err := toMap(c)
	if err != nil {
		return nil, err
	}
	var cur any = m
	for _, p := range strings.Split(key, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, i18n.Errorf("config.err.unknown_key", key)
		}
		if cur, ok = obj[p]; !ok {
			return nil, i18n.Errorf("config.err.unknown_key", key)
		}
	}
	return cur, nil
}

// Set assigns a scalar at a dotted key, coercing the string to the field's
// type, and returns the validated result. Lists (connectors) are managed by
// dedicated commands.
func Set(c Config, key, value string) (Config, error) {
	m, err := toMap(c)
	if err != nil {
		return c, err
	}
	parts := strings.Split(key, ".")
	addOptional(m, parts)
	cur := m
	for _, p := range parts[:len(parts)-1] {
		next, ok := cur[p].(map[string]any)
		if !ok {
			return c, i18n.Errorf("config.err.unknown_key", key)
		}
		cur = next
	}
	last := parts[len(parts)-1]
	old, ok := cur[last]
	if !ok {
		return c, i18n.Errorf("config.err.unknown_key", key)
	}
	switch old.(type) {
	case string:
		cur[last] = value
	case float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return c, i18n.Errorf("config.err.not_number", key, value)
		}
		cur[last] = f
	case bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return c, i18n.Errorf("config.err.not_bool", key, value)
		}
		cur[last] = b
	default:
		return c, i18n.Errorf("config.err.not_settable", key)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return c, err
	}
	out, err := Parse(b)
	if err != nil {
		return c, err
	}
	return out, nil
}
