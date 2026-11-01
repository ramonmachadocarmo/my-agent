package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandMentionsAttachesFiles(t *testing.T) {
	inDir(t, t.TempDir())
	writeFile(t, "notes.txt", "alpha beta\n")
	writeFile(t, "src/main.go", "package main")

	msg, atts, warns := expandMentions("resuma @notes.txt e compare com @src/main.go, por favor")
	if len(warns) != 0 || len(atts) != 2 {
		t.Fatalf("warns=%v atts=%v", warns, atts)
	}
	for _, want := range []string{
		"resuma @notes.txt e compare com @src/main.go, por favor", // the question is kept as typed
		"[Attached file: notes.txt]", "alpha beta",
		"[Attached file: src/main.go]", "package main",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message is missing %q:\n%s", want, msg)
		}
	}
	if !strings.HasPrefix(msg, "resuma") {
		t.Errorf("the user's text must come first: %q", msg)
	}
	if d := atts[0].describe(); !strings.Contains(d, "notes.txt") || !strings.Contains(d, "B") {
		t.Errorf("describe: %q", d)
	}
}

func TestExpandMentionsForgivesPunctuationAndDuplicates(t *testing.T) {
	inDir(t, t.TempDir())
	writeFile(t, "a.md", "conteúdo")
	for _, in := range []string{"veja @a.md.", "veja @a.md, ok", "(@a.md)", "veja @a.md?", `"@a.md"`} {
		if _, atts, _ := expandMentions(in); len(atts) != 1 {
			t.Errorf("%q: attached %d files", in, len(atts))
		}
	}
	if msg, atts, _ := expandMentions("@a.md e de novo @a.md"); len(atts) != 1 || strings.Count(msg, "[Attached file:") != 1 {
		t.Errorf("the same file must be attached once: %d", len(atts))
	}
}

func TestExpandMentionsLeavesNonFilesAlone(t *testing.T) {
	inDir(t, t.TempDir())
	// things that merely contain @ are not files and must not produce noise
	for _, in := range []string{"fale com @maria", "ramon@gmail.com", "use o @ como separador", "sem menções"} {
		msg, atts, warns := expandMentions(in)
		if msg != in || len(atts) != 0 || len(warns) != 0 {
			t.Errorf("%q: msg=%q atts=%v warns=%v", in, msg, atts, warns)
		}
	}
	// but something that looks like a path and is missing is worth a warning
	for _, in := range []string{"leia @nao-existe.txt", "leia @pasta/nada.md"} {
		_, atts, warns := expandMentions(in)
		if len(atts) != 0 || len(warns) != 1 || !strings.Contains(warns[0], "not found") {
			t.Errorf("%q: atts=%v warns=%v", in, atts, warns)
		}
	}
}

func TestExpandMentionsFoldersAndBinaries(t *testing.T) {
	inDir(t, t.TempDir())
	writeFile(t, "docs/a.md", "x")
	if err := os.WriteFile("blob.bin", []byte("abc\x00def"), 0o644); err != nil {
		t.Fatal(err)
	}
	msg, atts, warns := expandMentions("olhe @docs e @blob.bin")
	if len(atts) != 0 || msg != "olhe @docs e @blob.bin" || len(warns) != 2 {
		t.Fatalf("msg=%q atts=%v warns=%v", msg, atts, warns)
	}
	if !strings.Contains(warns[0], "folder") || !strings.Contains(warns[0], "/rag ingest") {
		t.Errorf("folder warning should point at /rag ingest: %q", warns[0])
	}
	if !strings.Contains(warns[1], "binary") {
		t.Errorf("binary warning: %q", warns[1])
	}
}

func TestExpandMentionsLimits(t *testing.T) {
	inDir(t, t.TempDir())
	big := bytes.Repeat([]byte("linha de texto\n"), maxAttachBytes/10) // well over the cap
	if err := os.WriteFile("big.txt", big, 0o644); err != nil {
		t.Fatal(err)
	}
	msg, atts, _ := expandMentions("@big.txt")
	if len(atts) != 1 || !atts[0].truncated || len(atts[0].content) != maxAttachBytes {
		t.Fatalf("truncation: %+v", atts)
	}
	if !strings.Contains(msg, "(content truncated)") || !strings.Contains(atts[0].describe(), "first") {
		t.Errorf("the truncation must be visible to the model and the user")
	}

	var words []string
	for i := 0; i < maxAttachFiles+2; i++ {
		name := filepath.Join("f", string(rune('a'+i))+".txt")
		writeFile(t, name, "x")
		words = append(words, "@"+filepath.ToSlash(name))
	}
	_, atts, warns := expandMentions(strings.Join(words, " "))
	if len(atts) != maxAttachFiles || len(warns) != 2 || !strings.Contains(warns[0], "at most") {
		t.Fatalf("limit of %d files: atts=%d warns=%v", maxAttachFiles, len(atts), warns)
	}
}
