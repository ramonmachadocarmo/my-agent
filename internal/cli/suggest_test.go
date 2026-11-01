package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"my-agent/internal/provider"
)

func TestSuggestSlashListsCommands(t *testing.T) {
	ce := newTestChat(t, "")
	got, start := ce.c.suggest("/")
	names := inserts(got)
	if start != 0 || !hasAll(names, "/config", "/rag", "/models", "/providers", "/connectors", "/chat", "/help", "/reset", "/sources", "/model", "/clear", "/exit") {
		t.Fatalf("start=%d names=%v", start, names)
	}
	// main areas come before the shortcuts
	if names[0] != "/config" {
		t.Errorf("first suggestion should be /config, got %v", names)
	}
	for _, s := range got {
		if s.desc == "" {
			t.Errorf("%s has no description", s.insert)
		}
	}
}

func TestSuggestFiltersByPrefixAndKnowsWhichTakeArguments(t *testing.T) {
	ce := newTestChat(t, "")
	got, _ := ce.c.suggest("/co")
	if names := inserts(got); len(names) != 2 || names[0] != "/config" || names[1] != "/connectors" {
		t.Fatalf("names=%v", names)
	}
	if !got[0].space {
		t.Error("/config has subcommands: a space should follow so the next level opens")
	}
	got, _ = ce.c.suggest("/rese")
	if len(got) != 1 || got[0].insert != "/reset" || got[0].space {
		t.Fatalf("/reset takes no arguments, so no trailing space: %+v", got)
	}
	if got, _ := ce.c.suggest("/zzz"); len(got) != 0 {
		t.Fatalf("no match should mean no suggestions: %v", inserts(got))
	}
}

func TestSuggestDrillsDownTheHierarchy(t *testing.T) {
	ce := newTestChat(t, "")
	cases := []struct {
		line  string
		start int
		want  []string
	}{
		{"/config ", 8, []string{"chat", "rag", "provider", "language", "show", "get", "set", "validate", "path"}},
		{"/config c", 8, []string{"chat"}},
		{"/config chat ", 13, []string{"provider", "model", "temperature", "max-tokens", "prompt", "rag"}},
		{"/config chat t", 13, []string{"temperature"}},
		{"/config rag ", 12, []string{"embedder", "top-k", "min-score", "chunk-size", "chunk-overlap", "store"}},
		{"/rag ", 5, []string{"ingest", "search", "list", "remove", "clear"}},
		{"/models ", 8, []string{"list", "ps", "select", "use", "pull"}},
		{"/providers ", 11, []string{"list", "test", "use"}},
		{"/connectors ", 12, []string{"list", "add", "remove", "sync"}},
		{"/help ", 6, []string{"config", "rag", "models", "providers", "connectors", "chat", "help"}},
		{"/help config ", 13, []string{"chat", "rag", "provider", "language", "show", "get", "set", "validate", "path"}},
	}
	for _, tc := range cases {
		got, start := ce.c.suggest(tc.line)
		if start != tc.start {
			t.Errorf("%q: token starts at %d, want %d", tc.line, start, tc.start)
		}
		if names := inserts(got); !hasAll(names, tc.want...) || len(names) != len(tc.want) {
			t.Errorf("%q: got %v, want %v", tc.line, names, tc.want)
		}
	}
}

func TestSuggestValuesForArguments(t *testing.T) {
	ce := newTestChat(t, "")
	ce.fp.models = []provider.ModelInfo{{Name: "llama3.2:latest"}, {Name: "qwen2.5:14b"}}
	src := filepath.Join(t.TempDir(), "doc.md")
	writeFile(t, src, "conteúdo sobre pinguins")
	if err := ce.c.e.ragCommand(ce.c.ctx, ce.c.a, []string{"ingest", src}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		line string
		want []string
	}{
		{"/config chat provider ", []string{"claude", "gemini", "ollama"}},
		{"/config chat provider g", []string{"gemini"}},
		{"/config chat rag ", []string{"on", "off"}},
		{"/config chat model ", []string{"llama3.2:latest", "qwen2.5:14b"}},
		{"/config chat model q", []string{"qwen2.5:14b"}},
		{"/config rag embedder ", []string{"local", "claude", "gemini", "ollama"}},
		{"/config provider ", []string{"claude", "gemini", "ollama"}},
		{"/config provider gemini ", providerFields},
		{"/config provider gemini api", []string{"api_key", "api_key_env"}},
		{"/config get rag.t", []string{"rag.top_k"}},
		{"/config set agent.", []string{"agent.max_tokens", "agent.name", "agent.provider", "agent.system_prompt", "agent.temperature", "agent.use_rag"}},
		{"/models use ", []string{"llama3.2:latest", "qwen2.5:14b"}},
		{"/providers use o", []string{"ollama"}},
		{"/providers test ", []string{"claude", "gemini", "ollama"}},
		{"/rag remove ", []string{src}},
	}
	for _, tc := range cases {
		got, _ := ce.c.suggest(tc.line)
		names := inserts(got)
		if !hasAll(names, tc.want...) || len(names) != len(tc.want) {
			t.Errorf("%q: got %v, want %v", tc.line, names, tc.want)
		}
	}
	// arguments already given are not suggested again
	if got, _ := ce.c.suggest("/config chat provider gemini "); len(got) != 0 {
		t.Errorf("a finished argument must not be re-suggested: %v", inserts(got))
	}
	if got, _ := ce.c.suggest("/config get rag.top_k "); len(got) != 0 {
		t.Errorf("after the key there is nothing to suggest: %v", inserts(got))
	}
}

func TestSuggestConnectorNames(t *testing.T) {
	ce := newTestChat(t, "")
	dir := t.TempDir()
	if err := ce.c.e.connectorsCommand(ce.c.ctx, ce.c.a, []string{"add", "--name", "docs", "--type", "file", "--path", dir}); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"/connectors remove ", "/connectors sync "} {
		got, _ := ce.c.suggest(line)
		if names := inserts(got); len(names) != 1 || names[0] != "docs" {
			t.Errorf("%q: %v", line, names)
		}
	}
}

func TestSuggestFilePathsForMentions(t *testing.T) {
	dir := t.TempDir()
	inDir(t, dir)
	writeFile(t, "README.md", "x")
	writeFile(t, "notes.txt", "yy")
	writeFile(t, "src/main.go", "z")
	writeFile(t, "src/util.go", "z")
	writeFile(t, ".secret", "hidden")
	ce := newTestChat(t, "")

	// "@" anywhere in a normal message completes paths, folders first
	got, start := ce.c.suggest("explique o @")
	names := inserts(got)
	if start != len("explique o ") {
		t.Fatalf("token start %d", start)
	}
	if len(names) != 3 || names[0] != "@src/" || names[1] != "@notes.txt" || names[2] != "@README.md" {
		t.Fatalf("folders first, then files, hidden files skipped: %v", names)
	}
	if got[0].space || !got[1].space {
		t.Error("a folder keeps going (no space); a file ends the word")
	}
	if got[0].desc != "folder" || got[1].desc == "" {
		t.Errorf("descriptions: %q %q", got[0].desc, got[1].desc)
	}

	if got, _ := ce.c.suggest("veja @sr"); len(got) != 1 || got[0].insert != "@src/" {
		t.Errorf("prefix: %v", inserts(got))
	}
	got, _ = ce.c.suggest("veja @src/")
	if names := inserts(got); len(names) != 2 || names[0] != "@src/main.go" || names[1] != "@src/util.go" {
		t.Errorf("inside a folder: %v", names)
	}
	if got, _ := ce.c.suggest("veja @SRC/M"); len(got) != 1 || got[0].insert != "@SRC/main.go" { // the folder stays as typed
		t.Errorf("case-insensitive: %v", inserts(got))
	}
	if got, _ := ce.c.suggest("veja @.s"); len(got) != 1 || got[0].insert != "@.secret" {
		t.Errorf("typing a dot shows hidden files: %v", inserts(got))
	}
	if got, _ := ce.c.suggest("veja @nao-existe/"); len(got) != 0 {
		t.Errorf("missing folder: %v", inserts(got))
	}
	// plain text without / or @ never suggests anything
	if got, _ := ce.c.suggest("olá, tudo bem?"); len(got) != 0 {
		t.Errorf("plain chat text: %v", inserts(got))
	}
	// @ also works inside a slash command's arguments
	if got, _ := ce.c.suggest("/config get @"); len(got) == 0 || !strings.HasPrefix(got[0].insert, "@") {
		t.Errorf("@ inside a command: %v", inserts(got))
	}
}

func TestSuggestPathsForRagIngestNeedNoAt(t *testing.T) {
	inDir(t, t.TempDir())
	writeFile(t, "docs/a.md", "x")
	writeFile(t, "docs/b.md", "x")
	ce := newTestChat(t, "")
	got, start := ce.c.suggest("/rag ingest do")
	if start != len("/rag ingest ") || len(got) != 1 || got[0].insert != "docs/" {
		t.Fatalf("start=%d %v", start, inserts(got))
	}
	got, _ = ce.c.suggest("/rag ingest docs/")
	if names := inserts(got); len(names) != 2 || names[0] != "docs/a.md" {
		t.Fatalf("%v", names)
	}
}

func TestSuggestModelCacheAvoidsHammeringTheServer(t *testing.T) {
	ce := newTestChat(t, "")
	ce.fp.models = []provider.ModelInfo{{Name: "one:latest"}}
	if got, _ := ce.c.suggest("/models use "); len(got) != 1 {
		t.Fatalf("%v", inserts(got))
	}
	ce.fp.models = append(ce.fp.models, provider.ModelInfo{Name: "two:latest"})
	if got, _ := ce.c.suggest("/models use "); len(got) != 1 {
		t.Fatalf("second call within the cache window must not re-query: %v", inserts(got))
	}
}
