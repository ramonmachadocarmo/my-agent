package cli

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/provider"
)

// chatEnv is a chat wired to the fake provider, with captured output.
type chatEnv struct {
	c        *chat
	out, err *bytes.Buffer
	fp       *fakeProvider
	cfgPath  string
}

// newTestChat builds a chat over a default config in a temp dir. script is
// what the user "types" (plain mode: one line per newline).
func newTestChat(t *testing.T, script string) *chatEnv {
	t.Helper()
	cfgPath, fp := setup(t)
	if err := config.Default().Save(cfgPath); err != nil {
		t.Fatal(err)
	}
	a, err := loadApp(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	rd := bufio.NewReader(strings.NewReader(script))
	e := &env{in: strings.NewReader(script), out: out, err: errb, cfgPath: cfgPath}
	c, err := e.newChat(context.Background(), a, rd, "", true)
	if err != nil {
		t.Fatal(err)
	}
	return &chatEnv{c: c, out: out, err: errb, fp: fp, cfgPath: cfgPath}
}

// inDir runs the test with dir as the working directory.
func inDir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func inserts(ss []suggestion) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.insert
	}
	return out
}

func hasAll(got []string, want ...string) bool {
	for _, w := range want {
		if !contains(got, w) {
			return false
		}
	}
	return true
}

var _ provider.Provider = (*fakeProvider)(nil)
var _ = app.Load
