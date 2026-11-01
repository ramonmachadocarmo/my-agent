package cli

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	cases := map[string][]string{
		"":                            nil,
		"   ":                         nil,
		"config chat model":           {"config", "chat", "model"},
		`  a   b  `:                   {"a", "b"},
		`prompt "Você é um pirata"`:   {"prompt", "Você é um pirata"},
		`prompt 'texto com espaço' x`: {"prompt", "texto com espaço", "x"},
		`say don't stop`:              {"say", "don't", "stop"},        // an apostrophe inside a word is just text
		`ingest C:\docs\a b.md`:       {"ingest", `C:\docs\a`, "b.md"}, // backslashes are left alone
		`ingest "C:\meus docs\a.md"`:  {"ingest", `C:\meus docs\a.md`},
		`"unterminated quote rest`:    {"unterminated quote rest"},
		`empty ""`:                    {"empty", ""},
		"tab\tseparated":              {"tab", "separated"},
	}
	for in, want := range cases {
		if got := splitArgs(in); !reflect.DeepEqual(got, want) {
			t.Errorf("splitArgs(%q) = %#v, want %#v", in, got, want)
		}
	}
}

func TestResolveWalksTheTree(t *testing.T) {
	ce := newTestChat(t, "")
	tree := ce.c.tree
	cases := []struct {
		in       string
		wantPath string
		wantArgs []string
	}{
		{"help", "help", nil},
		{"help config chat", "help", []string{"config", "chat"}},
		{"config chat model qwen2.5:7b", "config chat model", []string{"qwen2.5:7b"}},
		{"config chat prompt uma frase longa", "config chat prompt", []string{"uma", "frase", "longa"}},
		{"rag ingest ./docs ./mais", "rag ingest", []string{"./docs", "./mais"}},
		{"models use llama3.2", "models use", []string{"llama3.2"}},
		{"reset", "reset", nil},
		{"exit", "exit", nil},
		{"config", "config", nil},           // a group: prints its help
		{"CONFIG Chat", "config chat", nil}, // case-insensitive
		// a unique prefix works at every level
		{"he", "help", nil},
		{"conf ch mo qwen", "config chat model", []string{"qwen"}},
		{"prov use claude", "providers use", []string{"claude"}},
	}
	for _, tc := range cases {
		r, err := resolve(tree, splitArgs(tc.in))
		if err != nil {
			t.Errorf("%q: %v", tc.in, err)
			continue
		}
		if got := strings.Join(r.path, " "); got != tc.wantPath || !(len(r.args) == 0 && len(tc.wantArgs) == 0 || reflect.DeepEqual(r.args, tc.wantArgs)) {
			t.Errorf("%q: path=%q args=%v, want %q %v", tc.in, got, r.args, tc.wantPath, tc.wantArgs)
		}
	}
}

func TestResolveErrors(t *testing.T) {
	ce := newTestChat(t, "")
	cases := map[string]string{
		"co":             "ambiguous", // config and connectors
		"mod":            "ambiguous", // models and the /model shortcut: never guess
		"zzz":            "unknown command",
		"rag frobnicate": "unknown command",
		"config nope":    "unknown command",
		"":               "type a command",
	}
	for in, want := range cases {
		_, err := resolve(ce.c.tree, splitArgs(in))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want %q", in, err, want)
		}
	}
	// the message names the branch to look at
	_, err := resolve(ce.c.tree, []string{"rag", "frobnicate"})
	if err == nil || !strings.Contains(err.Error(), "/rag") {
		t.Errorf("error should point at /rag: %v", err)
	}
}

func TestHelpIsANumberedTreeGeneratedFromTheCommands(t *testing.T) {
	ce := newTestChat(t, "")
	ce.c.printHelp(nil)
	out := ce.out.String()
	for _, want := range []string{
		"1.    config", // top-level entries are numbered
		"1.1   chat",   // your hierarchy: config → chat, rag, ...
		"1.2   rag",
		"1.3   provider",
		"2.    rag",
		"3.    models",
		"4.    providers",
		"5.    connectors",
		"6.    chat",
		"7.    help",
		"Shortcuts:", "/reset", "/quit",
		"Attach:", "@",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("help is missing %q:\n%s", want, out)
		}
	}
	// aliases must not take a number of their own
	if strings.Contains(out, "8.") {
		t.Errorf("shortcuts must not be numbered:\n%s", out)
	}
}

func TestHelpForOneBranch(t *testing.T) {
	ce := newTestChat(t, "")
	ce.c.printHelp([]string{"config", "chat"})
	out := ce.out.String()
	for _, want := range []string{"1.1.", "chat", "1.1.1", "provider", "1.1.2", "model", "temperature", "max-tokens", "prompt", "rag"} {
		if !strings.Contains(out, want) {
			t.Errorf("branch help is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "connectors") {
		t.Errorf("branch help must stay inside its branch:\n%s", out)
	}
	ce.out.Reset()
	ce.c.printHelp([]string{"nope"})
	if !strings.Contains(ce.out.String(), "unknown command") {
		t.Errorf("unknown branch: %q", ce.out.String())
	}
}

func TestEveryCommandIsReachableAndDescribed(t *testing.T) {
	ce := newTestChat(t, "")
	var walk func(level []*node, path string)
	walk = func(level []*node, path string) {
		for _, n := range level {
			p := strings.TrimSpace(path + " " + n.name)
			if n.desc == "" {
				t.Errorf("/%s has no description (it would be blank in /help and autocomplete)", p)
			}
			if n.run == nil && len(n.children) == 0 {
				t.Errorf("/%s does nothing: no run function and no children", p)
			}
			if len(n.children) > 0 {
				walk(n.children, p)
			}
			if n.name != strings.ToLower(n.name) {
				t.Errorf("/%s: names must be lowercase (matching is case-insensitive on the input side only)", p)
			}
		}
	}
	walk(ce.c.tree, "")
}
