package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"my-agent/internal/config"
)

// lines writes each line followed by a newline, flushing so the client sees
// them as separate chunks.
func lines(w http.ResponseWriter, ls ...string) {
	fl, _ := w.(http.Flusher)
	for _, l := range ls {
		io.WriteString(w, l+"\n")
		if fl != nil {
			fl.Flush()
		}
	}
}

func collect(t *testing.T, p Provider) (string, []string, error) {
	t.Helper()
	var deltas []string
	got, err := ChatStream(context.Background(), p, req, func(d string) { deltas = append(deltas, d) })
	return got, deltas, err
}

func TestOllamaChatStream(t *testing.T) {
	p := newTest(t, "ollama", func(w http.ResponseWriter, r *http.Request) {
		b := readBody(t, r)
		if r.URL.Path != "/api/chat" || b["stream"] != true {
			t.Errorf("bad request %s %v", r.URL.Path, b)
		}
		lines(w,
			`{"message":{"role":"assistant","content":"Hel"}}`,
			`{"message":{"content":"lo"}}`,
			`{"message":{"content":""},"done":true}`)
	}, nil)
	got, deltas, err := collect(t, p)
	if err != nil || got != "Hello" || strings.Join(deltas, "|") != "Hel|lo" {
		t.Fatalf("got %q %v %v", got, deltas, err)
	}
}

func TestOllamaStreamErrorLine(t *testing.T) {
	p := newTest(t, "ollama", func(w http.ResponseWriter, r *http.Request) {
		lines(w, `{"message":{"content":"par"}}`, `{"error":"model requires more system memory"}`)
	}, nil)
	got, _, err := collect(t, p)
	if err == nil || !strings.Contains(err.Error(), "more system memory") || got != "par" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestClaudeChatStream(t *testing.T) {
	p := newTest(t, "claude", func(w http.ResponseWriter, r *http.Request) {
		b := readBody(t, r)
		if b["stream"] != true || r.Header.Get("x-api-key") != "k" {
			t.Errorf("bad request: %v %v", b, r.Header)
		}
		lines(w,
			`event: message_start`, `data: {"type":"message_start"}`, ``,
			`event: content_block_delta`, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hel"}}`, ``,
			`event: content_block_delta`, `data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{}"}}`, ``,
			`event: content_block_delta`, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"lo"}}`, ``,
			`event: message_stop`, `data: {"type":"message_stop"}`, ``)
	}, nil)
	got, deltas, err := collect(t, p)
	if err != nil || got != "Hello" || strings.Join(deltas, "|") != "Hel|lo" {
		t.Fatalf("got %q %v %v", got, deltas, err)
	}
}

func TestClaudeStreamErrorEventAndEmpty(t *testing.T) {
	p := newTest(t, "claude", func(w http.ResponseWriter, r *http.Request) {
		lines(w, `data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"partial"}}`,
			`data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
	}, nil)
	got, _, err := collect(t, p)
	if err == nil || !strings.Contains(err.Error(), "Overloaded") || got != "partial" {
		t.Fatalf("got %q, %v", got, err)
	}
	empty := newTest(t, "claude", func(w http.ResponseWriter, r *http.Request) {
		lines(w, `data: {"type":"message_stop"}`)
	}, nil)
	if _, _, err := collect(t, empty); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty stream should fail: %v", err)
	}
}

func TestGeminiChatStream(t *testing.T) {
	p := newTest(t, "gemini", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/m:streamGenerateContent" || r.URL.Query().Get("alt") != "sse" {
			t.Errorf("bad url %s", r.URL)
		}
		if r.Header.Get("x-goog-api-key") != "k" {
			t.Error("missing key")
		}
		lines(w,
			`data: {"candidates":[{"content":{"parts":[{"text":"Hel"}]}}]}`, ``,
			`data: {"candidates":[{"content":{"parts":[{"text":"lo"}]}}]}`, ``)
	}, nil)
	got, deltas, err := collect(t, p)
	if err != nil || got != "Hello" || strings.Join(deltas, "|") != "Hel|lo" {
		t.Fatalf("got %q %v %v", got, deltas, err)
	}
}

func TestStreamHTTPError(t *testing.T) {
	p := newTest(t, "claude", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid x-api-key"}`, http.StatusUnauthorized)
	}, nil)
	if _, _, err := collect(t, p); err == nil || !strings.Contains(err.Error(), "http 401") {
		t.Fatalf("got %v", err)
	}
}

func TestStreamIdleTimeout(t *testing.T) {
	p := newTest(t, "ollama", func(w http.ResponseWriter, r *http.Request) {
		lines(w, `{"message":{"content":"first"}}`)
		select { // then go silent
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}, func(pc *config.ProviderConfig) { pc.TimeoutSeconds = 1 })
	start := time.Now()
	got, _, err := collect(t, p)
	if err == nil || !strings.Contains(err.Error(), "no data received") {
		t.Fatalf("want idle timeout error, got %v", err)
	}
	if got != "first" || time.Since(start) > 5*time.Second {
		t.Fatalf("partial text %q, took %s", got, time.Since(start))
	}
}

func TestStreamKeepsFlowingPastTheIdleTimeout(t *testing.T) {
	// total time (1.6s) exceeds the 1s idle limit, but data keeps arriving
	p := newTest(t, "ollama", func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 4; i++ {
			lines(w, `{"message":{"content":"x"}}`)
			time.Sleep(400 * time.Millisecond)
		}
	}, func(pc *config.ProviderConfig) { pc.TimeoutSeconds = 1 })
	got, _, err := collect(t, p)
	if err != nil || got != "xxxx" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestStreamCancelledByCaller(t *testing.T) {
	p := newTest(t, "ollama", func(w http.ResponseWriter, r *http.Request) {
		lines(w, `{"message":{"content":"a"}}`)
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := ChatStream(ctx, p, req, func(string) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

type plainProvider struct{}

func (plainProvider) Name() string { return "plain" }
func (plainProvider) Chat(context.Context, ChatRequest) (string, error) {
	return "whole answer", nil
}
func (plainProvider) Embed(context.Context, []string) ([][]float32, error) { return nil, nil }

func TestChatStreamFallsBackToChat(t *testing.T) {
	var deltas []string
	got, err := ChatStream(context.Background(), plainProvider{}, req, func(d string) { deltas = append(deltas, d) })
	if err != nil || got != "whole answer" || len(deltas) != 1 || deltas[0] != "whole answer" {
		t.Fatalf("got %q %v %v", got, deltas, err)
	}
	if _, err := ChatStream(context.Background(), plainProvider{}, req, nil); err != nil { // nil callback is fine
		t.Fatal(err)
	}
}
