package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"my-agent/internal/i18n"
)

// Gemini implements Provider for the Google Generative Language API.
type Gemini struct{ base }

type gemFuncCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type gemFuncResponse struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type gemPart struct {
	Text             string           `json:"text,omitempty"`
	Thought          bool             `json:"thought,omitempty"`
	FunctionCall     *gemFuncCall     `json:"functionCall,omitempty"`
	FunctionResponse *gemFuncResponse `json:"functionResponse,omitempty"`
	ThoughtSignature string           `json:"thoughtSignature,omitempty"`
}

type gemContent struct {
	Role  string    `json:"role,omitempty"`
	Parts []gemPart `json:"parts"`
}

type gemResponse struct {
	Candidates []struct {
		Content gemContent `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

func (r gemResponse) parts() []gemPart {
	if len(r.Candidates) == 0 {
		return nil
	}
	return r.Candidates[0].Content.Parts
}

func (r gemResponse) text() string {
	var sb strings.Builder
	for _, p := range r.parts() {
		if !p.Thought {
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}

// gemContents converts the conversation. Tool results are functionResponse
// parts of a user turn; consecutive turns of the same role are merged.
func gemContents(msgs []Message) []gemContent {
	var out []gemContent
	push := func(role string, parts ...gemPart) {
		if n := len(out); n > 0 && out[n-1].Role == role {
			out[n-1].Parts = append(out[n-1].Parts, parts...)
			return
		}
		out = append(out, gemContent{Role: role, Parts: parts})
	}
	for _, m := range msgs {
		switch m.Role {
		case "tool":
			push("user", gemPart{FunctionResponse: &gemFuncResponse{Name: m.Name, Response: map[string]any{"content": m.Content, "is_error": m.IsError}}})
		case "assistant":
			var parts []gemPart
			if m.Content != "" {
				parts = append(parts, gemPart{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				parts = append(parts, gemPart{FunctionCall: &gemFuncCall{Name: tc.Name, Args: argsObject(tc.Args)}, ThoughtSignature: tc.Sig})
			}
			if len(parts) > 0 {
				push("model", parts...)
			}
		default:
			push("user", gemPart{Text: m.Content})
		}
	}
	return out
}

func (g *Gemini) body(req ChatRequest) map[string]any {
	gen := map[string]any{"temperature": req.Temperature}
	if req.MaxTokens > 0 {
		gen["maxOutputTokens"] = req.MaxTokens
	}
	body := map[string]any{"contents": gemContents(req.Messages), "generationConfig": gen}
	if req.System != "" {
		body["systemInstruction"] = gemContent{Parts: []gemPart{{Text: req.System}}}
	}
	if len(req.Tools) > 0 {
		decls := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			d := map[string]any{"name": t.Name, "description": t.Description}
			if s := gemSchema(t.Parameters); s != nil {
				if props, _ := s["properties"].(map[string]any); len(props) > 0 {
					d["parameters"] = s
				}
			}
			decls[i] = d
		}
		body["tools"] = []map[string]any{{"functionDeclarations": decls}}
	}
	return body
}

// gemSchema keeps the subset of JSON Schema that Gemini accepts. Schemas
// from MCP servers often carry $schema, additionalProperties, $ref...
func gemSchema(s map[string]any) map[string]any {
	if s == nil {
		return nil
	}
	out := map[string]any{}
	for k, v := range s {
		switch k {
		case "type":
			if list, ok := v.([]any); ok { // ["string","null"]
				for _, t := range list {
					if t != "null" {
						out["type"] = t
					} else {
						out["nullable"] = true
					}
				}
				continue
			}
			out[k] = v
		case "description", "enum", "format", "nullable", "minimum", "maximum", "minItems", "maxItems", "title":
			out[k] = v
		case "required":
			out[k] = v
		case "items":
			if m, ok := v.(map[string]any); ok {
				out[k] = gemSchema(m)
			}
		case "properties":
			if m, ok := v.(map[string]any); ok {
				props := map[string]any{}
				for name, p := range m {
					if pm, ok := p.(map[string]any); ok {
						props[name] = gemSchema(pm)
					}
				}
				out[k] = props
			}
		case "anyOf", "oneOf":
			// pick the first alternative: Gemini's support for unions is partial
			if list, ok := v.([]any); ok && len(list) > 0 {
				if m, ok := list[0].(map[string]any); ok {
					for kk, vv := range gemSchema(m) {
						if _, set := out[kk]; !set {
							out[kk] = vv
						}
					}
				}
			}
		}
	}
	if out["type"] == nil {
		if out["properties"] != nil {
			out["type"] = "object"
		} else {
			out["type"] = "string"
		}
	}
	if f, _ := out["format"].(string); f != "" && f != "enum" && f != "date-time" && out["type"] == "string" {
		delete(out, "format")
	}
	// "required" may only name existing properties
	if req, ok := out["required"].([]any); ok {
		props, _ := out["properties"].(map[string]any)
		var keep []any
		for _, r := range req {
			if name, ok := r.(string); ok && props[name] != nil {
				keep = append(keep, r)
			}
		}
		if len(keep) == 0 {
			delete(out, "required")
		} else {
			out["required"] = keep
		}
	}
	return out
}

func (g *Gemini) Chat(ctx context.Context, req ChatRequest) (string, error) {
	var out gemResponse
	url := g.url(fmt.Sprintf("/v1beta/models/%s:generateContent", g.cfg.Model))
	if err := g.post(ctx, url, nil, g.body(req), &out); err != nil {
		return "", err
	}
	if len(out.Candidates) == 0 {
		return "", i18n.Errorf("provider.err.no_candidates", g.name)
	}
	return out.text(), nil
}

// ChatStream streams the reply using streamGenerateContent (SSE).
func (g *Gemini) ChatStream(ctx context.Context, req ChatRequest, onDelta func(string)) (string, error) {
	t, err := g.Complete(ctx, req, onDelta)
	return t.Text, err
}

// Complete streams one turn. Gemini sends each functionCall whole.
func (g *Gemini) Complete(ctx context.Context, req ChatRequest, onDelta func(string)) (Turn, error) {
	var turn Turn
	var sb strings.Builder
	url := g.url(fmt.Sprintf("/v1beta/models/%s:streamGenerateContent?alt=sse", g.cfg.Model))
	err := g.postStream(ctx, url, nil, g.body(req), func(line []byte) error {
		data, ok := sseData(line)
		if !ok {
			return nil
		}
		var ev gemResponse
		if err := json.Unmarshal(data, &ev); err != nil {
			return i18n.Errorf("provider.err.bad_stream_event", g.name, err)
		}
		for _, p := range ev.parts() {
			switch {
			case p.FunctionCall != nil:
				args, _ := json.Marshal(p.FunctionCall.Args)
				id := p.FunctionCall.ID
				if id == "" {
					id = callID(len(turn.ToolCalls))
				}
				turn.ToolCalls = append(turn.ToolCalls, ToolCall{ID: id, Name: p.FunctionCall.Name, Args: rawArgs(args), Sig: p.ThoughtSignature})
			case p.Thought:
			default:
				sb.WriteString(p.Text)
				emit(onDelta, p.Text)
			}
		}
		if u := ev.UsageMetadata; u.PromptTokenCount > 0 {
			turn.Usage = Usage{Input: u.PromptTokenCount, Output: u.CandidatesTokenCount}
		}
		return nil
	})
	turn.Text = sb.String()
	return turn, err
}

// Models lists the models that can generate content.
func (g *Gemini) Models(ctx context.Context) ([]string, error) {
	var out struct {
		Models []struct {
			Name    string   `json:"name"`
			Methods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := g.get(ctx, g.url("/v1beta/models?pageSize=1000"), &out); err != nil {
		return nil, err
	}
	var names []string
	for _, m := range out.Models {
		for _, meth := range m.Methods {
			if meth == "generateContent" {
				names = append(names, strings.TrimPrefix(m.Name, "models/"))
				break
			}
		}
	}
	return names, nil
}

func (g *Gemini) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if g.cfg.EmbeddingModel == "" {
		return nil, ErrEmbeddingsUnsupported
	}
	model := "models/" + g.cfg.EmbeddingModel
	reqs := make([]map[string]any, len(texts))
	for i, t := range texts {
		reqs[i] = map[string]any{"model": model, "content": gemContent{Parts: []gemPart{{Text: t}}}}
	}
	var out struct {
		Embeddings []struct {
			Values []float32 `json:"values"`
		} `json:"embeddings"`
	}
	url := g.url(fmt.Sprintf("/v1beta/%s:batchEmbedContents", model))
	if err := g.post(ctx, url, nil, map[string]any{"requests": reqs}, &out); err != nil {
		return nil, err
	}
	if len(out.Embeddings) != len(texts) {
		return nil, i18n.Errorf("provider.err.embed_count", g.name, len(texts), len(out.Embeddings))
	}
	vecs := make([][]float32, len(texts))
	for i, e := range out.Embeddings {
		vecs[i] = e.Values
	}
	return vecs, nil
}
