package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/i18n"
	"my-agent/internal/provider"
)

type fakeProvider struct {
	last   provider.ChatRequest
	all    []provider.ChatRequest // every request, in order
	models []provider.ModelInfo
	pulled []string
}

func (f *fakeProvider) ListModels(context.Context) ([]provider.ModelInfo, error) {
	return f.models, nil
}
func (f *fakeProvider) PullModel(_ context.Context, name string, progress func(provider.PullProgress)) error {
	f.pulled = append(f.pulled, name)
	progress(provider.PullProgress{Status: "pulling abc", Total: 100, Completed: 50})
	progress(provider.PullProgress{Status: "success"})
	f.models = append(f.models, provider.ModelInfo{Name: name + ":latest", Size: 1 << 30})
	return nil
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Chat(_ context.Context, r provider.ChatRequest) (string, error) {
	f.last = r
	f.all = append(f.all, r)
	return "echo: " + r.Messages[len(r.Messages)-1].Content, nil
}
func (f *fakeProvider) Embed(context.Context, []string) ([][]float32, error) {
	return nil, provider.ErrEmbeddingsUnsupported
}

// setup returns a config path in a temp dir and swaps in the fake provider.
func setup(t *testing.T) (cfg string, fp *fakeProvider) {
	t.Helper()
	i18n.Set(i18n.EN) // a previous test may have left another language selected
	t.Cleanup(func() { i18n.Set(i18n.EN) })
	fp = &fakeProvider{}
	orig := loadApp
	loadApp = func(path string) (*app.App, error) {
		a, err := app.Load(path)
		if err == nil {
			a.NewProvider = func(string, config.ProviderConfig) (provider.Provider, error) { return fp, nil }
		}
		return a, err
	}
	t.Cleanup(func() { loadApp = orig })
	cfg = filepath.Join(t.TempDir(), "my-agent.json")
	return cfg, fp
}

func run(t *testing.T, stdin string, args ...string) (code int, out, errOut string) {
	t.Helper()
	var o, e bytes.Buffer
	code = Run(args, strings.NewReader(stdin), &o, &e)
	return code, o.String(), e.String()
}

func mustRun(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	code, out, errOut := run(t, stdin, args...)
	if code != 0 {
		t.Fatalf("%v exited %d\nstdout: %s\nstderr: %s", args, code, out, errOut)
	}
	return out
}

func TestNoArgsAndVersionAndUnknown(t *testing.T) {
	if out := mustRun(t, ""); !strings.Contains(out, "Usage:") {
		t.Fatalf("usage missing: %s", out)
	}
	if out := mustRun(t, "", "version"); !strings.Contains(out, Version) {
		t.Fatalf("version: %s", out)
	}
	if code, _, e := run(t, "", "bogus"); code != 2 || !strings.Contains(e, "unknown command") {
		t.Fatalf("unknown command: %d %s", code, e)
	}
}

func TestInitAndConfig(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	if _, err := os.Stat(cfg); err != nil {
		t.Fatal(err)
	}
	if code, _, e := run(t, "", "-config", cfg, "init"); code == 0 || !strings.Contains(e, "already exists") {
		t.Fatalf("second init should fail: %d %s", code, e)
	}
	mustRun(t, "", "-config", cfg, "init", "--force")

	mustRun(t, "", "-config", cfg, "config", "set", "rag.top_k", "7")
	if got := strings.TrimSpace(mustRun(t, "", "-config", cfg, "config", "get", "rag.top_k")); got != "7" {
		t.Fatalf("get: %q", got)
	}
	mustRun(t, "", "-config", cfg, "config", "set", "providers.claude.api_key", "sk-secret")
	if out := mustRun(t, "", "-config", cfg, "config", "show"); strings.Contains(out, "sk-secret") || !strings.Contains(out, config.MaskedKey) {
		t.Fatalf("show leaked or missed mask:\n%s", out)
	}
	if code, _, _ := run(t, "", "-config", cfg, "config", "set", "rag.top_k", "0"); code != 1 {
		t.Fatalf("invalid set should exit 1, got %d", code)
	}
	if got := strings.TrimSpace(mustRun(t, "", "-config", cfg, "config", "get", "rag.top_k")); got != "7" {
		t.Fatalf("failed set must not persist: %q", got)
	}
	mustRun(t, "", "-config", cfg, "config", "validate")
}

func TestMissingConfigHint(t *testing.T) {
	cfg, _ := setup(t)
	if code, _, e := run(t, "", "-config", cfg, "ask", "hi"); code != 1 || !strings.Contains(e, "my-agent init") {
		t.Fatalf("%d %s", code, e)
	}
}

func TestEnvConfigPath(t *testing.T) {
	cfg, _ := setup(t)
	t.Setenv(config.EnvConfigPath, cfg)
	if got := strings.TrimSpace(mustRun(t, "", "config", "path")); got != cfg {
		t.Fatalf("path: %q", got)
	}
}

func TestProvidersList(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	out := mustRun(t, "", "-config", cfg, "providers", "list")
	for _, n := range []string{"claude", "gemini", "ollama"} {
		if !strings.Contains(out, n) {
			t.Errorf("missing %s in:\n%s", n, out)
		}
	}
	if out := mustRun(t, "", "-config", cfg, "providers", "test", "ollama"); !strings.Contains(out, "OK") {
		t.Fatalf("test: %s", out)
	}
}

func TestAskAndChat(t *testing.T) {
	cfg, fp := setup(t)
	mustRun(t, "", "-config", cfg, "init")

	if out := mustRun(t, "", "-config", cfg, "ask", "--no-rag", "hello", "world"); strings.TrimSpace(out) != "echo: hello world" {
		t.Fatalf("ask: %q", out)
	}

	out := mustRun(t, "one\n/reset\ntwo\n/exit\n", "-config", cfg, "chat", "--no-rag")
	if !strings.Contains(out, "agent> echo: one") || !strings.Contains(out, "agent> echo: two") || !strings.Contains(out, "history cleared") {
		t.Fatalf("chat output:\n%s", out)
	}
	if len(fp.last.Messages) != 1 {
		t.Fatalf("history should be reset before 'two', sent %d messages", len(fp.last.Messages))
	}
}

func TestRAGWorkflowAndAskUsesContext(t *testing.T) {
	cfg, fp := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	docs := filepath.Join(filepath.Dir(cfg), "docs")
	os.MkdirAll(docs, 0o755)
	os.WriteFile(filepath.Join(docs, "warp.md"), []byte("The warp core needs dilithium crystals to operate."), 0o644)
	os.WriteFile(filepath.Join(docs, "fruit.txt"), []byte("Bananas and apples are fruits."), 0o644)

	if out := mustRun(t, "", "-config", cfg, "rag", "ingest", docs); !strings.Contains(out, "2 document(s)") {
		t.Fatalf("ingest: %s", out)
	}
	if out := mustRun(t, "", "-config", cfg, "rag", "list"); !strings.Contains(out, "2 source(s)") || !strings.Contains(out, "local-hash") {
		t.Fatalf("list: %s", out)
	}
	if out := mustRun(t, "", "-config", cfg, "rag", "search", "dilithium", "crystals"); !strings.Contains(out, "warp.md") {
		t.Fatalf("search: %s", out)
	}
	out := mustRun(t, "", "-config", cfg, "ask", "--sources", "what does the warp core need?")
	if !strings.Contains(fp.last.System, "dilithium") || !strings.Contains(out, "Sources:") {
		t.Fatalf("context not used.\nsystem: %s\nout: %s", fp.last.System, out)
	}

	src := filepath.Join(docs, "fruit.txt")
	abs, _ := filepath.Abs(src)
	if out := mustRun(t, "", "-config", cfg, "rag", "remove", abs); !strings.Contains(out, "removed 1") {
		t.Fatalf("remove: %s", out)
	}
	mustRun(t, "", "-config", cfg, "rag", "clear")
	if out := mustRun(t, "", "-config", cfg, "rag", "search", "warp"); !strings.Contains(out, "no results") {
		t.Fatalf("after clear: %s", out)
	}
}

func TestConnectorsLifecycle(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	docs := filepath.Join(filepath.Dir(cfg), "kb")
	os.MkdirAll(docs, 0o755)
	os.WriteFile(filepath.Join(docs, "n.md"), []byte("connector content"), 0o644)

	mustRun(t, "", "-config", cfg, "connectors", "add", "--name", "kb", "--type", "file", "--path", docs)
	if out := mustRun(t, "", "-config", cfg, "connectors", "list"); !strings.Contains(out, "kb") {
		t.Fatalf("list: %s", out)
	}
	if code, _, _ := run(t, "", "-config", cfg, "connectors", "add", "--name", "kb", "--type", "file", "--path", docs); code == 0 {
		t.Fatal("duplicate name should fail")
	}
	if code, _, _ := run(t, "", "-config", cfg, "connectors", "add", "--name", "bad", "--type", "url"); code == 0 {
		t.Fatal("url connector without url should fail")
	}
	if out := mustRun(t, "", "-config", cfg, "connectors", "sync"); !strings.Contains(out, "kb: 1 document(s)") {
		t.Fatalf("sync: %s", out)
	}
	mustRun(t, "", "-config", cfg, "connectors", "remove", "kb")
	if code, _, _ := run(t, "", "-config", cfg, "connectors", "remove", "kb"); code == 0 {
		t.Fatal("removing twice should fail")
	}
}

func TestCompletion(t *testing.T) {
	if out := mustRun(t, "", "completion", "bash"); !strings.Contains(out, "complete -F") {
		t.Fatalf("bash: %s", out)
	}
	if out := mustRun(t, "", "completion", "powershell"); !strings.Contains(out, "Register-ArgumentCompleter") {
		t.Fatalf("powershell: %s", out)
	}
	if out := mustRun(t, "", "completion", "zsh"); !strings.Contains(out, "bashcompinit") {
		t.Fatalf("zsh: %s", out)
	}
	if out := mustRun(t, "", "completion", "fish"); !strings.Contains(out, "complete -c my-agent") {
		t.Fatalf("fish: %s", out)
	}
	if code, _, _ := run(t, "", "completion", "tcsh"); code != 2 {
		t.Fatalf("unsupported shell exit = %d", code)
	}
}

func TestProvidersAddAcceptsNameBeforeOrAfterFlags(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	mustRun(t, "", "-config", cfg, "providers", "add", "openrouter", "--key", "k1")
	mustRun(t, "", "-config", cfg, "providers", "add", "--key", "k2", "openrouter")
	mustRun(t, "", "-config", cfg, "providers", "add", "lab", "--preset", "openai", "--base-url", "http://localhost:8000/v1", "--model", "m")
	c, err := config.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if p := c.Providers["openrouter"]; p.APIKey != "k1" || len(p.APIKeys) != 1 || p.APIKeys[0] != "k2" {
		t.Fatalf("openrouter: %+v", p)
	}
	if p := c.Providers["lab"]; p.Type != "openai" || p.BaseURL != "http://localhost:8000/v1" || p.Model != "m" {
		t.Fatalf("lab: %+v", p)
	}
	if code, _, _ := run(t, "", "-config", cfg, "providers", "add", "a", "b"); code != 2 {
		t.Fatalf("two names must be a usage error, got %d", code)
	}
	if code, _, _ := run(t, "", "-config", cfg, "providers", "add", "nosuchpreset"); code == 0 {
		t.Fatal("an unknown preset must fail")
	}
}
