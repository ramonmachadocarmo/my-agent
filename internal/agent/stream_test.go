package agent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"my-agent/internal/provider"
	"my-agent/internal/rag"
)

type streamProvider struct {
	fakeProvider
	pieces []string
	err    error
}

func (s *streamProvider) ChatStream(_ context.Context, r provider.ChatRequest, onDelta func(string)) (string, error) {
	s.got = r
	for _, p := range s.pieces {
		if onDelta != nil {
			onDelta(p)
		}
	}
	return strings.Join(s.pieces, ""), s.err
}

func TestAskStreamDeliversPiecesAndFullAnswer(t *testing.T) {
	sp := &streamProvider{pieces: []string{"Hel", "lo ", "there"}}
	a := &Agent{Config: cfg(), Provider: sp}
	var got []string
	ans, err := a.AskStream(context.Background(), nil, "hi", func(d string) { got = append(got, d) })
	if err != nil || ans.Text != "Hello there" || strings.Join(got, "|") != "Hel|lo |there" {
		t.Fatalf("%+v %v %v", ans, got, err)
	}
	if sp.got.Messages[0].Content != "hi" {
		t.Fatalf("request not forwarded: %+v", sp.got)
	}
}

func TestAskStreamFallsBackForNonStreamingProviders(t *testing.T) {
	a := &Agent{Config: cfg(), Provider: &fakeProvider{resp: "whole"}}
	var got []string
	ans, err := a.AskStream(context.Background(), nil, "hi", func(d string) { got = append(got, d) })
	if err != nil || ans.Text != "whole" || len(got) != 1 || got[0] != "whole" {
		t.Fatalf("%+v %v %v", ans, got, err)
	}
}

func TestSessionStreamHistory(t *testing.T) {
	sp := &streamProvider{pieces: []string{"a", "b"}}
	s := &Session{Agent: &Agent{Config: cfg(), Provider: sp}}
	if _, err := s.SendStream(context.Background(), "q1", nil); err != nil {
		t.Fatal(err)
	}
	if len(s.History) != 2 || s.History[1].Content != "ab" {
		t.Fatalf("history: %+v", s.History)
	}
	// an interrupted or failed stream must not enter the history
	sp.err = errors.New("cut")
	if _, err := s.SendStream(context.Background(), "q2", nil); err == nil {
		t.Fatal("expected error")
	}
	if len(s.History) != 2 {
		t.Fatalf("failed turn was recorded: %+v", s.History)
	}
}

func TestAskStreamQuerySearchesOnlyTheQuestionButSendsTheWholeInput(t *testing.T) {
	ctx := context.Background()
	st, _ := rag.OpenStore(filepath.Join(t.TempDir(), "s.json"))
	ix := &rag.Index{Store: st, Embedder: rag.LocalEmbedder{}, ChunkSize: 200, TopK: 2, MinScore: 0.01}
	ix.IngestText(ctx, "manual.md", "The warp drive requires dilithium crystals.")

	fp := &fakeProvider{resp: "ok"}
	a := &Agent{Config: cfg(), Provider: fp, Index: ix}
	// the sent input carries a long attached file full of unrelated words
	attachment := strings.Repeat("banana laranja abacaxi melancia ", 200)
	input := "what does the warp drive require?\n\n[file]\n" + attachment
	ans, err := a.AskStreamQuery(ctx, nil, input, "what does the warp drive require?", nil)
	if err != nil || len(ans.Sources) == 0 || ans.Sources[0].Chunk.Source != "manual.md" {
		t.Fatalf("retrieval must use the question only: %+v %v", ans.Sources, err)
	}
	if got := fp.got.Messages[0].Content; got != input {
		t.Fatalf("the model must receive the full input, including the attachment (%d bytes)", len(got))
	}
	// with the attachment as the query the relevant chunk would be drowned out
	ans, _ = a.AskStream(ctx, nil, input, nil)
	if len(ans.Sources) != 0 && ans.Sources[0].Score > 0.3 {
		t.Logf("note: a long attachment as the query scores %.2f (why the query is separate)", ans.Sources[0].Score)
	}

	s := &Session{Agent: a}
	if _, err := s.SendStreamQuery(ctx, input, "warp drive", nil); err != nil {
		t.Fatal(err)
	}
	if s.History[0].Content != input {
		t.Fatal("the history keeps the full input so follow-up questions still see the file")
	}
}
