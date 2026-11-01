package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/provider"
)

type streamingProvider struct {
	fakeProvider
	fail bool
}

func (s streamingProvider) ChatStream(_ context.Context, _ provider.ChatRequest, onDelta func(string)) (string, error) {
	onDelta("Hel")
	onDelta("lo")
	if s.fail {
		return "Hello", errors.New("model crashed")
	}
	return "Hello", nil
}

func newStreamServer(t *testing.T, fail bool) *httptest.Server {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfg.json")
	if err := config.Default().Save(path); err != nil {
		t.Fatal(err)
	}
	a, err := app.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a.NewProvider = func(string, config.ProviderConfig) (provider.Provider, error) {
		return streamingProvider{fail: fail}, nil
	}
	srv := httptest.NewServer(New(a).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func TestChatStreamEndpoint(t *testing.T) {
	srv := newStreamServer(t, false)
	code, _, raw := call(t, srv, "POST", "/api/chat/stream", map[string]any{"message": "hi", "use_rag": false})
	lines := ndjson(t, raw)
	if code != 200 || len(lines) != 3 {
		t.Fatalf("%d %s", code, raw)
	}
	if lines[0]["delta"] != "Hel" || lines[1]["delta"] != "lo" {
		t.Fatalf("deltas: %v", lines)
	}
	if last := lines[2]; last["done"] != true || last["text"] != "Hello" {
		t.Fatalf("final: %v", last)
	}
}

func TestChatStreamEndpointFallsBackWithoutStreaming(t *testing.T) {
	srv, _ := newServer(t) // its provider cannot stream
	_, _, raw := call(t, srv, "POST", "/api/chat/stream", map[string]any{"message": "yo", "use_rag": false})
	lines := ndjson(t, raw)
	if len(lines) != 2 || lines[0]["delta"] != "echo: yo" || lines[1]["done"] != true {
		t.Fatalf("%v", lines)
	}
}

func TestChatStreamEndpointReportsMidStreamError(t *testing.T) {
	srv := newStreamServer(t, true)
	_, _, raw := call(t, srv, "POST", "/api/chat/stream", map[string]any{"message": "hi", "use_rag": false})
	lines := ndjson(t, raw)
	last := lines[len(lines)-1]
	if last["error"] != "model crashed" || last["done"] != nil {
		t.Fatalf("%v", lines)
	}
}

func TestChatStreamEndpointValidation(t *testing.T) {
	srv, _ := newServer(t)
	if code, _, _ := call(t, srv, "POST", "/api/chat/stream", map[string]any{"message": "  "}); code != 400 {
		t.Errorf("empty message: %d", code)
	}
	if code, _, _ := call(t, srv, "POST", "/api/chat/stream", map[string]any{"message": "x", "provider": "ghost"}); code != 400 {
		t.Errorf("unknown provider: %d", code)
	}
	resp, err := http.Post(srv.URL+"/api/chat/stream", "text/plain", strings.NewReader("message=x"))
	if err != nil || resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("non-JSON must be rejected: %v %v", resp, err)
	}
}
