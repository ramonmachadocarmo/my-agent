package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"my-agent/internal/i18n"
)

// OpenAI implements Provider for the Chat Completions API, spoken by OpenAI
// and by most other services and local servers (OpenRouter, Groq, DeepSeek,
// Mistral, LM Studio, vLLM...). base_url includes the version, e.g.
// https://api.openai.com/v1.
type OpenAI struct{ base }

type oaiCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaiMessage struct {
	Role       string    `json:"role"`
	Content    *string   `json:"content"`
	ToolCalls  []oaiCall `json:"tool_calls,omitempty"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
}

func oaiMessages(system string, in []Message) []oaiMessage {
	out := make([]oaiMessage, 0, len(in)+1)
	str := func(s string) *string { return &s }
	if system != "" {
		out = append(out, oaiMessage{Role: "system", Content: str(system)})
	}
	for _, m := range in {
		om := oaiMessage{Role: m.Role, Content: str(m.Content), ToolCallID: m.ToolCallID}
		for i, tc := range m.ToolCalls {
			c := oaiCall{Index: i, ID: tc.ID, Type: "function"}
			c.Function.Name, c.Function.Arguments = tc.Name, string(rawArgs(tc.Args))
			om.ToolCalls = append(om.ToolCalls, c)
		}
		if len(om.ToolCalls) > 0 && m.Content == "" {
			om.Content = nil
		}
		out = append(out, om)
	}
	return out
}

func (o *OpenAI) body(req ChatRequest) map[string]any {
	body := map[string]any{
		"model":       o.cfg.Model,
		"messages":    oaiMessages(req.System, req.Messages),
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if len(req.Tools) > 0 {
		body["tools"] = openAITools(req.Tools)
	}
	return body
}

func (o *OpenAI) Chat(ctx context.Context, req ChatRequest) (string, error) {
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := o.post(ctx, o.url("/chat/completions"), nil, o.body(req), &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", i18n.Errorf("provider.err.empty_response", o.name)
	}
	return out.Choices[0].Message.Content, nil
}

func (o *OpenAI) ChatStream(ctx context.Context, req ChatRequest, onDelta func(string)) (string, error) {
	t, err := o.Complete(ctx, req, onDelta)
	return t.Text, err
}

// Complete streams one turn. Tool call arguments arrive in pieces, keyed by
// the call's index.
func (o *OpenAI) Complete(ctx context.Context, req ChatRequest, onDelta func(string)) (Turn, error) {
	body := o.body(req)
	body["stream"] = true
	body["stream_options"] = map[string]any{"include_usage": true}
	var turn Turn
	var sb strings.Builder
	type call struct {
		id, name string
		args     strings.Builder
	}
	calls := map[int]*call{}
	err := o.postStream(ctx, o.url("/chat/completions"), nil, body, func(line []byte) error {
		data, ok := sseData(line)
		if !ok {
			return nil
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Content   string    `json:"content"`
					ToolCalls []oaiCall `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return i18n.Errorf("provider.err.bad_stream_event", o.name, err)
		}
		if ev.Error != nil {
			return fmt.Errorf("%s: %s", o.name, ev.Error.Message)
		}
		if ev.Usage != nil {
			turn.Usage = Usage{Input: ev.Usage.PromptTokens, Output: ev.Usage.CompletionTokens}
		}
		for _, ch := range ev.Choices {
			if ch.Delta.Content != "" {
				sb.WriteString(ch.Delta.Content)
				emit(onDelta, ch.Delta.Content)
			}
			for _, tc := range ch.Delta.ToolCalls {
				c := calls[tc.Index]
				if c == nil {
					c = &call{}
					calls[tc.Index] = c
				}
				if tc.ID != "" {
					c.id = tc.ID
				}
				if tc.Function.Name != "" {
					c.name = tc.Function.Name
				}
				c.args.WriteString(tc.Function.Arguments)
			}
		}
		return nil
	})
	turn.Text = sb.String()
	idx := make([]int, 0, len(calls))
	for i := range calls {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for _, i := range idx {
		c := calls[i]
		id := c.id
		if id == "" {
			id = callID(i)
		}
		turn.ToolCalls = append(turn.ToolCalls, ToolCall{ID: id, Name: c.name, Args: rawArgs(json.RawMessage(c.args.String()))})
	}
	return turn, err
}

// Models lists the models offered by the server.
func (o *OpenAI) Models(ctx context.Context) ([]string, error) {
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := o.get(ctx, o.url("/models"), &out); err != nil {
		return nil, err
	}
	names := make([]string, len(out.Data))
	for i, m := range out.Data {
		names[i] = m.ID
	}
	sort.Strings(names)
	return names, nil
}

func (o *OpenAI) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if o.cfg.EmbeddingModel == "" {
		return nil, ErrEmbeddingsUnsupported
	}
	var out struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := o.post(ctx, o.url("/embeddings"), nil, map[string]any{"model": o.cfg.EmbeddingModel, "input": texts}, &out); err != nil {
		return nil, err
	}
	if len(out.Data) != len(texts) {
		return nil, i18n.Errorf("provider.err.embed_count", o.name, len(texts), len(out.Data))
	}
	vecs := make([][]float32, len(texts))
	for _, d := range out.Data {
		if d.Index >= 0 && d.Index < len(vecs) {
			vecs[d.Index] = d.Embedding
		}
	}
	return vecs, nil
}
