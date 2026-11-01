package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/provider"
)

type fakeProvider struct{ models *[]provider.ModelInfo }

func (f fakeProvider) ListModels(context.Context) ([]provider.ModelInfo, error) {
	return *f.models, nil
}
func (f fakeProvider) PullModel(_ context.Context, name string, progress func(provider.PullProgress)) error {
	progress(provider.PullProgress{Status: "pulling abc", Total: 100, Completed: 25})
	progress(provider.PullProgress{Status: "success"})
	*f.models = append(*f.models, provider.ModelInfo{Name: name + ":latest"})
	return nil
}

func (fakeProvider) Name() string { return "fake" }
func (fakeProvider) Chat(_ context.Context, r provider.ChatRequest) (string, error) {
	return "echo: " + r.Messages[len(r.Messages)-1].Content, nil
}
func (fakeProvider) Embed(context.Context, []string) ([][]float32, error) {
	return nil, provider.ErrEmbeddingsUnsupported
}

func newServer(t *testing.T) (*httptest.Server, *app.App) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfg.json")
	if err := config.Default().Save(path); err != nil {
		t.Fatal(err)
	}
	a, err := app.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	models := &[]provider.ModelInfo{{Name: "llama3.2:latest", Size: 1 << 30}}
	a.NewProvider = func(string, config.ProviderConfig) (provider.Provider, error) { return fakeProvider{models}, nil }
	srv := httptest.NewServer(New(a).Handler())
	t.Cleanup(srv.Close)
	return srv, a
}

func call(t *testing.T, srv *httptest.Server, method, path string, body any) (int, map[string]any, []byte) {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, srv.URL+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	var m map[string]any
	json.Unmarshal(buf.Bytes(), &m)
	return resp.StatusCode, m, buf.Bytes()
}

func TestIndexPage(t *testing.T) {
	srv, _ := newServer(t)
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("status %d, type %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if r2, _ := http.Get(srv.URL + "/nope"); r2.StatusCode != 404 {
		t.Fatalf("unknown path should 404, got %d", r2.StatusCode)
	}
}

func TestConfigGetPutKeepsMaskedKey(t *testing.T) {
	srv, a := newServer(t)
	p := a.Cfg.Providers["claude"]
	p.APIKey = "sk-real"
	a.Cfg.Providers["claude"] = p

	code, _, raw := call(t, srv, "GET", "/api/config", nil)
	if code != 200 || bytes.Contains(raw, []byte("sk-real")) {
		t.Fatalf("GET config: %d, leaked=%v", code, bytes.Contains(raw, []byte("sk-real")))
	}
	var cfg config.Config
	json.Unmarshal(raw, &cfg)
	cfg.Agent.Name = "renamed" // key stays masked in the payload
	if code, _, _ := call(t, srv, "PUT", "/api/config", cfg); code != 200 {
		t.Fatalf("PUT: %d", code)
	}
	if a.Cfg.Agent.Name != "renamed" || a.Cfg.Providers["claude"].APIKey != "sk-real" {
		t.Fatalf("masked key must be preserved, got %q", a.Cfg.Providers["claude"].APIKey)
	}
	saved, _ := config.Load(a.Path)
	if saved.Agent.Name != "renamed" {
		t.Fatal("PUT did not persist")
	}

	cfg.RAG.TopK = 0
	code, m, _ := call(t, srv, "PUT", "/api/config", cfg)
	if code != 400 || !strings.Contains(m["error"].(string), "rag.top_k") {
		t.Fatalf("invalid config: %d %v", code, m)
	}
	if a.Cfg.RAG.TopK == 0 {
		t.Fatal("invalid config must not be applied")
	}
}

func TestChat(t *testing.T) {
	srv, _ := newServer(t)
	code, m, _ := call(t, srv, "POST", "/api/chat", map[string]any{"message": "hello", "use_rag": false})
	if code != 200 || m["text"] != "echo: hello" {
		t.Fatalf("%d %v", code, m)
	}
	if code, _, _ := call(t, srv, "POST", "/api/chat", map[string]any{"message": "  "}); code != 400 {
		t.Fatalf("empty message: %d", code)
	}
	if code, _, _ := call(t, srv, "POST", "/api/chat", map[string]any{"message": "x", "provider": "ghost"}); code != 400 {
		t.Fatalf("unknown provider: %d", code)
	}
}

func TestRAGEndpoints(t *testing.T) {
	srv, _ := newServer(t)
	code, m, _ := call(t, srv, "POST", "/api/rag/ingest", map[string]any{"name": "note.txt", "text": "The launch code word is pineapple."})
	if code != 200 || m["chunks"].(float64) != 1 {
		t.Fatalf("ingest: %d %v", code, m)
	}
	if code, _, _ := call(t, srv, "POST", "/api/rag/ingest", map[string]any{"text": "no name"}); code != 400 {
		t.Fatalf("text without name: %d", code)
	}
	_, m, _ = call(t, srv, "GET", "/api/rag", nil)
	if m["chunks"].(float64) != 1 || len(m["sources"].([]any)) != 1 {
		t.Fatalf("status: %v", m)
	}
	code, _, raw := call(t, srv, "POST", "/api/rag/search", map[string]any{"query": "code word pineapple"})
	if code != 200 || !bytes.Contains(raw, []byte("note.txt")) {
		t.Fatalf("search: %d %s", code, raw)
	}
	_, m, _ = call(t, srv, "POST", "/api/rag/remove", map[string]any{"source": "note.txt"})
	if m["removed"].(float64) != 1 {
		t.Fatalf("remove: %v", m)
	}
	call(t, srv, "POST", "/api/rag/ingest", map[string]any{"name": "x", "text": "abc"})
	if code, _, _ := call(t, srv, "POST", "/api/rag/clear", map[string]any{}); code != 200 {
		t.Fatal("clear failed")
	}
	_, m, _ = call(t, srv, "GET", "/api/rag", nil)
	if m["chunks"].(float64) != 0 {
		t.Fatalf("after clear: %v", m)
	}
}

func TestConnectorEndpoints(t *testing.T) {
	srv, a := newServer(t)
	dir := t.TempDir()
	if code, _, _ := call(t, srv, "POST", "/api/connectors", map[string]any{"name": "d", "type": "file", "path": dir}); code != 200 {
		t.Fatalf("add: %d", code)
	}
	if code, _, _ := call(t, srv, "POST", "/api/connectors", map[string]any{"name": "d", "type": "file", "path": dir}); code != 400 {
		t.Fatalf("duplicate: %d", code)
	}
	if code, _, _ := call(t, srv, "POST", "/api/connectors/d/sync", map[string]any{}); code != 200 {
		t.Fatalf("sync: %d", code)
	}
	if code, _, _ := call(t, srv, "POST", "/api/connectors/ghost/sync", map[string]any{}); code != 404 {
		t.Fatalf("sync unknown: %d", code)
	}
	if code, _, _ := call(t, srv, "DELETE", "/api/connectors/d", nil); code != 200 || len(a.Cfg.Connectors) != 0 {
		t.Fatalf("delete: %d %v", code, a.Cfg.Connectors)
	}
	if code, _, _ := call(t, srv, "DELETE", "/api/connectors/d", nil); code != 404 {
		t.Fatalf("delete twice: %d", code)
	}
}

func TestRejectsNonJSONPost(t *testing.T) {
	srv, _ := newServer(t)
	resp, err := http.Post(srv.URL+"/api/rag/clear", "application/x-www-form-urlencoded", strings.NewReader("a=b"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func TestProvidersAndTest(t *testing.T) {
	srv, _ := newServer(t)
	code, _, raw := call(t, srv, "GET", "/api/providers", nil)
	if code != 200 || !bytes.Contains(raw, []byte("ollama")) {
		t.Fatalf("%d %s", code, raw)
	}
	if code, m, _ := call(t, srv, "POST", "/api/providers/ollama/test", nil); code != 200 || m["ok"] != true {
		t.Fatalf("test: %d %v", code, m)
	}
	if code, _, _ := call(t, srv, "POST", "/api/providers/ghost/test", nil); code != 400 {
		t.Fatalf("ghost: %d", code)
	}
}

func (f fakeProvider) RunningModels(context.Context) ([]provider.RunningModel, error) {
	return []provider.RunningModel{{Name: "llama3.2:latest", Size: 2000, SizeVRAM: 800, ContextLength: 4096}}, nil
}

func TestPutConfigKeepsHiddenMisplacedKeyEnv(t *testing.T) {
	srv, a := newServer(t)
	g := a.Cfg.Providers["gemini"]
	g.APIKeyEnv = "AQ.pasted-secret"
	a.Cfg.Providers["gemini"] = g

	code, _, raw := call(t, srv, "GET", "/api/config", nil)
	if code != 200 || bytes.Contains(raw, []byte("pasted-secret")) {
		t.Fatalf("GET must not leak it: %d %s", code, raw)
	}
	var cfg config.Config
	json.Unmarshal(raw, &cfg)
	cfg.Agent.Name = "renamed"
	if code, _, _ := call(t, srv, "PUT", "/api/config", cfg); code != 200 {
		t.Fatalf("PUT: %d", code)
	}
	if got := a.Cfg.Providers["gemini"].APIKeyEnv; got != "AQ.pasted-secret" {
		t.Fatalf("the masked placeholder must not overwrite the stored value, got %q", got)
	}
}
