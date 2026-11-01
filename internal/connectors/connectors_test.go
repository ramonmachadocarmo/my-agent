package connectors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"my-agent/internal/config"
	"my-agent/internal/rag"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func names(docs []Document) []string {
	var out []string
	for _, d := range docs {
		out = append(out, filepath.Base(d.Source))
	}
	sort.Strings(out)
	return out
}

func TestFileLoaderDirectory(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.md"), "alpha")
	write(t, filepath.Join(dir, "b.txt"), "beta")
	write(t, filepath.Join(dir, "img.png"), "not included")
	write(t, filepath.Join(dir, "sub", "c.md"), "gamma")
	write(t, filepath.Join(dir, ".git", "d.md"), "hidden")
	write(t, filepath.Join(dir, "node_modules", "e.md"), "deps")
	if err := os.WriteFile(filepath.Join(dir, "bin.txt"), []byte("a\x00b"), 0o644); err != nil {
		t.Fatal(err)
	}

	docs, err := FileLoader{Path: dir, Recursive: true}.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names(docs), ","); got != "a.md,b.txt,c.md" {
		t.Fatalf("recursive: %s", got)
	}

	docs, _ = FileLoader{Path: dir, Recursive: false}.Load(context.Background())
	if got := strings.Join(names(docs), ","); got != "a.md,b.txt" {
		t.Fatalf("non-recursive: %s", got)
	}

	docs, _ = FileLoader{Path: dir, Include: []string{"*.md"}, Recursive: true}.Load(context.Background())
	if got := strings.Join(names(docs), ","); got != "a.md,c.md" {
		t.Fatalf("include: %s", got)
	}
}

func TestFileLoaderSingleFileAndErrors(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "x.weird")
	write(t, f, "content")
	docs, err := FileLoader{Path: f}.Load(context.Background())
	if err != nil || len(docs) != 1 || docs[0].Text != "content" || !filepath.IsAbs(docs[0].Source) {
		t.Fatalf("single file: %v %v", docs, err)
	}
	if _, err := (FileLoader{Path: filepath.Join(dir, "missing")}).Load(context.Background()); err == nil {
		t.Fatal("missing path should error")
	}
}

func TestURLLoaderStripsHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><style>p{color:red}</style><script>alert(1)</script></head>
<body><h1>Title</h1><p>Hello &amp; welcome</p></body></html>`))
	}))
	defer srv.Close()
	docs, err := URLLoader{URL: srv.URL}.Load(context.Background())
	if err != nil || len(docs) != 1 {
		t.Fatal(docs, err)
	}
	txt := docs[0].Text
	if !strings.Contains(txt, "Title") || !strings.Contains(txt, "Hello & welcome") {
		t.Fatalf("text: %q", txt)
	}
	if strings.Contains(txt, "alert") || strings.Contains(txt, "color:red") || strings.Contains(txt, "<") {
		t.Fatalf("markup leaked: %q", txt)
	}
}

func TestURLLoaderHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if _, err := (URLLoader{URL: srv.URL}).Load(context.Background()); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("got %v", err)
	}
}

func TestNew(t *testing.T) {
	if _, err := New(config.ConnectorConfig{Type: "file"}); err == nil {
		t.Error("file without path")
	}
	if _, err := New(config.ConnectorConfig{Type: "url"}); err == nil {
		t.Error("url without url")
	}
	if _, err := New(config.ConnectorConfig{Type: "ftp"}); err == nil {
		t.Error("unknown type")
	}
	if l, err := New(config.ConnectorConfig{Type: "file", Path: "."}); err != nil || l == nil {
		t.Errorf("valid file connector: %v", err)
	}
}

func TestIngest(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "docs", "go.md"), "Goroutines are lightweight threads managed by the Go runtime.")
	write(t, filepath.Join(dir, "docs", "cook.md"), "Knead the dough and let it rise before baking.")

	st, _ := rag.OpenStore(filepath.Join(dir, "store.json"))
	ix := &rag.Index{Store: st, Embedder: rag.LocalEmbedder{}, ChunkSize: 200, TopK: 1, MinScore: 0.01}
	l, _ := New(config.ConnectorConfig{Type: "file", Path: filepath.Join(dir, "docs"), Recursive: true})

	stats, err := Ingest(context.Background(), ix, l)
	if err != nil || stats.Documents != 2 || stats.Chunks != 2 {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}
	// Syncing again is idempotent.
	if _, err := Ingest(context.Background(), ix, l); err != nil || st.Len() != 2 {
		t.Fatalf("re-ingest: len=%d err=%v", st.Len(), err)
	}
	res, _ := ix.Search(context.Background(), "goroutines threads")
	if len(res) != 1 || filepath.Base(res[0].Chunk.Source) != "go.md" {
		t.Fatalf("search: %v", res)
	}
}
