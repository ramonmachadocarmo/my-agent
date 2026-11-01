package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"my-agent/internal/i18n"
)

// Ollama implements Provider for a local (or remote) Ollama server.
type Ollama struct{ base }

type ollamaCall struct {
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

type ollamaMessage struct {
	Role      string       `json:"role"`
	Content   string       `json:"content"`
	ToolCalls []ollamaCall `json:"tool_calls,omitempty"`
	ToolName  string       `json:"tool_name,omitempty"`
}

func ollamaMessages(system string, in []Message) []ollamaMessage {
	msgs := make([]ollamaMessage, 0, len(in)+1)
	if system != "" {
		msgs = append(msgs, ollamaMessage{Role: "system", Content: system})
	}
	for _, m := range in {
		om := ollamaMessage{Role: m.Role, Content: m.Content}
		if m.Role == "tool" {
			om.ToolName = m.Name
		}
		for _, tc := range m.ToolCalls {
			var c ollamaCall
			c.Function.Name, c.Function.Arguments = tc.Name, argsObject(tc.Args)
			om.ToolCalls = append(om.ToolCalls, c)
		}
		msgs = append(msgs, om)
	}
	return msgs
}

// openAITools is the tools list in the OpenAI format, also used by Ollama.
func openAITools(tools []Tool) []map[string]any {
	out := make([]map[string]any, len(tools))
	for i, t := range tools {
		params := t.Parameters
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out[i] = map[string]any{"type": "function", "function": map[string]any{"name": t.Name, "description": t.Description, "parameters": params}}
	}
	return out
}

func (o *Ollama) body(req ChatRequest, stream bool) map[string]any {
	msgs := ollamaMessages(req.System, req.Messages)
	opts := map[string]any{"temperature": req.Temperature}
	if req.MaxTokens > 0 {
		opts["num_predict"] = req.MaxTokens
	}
	if o.cfg.NumCtx > 0 {
		opts["num_ctx"] = o.cfg.NumCtx
	}
	body := map[string]any{"model": o.cfg.Model, "messages": msgs, "stream": stream, "options": opts, "keep_alive": o.keepAlive()}
	if len(req.Tools) > 0 {
		body["tools"] = openAITools(req.Tools)
	}
	return body
}

// keepAlive is how long Ollama keeps the model loaded after a request. Its
// own default is 5 minutes, so an idle pause means a reload; we default to 30.
func (o *Ollama) keepAlive() any {
	s := strings.TrimSpace(o.cfg.KeepAlive)
	if s == "" {
		return "30m"
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n // seconds; negative keeps the model loaded forever
	}
	return s
}

// Warmup loads the model into memory without generating anything.
func (o *Ollama) Warmup(ctx context.Context) error {
	body := map[string]any{"model": o.cfg.Model, "messages": []ollamaMessage{}, "stream": false, "keep_alive": o.keepAlive()}
	if o.cfg.NumCtx > 0 {
		body["options"] = map[string]any{"num_ctx": o.cfg.NumCtx}
	}
	var out struct{}
	return o.post(ctx, o.url("/api/chat"), nil, body, &out)
}

// RunningModels lists the models currently loaded, and how much of each sits
// in GPU memory.
func (o *Ollama) RunningModels(ctx context.Context) ([]RunningModel, error) {
	var out struct {
		Models []RunningModel `json:"models"`
	}
	if err := o.get(ctx, o.url("/api/ps"), &out); err != nil {
		return nil, err
	}
	return out.Models, nil
}

func (o *Ollama) Chat(ctx context.Context, req ChatRequest) (string, error) {
	var out struct {
		Message ollamaMessage `json:"message"`
	}
	if err := o.explain(o.post(ctx, o.url("/api/chat"), nil, o.body(req, false), &out)); err != nil {
		return "", err
	}
	return out.Message.Content, nil
}

// explain turns "model not found" into a message that says how to fix it.
func (o *Ollama) explain(err error) error {
	var h *HTTPError
	if errors.As(err, &h) && h.Status == 404 && strings.Contains(h.Body, "not found") {
		return fmt.Errorf("%s (%w)", i18n.T("provider.err.ollama_no_model", o.name, o.cfg.Model, o.cfg.Model), err)
	}
	return err
}

// ChatStream streams the reply as newline-delimited JSON.
func (o *Ollama) ChatStream(ctx context.Context, req ChatRequest, onDelta func(string)) (string, error) {
	t, err := o.Complete(ctx, req, onDelta)
	return t.Text, err
}

// Complete streams one turn. Ollama sends tool calls whole.
func (o *Ollama) Complete(ctx context.Context, req ChatRequest, onDelta func(string)) (turn Turn, err error) {
	defer func() { err = o.explain(err) }()
	var sb strings.Builder
	err = o.postStream(ctx, o.url("/api/chat"), nil, o.body(req, true), func(line []byte) error {
		var ev struct {
			Message         ollamaMessage `json:"message"`
			Error           string        `json:"error"`
			PromptEvalCount int           `json:"prompt_eval_count"`
			EvalCount       int           `json:"eval_count"`
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			return i18n.Errorf("provider.err.bad_stream_line", o.name, err)
		}
		if ev.Error != "" {
			return fmt.Errorf("%s: %s", o.name, ev.Error)
		}
		sb.WriteString(ev.Message.Content)
		emit(onDelta, ev.Message.Content)
		for _, c := range ev.Message.ToolCalls {
			args, _ := json.Marshal(c.Function.Arguments)
			turn.ToolCalls = append(turn.ToolCalls, ToolCall{ID: callID(len(turn.ToolCalls)), Name: c.Function.Name, Args: rawArgs(args)})
		}
		if ev.EvalCount > 0 {
			turn.Usage = Usage{Input: ev.PromptEvalCount, Output: ev.EvalCount}
		}
		return nil
	})
	turn.Text = sb.String()
	return turn, err
}

// Models lists the installed models.
func (o *Ollama) Models(ctx context.Context) ([]string, error) {
	ms, err := o.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(ms))
	for i, m := range ms {
		names[i] = m.Name
	}
	return names, nil
}

func (o *Ollama) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	model := o.cfg.EmbeddingModel
	if model == "" {
		return nil, ErrEmbeddingsUnsupported
	}
	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := o.post(ctx, o.url("/api/embed"), nil, map[string]any{"model": model, "input": texts}, &out); err != nil {
		return nil, err
	}
	if len(out.Embeddings) != len(texts) {
		return nil, i18n.Errorf("provider.err.embed_count", o.name, len(texts), len(out.Embeddings))
	}
	return out.Embeddings, nil
}

// ListModels returns the models installed on the Ollama server.
func (o *Ollama) ListModels(ctx context.Context) ([]ModelInfo, error) {
	var out struct {
		Models []ModelInfo `json:"models"`
	}
	if err := o.get(ctx, o.url("/api/tags"), &out); err != nil {
		return nil, err
	}
	return out.Models, nil
}

// PullModel downloads a model, reporting progress. It has no overall timeout
// (downloads are large); cancel through ctx.
func (o *Ollama) PullModel(ctx context.Context, name string, progress func(PullProgress)) error {
	payload, err := json.Marshal(map[string]any{"model": name, "stream": true})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url("/api/pull"), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", o.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return fmt.Errorf("%s: http %d: %s", o.name, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	success := false
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var m struct {
			Status    string `json:"status"`
			Total     int64  `json:"total"`
			Completed int64  `json:"completed"`
			Error     string `json:"error"`
		}
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			return i18n.Errorf("provider.err.pull_bad", o.name, err)
		}
		if m.Error != "" {
			return i18n.Errorf("provider.err.pull", o.name, name, m.Error)
		}
		success = success || m.Status == "success"
		if progress != nil {
			progress(PullProgress{Status: m.Status, Total: m.Total, Completed: m.Completed})
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("%s: %w", o.name, err)
	}
	if !success {
		return i18n.Errorf("provider.err.pull_interrupted", o.name, name)
	}
	return nil
}
