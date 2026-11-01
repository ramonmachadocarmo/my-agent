package rag

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestChunkText(t *testing.T) {
	if got := ChunkText("   ", 100, 10); got != nil {
		t.Fatalf("blank text: %v", got)
	}
	if got := ChunkText("short", 100, 10); len(got) != 1 || got[0] != "short" {
		t.Fatalf("short text: %v", got)
	}
	text := strings.Repeat("palavra ", 200) // 1600 chars
	chunks := ChunkText(text, 300, 50)
	if len(chunks) < 5 {
		t.Fatalf("expected several chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if n := len([]rune(c)); n > 300 || n == 0 {
			t.Fatalf("chunk size %d out of bounds", n)
		}
		if strings.HasPrefix(c, "alavra") { // must not split words mid-way
			t.Fatalf("chunk begins mid-word: %q", c[:10])
		}
	}
}

func TestChunkTextPrefersParagraphs(t *testing.T) {
	text := strings.Repeat("a", 60) + "\n\n" + strings.Repeat("b", 60)
	chunks := ChunkText(text, 100, 0)
	if len(chunks) != 2 || strings.Contains(chunks[0], "b") {
		t.Fatalf("want split at paragraph, got %q", chunks)
	}
}

func TestChunkTextTerminatesWithBadOverlap(t *testing.T) {
	if got := ChunkText(strings.Repeat("x", 500), 100, 100); len(got) == 0 {
		t.Fatal("expected chunks")
	}
	if got := ChunkText("héllo wörld ñ", 5, 1); len(got) == 0 { // multibyte runes
		t.Fatal("expected chunks")
	}
}

func TestCosine(t *testing.T) {
	if got := Cosine([]float32{1, 0}, []float32{1, 0}); got < 0.999 {
		t.Fatalf("identical: %v", got)
	}
	if got := Cosine([]float32{1, 0}, []float32{0, 1}); got != 0 {
		t.Fatalf("orthogonal: %v", got)
	}
	if Cosine([]float32{1}, []float32{1, 2}) != 0 || Cosine(nil, nil) != 0 || Cosine([]float32{0, 0}, []float32{1, 1}) != 0 {
		t.Fatal("degenerate inputs should score 0")
	}
}

func newIndex(t *testing.T) *Index {
	t.Helper()
	st, err := OpenStore(filepath.Join(t.TempDir(), "s.json"))
	if err != nil {
		t.Fatal(err)
	}
	return &Index{Store: st, Embedder: LocalEmbedder{}, ChunkSize: 200, Overlap: 20, TopK: 2, MinScore: 0.01}
}

func TestIndexIngestAndSearch(t *testing.T) {
	ctx := context.Background()
	ix := newIndex(t)
	docs := map[string]string{
		"go.txt":      "Go is a compiled language with goroutines and channels for concurrency.",
		"cook.txt":    "To bake bread mix flour, water, yeast and salt, then let the dough rise.",
		"planets.txt": "Jupiter is the largest planet in the solar system.",
	}
	for name, text := range docs {
		if _, err := ix.IngestText(ctx, name, text); err != nil {
			t.Fatal(err)
		}
	}
	res, err := ix.Search(ctx, "how do goroutines work in Go concurrency")
	if err != nil || len(res) == 0 {
		t.Fatalf("search: %v %v", res, err)
	}
	if res[0].Chunk.Source != "go.txt" {
		t.Fatalf("top hit = %s, want go.txt", res[0].Chunk.Source)
	}
	if len(res) > ix.TopK {
		t.Fatalf("returned %d > topK", len(res))
	}
}

func TestIngestReplacesSource(t *testing.T) {
	ctx := context.Background()
	ix := newIndex(t)
	ix.IngestText(ctx, "a", "alpha beta gamma")
	ix.IngestText(ctx, "a", "delta epsilon")
	if n := ix.Store.Len(); n != 1 {
		t.Fatalf("want 1 chunk after re-ingest, got %d", n)
	}
	res, _ := ix.Search(ctx, "alpha")
	if len(res) != 0 {
		t.Fatalf("old content still searchable: %v", res)
	}
}

func TestStorePersistenceAndRemove(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "s.json")
	st, _ := OpenStore(path)
	ix := &Index{Store: st, Embedder: LocalEmbedder{}, ChunkSize: 100, TopK: 3}
	ix.IngestText(ctx, "one", "hello world")
	ix.IngestText(ctx, "two", "goodbye world")

	re, err := OpenStore(path)
	if err != nil || re.Len() != 2 || re.EmbedderID() != (LocalEmbedder{}).ID() {
		t.Fatalf("reload: len=%d id=%q err=%v", re.Len(), re.EmbedderID(), err)
	}
	if srcs := re.Sources(); len(srcs) != 2 || srcs[0].Source != "one" {
		t.Fatalf("sources: %v", srcs)
	}
	if n, _ := re.RemoveSource("one"); n != 1 {
		t.Fatalf("removed %d", n)
	}
	if n, _ := re.RemoveSource("missing"); n != 0 {
		t.Fatalf("removed %d for missing", n)
	}
	re.Clear()
	if re.Len() != 0 || re.EmbedderID() != "" {
		t.Fatal("clear did not reset")
	}
}

func TestEmbedderMismatch(t *testing.T) {
	ctx := context.Background()
	ix := newIndex(t)
	ix.IngestText(ctx, "a", "some text here")

	other := FuncEmbedder{Name: "other", Fn: LocalEmbedder{}.Embed}
	ix2 := &Index{Store: ix.Store, Embedder: other, ChunkSize: 100, TopK: 1}
	if _, err := ix2.IngestText(ctx, "b", "more text"); !errors.Is(err, ErrEmbedderMismatch) {
		t.Fatalf("ingest: want mismatch, got %v", err)
	}
	if _, err := ix2.Search(ctx, "text"); !errors.Is(err, ErrEmbedderMismatch) {
		t.Fatalf("search: want mismatch, got %v", err)
	}
	// Replacing the only source with a new embedder is allowed.
	if _, err := ix2.IngestText(ctx, "a", "replaced"); err != nil {
		t.Fatalf("replacing sole source should be allowed: %v", err)
	}
}

func TestFailedEmbeddingKeepsExistingData(t *testing.T) {
	ctx := context.Background()
	ix := newIndex(t)
	ix.IngestText(ctx, "a", "keep me")
	ix.Embedder = FuncEmbedder{Name: LocalEmbedder{}.ID(), Fn: func(context.Context, []string) ([][]float32, error) {
		return nil, errors.New("boom")
	}}
	if _, err := ix.IngestText(ctx, "a", "new content"); err == nil {
		t.Fatal("expected error")
	}
	if ix.Store.Len() != 1 || ix.Store.Sources()[0].Chunks != 1 {
		t.Fatal("existing data was lost")
	}
}

func TestSearchEmptyStoreSkipsEmbedder(t *testing.T) {
	ix := newIndex(t)
	ix.Embedder = FuncEmbedder{Name: "x", Fn: func(context.Context, []string) ([][]float32, error) {
		t.Fatal("embedder must not be called for an empty store")
		return nil, nil
	}}
	res, err := ix.Search(context.Background(), "q")
	if err != nil || res != nil {
		t.Fatalf("got %v, %v", res, err)
	}
}

func TestBatching(t *testing.T) {
	var calls, total int
	ix := newIndex(t)
	ix.ChunkSize, ix.Overlap = 10, 0
	ix.Embedder = FuncEmbedder{Name: "b", Fn: func(ctx context.Context, tx []string) ([][]float32, error) {
		calls++
		total += len(tx)
		return LocalEmbedder{}.Embed(ctx, tx)
	}}
	n, err := ix.IngestText(context.Background(), "big", strings.Repeat("word ", 400))
	if err != nil || n <= embedBatch {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if calls < 2 || total != n {
		t.Fatalf("calls=%d total=%d n=%d", calls, total, n)
	}
}
