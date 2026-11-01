package cli

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"my-agent/internal/i18n"
)

// node is one entry of the in-chat command tree. The same tree drives the
// numbered /help, the autocomplete list and command execution, so they can
// never disagree.
type node struct {
	name     string
	usage    string // argument hint, e.g. "<name> <field> [value]"
	desc     string
	children []*node
	// run executes the command; nil for pure groups (which print their help).
	run func(c *chat, args []string) error
	// complete suggests values for the argument being typed. args are the
	// complete words typed after the command; prefix is the partial word.
	complete func(c *chat, args []string, prefix string) []suggestion
	alias    bool // shortcut of another command: hidden from the numbered help
}

// errQuit ends the chat.
var errQuit = errors.New("quit")

// splitArgs splits a command line on whitespace. A word that starts with a
// double or single quote runs to the matching quote, so arguments can hold
// spaces. Quotes inside a word (don't) and backslashes (Windows paths) are
// left alone.
func splitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	inWord := false
	var quote rune
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case (r == '"' || r == '\'') && !inWord:
			quote, inWord = r, true
		case unicode.IsSpace(r):
			if inWord {
				out = append(out, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		out = append(out, cur.String())
	}
	return out
}

// matchNode finds word among level: an exact name, else a unique prefix.
// When the prefix is ambiguous it returns the candidates.
func matchNode(level []*node, word string) (*node, []*node) {
	w := strings.ToLower(word)
	for _, n := range level {
		if n.name == w {
			return n, nil
		}
	}
	var cands []*node
	for _, n := range level {
		if strings.HasPrefix(n.name, w) {
			cands = append(cands, n)
		}
	}
	if len(cands) == 1 {
		return cands[0], nil
	}
	return nil, cands
}

// resolved is the result of walking the tree with a list of words.
type resolved struct {
	node *node
	path []string // names of the nodes walked, e.g. [config chat model]
	args []string // the words left over: arguments for node
}

// resolve walks the tree with tokens (the words after the leading "/").
func resolve(tree []*node, tokens []string) (resolved, error) {
	var r resolved
	level := tree
	for i := 0; i < len(tokens); i++ {
		m, cands := matchNode(level, tokens[i])
		if m == nil {
			if r.node == nil || r.node.run == nil {
				return r, unknownError(tokens[i], cands, r.path)
			}
			r.args = tokens[i:]
			return r, nil
		}
		r.node, r.path = m, append(r.path, m.name)
		if len(m.children) == 0 {
			r.args = tokens[i+1:]
			return r, nil
		}
		level = m.children
	}
	if r.node == nil {
		return r, i18n.Err("chat.err.type_command")
	}
	return r, nil
}

func unknownError(word string, cands []*node, parent []string) error {
	where := "/" + strings.Join(parent, " ")
	if len(parent) == 0 {
		where = ""
	}
	if len(cands) > 1 {
		names := make([]string, len(cands))
		for i, n := range cands {
			names[i] = n.name
		}
		return i18n.Errorf("chat.err.ambiguous", word, strings.Join(names, ", "))
	}
	help := "/help"
	if where != "" {
		help = where
	}
	return i18n.Errorf("chat.err.unknown_command", strings.TrimSpace(where+" "+word), help)
}

// printHelp prints the numbered command tree, or one branch of it.
func (c *chat) printHelp(path []string) {
	out, st := c.e.out, c.st
	if len(path) == 0 {
		fmt.Fprintln(out, st.bold(i18n.T("chat.help.title"))+st.dim(i18n.T("chat.help.subtitle")))
		n := 0
		for _, t := range c.tree {
			if t.alias {
				continue
			}
			n++
			c.writeNode(t, fmt.Sprint(n), 0, 1)
		}
		var shortcuts []string
		for _, t := range c.tree {
			if t.alias {
				shortcuts = append(shortcuts, "/"+t.name)
			}
		}
		fmt.Fprintf(out, "\n%s %s\n", st.dim(i18n.T("chat.help.shortcuts")), strings.Join(shortcuts, " "))
		fmt.Fprintf(out, "%s %s\n", st.dim(i18n.T("chat.help.attach")), i18n.T("chat.help.attach_text"))
		fmt.Fprintf(out, "%s %s\n", st.dim(i18n.T("chat.help.tip")), i18n.T("chat.help.tip_text"))
		return
	}
	r, err := resolve(c.tree, path)
	if err != nil {
		fmt.Fprintln(out, st.red(err.Error()))
		return
	}
	c.writeNode(r.node, c.number(r.path), 0, 2)
}

// number returns the numbering of a path in the help, e.g. [config chat] -> "1.1".
func (c *chat) number(path []string) string {
	level := c.tree
	var parts []string
	for _, name := range path {
		i, pos := 0, 0
		for _, n := range level {
			if n.alias && len(parts) == 0 {
				continue
			}
			i++
			if n.name == name {
				pos = i
				level = n.children
				break
			}
		}
		if pos == 0 {
			return ""
		}
		parts = append(parts, fmt.Sprint(pos))
	}
	return strings.Join(parts, ".")
}

// writeNode prints n and its descendants down to maxDepth levels below it.
func (c *chat) writeNode(n *node, num string, depth, maxDepth int) {
	st := c.st
	indent := strings.Repeat("   ", depth)
	label := num
	if depth == 0 && !strings.Contains(num, ".") {
		label += "." // "1." for a top-level entry, "1.1" below it
	}
	name := n.name
	if depth == 0 {
		name = st.bold(st.cyan(name))
	}
	left, plain := name, n.name
	if n.usage != "" {
		left += " " + st.dim(n.usage)
		plain += " " + n.usage
	}
	line := fmt.Sprintf("%s%-5s %s", indent, label, pad(left, 36, plain))
	line += " " + n.desc
	if len(n.children) > 0 && depth >= maxDepth {
		names := make([]string, len(n.children))
		for i, ch := range n.children {
			names[i] = ch.name
		}
		line += "  " + st.dim(strings.Join(names, " · "))
	}
	if depth == 0 {
		fmt.Fprintln(c.e.out) // blank line between top-level entries
	}
	fmt.Fprintln(c.e.out, line)
	if depth >= maxDepth {
		return
	}
	for i, ch := range n.children {
		c.writeNode(ch, fmt.Sprintf("%s.%d", num, i+1), depth+1, maxDepth)
	}
}

// pad right-pads s (which may contain ANSI codes) to width, measuring plain.
func pad(s string, width int, plain string) string {
	if n := width - len([]rune(plain)); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}
