package cli

import (
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"my-agent/internal/i18n"
)

// suggestion is one autocomplete candidate.
type suggestion struct {
	insert string // replaces the word being typed
	label  string
	desc   string
	space  bool // add a space after inserting (more arguments can follow)
}

type suggestFunc func(line string) ([]suggestion, int)

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// filterVals turns values into suggestions, keeping those that start with
// prefix (case-insensitive).
func filterVals(vals []string, prefix string) []suggestion {
	p := strings.ToLower(prefix)
	var out []suggestion
	for _, v := range vals {
		if strings.HasPrefix(strings.ToLower(v), p) {
			out = append(out, suggestion{insert: v, label: v, space: true})
		}
	}
	return out
}

// suggest returns the completions for the word under the cursor and the
// rune index where that word starts. line is the text before the cursor.
//
//   - "/..."  completes the command tree, then per-command arguments
//   - "@..."  completes file paths (anywhere in the message)
func (c *chat) suggest(line string) ([]suggestion, int) {
	ts := strings.LastIndexAny(line, " \t") + 1
	tok := line[ts:]
	start := utf8.RuneCountInString(line[:ts])

	if strings.HasPrefix(tok, "@") {
		return pathSuggestions(tok[1:], "@"), start
	}
	if !strings.HasPrefix(line, "/") {
		return nil, start
	}
	if ts == 0 { // still typing the command name
		return childSuggestions(c.tree, tok[1:], "/"), start
	}

	words := splitArgs(line[1:ts])
	level := c.tree
	var cur *node
	i := 0
	for ; i < len(words); i++ {
		m, _ := matchNode(level, words[i])
		if m == nil {
			break
		}
		cur = m
		if len(m.children) == 0 {
			i++
			break
		}
		level = m.children
	}
	switch {
	case cur == nil:
		return nil, start
	case len(cur.children) > 0 && i == len(words):
		return childSuggestions(cur.children, tok, ""), start
	case cur.complete != nil:
		return cur.complete(c, words[i:], tok), start
	}
	return nil, start
}

func childSuggestions(level []*node, prefix, lead string) []suggestion {
	p := strings.ToLower(prefix)
	var out []suggestion
	for _, n := range level {
		if !strings.HasPrefix(n.name, p) {
			continue
		}
		desc := n.desc
		if n.usage != "" {
			desc = n.usage + "  " + n.desc
		}
		out = append(out, suggestion{
			insert: lead + n.name,
			label:  lead + n.name,
			desc:   desc,
			space:  len(n.children) > 0 || n.usage != "",
		})
	}
	return out
}

// pathSuggestions completes a file system path. lead ("@" or "") is kept in
// front of every insertion. Folders end with "/" so completion can continue.
func pathSuggestions(prefix, lead string) []suggestion {
	i := strings.LastIndexAny(prefix, `/\`)
	dir, base := prefix[:i+1], prefix[i+1:]
	readDir := dir
	if readDir == "" {
		readDir = "."
	}
	entries, err := os.ReadDir(readDir)
	if err != nil {
		return nil
	}
	lowBase := strings.ToLower(base)
	type item struct {
		name  string
		isDir bool
		size  int64
	}
	var items []item
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(name), lowBase) {
			continue
		}
		it := item{name: name, isDir: e.IsDir()}
		if info, err := os.Stat(readDir + "/" + name); err == nil { // follows symlinks
			it.isDir, it.size = info.IsDir(), info.Size()
		}
		items = append(items, it)
	}
	sort.Slice(items, func(a, b int) bool {
		if items[a].isDir != items[b].isDir {
			return items[a].isDir
		}
		return strings.ToLower(items[a].name) < strings.ToLower(items[b].name)
	})
	if len(items) > 60 {
		items = items[:60]
	}
	out := make([]suggestion, 0, len(items))
	for _, it := range items {
		s := suggestion{insert: lead + dir + it.name, label: it.name}
		if it.isDir {
			s.insert += "/"
			s.label += "/"
			s.desc = i18n.T("suggest.folder")
		} else {
			s.desc = formatBytes(it.size)
			s.space = true
		}
		out = append(out, s)
	}
	return out
}
