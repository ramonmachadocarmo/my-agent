package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"my-agent/internal/config"
)

var toolReq = ChatRequest{
	System: "sys",
	Messages: []Message{
		{Role: "user", Content: "list files"},
		{Role: "assistant", Content: "let me look", ToolCalls: []ToolCall{{ID: "c1", Name: "ls", Args: json.RawMessage(`{"path":"."}`), Sig: "sig-1"}}},
		{Role: "tool", ToolCallID: "c1", Name: "ls", Content: "a.go\nb.go"},
	},
	Tools: []Tool{{Name: "ls", Description: "list", Parameters: map[string]any{
		"type": "object", "$schema": "x", "additionalProperties": false,
		"properties": map[string]any{"path": map[string]any{"type": []any{"string", "null"}}},
	}}},
}

func TestClaudeToolCalling(t *testing.T) {
	p := newTest(t, "claude", func(w http.ResponseWriter, r *http.Request) {
		b := readBody(t, r)
		msgs := b["messages"].([]any)
		asst := msgs[1].(map[string]any)["content"].([]any)
		if asst[1].(map[string]any)["type"] != "tool_use" || asst[1].(map[string]any)["id"] != "c1" {
			t.Errorf("assistant tool_use block: %v", asst)
		}
		res := msgs[2].(map[string]any)
		if res["role"] != "user" || res["content"].([]any)[0].(map[string]any)["tool_use_id"] != "c1" {
			t.Errorf("tool_result: %v", res)
		}
		if b["tools"].([]any)[0].(map[string]any)["input_schema"] == nil {
			t.Errorf("tools: %v", b["tools"])
		}
		lines(w,
			`data: {"type":"message_start","message":{"usage":{"input_tokens":12}}}`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok "}}`,
			`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"t9","name":"read"}}`,
			`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"path\":"}}`,
			`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"a.go\"}"}}`,
			`data: {"type":"message_delta","usage":{"output_tokens":5}}`,
		)
	}, nil)
	turn, err := Complete(context.Background(), p, toolReq, nil)
	if err != nil {
		t.Fatal(err)
	}
	if turn.Text != "ok " || len(turn.ToolCalls) != 1 || turn.ToolCalls[0].ID != "t9" || string(turn.ToolCalls[0].Args) != `{"path":"a.go"}` {
		t.Fatalf("turn: %+v", turn)
	}
	if turn.Usage != (Usage{12, 5}) {
		t.Fatalf("usage: %+v", turn.Usage)
	}
}

func TestGeminiToolCallingKeepsThoughtSignature(t *testing.T) {
	p := newTest(t, "gemini", func(w http.ResponseWriter, r *http.Request) {
		b := readBody(t, r)
		c := b["contents"].([]any)
		model := c[1].(map[string]any)
		part := model["parts"].([]any)[1].(map[string]any)
		if part["thoughtSignature"] != "sig-1" || part["functionCall"].(map[string]any)["name"] != "ls" {
			t.Errorf("model turn: %v", model)
		}
		fr := c[2].(map[string]any)["parts"].([]any)[0].(map[string]any)["functionResponse"].(map[string]any)
		if fr["name"] != "ls" {
			t.Errorf("function response: %v", fr)
		}
		decl := b["tools"].([]any)[0].(map[string]any)["functionDeclarations"].([]any)[0].(map[string]any)
		params := decl["parameters"].(map[string]any)
		if params["$schema"] != nil || params["additionalProperties"] != nil {
			t.Errorf("unsupported schema keys were sent: %v", params)
		}
		path := params["properties"].(map[string]any)["path"].(map[string]any)
		if path["type"] != "string" || path["nullable"] != true {
			t.Errorf("type list not converted: %v", path)
		}
		lines(w,
			`data: {"candidates":[{"content":{"parts":[{"text":"thinking","thought":true},{"text":"hi"}]}}]}`,
			`data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"read","args":{"path":"x"}},"thoughtSignature":"sig-2"}]}}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":4}}`,
		)
	}, nil)
	turn, err := Complete(context.Background(), p, toolReq, nil)
	if err != nil {
		t.Fatal(err)
	}
	if turn.Text != "hi" || len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Sig != "sig-2" || turn.ToolCalls[0].Name != "read" {
		t.Fatalf("turn: %+v", turn)
	}
}

func TestOllamaToolCalling(t *testing.T) {
	p := newTest(t, "ollama", func(w http.ResponseWriter, r *http.Request) {
		b := readBody(t, r)
		msgs := b["messages"].([]any)
		if msgs[2].(map[string]any)["tool_calls"] == nil || msgs[3].(map[string]any)["tool_name"] != "ls" {
			t.Errorf("messages: %v", msgs)
		}
		lines(w,
			`{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"read","arguments":{"path":"x"}}}]}}`,
			`{"message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":7,"eval_count":2}`,
		)
	}, func(pc *config.ProviderConfig) { pc.APIKey = "" })
	turn, err := Complete(context.Background(), p, toolReq, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Name != "read" || turn.ToolCalls[0].ID == "" || turn.Usage.Output != 2 {
		t.Fatalf("turn: %+v", turn)
	}
}

func TestOpenAICompatibleToolCalling(t *testing.T) {
	p := newTest(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("request: %s %v", r.URL.Path, r.Header)
		}
		b := readBody(t, r)
		msgs := b["messages"].([]any)
		if msgs[0].(map[string]any)["role"] != "system" || msgs[3].(map[string]any)["tool_call_id"] != "c1" {
			t.Errorf("messages: %v", msgs)
		}
		lines(w,
			`data: {"choices":[{"delta":{"content":"one "}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"x1","function":{"name":"grep","arguments":"{\"pat"}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"tern\":\"a\"}"}}]}}]}`,
			`data: {"choices":[],"usage":{"prompt_tokens":9,"completion_tokens":3}}`,
			`data: [DONE]`,
		)
	}, nil)
	var deltas []string
	turn, err := Complete(context.Background(), p, toolReq, func(d string) { deltas = append(deltas, d) })
	if err != nil {
		t.Fatal(err)
	}
	if turn.Text != "one " || len(deltas) != 1 || len(turn.ToolCalls) != 1 || string(turn.ToolCalls[0].Args) != `{"pattern":"a"}` || turn.Usage.Input != 9 {
		t.Fatalf("turn: %+v", turn)
	}
}

func TestKeysRotateOnRateLimit(t *testing.T) {
	var seen []string
	p := newTest(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		k := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		seen = append(seen, k)
		if k != "k3" {
			w.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(w, `{"error":"slow down"}`)
			return
		}
		io.WriteString(w, `{"choices":[{"message":{"content":"done"}}]}`)
	}, func(pc *config.ProviderConfig) { pc.APIKey, pc.APIKeys = "k1", []string{"k2", "k3"} })
	got, err := p.Chat(context.Background(), req)
	if err != nil || got != "done" {
		t.Fatalf("got %q, %v", got, err)
	}
	if strings.Join(seen, ",") != "k1,k2,k3" {
		t.Fatalf("keys tried: %v", seen)
	}
	// the key that worked stays current
	seen = nil
	if _, err := p.Chat(context.Background(), req); err != nil || strings.Join(seen, ",") != "k3" {
		t.Fatalf("second call tried %v, %v", seen, err)
	}
}

func TestKeysDoNotRotateOnOtherErrors(t *testing.T) {
	calls := 0
	p := newTest(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	}, func(pc *config.ProviderConfig) { pc.APIKeys = []string{"k2"} })
	if _, err := p.Chat(context.Background(), req); err == nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestTemporaryFailuresAreRetried(t *testing.T) {
	old := retryDelays
	retryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	defer func() { retryDelays = old }()
	calls := 0
	p := newTest(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, `{"error":"high demand"}`)
			return
		}
		lines(w, `data: {"choices":[{"delta":{"content":"back"}}]}`)
	}, nil)
	turn, err := Complete(context.Background(), p, req, nil)
	if err != nil || turn.Text != "back" || calls != 3 {
		t.Fatalf("calls=%d turn=%+v err=%v", calls, turn, err)
	}
	calls = -10 // keeps failing: gives up after the last retry
	if _, err := Complete(context.Background(), p, req, nil); err == nil || calls != -7 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestLocalOpenAIServerNeedsNoKey(t *testing.T) {
	_, err := New("lm", config.ProviderConfig{Type: "openai", Model: "m", BaseURL: "http://localhost:1234/v1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New("oa", config.ProviderConfig{Type: "openai", Model: "m", BaseURL: "https://api.openai.com/v1"}); err == nil {
		t.Fatal("a remote server without a key must fail")
	}
}

func TestOllamaMissingModelExplainsTheFix(t *testing.T) {
	p := newTest(t, "ollama", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":"model 'llama3.2' not found"}`)
	}, func(pc *config.ProviderConfig) { pc.APIKey, pc.Model = "", "llama3.2" })
	_, err := Complete(context.Background(), p, req, nil)
	var h *HTTPError
	if err == nil || !strings.Contains(err.Error(), "ollama pull llama3.2") || !strings.Contains(err.Error(), "ctrl+o") || !errors.As(err, &h) {
		t.Fatalf("err = %v", err)
	}
}
