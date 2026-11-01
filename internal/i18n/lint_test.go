package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// User-facing text must come from the catalog. This test reads the Go sources
// and reports any sentence written directly in a call that prints it or turns
// it into an error, so a new hard-coded message cannot slip in untranslated.
//
// Mark a genuine exception with a trailing comment:  // i18n:ok
//
// Only prose is flagged: a string with at least two words (after removing
// printf verbs and punctuation). Identifiers, keys and syntax such as
// "providers.%s.model = %s" or "usage" tokens are not prose.

var printfVerb = regexp.MustCompile(`%(?:\[\d+\])?[-+# 0]*(?:\d+|\*)?(?:\.\d+)?[a-zA-Z%]`)

func isProse(s string) bool {
	s = printfVerb.ReplaceAllString(s, "#") // a verb makes its token "impure" (# is not a letter): not a word
	words := 0
	for _, tok := range strings.Fields(s) {
		letters := 0
		pure := true
		if tok == strings.ToUpper(tok) { // GET, GPU, CPU, OK: acronyms and status markers
			continue
		}
		for _, r := range tok {
			switch {
			case unicode.IsLetter(r):
				letters++
			case strings.ContainsRune(".,;:!?()[]{}'\"“”…—-", r):
				// punctuation around a word does not make it a non-word
			default:
				pure = false
			}
		}
		if pure && letters >= 3 {
			words++
		}
	}
	return words >= 2
}

// checkedCall says which arguments of a call carry user-facing text.
func checkedArgs(call *ast.CallExpr) []ast.Expr {
	var name, pkg string
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		name = fn.Sel.Name
		if id, ok := fn.X.(*ast.Ident); ok {
			pkg = id.Name
		}
	case *ast.Ident:
		name = fn.Name
	default:
		return nil
	}
	switch {
	case pkg == "fmt" && strings.HasPrefix(name, "Fprint"):
		if len(call.Args) > 1 {
			return call.Args[1:]
		}
	case pkg == "fmt" && name == "Errorf", pkg == "errors" && name == "New":
		return call.Args
	case name == "String" || name == "Bool" || name == "Int" || name == "StringVar": // flag descriptions
		if n := len(call.Args); n >= 3 {
			return call.Args[n-1:]
		}
	}
	return nil
}

func TestNoHardCodedUserFacingText(t *testing.T) {
	dirs := []string{"cli", "config", "provider", "app", "connectors", "rag", "agent", "web", "tui", "session"}
	fset := token.NewFileSet()
	found := 0
	for _, d := range dirs {
		dir := filepath.Join("..", d)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(src), "\n")
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				for _, arg := range checkedArgs(call) {
					lit, ok := arg.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					text, err := strconv.Unquote(lit.Value)
					if err != nil || !isProse(text) {
						continue
					}
					pos := fset.Position(lit.Pos())
					if pos.Line-1 < len(lines) && strings.Contains(lines[pos.Line-1], "i18n:ok") {
						continue
					}
					found++
					t.Errorf("%s:%d: hard-coded text %q: move it to the catalog (or mark the line // i18n:ok)", path, pos.Line, text)
				}
				return true
			})
		}
	}
	if found == 0 {
		t.Log("no hard-coded user-facing text found")
	}
}

func TestProseDetector(t *testing.T) {
	for s, want := range map[string]bool{
		"providers.%s.model = %s":         false,
		"%s: %w":                          false,
		"OK   %-10s %s":                   false,
		"FAIL %-10s %v":                   false,
		"  [%d] %s (score %.2f)":          false,
		"\r%-72s":                         false,
		"%d%% GPU / %d%% CPU":             false, // GPU/CPU are single tokens; no prose
		"quit":                            false,
		"model name is empty":             true,
		"%s ready (%s). Commands: /help":  true,
		"%s: %d document(s), %d chunk(s)": true,
		"nenhum modelo instalado":         true,
		"você":                            false,
		"unknown %s subcommand %q":        true,
		"GET %s: http %d":                 false,
		"file connector needs a path":     true,
	} {
		if got := isProse(s); got != want {
			t.Errorf("isProse(%q) = %v, want %v", s, got, want)
		}
	}
}
