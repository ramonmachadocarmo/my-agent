package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "cfg.json")
	c := Default()
	c.Agent.Name = "bob"
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Agent.Name != "bob" || got.Providers["claude"].Model != c.Providers["claude"].Model {
		t.Fatalf("round trip mismatch: %+v", got.Agent)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if !os.IsNotExist(err) {
		t.Fatalf("want not-exist error, got %v", err)
	}
}

func TestParseRejectsUnknownFields(t *testing.T) {
	if _, err := Parse([]byte(`{"bogus":1}`)); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestValidateReportsAllProblems(t *testing.T) {
	c := Default()
	c.Agent.Provider = "nope"
	c.RAG.ChunkSize = 0
	c.RAG.Embedder = "ghost"
	c.Connectors = []ConnectorConfig{{Name: "a", Type: "file"}, {Name: "a", Type: "url", URL: "ftp://x"}}
	err := c.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"agent.provider", "rag.chunk_size", "rag.embedder", "connectors[0].path", "connectors[1].name", "connectors[1].url"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%v", want, err)
		}
	}
}

func TestSetCoercesTypes(t *testing.T) {
	c := Default()
	tests := []struct {
		key, val string
		check    func(Config) bool
	}{
		{"agent.provider", "claude", func(c Config) bool { return c.Agent.Provider == "claude" }},
		{"agent.temperature", "0.7", func(c Config) bool { return c.Agent.Temperature == 0.7 }},
		{"rag.top_k", "9", func(c Config) bool { return c.RAG.TopK == 9 }},
		{"agent.use_rag", "false", func(c Config) bool { return !c.Agent.UseRAG }},
		{"providers.claude.model", "x-model", func(c Config) bool { return c.Providers["claude"].Model == "x-model" }},
	}
	for _, tc := range tests {
		got, err := Set(c, tc.key, tc.val)
		if err != nil {
			t.Errorf("Set(%s): %v", tc.key, err)
		} else if !tc.check(got) {
			t.Errorf("Set(%s=%s) not applied", tc.key, tc.val)
		}
	}
}

func TestSetErrors(t *testing.T) {
	c := Default()
	for _, tc := range [][2]string{
		{"nope.key", "1"},
		{"rag.nope", "1"},
		{"rag.top_k", "abc"},
		{"rag.top_k", "1.5"},   // not an int
		{"rag.top_k", "0"},     // fails validation
		{"agent.use_rag", "x"}, // not a bool
		{"connectors", "x"},    // lists are not settable
		{"agent.provider", "ghost"},
	} {
		if _, err := Set(c, tc[0], tc[1]); err == nil {
			t.Errorf("Set(%s=%s) should fail", tc[0], tc[1])
		}
	}
}

func TestGet(t *testing.T) {
	c := Default()
	v, err := Get(c, "rag.chunk_size")
	if err != nil || v.(float64) != 800 {
		t.Fatalf("got %v, %v", v, err)
	}
	if _, err := Get(c, "rag.zzz"); err == nil {
		t.Fatal("expected unknown key error")
	}
}

func TestMaskedHidesKeys(t *testing.T) {
	c := Default()
	p := c.Providers["claude"]
	p.APIKey = "secret"
	c.Providers["claude"] = p
	if got := c.Masked().Providers["claude"].APIKey; got != MaskedKey {
		t.Fatalf("key not masked: %q", got)
	}
	if c.Providers["claude"].APIKey != "secret" {
		t.Fatal("Masked mutated the original")
	}
}

func TestResolveAPIKey(t *testing.T) {
	t.Setenv("MY_TEST_KEY", "from-env")
	if got := (ProviderConfig{APIKeyEnv: "MY_TEST_KEY"}).ResolveAPIKey(); got != "from-env" {
		t.Fatalf("got %q", got)
	}
	if got := (ProviderConfig{APIKey: "lit", APIKeyEnv: "MY_TEST_KEY"}).ResolveAPIKey(); got != "lit" {
		t.Fatalf("literal should win, got %q", got)
	}
}

func TestKeepAliveAndNumCtxValidation(t *testing.T) {
	c := Default()
	for _, ok := range []string{"", "30m", "1h30m", "600", "-1", "-1m"} {
		pc := c.Providers["ollama"]
		pc.KeepAlive = ok
		c.Providers["ollama"] = pc
		if err := c.Validate(); err != nil {
			t.Errorf("keep_alive %q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"soon", "5 minutes", "1x"} {
		if _, err := Set(Default(), "providers.ollama.keep_alive", bad); err == nil {
			t.Errorf("keep_alive %q should be rejected", bad)
		}
	}
	if _, err := Set(Default(), "providers.ollama.num_ctx", "-5"); err == nil {
		t.Error("negative num_ctx should be rejected")
	}
	got, err := Set(Default(), "providers.ollama.num_ctx", "2048")
	if err != nil || got.Providers["ollama"].NumCtx != 2048 {
		t.Fatalf("num_ctx: %v %v", got.Providers["ollama"].NumCtx, err)
	}
	if Default().Providers["ollama"].KeepAlive != "30m" {
		t.Error("ollama should keep the model loaded for 30m by default")
	}
}

func TestOldConfigWithoutNewFieldsStillLoads(t *testing.T) {
	// a config written before keep_alive/num_ctx existed
	old := `{"agent":{"name":"a","system_prompt":"s","provider":"ollama","temperature":0.2,"max_tokens":100,"use_rag":true},
	"providers":{"ollama":{"type":"ollama","model":"m","base_url":"http://x","api_key":"","api_key_env":"","embedding_model":"","timeout_seconds":120}},
	"rag":{"store_path":"d","embedder":"local","chunk_size":100,"chunk_overlap":10,"top_k":3,"min_score":0.1},
	"connectors":[],"server":{"addr":"127.0.0.1:1"}}`
	c, err := Parse([]byte(old))
	if err != nil {
		t.Fatal(err)
	}
	if c.Providers["ollama"].KeepAlive != "" || c.Providers["ollama"].NumCtx != 0 {
		t.Fatalf("missing fields must default to zero values: %+v", c.Providers["ollama"])
	}
}

func TestEnvNameValidationAndMisplacedKeys(t *testing.T) {
	for s, want := range map[string]bool{
		"": true, "GEMINI_API_KEY": true, "_x1": true,
		"AQ.Ab8RN6-KoDEJ": false, "sk-ant-api03-xyz": false, "has space": false, "1ABC": false,
	} {
		if got := ValidEnvName(s); got != want {
			t.Errorf("ValidEnvName(%q) = %v, want %v", s, got, want)
		}
	}
	c := Default()
	if w := c.Warnings(); len(w) != 0 {
		t.Fatalf("default config must not warn: %v", w)
	}
	g := c.Providers["gemini"]
	g.APIKeyEnv = "AQ.SECRET-value"
	c.Providers["gemini"] = g
	if err := c.Validate(); err != nil {
		t.Fatalf("a misplaced key is a warning, not an error (so the config can still be fixed): %v", err)
	}
	w := c.Warnings()
	if len(w) != 1 || !strings.Contains(w[0], "providers.gemini.api_key_env") {
		t.Fatalf("warnings: %v", w)
	}
	if strings.Contains(w[0], "SECRET") {
		t.Fatal("a warning must never contain the value")
	}
	if got := c.Masked().Providers["gemini"].APIKeyEnv; got != MaskedKey {
		t.Fatalf("misplaced key must be masked, got %q", got)
	}
	if got := c.Masked().Providers["claude"].APIKeyEnv; got != "ANTHROPIC_API_KEY" {
		t.Fatalf("a real variable name must stay visible, got %q", got)
	}
}
