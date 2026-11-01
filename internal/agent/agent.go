// Package agent combines a provider, a system prompt and optional RAG
// retrieval into a conversational agent.
package agent

import (
	"context"
	"fmt"
	"strings"

	"my-agent/internal/config"
	"my-agent/internal/i18n"
	"my-agent/internal/provider"
	"my-agent/internal/rag"
)

// MaxHistory caps how many past messages a Session sends to the model.
const MaxHistory = 20

type Agent struct {
	Config   config.AgentConfig
	Provider provider.Provider
	Index    *rag.Index // nil disables retrieval
}

type Answer struct {
	Text    string       `json:"text"`
	Sources []rag.Result `json:"sources"`
}

// Ask answers input given prior history, retrieving context when RAG is on.
func (a *Agent) Ask(ctx context.Context, history []provider.Message, input string) (Answer, error) {
	return a.AskStream(ctx, history, input, nil)
}

// AskStream is Ask, delivering the reply piece by piece through onDelta while
// it is generated (when the provider supports streaming). The returned Answer
// still holds the complete text and the sources.
func (a *Agent) AskStream(ctx context.Context, history []provider.Message, input string, onDelta func(string)) (Answer, error) {
	return a.AskStreamQuery(ctx, history, input, input, onDelta)
}

// AskStreamQuery is AskStream with a separate retrieval query: input is what
// the model receives (it may carry attached file contents) while query is
// what is searched in the knowledge base (just what the user asked).
func (a *Agent) AskStreamQuery(ctx context.Context, history []provider.Message, input, query string, onDelta func(string)) (Answer, error) {
	var hits []rag.Result
	if a.Index != nil && a.Config.UseRAG {
		var err error
		if hits, err = a.Index.Search(ctx, query); err != nil {
			return Answer{}, i18n.Errorf("agent.err.rag_search", err)
		}
	}
	if len(history) > MaxHistory {
		history = history[len(history)-MaxHistory:]
	}
	msgs := append(append([]provider.Message{}, history...), provider.Message{Role: "user", Content: input})
	text, err := provider.ChatStream(ctx, a.Provider, provider.ChatRequest{
		System:      BuildSystem(a.Config.SystemPrompt, hits),
		Messages:    msgs,
		Temperature: a.Config.Temperature,
		MaxTokens:   a.Config.MaxTokens,
	}, onDelta)
	if err != nil {
		return Answer{}, err
	}
	return Answer{Text: text, Sources: hits}, nil
}

// BuildSystem appends retrieved context to the base system prompt.
func BuildSystem(base string, hits []rag.Result) string {
	if len(hits) == 0 {
		return base
	}
	var sb strings.Builder
	sb.WriteString(base)
	sb.WriteString("\n\nUse the following context to answer when it is relevant. Cite sources as [n]. If the context does not help, say so and answer from general knowledge.\n")
	for i, h := range hits {
		fmt.Fprintf(&sb, "\n[%d] (source: %s)\n%s\n", i+1, h.Chunk.Source, h.Chunk.Text)
	}
	return sb.String()
}

// Session keeps conversation history across turns.
type Session struct {
	Agent   *Agent
	History []provider.Message
}

func (s *Session) Send(ctx context.Context, input string) (Answer, error) {
	return s.SendStream(ctx, input, nil)
}

// SendStream is Send with incremental delivery of the reply. Failed or
// interrupted turns are not added to the history.
func (s *Session) SendStream(ctx context.Context, input string, onDelta func(string)) (Answer, error) {
	return s.SendStreamQuery(ctx, input, input, onDelta)
}

// SendStreamQuery is SendStream with a separate retrieval query (see
// Agent.AskStreamQuery).
func (s *Session) SendStreamQuery(ctx context.Context, input, query string, onDelta func(string)) (Answer, error) {
	ans, err := s.Agent.AskStreamQuery(ctx, s.History, input, query, onDelta)
	if err != nil {
		return ans, err
	}
	s.History = append(s.History,
		provider.Message{Role: "user", Content: input},
		provider.Message{Role: "assistant", Content: ans.Text})
	return ans, nil
}

func (s *Session) Reset() { s.History = nil }
