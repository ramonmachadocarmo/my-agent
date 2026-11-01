package agent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"my-agent/internal/config"
	"my-agent/internal/provider"
	"my-agent/internal/rag"
)

type fakeProvider struct {
	got  provider.ChatRequest
	resp string
	err  error
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Chat(_ context.Context, r provider.ChatRequest) (string, error) {
	f.got = r
	return f.resp, f.err
}
func (f *fakeProvider) Embed(context.Context, []string) ([][]float32, error) {
	return nil, provider.ErrEmbeddingsUnsupported
}

func cfg() config.AgentConfig {
	return config.AgentConfig{SystemPrompt: "BASE", Temperature: 0.3, MaxTokens: 77, UseRAG: true}
}

func TestAskWithoutRAG(t *testing.T) {
	fp := &fakeProvider{resp: "hi"}
	a := &Agent{Config: cfg(), Provider: fp}
	ans, err := a.Ask(context.Background(), []provider.Message{{Role: "user", Content: "old"}, {Role: "assistant", Content: "older"}}, "new")
	if err != nil || ans.Text != "hi" || len(ans.Sources) != 0 {
		t.Fatalf("%+v %v", ans, err)
	}
	if fp.got.System != "BASE" || fp.got.MaxTokens != 77 || fp.got.Temperature != 0.3 {
		t.Fatalf("request: %+v", fp.got)
	}
	if n := len(fp.got.Messages); n != 3 || fp.got.Messages[2].Content != "new" {
		t.Fatalf("messages: %+v", fp.got.Messages)
	}
}

func TestAskInjectsRetrievedContext(t *testing.T) {
	ctx := context.Background()
	st, _ := rag.OpenStore(filepath.Join(t.TempDir(), "s.json"))
	ix := &rag.Index{Store: st, Embedder: rag.LocalEmbedder{}, ChunkSize: 200, TopK: 2, MinScore: 0.01}
	ix.IngestText(ctx, "manual.md", "The warp drive requires dilithium crystals.")
	ix.IngestText(ctx, "other.md", "Bananas are yellow fruit.")

	fp := &fakeProvider{resp: "answer"}
	a := &Agent{Config: cfg(), Provider: fp, Index: ix}
	ans, err := a.Ask(ctx, nil, "what does the warp drive require?")
	if err != nil {
		t.Fatal(err)
	}
	if len(ans.Sources) == 0 || ans.Sources[0].Chunk.Source != "manual.md" {
		t.Fatalf("sources: %+v", ans.Sources)
	}
	if !strings.HasPrefix(fp.got.System, "BASE") || !strings.Contains(fp.got.System, "dilithium") || !strings.Contains(fp.got.System, "manual.md") {
		t.Fatalf("system prompt lacks context:\n%s", fp.got.System)
	}

	// use_rag=false must bypass retrieval entirely.
	a.Config.UseRAG = false
	ans, _ = a.Ask(ctx, nil, "warp drive")
	if len(ans.Sources) != 0 || fp.got.System != "BASE" {
		t.Fatalf("RAG should be disabled: %+v / %q", ans.Sources, fp.got.System)
	}
}

func TestAskTruncatesHistory(t *testing.T) {
	fp := &fakeProvider{resp: "x"}
	a := &Agent{Config: cfg(), Provider: fp}
	var h []provider.Message
	for i := 0; i < 60; i++ {
		h = append(h, provider.Message{Role: "user", Content: "m"})
	}
	a.Ask(context.Background(), h, "q")
	if n := len(fp.got.Messages); n != MaxHistory+1 {
		t.Fatalf("sent %d messages, want %d", n, MaxHistory+1)
	}
}

func TestProviderErrorPropagates(t *testing.T) {
	a := &Agent{Config: cfg(), Provider: &fakeProvider{err: errors.New("down")}}
	if _, err := a.Ask(context.Background(), nil, "q"); err == nil || !strings.Contains(err.Error(), "down") {
		t.Fatalf("got %v", err)
	}
}

func TestSessionKeepsAndResetsHistory(t *testing.T) {
	fp := &fakeProvider{resp: "r1"}
	s := &Session{Agent: &Agent{Config: cfg(), Provider: fp}}
	s.Send(context.Background(), "q1")
	fp.resp = "r2"
	s.Send(context.Background(), "q2")
	if len(s.History) != 4 || len(fp.got.Messages) != 3 {
		t.Fatalf("history=%d sent=%d", len(s.History), len(fp.got.Messages))
	}
	s.Reset()
	if len(s.History) != 0 {
		t.Fatal("reset failed")
	}
	fp.err = errors.New("fail")
	s.Send(context.Background(), "q3")
	if len(s.History) != 0 {
		t.Fatal("failed turns must not enter history")
	}
}
