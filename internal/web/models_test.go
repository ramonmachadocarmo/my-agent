package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"my-agent/internal/config"
)

func ndjson(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("bad ndjson line %q: %v", l, err)
		}
		out = append(out, m)
	}
	return out
}

func TestListModelsEndpoint(t *testing.T) {
	srv, _ := newServer(t)
	code, m, _ := call(t, srv, "GET", "/api/providers/ollama/models", nil)
	models, _ := m["models"].([]any)
	if code != 200 || len(models) != 1 || m["model"] != "llama3.2" {
		t.Fatalf("%d %v", code, m)
	}
	if code, _, _ := call(t, srv, "GET", "/api/providers/claude/models", nil); code != 400 {
		t.Fatalf("non-ollama provider: %d", code)
	}
	if code, _, _ := call(t, srv, "GET", "/api/providers/ghost/models", nil); code != 400 {
		t.Fatalf("unknown provider: %d", code)
	}
}

func TestPullModelEndpointDownloadsAndConfigures(t *testing.T) {
	srv, a := newServer(t)
	code, _, raw := call(t, srv, "POST", "/api/providers/ollama/models/pull", map[string]any{"model": "qwen2.5:7b", "set_as": "model"})
	lines := ndjson(t, raw)
	last := lines[len(lines)-1]
	if code != 200 || last["done"] != true || last["downloaded"] != true || last["configured"] != true {
		t.Fatalf("%d %s", code, raw)
	}
	if lines[0]["status"] != "pulling abc" || lines[0]["completed"].(float64) != 25 {
		t.Fatalf("progress not streamed: %v", lines)
	}
	if a.Cfg.Providers["ollama"].Model != "qwen2.5:7b" {
		t.Fatalf("model not configured: %q", a.Cfg.Providers["ollama"].Model)
	}
	saved, _ := config.Load(a.Path)
	if saved.Providers["ollama"].Model != "qwen2.5:7b" {
		t.Fatal("config not persisted")
	}

	// already installed (llama3.2) + embedding target: no download
	_, _, raw = call(t, srv, "POST", "/api/providers/ollama/models/pull", map[string]any{"model": "llama3.2", "set_as": "embedding"})
	lines = ndjson(t, raw)
	if last := lines[len(lines)-1]; last["downloaded"] != false || a.Cfg.Providers["ollama"].EmbeddingModel != "llama3.2" {
		t.Fatalf("%v / %q", lines, a.Cfg.Providers["ollama"].EmbeddingModel)
	}

	// download only: config untouched
	call(t, srv, "POST", "/api/providers/ollama/models/pull", map[string]any{"model": "phi4"})
	if a.Cfg.Providers["ollama"].Model != "qwen2.5:7b" {
		t.Fatal("download-only must not change config")
	}
}

func TestPullModelEndpointValidation(t *testing.T) {
	srv, _ := newServer(t)
	for name, body := range map[string]map[string]any{
		"empty model": {"model": " "},
		"bad set_as":  {"model": "x", "set_as": "everything"},
	} {
		if code, _, _ := call(t, srv, "POST", "/api/providers/ollama/models/pull", body); code != 400 {
			t.Errorf("%s: got %d", name, code)
		}
	}
	if code, _, _ := call(t, srv, "POST", "/api/providers/claude/models/pull", map[string]any{"model": "x"}); code != 400 {
		t.Errorf("non-ollama: %d", code)
	}
	resp, err := http.Post(srv.URL+"/api/providers/ollama/models/pull", "text/plain", strings.NewReader("model=x"))
	if err != nil || resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("non-JSON must be rejected: %v %v", resp, err)
	}
}

func TestListModelsReportsRunningModelsAndGPUShare(t *testing.T) {
	srv, _ := newServer(t)
	_, m, _ := call(t, srv, "GET", "/api/providers/ollama/models", nil)
	running, _ := m["running"].([]any)
	if len(running) != 1 {
		t.Fatalf("running: %v", m["running"])
	}
	r := running[0].(map[string]any)
	if r["name"] != "llama3.2:latest" || r["size"].(float64) != 2000 || r["size_vram"].(float64) != 800 {
		t.Fatalf("the UI needs size and size_vram to show the CPU offload: %v", r)
	}
}
