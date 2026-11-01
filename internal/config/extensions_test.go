package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuiltinAgentsAndOverrides(t *testing.T) {
	c := Default()
	temp := 0.9
	c.Agents = map[string]AgentDef{
		"plan":   {Model: "big"},
		"review": {Description: "reviews code", Provider: "claude", Temperature: &temp, Tools: map[string]bool{"write": false}},
		"chat":   {Disabled: true},
	}
	names := c.AgentNames()
	if strings.Join(names, ",") != "build,plan,review" {
		t.Fatalf("names: %v", names)
	}
	plan, err := c.ResolveAgent("plan")
	if err != nil || plan.Model != "big" || plan.Enabled("write") || !plan.Enabled("read") || plan.Instructions == "" {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	r, _ := c.ResolveAgent("review")
	if r.Provider != "claude" || r.Temperature != 0.9 || r.SystemPrompt != c.Agent.SystemPrompt || r.Enabled("write") {
		t.Fatalf("review: %+v", r)
	}
	if _, err := c.ResolveAgent("chat"); err == nil {
		t.Fatal("a disabled agent must not resolve")
	}
	chat, _ := Default().ResolveAgent("chat")
	if chat.Enabled("read") || !chat.Enabled("rag_search") {
		t.Fatal("chat agent tools")
	}
}

func TestPermissions(t *testing.T) {
	c := Default()
	c.Permission = map[string]string{"bash:git *": Allow, "bash:git push*": Ask, "webfetch": Deny}
	a, _ := c.ResolveAgent("build")
	for _, tc := range []struct{ tool, cmd, want string }{
		{"read", "", Allow},
		{"edit", "", Ask},
		{"bash", "ls", Ask},
		{"bash", "git status", Allow},
		{"bash", "git push origin main", Ask},
		{"webfetch", "", Deny},
		{"mcp_github_create_issue", "", Ask},
		{"email_send", "", Ask},
	} {
		if got := a.PermissionFor(tc.tool, tc.cmd); got != tc.want {
			t.Errorf("%s %q = %s, want %s", tc.tool, tc.cmd, got, tc.want)
		}
	}
}

func TestExtensionsValidateAndMask(t *testing.T) {
	c := Default()
	c.Permission = map[string]string{"bash": "maybe"}
	c.MCP = map[string]MCPConfig{"x": {Type: "local"}, "y": {Type: "remote", URL: "ftp://no"}}
	c.Connectors = []ConnectorConfig{{Name: "db", Type: "database", Database: &DatabaseConn{Driver: "oracle"}}}
	err := c.Validate()
	for _, want := range []string{"permission.bash", "mcp.x.command", "mcp.y.url", "database.driver", "database.dsn"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}

	c = Default()
	p := c.Providers["gemini"]
	p.APIKeys = []string{"secret-2", "env:OTHER_KEY"}
	c.Providers["gemini"] = p
	c.MCP = map[string]MCPConfig{"gh": {Type: "local", Command: []string{"x"}, Environment: map[string]string{"TOKEN": "secret-3"}}}
	c.Connectors = []ConnectorConfig{
		{Name: "db", Type: "database", Database: &DatabaseConn{Driver: "postgres", DSN: "postgres://u:secret-4@h/db"}},
		{Name: "mail", Type: "email", Email: &EmailConn{IMAPHost: "imap.x", Username: "me", Password: "secret-5"}},
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(c.Masked())
	if strings.Contains(string(out), "secret") {
		t.Fatalf("a secret leaked: %s", out)
	}
	if !strings.Contains(string(out), "env:OTHER_KEY") {
		t.Fatal("env references are not secrets and stay visible")
	}
	if c.Connectors[0].Database.DSN == MaskedKey {
		t.Fatal("Masked must not change the original")
	}
}

func TestSetOptionalFields(t *testing.T) {
	c, err := Set(Default(), "default_agent", "plan")
	if err != nil || c.DefaultAgent != "plan" {
		t.Fatalf("%v %v", c.DefaultAgent, err)
	}
	if c, err = Set(c, "theme", "light"); err != nil || c.Theme != "light" {
		t.Fatalf("theme: %v", err)
	}
	if _, err = Set(c, "theme", "purple"); err == nil {
		t.Fatal("an invalid theme must be refused")
	}
	// a new agent is created field by field; unset fields stay unset
	if c, err = Set(c, "agents.review.model", "big"); err != nil {
		t.Fatal(err)
	}
	if c, err = Set(c, "agents.review.temperature", "0.3"); err != nil {
		t.Fatal(err)
	}
	a := c.Agents["review"]
	if a.Model != "big" || a.Temperature == nil || *a.Temperature != 0.3 || a.UseRAG != nil {
		t.Fatalf("agent: %+v", a)
	}
	if c, err = Set(c, "agents.plan.provider", "gemini"); err != nil || c.Agents["plan"].Provider != "gemini" {
		t.Fatalf("plan: %v", err)
	}
	if _, err = Set(c, "agents.review.provider", "nope"); err == nil {
		t.Fatal("an unknown provider must be refused")
	}
	if _, err = Set(c, "agents.review.nope", "x"); err == nil {
		t.Fatal("an unknown agent field must be refused")
	}
}

func TestResolveAPIKeys(t *testing.T) {
	t.Setenv("MY_K1", "from-env")
	t.Setenv("MY_K2", "second-env")
	p := ProviderConfig{APIKey: "lit", APIKeys: []string{"lit", "two", "env:MY_K2", "env:MISSING_ONE"}, APIKeyEnv: "MY_K1"}
	if got := strings.Join(p.ResolveAPIKeys(), ","); got != "lit,two,second-env,from-env" {
		t.Fatalf("keys: %s", got)
	}
}

func TestOldConfigStillParses(t *testing.T) {
	old := `{"language":"auto","agent":{"name":"a","system_prompt":"p","provider":"ollama","temperature":0.2,"max_tokens":10,"use_rag":false},
	"providers":{"ollama":{"type":"ollama","model":"m","base_url":"http://localhost:11434","api_key":"","api_key_env":"","embedding_model":"","timeout_seconds":0,"keep_alive":"","num_ctx":0}},
	"rag":{"store_path":"s.json","embedder":"local","chunk_size":100,"chunk_overlap":10,"top_k":2,"min_score":0},"connectors":[],"server":{"addr":"x:1"}}`
	c, err := Parse([]byte(old))
	if err != nil {
		t.Fatal(err)
	}
	if c.DefaultAgentName() != "build" {
		t.Fatal(c.DefaultAgentName())
	}
}
