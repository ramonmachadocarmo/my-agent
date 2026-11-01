package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"my-agent/internal/config"
	"my-agent/internal/i18n"
)

func newTest(t *testing.T, typ string, h http.HandlerFunc, mod func(*config.ProviderConfig)) Provider {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	pc := config.ProviderConfig{Type: typ, Model: "m", BaseURL: srv.URL, APIKey: "k", EmbeddingModel: "emb"}
	if mod != nil {
		mod(&pc)
	}
	p, err := New(typ, pc)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func readBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	b, _ := io.ReadAll(r.Body)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("bad request body %q: %v", b, err)
	}
	return m
}

var req = ChatRequest{System: "sys", Messages: []Message{{Role: "user", Content: "hi"}}, Temperature: 0.5, MaxTokens: 42}

func TestClaudeChat(t *testing.T) {
	p := newTest(t, "claude", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "k" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("bad request: %s %v", r.URL.Path, r.Header)
		}
		b := readBody(t, r)
		if b["system"] != "sys" || b["max_tokens"].(float64) != 42 || b["model"] != "m" {
			t.Errorf("unexpected body: %v", b)
		}
		io.WriteString(w, `{"content":[{"type":"text","text":"hel"},{"type":"text","text":"lo"}]}`)
	}, nil)
	got, err := p.Chat(context.Background(), req)
	if err != nil || got != "hello" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := p.Embed(context.Background(), []string{"x"}); !errors.Is(err, ErrEmbeddingsUnsupported) {
		t.Fatalf("claude embed: %v", err)
	}
}

func TestGeminiChatAndEmbed(t *testing.T) {
	p := newTest(t, "gemini", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "k" {
			t.Error("missing api key header")
		}
		switch r.URL.Path {
		case "/v1beta/models/m:generateContent":
			b := readBody(t, r)
			contents := b["contents"].([]any)
			if contents[0].(map[string]any)["role"] != "user" || b["systemInstruction"] == nil {
				t.Errorf("unexpected body: %v", b)
			}
			io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"gem"}]}}]}`)
		case "/v1beta/models/emb:batchEmbedContents":
			b := readBody(t, r)
			if len(b["requests"].([]any)) != 2 {
				t.Errorf("want 2 requests: %v", b)
			}
			io.WriteString(w, `{"embeddings":[{"values":[1,0]},{"values":[0,1]}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}, nil)
	if got, err := p.Chat(context.Background(), req); err != nil || got != "gem" {
		t.Fatalf("chat: %q, %v", got, err)
	}
	vecs, err := p.Embed(context.Background(), []string{"a", "b"})
	if err != nil || len(vecs) != 2 || vecs[1][1] != 1 {
		t.Fatalf("embed: %v, %v", vecs, err)
	}
}

func TestGeminiMapsAssistantToModelRole(t *testing.T) {
	p := newTest(t, "gemini", func(w http.ResponseWriter, r *http.Request) {
		c := readBody(t, r)["contents"].([]any)
		if c[1].(map[string]any)["role"] != "model" {
			t.Errorf("assistant should map to model: %v", c)
		}
		io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`)
	}, nil)
	_, err := p.Chat(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "a"}, {Role: "assistant", Content: "b"}, {Role: "user", Content: "c"}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOllamaChatAndEmbed(t *testing.T) {
	p := newTest(t, "ollama", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/chat":
			b := readBody(t, r)
			msgs := b["messages"].([]any)
			if msgs[0].(map[string]any)["role"] != "system" || b["stream"] != false {
				t.Errorf("unexpected body: %v", b)
			}
			if b["options"].(map[string]any)["num_predict"].(float64) != 42 {
				t.Errorf("num_predict missing: %v", b)
			}
			io.WriteString(w, `{"message":{"role":"assistant","content":"llama"}}`)
		case "/api/embed":
			io.WriteString(w, `{"embeddings":[[0.5,0.5]]}`)
		}
	}, func(pc *config.ProviderConfig) { pc.APIKey = "" })
	if got, err := p.Chat(context.Background(), req); err != nil || got != "llama" {
		t.Fatalf("chat: %q, %v", got, err)
	}
	if v, err := p.Embed(context.Background(), []string{"a"}); err != nil || v[0][0] != 0.5 {
		t.Fatalf("embed: %v, %v", v, err)
	}
}

func TestHTTPErrorIncludesStatusAndBody(t *testing.T) {
	p := newTest(t, "ollama", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}, nil)
	_, err := p.Chat(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "model not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRequiresKeyForCloudProviders(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	for _, typ := range []string{"claude", "gemini"} {
		_, err := New(typ, config.ProviderConfig{Type: typ, Model: "m", BaseURL: "http://x"})
		if err == nil || !strings.Contains(err.Error(), "API key") {
			t.Errorf("%s: want missing key error, got %v", typ, err)
		}
	}
	if _, err := New("o", config.ProviderConfig{Type: "ollama", Model: "m", BaseURL: "http://x"}); err != nil {
		t.Errorf("ollama needs no key: %v", err)
	}
	if _, err := New("z", config.ProviderConfig{Type: "zzz"}); err == nil {
		t.Error("unknown type should fail")
	}
}

func TestMissingKeyErrorNeverEchoesAMisplacedKey(t *testing.T) {
	const secret = "AQ.SuperSecretKeyValue"
	_, err := New("gemini", config.ProviderConfig{Type: "gemini", Model: "m", BaseURL: "http://x", APIKeyEnv: secret})
	if err == nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "NAME") {
		t.Fatalf("got %v", err)
	}
	// a proper variable name is still shown as a hint
	t.Setenv("GEMINI_API_KEY", "")
	_, err = New("gemini", config.ProviderConfig{Type: "gemini", Model: "m", BaseURL: "http://x", APIKeyEnv: "GEMINI_API_KEY"})
	if err == nil || !strings.Contains(err.Error(), "env GEMINI_API_KEY") {
		t.Fatalf("got %v", err)
	}
}

func TestLocalizedErrorsKeepWorkingWithErrorsIs(t *testing.T) {
	defer i18n.Set(i18n.EN)
	for lang, want := range map[i18n.Lang]string{
		i18n.EN:   "provider does not support embeddings",
		i18n.PtBR: "o provedor não suporta embeddings",
		i18n.ES:   "el proveedor no admite embeddings",
	} {
		i18n.Set(lang)
		p, _ := New("c", config.ProviderConfig{Type: "claude", Model: "m", BaseURL: "http://x", APIKey: "k"})
		_, err := p.Embed(context.Background(), []string{"x"})
		if !errors.Is(err, ErrEmbeddingsUnsupported) || err.Error() != want {
			t.Errorf("%s: %v", lang, err)
		}
	}
}

func TestMissingKeyMessageInEachLanguage(t *testing.T) {
	defer i18n.Set(i18n.EN)
	t.Setenv("GEMINI_API_KEY", "")
	for lang, want := range map[i18n.Lang]string{
		i18n.EN:   "missing API key",
		i18n.PtBR: "falta a chave de API",
		i18n.ES:   "falta la clave de API",
	} {
		i18n.Set(lang)
		_, err := New("gemini", config.ProviderConfig{Type: "gemini", Model: "m", BaseURL: "http://x", APIKeyEnv: "GEMINI_API_KEY"})
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "GEMINI_API_KEY") {
			t.Errorf("%s: %v", lang, err)
		}
	}
}
