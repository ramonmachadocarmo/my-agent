package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"my-agent/internal/i18n"
)

// Claude implements Provider for the Anthropic Messages API.
type Claude struct{ base }

var claudeFixed = map[string]string{"anthropic-version": "2023-06-01"}

// claudeMessages converts the conversation to Messages API content blocks.
// Tool results become tool_result blocks of a user message, and consecutive
// messages of the same role are merged, as the API requires.
func claudeMessages(msgs []Message) []map[string]any {
	var out []map[string]any
	push := func(role string, blocks ...map[string]any) {
		if n := len(out); n > 0 && out[n-1]["role"] == role {
			out[n-1]["content"] = append(out[n-1]["content"].([]map[string]any), blocks...)
			return
		}
		out = append(out, map[string]any{"role": role, "content": blocks})
	}
	for _, m := range msgs {
		switch m.Role {
		case "tool":
			b := map[string]any{"type": "tool_result", "tool_use_id": m.ToolCallID, "content": m.Content}
			if m.IsError {
				b["is_error"] = true
			}
			push("user", b)
		case "assistant":
			var blocks []map[string]any
			if m.Content != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": m.Content})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, map[string]any{"type": "tool_use", "id": tc.ID, "name": tc.Name, "input": argsObject(tc.Args)})
			}
			if len(blocks) > 0 {
				push("assistant", blocks...)
			}
		default:
			push("user", map[string]any{"type": "text", "text": m.Content})
		}
	}
	return out
}

func (c *Claude) body(req ChatRequest) map[string]any {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	body := map[string]any{
		"model":       c.cfg.Model,
		"max_tokens":  maxTokens,
		"temperature": req.Temperature,
		"messages":    claudeMessages(req.Messages),
	}
	if req.System != "" {
		body["system"] = req.System
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{"name": t.Name, "description": t.Description, "input_schema": t.Parameters}
		}
		body["tools"] = tools
	}
	return body
}

func (c *Claude) Chat(ctx context.Context, req ChatRequest) (string, error) {
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := c.post(ctx, c.url("/v1/messages"), claudeFixed, c.body(req), &out); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, part := range out.Content {
		if part.Type == "text" {
			sb.WriteString(part.Text)
		}
	}
	if sb.Len() == 0 {
		return "", i18n.Errorf("provider.err.empty_response", c.name)
	}
	return sb.String(), nil
}

// ChatStream streams the reply using the Messages API server-sent events.
func (c *Claude) ChatStream(ctx context.Context, req ChatRequest, onDelta func(string)) (string, error) {
	t, err := c.Complete(ctx, req, onDelta)
	if err == nil && t.Text == "" && len(t.ToolCalls) == 0 {
		return "", i18n.Errorf("provider.err.empty_response", c.name)
	}
	return t.Text, err
}

// Complete streams one turn, collecting text and tool_use blocks.
func (c *Claude) Complete(ctx context.Context, req ChatRequest, onDelta func(string)) (Turn, error) {
	body := c.body(req)
	body["stream"] = true
	var turn Turn
	var sb strings.Builder
	type block struct {
		id, name string
		args     strings.Builder
	}
	blocks := map[int]*block{}
	var order []int
	err := c.postStream(ctx, c.url("/v1/messages"), claudeFixed, body, func(line []byte) error {
		data, ok := sseData(line)
		if !ok {
			return nil
		}
		var ev struct {
			Type         string `json:"type"`
			Index        int    `json:"index"`
			ContentBlock struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
			Message struct {
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return i18n.Errorf("provider.err.bad_stream_event", c.name, err)
		}
		switch ev.Type {
		case "message_start":
			turn.Usage.Input = ev.Message.Usage.InputTokens
		case "content_block_start":
			if ev.ContentBlock.Type == "tool_use" {
				blocks[ev.Index] = &block{id: ev.ContentBlock.ID, name: ev.ContentBlock.Name}
				order = append(order, ev.Index)
			}
		case "content_block_delta":
			switch ev.Delta.Type {
			case "text_delta":
				sb.WriteString(ev.Delta.Text)
				emit(onDelta, ev.Delta.Text)
			case "input_json_delta":
				if b := blocks[ev.Index]; b != nil {
					b.args.WriteString(ev.Delta.PartialJSON)
				}
			}
		case "message_delta":
			turn.Usage.Output = ev.Usage.OutputTokens
		case "error":
			return fmt.Errorf("%s: %s", c.name, ev.Error.Message)
		}
		return nil
	})
	turn.Text = sb.String()
	for _, i := range order {
		b := blocks[i]
		turn.ToolCalls = append(turn.ToolCalls, ToolCall{ID: b.id, Name: b.name, Args: rawArgs(json.RawMessage(b.args.String()))})
	}
	return turn, err
}

// Models lists the models the key can use.
func (c *Claude) Models(ctx context.Context) ([]string, error) {
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.getFixed(ctx, c.url("/v1/models?limit=100"), claudeFixed, &out); err != nil {
		return nil, err
	}
	names := make([]string, len(out.Data))
	for i, m := range out.Data {
		names[i] = m.ID
	}
	return names, nil
}

// Embed is unsupported: Anthropic has no embeddings endpoint.
func (c *Claude) Embed(context.Context, []string) ([][]float32, error) {
	return nil, ErrEmbeddingsUnsupported
}
