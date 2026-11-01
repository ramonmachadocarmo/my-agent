package cli

import (
	"strings"
	"testing"

	"my-agent/internal/provider"
)

func TestModelsCommands(t *testing.T) {
	cfg, fp := setup(t)
	fp.models = []provider.ModelInfo{{Name: "llama3.2:latest", Size: 2 << 30}, {Name: "qwen2.5:7b", Size: 4 << 30}}
	mustRun(t, "", "-config", cfg, "init")
	get := func(key string) string {
		return strings.TrimSpace(mustRun(t, "", "-config", cfg, "config", "get", key))
	}

	out := mustRun(t, "", "-config", cfg, "models", "list")
	if !strings.Contains(out, "qwen2.5:7b") || !strings.Contains(out, "llama3.2:latest") || !strings.Contains(out, "<- chat") {
		t.Fatalf("list:\n%s", out)
	}

	// installed model: no download, config updated
	out = mustRun(t, "", "-config", cfg, "models", "use", "qwen2.5:7b")
	if !strings.Contains(out, "already installed") || len(fp.pulled) != 0 {
		t.Fatalf("use installed: %s / pulled=%v", out, fp.pulled)
	}
	if got := get("providers.ollama.model"); got != "qwen2.5:7b" {
		t.Fatalf("model not configured: %q", got)
	}

	// missing model: downloaded, then configured (embedding flavour)
	out = mustRun(t, "", "-config", cfg, "models", "use", "--embedding", "bge-m3")
	if len(fp.pulled) != 1 || fp.pulled[0] != "bge-m3" || !strings.Contains(out, "downloaded bge-m3") || !strings.Contains(out, "50%") {
		t.Fatalf("use missing: %s / pulled=%v", out, fp.pulled)
	}
	if got := get("providers.ollama.embedding_model"); got != "bge-m3" {
		t.Fatalf("embedding model: %q", got)
	}

	// pull only downloads; config untouched
	mustRun(t, "", "-config", cfg, "models", "pull", "mistral")
	if got := get("providers.ollama.model"); got != "qwen2.5:7b" {
		t.Fatalf("pull must not configure: %q", got)
	}
}

func TestModelsSelect(t *testing.T) {
	cfg, fp := setup(t)
	fp.models = []provider.ModelInfo{{Name: "llama3.2:latest"}, {Name: "qwen2.5:7b"}}
	mustRun(t, "", "-config", cfg, "init")
	model := func() string {
		return strings.TrimSpace(mustRun(t, "", "-config", cfg, "config", "get", "providers.ollama.model"))
	}

	out := mustRun(t, "2\n", "-config", cfg, "models", "select")
	if !strings.Contains(out, "1) llama3.2:latest") || !strings.Contains(out, "(current)") || !strings.Contains(out, "0) Other") || model() != "qwen2.5:7b" {
		t.Fatalf("pick by number: model=%s\n%s", model(), out)
	}
	mustRun(t, "0\nphi4\n", "-config", cfg, "models", "select") // "other": type a name
	if model() != "phi4" || len(fp.pulled) != 1 {
		t.Fatalf("other: model=%s pulled=%v", model(), fp.pulled)
	}
	mustRun(t, "gemma3\n", "-config", cfg, "models", "select") // typing a name directly
	if model() != "gemma3" {
		t.Fatalf("typed name: %s", model())
	}
	if code, _, _ := run(t, "9\n", "-config", cfg, "models", "select"); code != 1 {
		t.Fatalf("out-of-range choice should fail, got %d", code)
	}
	if code, _, _ := run(t, "", "-config", cfg, "models", "select"); code != 1 {
		t.Fatalf("empty input should fail, got %d", code)
	}
}

func TestModelsSelectStartsChat(t *testing.T) {
	cfg, fp := setup(t)
	fp.models = []provider.ModelInfo{{Name: "llama3.2:latest"}, {Name: "qwen2.5:7b"}}
	mustRun(t, "", "-config", cfg, "init")

	// same stdin: "2" picks the model, the following lines go to the chat
	out := mustRun(t, "2\nhello there\n/exit\n", "-config", cfg, "models", "select")
	if !strings.Contains(out, "providers.ollama.model = qwen2.5:7b") || !strings.Contains(out, "agent> echo: hello there") {
		t.Fatalf("chat did not start after select:\n%s", out)
	}
	// the chat must use the freshly configured model
	if got := strings.TrimSpace(mustRun(t, "", "-config", cfg, "config", "get", "providers.ollama.model")); got != "qwen2.5:7b" {
		t.Fatalf("model: %q", got)
	}

	out = mustRun(t, "1\nhello\n", "-config", cfg, "models", "select", "--no-chat")
	if strings.Contains(out, "agent>") || strings.Contains(out, "ready (") {
		t.Fatalf("--no-chat must not start the chat:\n%s", out)
	}
}

func TestChatSlashCommands(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	out := mustRun(t, "/help\n/model\n/sources\n/exit\n", "-config", cfg, "chat", "--no-rag")
	for _, want := range []string{"Commands", "1.1   chat", "ollama · llama3.2", "show sources: true"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestModelsSelectWithNoInstalledModels(t *testing.T) {
	cfg, fp := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	out := mustRun(t, "llama3.2\n", "-config", cfg, "models", "select")
	if !strings.Contains(out, "No models installed") || len(fp.pulled) != 1 {
		t.Fatalf("pulled=%v\n%s", fp.pulled, out)
	}
}

func TestModelsRejectsNonOllamaProvider(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	if code, _, e := run(t, "", "-config", cfg, "models", "list", "--provider", "claude"); code != 1 || !strings.Contains(e, "only available for ollama") {
		t.Fatalf("%d %s", code, e)
	}
}

func TestFormatBytes(t *testing.T) {
	for in, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KiB", 2 << 30: "2.0 GiB"} {
		if got := formatBytes(in); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", in, got, want)
		}
	}
}
