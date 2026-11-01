package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func run(t *testing.T, env *Env, name string, args map[string]any) (Result, error) {
	t.Helper()
	s := NewSet(Builtins(env)...)
	tool, ok := s.Get(name)
	if !ok {
		t.Fatalf("no tool %s", name)
	}
	return tool.Run(context.Background(), env, args)
}

func testEnv(t *testing.T) *Env {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src", "sub"), 0o755)
	os.MkdirAll(filepath.Join(dir, "node_modules", "x"), 0o755)
	os.WriteFile(filepath.Join(dir, "src", "a.go"), []byte("package a\n\nfunc Hello() {}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "src", "sub", "b.go"), []byte("package sub\n// TODO: fix\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "node_modules", "x", "c.go"), []byte("TODO"), 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hi\n"), 0o644)
	sh := []string{"/bin/sh", "-c"}
	if runtime.GOOS == "windows" {
		sh = []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command"}
	}
	return &Env{Dir: dir, Shell: sh}
}

func TestReadWithLineNumbersAndOffset(t *testing.T) {
	env := testEnv(t)
	res, err := run(t, env, "read", map[string]any{"path": "src/a.go"})
	if err != nil || !strings.Contains(res.Output, "     3\tfunc Hello() {}") {
		t.Fatalf("%q %v", res.Output, err)
	}
	res, err = run(t, env, "read", map[string]any{"path": "src/a.go", "offset": float64(3), "limit": float64(1)})
	if err != nil || strings.Contains(res.Output, "package") || !strings.Contains(res.Output, "Hello") {
		t.Fatalf("%q %v", res.Output, err)
	}
	if _, err := run(t, env, "read", map[string]any{"path": "src"}); err == nil {
		t.Fatal("reading a directory must fail")
	}
}

func TestWriteAndEdit(t *testing.T) {
	env := testEnv(t)
	if _, err := run(t, env, "write", map[string]any{"path": "new/dir/f.txt", "content": "one\ntwo\nthree\n"}); err != nil {
		t.Fatal(err)
	}
	res, err := run(t, env, "edit", map[string]any{"path": "new/dir/f.txt", "old_string": "two", "new_string": "2"})
	if err != nil || !strings.Contains(res.Display, "- two") || !strings.Contains(res.Display, "+ 2") {
		t.Fatalf("%+v %v", res, err)
	}
	data, _ := os.ReadFile(filepath.Join(env.Dir, "new", "dir", "f.txt"))
	if string(data) != "one\n2\nthree\n" {
		t.Fatalf("file: %q", data)
	}
	if _, err := run(t, env, "edit", map[string]any{"path": "new/dir/f.txt", "old_string": "missing", "new_string": "x"}); err == nil {
		t.Fatal("editing text that is not there must fail")
	}
	os.WriteFile(filepath.Join(env.Dir, "dup.txt"), []byte("a a a"), 0o644)
	if _, err := run(t, env, "edit", map[string]any{"path": "dup.txt", "old_string": "a", "new_string": "b"}); err == nil {
		t.Fatal("an ambiguous edit must fail")
	}
	if _, err := run(t, env, "edit", map[string]any{"path": "dup.txt", "old_string": "a", "new_string": "b", "replace_all": true}); err != nil {
		t.Fatal(err)
	}
	// CRLF files accept LF snippets
	os.WriteFile(filepath.Join(env.Dir, "crlf.txt"), []byte("x\r\ny\r\n"), 0o644)
	if _, err := run(t, env, "edit", map[string]any{"path": "crlf.txt", "old_string": "x\ny", "new_string": "x\nz"}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(env.Dir, "crlf.txt"))
	if string(data) != "x\r\nz\r\n" {
		t.Fatalf("crlf: %q", data)
	}
}

func TestGlobAndGrepSkipDependencyFolders(t *testing.T) {
	env := testEnv(t)
	res, err := run(t, env, "glob", map[string]any{"pattern": "**/*.go"})
	if err != nil || !strings.Contains(res.Output, "src/a.go") || !strings.Contains(res.Output, "src/sub/b.go") || strings.Contains(res.Output, "node_modules") {
		t.Fatalf("%q %v", res.Output, err)
	}
	res, err = run(t, env, "glob", map[string]any{"pattern": "src/*.go"})
	if err != nil || strings.Contains(res.Output, "b.go") {
		t.Fatalf("single star must not cross folders: %q", res.Output)
	}
	res, err = run(t, env, "grep", map[string]any{"pattern": "TODO", "include": "*.go"})
	if err != nil || !strings.Contains(res.Output, "src/sub/b.go:2:") || strings.Contains(res.Output, "node_modules") {
		t.Fatalf("%q %v", res.Output, err)
	}
}

func TestBashReportsExitCode(t *testing.T) {
	env := testEnv(t)
	res, err := run(t, env, "bash", map[string]any{"command": "echo hello"})
	if err != nil || !strings.Contains(res.Output, "hello") || !strings.Contains(res.Output, "[exit code 0]") {
		t.Fatalf("%q %v", res.Output, err)
	}
	res, err = run(t, env, "bash", map[string]any{"command": "exit 3"})
	if err != nil || !strings.Contains(res.Output, "[exit code 3]") {
		t.Fatalf("%q %v", res.Output, err)
	}
}

func TestDiff(t *testing.T) {
	d := Diff("a\nb\nc\nd\n", "a\nb\nX\nd\n")
	if !strings.Contains(d, "- c") || !strings.Contains(d, "+ X") || !strings.Contains(d, "  b") {
		t.Fatalf("%q", d)
	}
}
