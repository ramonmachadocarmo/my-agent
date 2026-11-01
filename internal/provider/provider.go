// Package provider talks to LLM back-ends: Claude, Gemini, Ollama and any
// OpenAI-compatible server.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"my-agent/internal/config"
	"my-agent/internal/i18n"
)

// ErrEmbeddingsUnsupported is returned by providers without an embeddings API.
var ErrEmbeddingsUnsupported = i18n.Err("provider.err.no_embeddings")

type Message struct {
	Role    string `json:"role"` // "user", "assistant" or "tool"
	Content string `json:"content"`

	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // assistant: tools it asked to run
	ToolCallID string     `json:"tool_call_id,omitempty"` // tool: the call this result answers
	Name       string     `json:"name,omitempty"`         // tool: the tool's name
	IsError    bool       `json:"is_error,omitempty"`     // tool: the call failed
}

// ToolCall is a model's request to run a tool.
type ToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
	// Sig is opaque provider data that must be sent back with the call
	// (Gemini's thought signature).
	Sig string `json:"sig,omitempty"`
}

// Tool describes a function the model may call. Parameters is a JSON Schema
// object.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ChatRequest struct {
	System      string
	Messages    []Message
	Temperature float64
	MaxTokens   int
	Tools       []Tool
}

// Usage counts the tokens of one model call (0 when unknown).
type Usage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// Turn is one model reply: text, tool calls, or both.
type Turn struct {
	Text      string
	ToolCalls []ToolCall
	Usage     Usage
}

// Completer is implemented by providers that support tool calling. Complete
// streams text through onDelta (which may be nil) and returns the whole turn.
type Completer interface {
	Complete(ctx context.Context, req ChatRequest, onDelta func(string)) (Turn, error)
}

// Complete runs one model turn. Providers without tool calling fall back to
// ChatStream (tools are then ignored).
func Complete(ctx context.Context, p Provider, req ChatRequest, onDelta func(string)) (Turn, error) {
	if c, ok := p.(Completer); ok {
		return c.Complete(ctx, req, onDelta)
	}
	text, err := ChatStream(ctx, p, req, onDelta)
	return Turn{Text: text}, err
}

// ModelLister is implemented by providers that can list their models.
type ModelLister interface {
	Models(ctx context.Context) ([]string, error)
}

type Provider interface {
	Name() string
	Chat(ctx context.Context, req ChatRequest) (string, error)
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// New builds the provider described by pc. name is only used for messages.
func New(name string, pc config.ProviderConfig) (Provider, error) {
	timeout := time.Duration(pc.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	b := base{name: name, cfg: pc, http: &http.Client{Timeout: timeout}, keys: pc.ResolveAPIKeys(), cur: new(atomic.Int32)}
	switch pc.Type {
	case config.TypeClaude:
		if len(b.keys) == 0 {
			return nil, missingKey(name, pc)
		}
		b.auth = func(k string) map[string]string { return map[string]string{"x-api-key": k} }
		return &Claude{b}, nil
	case config.TypeGemini:
		if len(b.keys) == 0 {
			return nil, missingKey(name, pc)
		}
		b.auth = func(k string) map[string]string { return map[string]string{"x-goog-api-key": k} }
		return &Gemini{b}, nil
	case config.TypeOllama:
		b.auth = bearer // Ollama's cloud models take a key; a local server ignores it
		return &Ollama{b}, nil
	case config.TypeOpenAI:
		if !pc.HasKey() {
			return nil, missingKey(name, pc)
		}
		b.auth = bearer
		return &OpenAI{b}, nil
	}
	return nil, i18n.Errorf("provider.err.unknown_type", name, pc.Type)
}

func missingKey(name string, pc config.ProviderConfig) error {
	var hint string
	switch {
	case pc.APIKeyEnv == "":
		hint = i18n.T("provider.hint.key", name)
	case config.ValidEnvName(pc.APIKeyEnv):
		hint = i18n.T("provider.hint.key_or_env", name, pc.APIKeyEnv)
	default: // never echo it: it is probably a secret in the wrong field
		hint = i18n.T("provider.hint.env_not_name", name)
	}
	return i18n.Errorf("provider.err.missing_key", name, hint)
}

func bearer(k string) map[string]string { return map[string]string{"Authorization": "Bearer " + k} }

type base struct {
	name string
	cfg  config.ProviderConfig
	http *http.Client

	keys []string                           // API keys, tried in turn
	cur  *atomic.Int32                      // index of the key in use (shared by copies)
	auth func(key string) map[string]string // headers that carry a key
}

func (b base) Name() string { return b.name }

func (b base) url(path string) string {
	return strings.TrimRight(b.cfg.BaseURL, "/") + path
}

// HTTPError is a non-2xx reply.
type HTTPError struct {
	Provider string
	Status   int
	Body     string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: http %d: %s", e.Provider, e.Status, e.Body)
}

// rotatable reports whether another key may succeed where this one failed:
// rejected (401/403), out of credit (402) or rate limited (429).
func rotatable(err error) bool {
	var h *HTTPError
	return errors.As(err, &h) && (h.Status == 401 || h.Status == 402 || h.Status == 403 || h.Status == 429)
}

// headers merges the fixed headers, the configured extra ones and the key
// with index i.
func (b base) headers(fixed map[string]string, i int) map[string]string {
	h := map[string]string{}
	for k, v := range fixed {
		h[k] = v
	}
	for k, v := range b.cfg.Headers {
		h[k] = v
	}
	if b.auth != nil && len(b.keys) > 0 {
		for k, v := range b.auth(b.keys[i%len(b.keys)]) {
			h[k] = v
		}
	}
	return h
}

// withKeys runs do with the current key and, when it is rejected or rate
// limited, with each of the other keys. The key that worked stays current.
// Temporary failures (overload, 5xx, rate limits on every key) are retried
// after a pause of 1s, 2s and 4s.
func (b base) withKeys(ctx context.Context, fixed map[string]string, do func(headers map[string]string) error) error {
	var err error
	for attempt := 0; ; attempt++ {
		if err = b.tryKeys(fixed, do); err == nil || !retryable(err) || attempt == len(retryDelays) {
			return err
		}
		select {
		case <-time.After(retryDelays[attempt]):
		case <-ctx.Done():
			return err
		}
	}
}

// retryDelays are the pauses before each retry of a temporary failure.
var retryDelays = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// retryable reports a temporary failure worth another try.
func retryable(err error) bool {
	var h *HTTPError
	if !errors.As(err, &h) {
		return false
	}
	switch h.Status {
	case 408, 429, 500, 502, 503, 504, 529:
		return true
	}
	return false
}

func (b base) tryKeys(fixed map[string]string, do func(headers map[string]string) error) error {
	n := max(len(b.keys), 1)
	start := 0
	if b.cur != nil {
		start = int(b.cur.Load())
	}
	var err error
	for try := 0; try < n; try++ {
		i := (start + try) % n
		if err = do(b.headers(fixed, i)); err == nil || !rotatable(err) {
			if err == nil && i != start && b.cur != nil {
				b.cur.Store(int32(i))
			}
			return err
		}
	}
	return err
}

func (b base) post(ctx context.Context, url string, fixed map[string]string, in, out any) error {
	payload, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return b.withKeys(ctx, fixed, func(headers map[string]string) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		return b.send(req, out)
	})
}

func (b base) get(ctx context.Context, url string, out any) error {
	return b.getFixed(ctx, url, nil, out)
}

func (b base) getFixed(ctx context.Context, url string, fixed map[string]string, out any) error {
	return b.withKeys(ctx, fixed, func(headers map[string]string) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		return b.send(req, out)
	})
}

func (b base) send(req *http.Request, out any) error {
	resp, err := b.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", b.name, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("%s: %w", b.name, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 500 {
			msg = msg[:500] + "..."
		}
		return &HTTPError{b.name, resp.StatusCode, msg}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return i18n.Errorf("provider.err.decoding", b.name, err)
	}
	return nil
}

// argsObject decodes tool call arguments into an object; empty or invalid
// arguments become {}.
func argsObject(raw json.RawMessage) map[string]any {
	m := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

// rawArgs is the JSON of a tool call's arguments, never empty.
func rawArgs(raw json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage("{}")
	}
	return raw
}

// callID makes an ID for providers that do not give tool calls one.
func callID(i int) string { return fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), i) }
