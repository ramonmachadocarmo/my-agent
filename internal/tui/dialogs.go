package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"my-agent/internal/i18n"
)

// pickItem is one entry of a picker.
type pickItem struct {
	title, desc, group string
	value              string
	current            bool
}

// picker is a filterable list shown over the transcript.
type picker struct {
	title    string
	items    []pickItem
	filter   textinput.Model
	idx      int
	onPick   func(m *model, it pickItem) tea.Cmd
	onDelete func(m *model, it pickItem) // optional: ctrl+x removes the item
}

func newPicker(th theme, title string, items []pickItem, onPick func(*model, pickItem) tea.Cmd) *picker {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.Placeholder = i18n.T("tui.filter")
	ti.PromptStyle = th.fg(th.accent)
	ti.PlaceholderStyle = th.mutedS()
	ti.Focus()
	p := &picker{title: title, items: items, filter: ti, onPick: onPick}
	for i, it := range items {
		if it.current {
			p.idx = i
			break
		}
	}
	return p
}

func (p *picker) visible() []pickItem {
	q := strings.Fields(strings.ToLower(p.filter.Value()))
	if len(q) == 0 {
		return p.items
	}
	var out []pickItem
	for _, it := range p.items {
		hay := strings.ToLower(it.title + " " + it.desc + " " + it.group)
		ok := true
		for _, w := range q {
			if !strings.Contains(hay, w) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, it)
		}
	}
	return out
}

func (p *picker) update(m *model, k tea.KeyMsg) tea.Cmd {
	vis := p.visible()
	switch k.String() {
	case "esc", "ctrl+c":
		m.dialog = nil
		return nil
	case "up", "ctrl+k":
		if len(vis) > 0 {
			p.idx = (p.idx - 1 + len(vis)) % len(vis)
		}
		return nil
	case "down", "ctrl+j", "tab":
		if len(vis) > 0 {
			p.idx = (p.idx + 1) % len(vis)
		}
		return nil
	case "pgup":
		p.idx = max(p.idx-10, 0)
		return nil
	case "pgdown":
		p.idx = min(p.idx+10, max(len(vis)-1, 0))
		return nil
	case "enter":
		if p.idx < len(vis) {
			it := vis[p.idx]
			m.dialog = nil
			return p.onPick(m, it)
		}
		return nil
	case "ctrl+x":
		if p.onDelete != nil && p.idx < len(vis) {
			it := vis[p.idx]
			p.onDelete(m, it)
			for i := range p.items {
				if p.items[i].value == it.value {
					p.items = append(p.items[:i], p.items[i+1:]...)
					break
				}
			}
			p.idx = min(p.idx, max(len(p.visible())-1, 0))
		}
		return nil
	}
	var cmd tea.Cmd
	before := p.filter.Value()
	p.filter, cmd = p.filter.Update(k)
	if p.filter.Value() != before {
		p.idx = 0
	}
	return cmd
}

func (p *picker) view(m *model) string {
	t := m.th
	w := min(max(m.width-8, 30), 90)
	h := max(min(m.vp.Height-8, 16), 3)
	vis := p.visible()
	start := 0
	if p.idx >= h {
		start = p.idx - h + 1
	}
	var rows []string
	lastGroup := ""
	col := 0 // titles are padded to one column when they are short enough
	for _, it := range vis {
		col = max(col, lipgloss.Width(it.title)+2)
	}
	col = min(col, w/2)
	for i := start; i < len(vis) && i < start+h; i++ {
		it := vis[i]
		if it.group != "" && it.group != lastGroup {
			rows = append(rows, t.fg(t.secondary).Bold(true).Render(it.group))
			lastGroup = it.group
		}
		title := it.title
		if it.current {
			title += " ●"
		}
		if pad := col - lipgloss.Width(title); pad > 0 {
			title += strings.Repeat(" ", pad)
		}
		desc := it.desc
		room := w - 6 - lipgloss.Width(title)
		if lipgloss.Width(desc) > room {
			r := []rune(desc)
			desc = string(r[:max(min(len(r), room-1), 0)])
			if room > 1 {
				desc += "…"
			}
		}
		line := fmt.Sprintf("%s  %s", title, t.mutedS().Render(desc))
		if i == p.idx {
			line = lipgloss.NewStyle().Background(t.primary).Foreground(t.onAccent).Bold(true).Width(w - 4).Render(title + "  " + desc)
		} else {
			line = lipgloss.NewStyle().Width(w - 4).Render(line)
		}
		rows = append(rows, line)
	}
	if len(vis) == 0 {
		rows = append(rows, t.mutedS().Render(i18n.T("tui.no_matches")))
	}
	hint := i18n.T("tui.picker_hint")
	if p.onDelete != nil {
		hint += " · " + i18n.T("tui.picker_delete")
	}
	content := lipgloss.JoinVertical(lipgloss.Left,
		t.fg(t.primary).Bold(true).Render(p.title)+t.mutedS().Render(fmt.Sprintf("  %d", len(vis))),
		p.filter.View(), "", strings.Join(rows, "\n"), "", t.mutedS().Render(hint))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.primary).Padding(1, 2).Width(w).Render(content)
}

// promptDialog asks for one value (optionally hidden, for API keys).
type promptDialog struct {
	title, hint string
	in          textinput.Model
	onSubmit    func(m *model, value string) tea.Cmd
}

func newPrompt(th theme, title, hint string, secret bool, onSubmit func(*model, string) tea.Cmd) *promptDialog {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.PromptStyle = th.fg(th.accent)
	if secret {
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '•'
	}
	ti.Focus()
	return &promptDialog{title: title, hint: hint, in: ti, onSubmit: onSubmit}
}

func (p *promptDialog) update(m *model, k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "esc", "ctrl+c":
		m.prompt = nil
		return nil
	case "enter":
		m.prompt = nil
		return p.onSubmit(m, strings.TrimSpace(p.in.Value()))
	}
	var cmd tea.Cmd
	p.in, cmd = p.in.Update(k)
	return cmd
}

func (p *promptDialog) view(m *model) string {
	t := m.th
	w := min(max(m.width-8, 30), 80)
	p.in.Width = w - 8
	content := lipgloss.JoinVertical(lipgloss.Left,
		t.fg(t.primary).Bold(true).Render(p.title), "", lipgloss.NewStyle().Width(w-6).Foreground(t.muted).Render(p.hint), "",
		p.in.View(), "", t.mutedS().Render(i18n.T("tui.prompt_hint")))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.primary).Padding(1, 2).Width(w).Render(content)
}

// ---- permission ----

func (m *model) permissionView() string {
	t := m.th
	req := m.perm.req
	w := max(m.width-2, 20)
	subject := req.Subject
	if lines := strings.Split(subject, "\n"); len(lines) > 6 {
		subject = strings.Join(lines[:6], "\n") + "\n…"
	}
	title := t.fg(t.warning).Bold(true).Render("⚠ " + i18n.T("tui.perm_title", req.Tool.Name))
	if req.Tool.Source != "" && req.Tool.Source != "builtin" {
		title += t.mutedS().Render("  (" + req.Tool.Source + ")")
	}
	body := lipgloss.NewStyle().Foreground(t.text).Width(w - 4).Render(subject)
	keys := t.fg(t.success).Render("[y] "+i18n.T("tui.perm_once")) + "   " +
		t.fg(t.accent).Render("[a] "+i18n.T("tui.perm_always")) + "   " +
		t.fg(t.danger).Render("[n] "+i18n.T("tui.perm_deny"))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.warning).Padding(0, 1).Width(w).
		Render(lipgloss.JoinVertical(lipgloss.Left, title, body, keys))
}

// ---- suggestions (/commands and @files) ----

type suggestion struct {
	text  string // replaces the word being typed
	label string
	desc  string
}

func (m *model) updateSuggestions() {
	m.sugg, m.suggI = nil, 0
	v := m.input.Value()
	if strings.Contains(v, "\n") {
		return
	}
	if strings.HasPrefix(v, "/") {
		m.sugg = m.commandSuggestions(v)
		return
	}
	// @file: the last word
	i := strings.LastIndexAny(v, " \t")
	word := v[i+1:]
	if strings.HasPrefix(word, "@") {
		m.sugg = fileSuggestions(m.ws.Dir, word[1:])
	}
}

func (m *model) acceptSuggestion(enter bool) (submit bool) {
	s := m.sugg[m.suggI]
	v := m.input.Value()
	var nv string
	if strings.HasPrefix(v, "/") {
		nv = s.text
	} else {
		i := strings.LastIndexAny(v, " \t")
		nv = v[:i+1] + "@" + s.text
	}
	complete := nv == strings.TrimSpace(v)
	m.input.SetValue(nv)
	m.input.CursorEnd()
	m.sugg = nil
	if strings.HasSuffix(nv, "/") && !strings.HasPrefix(nv, "/") { // a folder: keep completing inside it
		m.updateSuggestions()
		return false
	}
	if strings.HasPrefix(nv, "/") && !strings.HasSuffix(nv, " ") {
		if enter && (complete || !m.commandTakesArgs(nv)) {
			return true
		}
		if m.commandTakesArgs(nv) {
			m.input.SetValue(nv + " ")
			m.input.CursorEnd()
			m.updateSuggestions()
		}
		return false
	}
	return false
}

func (m *model) suggestionsView() string {
	t := m.th
	const maxShown = 8
	start := 0
	if m.suggI >= maxShown {
		start = m.suggI - maxShown + 1
	}
	col := 0
	for _, s := range m.sugg {
		col = max(col, lipgloss.Width(s.label))
	}
	col = min(col, max(m.width/2, 20))
	var rows []string
	for i := start; i < len(m.sugg) && i < start+maxShown; i++ {
		s := m.sugg[i]
		label := s.label + strings.Repeat(" ", max(col-lipgloss.Width(s.label), 0))
		if i == m.suggI {
			rows = append(rows, lipgloss.NewStyle().Background(t.primary).Foreground(t.onAccent).Bold(true).Render(" "+label+" ")+" "+t.fg(t.text).Render(s.desc))
		} else {
			rows = append(rows, t.fg(t.accent).Render(" "+label+" ")+" "+t.mutedS().Render(s.desc))
		}
	}
	if len(m.sugg) > maxShown {
		rows = append(rows, t.mutedS().Render(fmt.Sprintf("  … %d", len(m.sugg))))
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.border).Width(max(m.width-2, 20)).Render(strings.Join(rows, "\n"))
}

func fileSuggestions(root, prefix string) []suggestion {
	dir, base := filepath.Split(filepath.FromSlash(prefix))
	abs := dir
	if !filepath.IsAbs(dir) {
		abs = filepath.Join(root, dir)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil
	}
	var out []suggestion
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(base)) {
			continue
		}
		p := filepath.ToSlash(filepath.Join(dir, name))
		desc := ""
		if e.IsDir() {
			p += "/"
			desc = i18n.T("tui.folder")
		}
		out = append(out, suggestion{text: p, label: p, desc: desc})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].text < out[j].text })
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}

// expandMentions attaches the text files named with @path.
func expandMentions(root, text string) (full string, attached, warnings []string) {
	var sb strings.Builder
	sb.WriteString(text)
	seen := map[string]bool{}
	for _, w := range strings.Fields(text) {
		if !strings.HasPrefix(w, "@") || len(w) < 2 {
			continue
		}
		p := strings.TrimRight(w[1:], ".,;:!?)")
		if seen[p] {
			continue
		}
		seen[p] = true
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, filepath.FromSlash(p))
		}
		fi, err := os.Stat(abs)
		if err != nil {
			if strings.ContainsAny(p, "/\\.") {
				warnings = append(warnings, i18n.T("tui.mention_missing", p))
			}
			continue
		}
		if fi.IsDir() {
			continue // the agent can list it with its tools
		}
		if fi.Size() > 100<<10 {
			warnings = append(warnings, i18n.T("tui.mention_large", p))
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil || strings.ContainsRune(string(data[:min(len(data), 8000)]), 0) {
			warnings = append(warnings, i18n.T("tui.mention_binary", p))
			continue
		}
		fmt.Fprintf(&sb, "\n\n<attached file=%q>\n%s\n</attached>", p, data)
		attached = append(attached, p)
	}
	return sb.String(), attached, warnings
}
