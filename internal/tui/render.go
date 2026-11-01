package tui

import (
	"context"
	"errors"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"my-agent/internal/i18n"
	"my-agent/internal/provider"
	"my-agent/internal/tools"
)

type itemKind int

const (
	kindUser itemKind = iota
	kindAssistant
	kindTool
	kindInfo
	kindError
	kindMeta
)

// item is one block of the transcript.
type item struct {
	kind        itemKind
	text        string
	attachments []string
	done        bool

	call    provider.ToolCall
	subject string
	result  tools.Result
	err     error
	denied  bool

	cache  string
	cacheW int
}

func (m *model) addItem(it *item) {
	m.items = append(m.items, it)
	m.dirty = true
}

func (m *model) lastItem() *item {
	if len(m.items) == 0 {
		return nil
	}
	return m.items[len(m.items)-1]
}

func (m *model) toolItem(id string) *item {
	for i := len(m.items) - 1; i >= 0; i-- {
		if it := m.items[i]; it.kind == kindTool && it.call.ID == id {
			return it
		}
	}
	return nil
}

// itemsFromMessages rebuilds the transcript of a saved session.
func itemsFromMessages(msgs []provider.Message) []*item {
	var out []*item
	results := map[string]provider.Message{}
	for _, msg := range msgs {
		if msg.Role == "tool" {
			results[msg.ToolCallID] = msg
		}
	}
	for _, msg := range msgs {
		switch msg.Role {
		case "user":
			text := msg.Content
			if i := strings.Index(text, "\n\n<attached "); i >= 0 {
				text = text[:i]
			}
			out = append(out, &item{kind: kindUser, text: text, done: true})
		case "assistant":
			if strings.TrimSpace(msg.Content) != "" {
				out = append(out, &item{kind: kindAssistant, text: msg.Content, done: true})
			}
			for _, c := range msg.ToolCalls {
				it := &item{kind: kindTool, call: c, done: true, subject: summarizeArgs(c)}
				if r, ok := results[c.ID]; ok {
					if r.IsError {
						it.err = errors.New(strings.TrimPrefix(r.Content, "error: "))
					} else {
						it.result = tools.Result{Output: r.Content}
					}
				}
				out = append(out, it)
			}
		}
	}
	return out
}

// summarizeArgs shows the most telling argument of a call.
func summarizeArgs(c provider.ToolCall) string {
	args := tools.Args(c.Args)
	for _, k := range []string{"command", "path", "pattern", "url", "query", "sql", "to", "uid"} {
		if v, ok := args[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func (m *model) transcript() string {
	if len(m.items) == 0 {
		return ""
	}
	w := max(m.width, 20)
	var blocks []string
	for i, it := range m.items {
		if it.cache == "" || it.cacheW != w || !it.done {
			it.cache, it.cacheW = m.renderItem(it, w), w
		}
		sep := "\n"
		if i > 0 && (it.kind == kindTool && m.items[i-1].kind == kindTool || it.kind == kindMeta) {
			sep = ""
		}
		if i > 0 {
			blocks = append(blocks, sep+it.cache)
		} else {
			blocks = append(blocks, it.cache)
		}
	}
	return strings.Join(blocks, "\n") + "\n"
}

func (m *model) renderItem(it *item, w int) string {
	t := m.th
	body := w - 4
	switch it.kind {
	case kindUser:
		color := t.agentColor(m.ra.Name, m.ra.Color)
		st := lipgloss.NewStyle().Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(color).
			PaddingLeft(1).MarginLeft(1).Width(body).Foreground(t.text)
		text := it.text
		for _, a := range it.attachments {
			text += "\n" + t.mutedS().Render("📎 "+a)
		}
		return st.Render(text)
	case kindAssistant:
		return m.md.render(it.text, !it.done)
	case kindTool:
		return m.renderTool(it, body)
	case kindInfo:
		return lipgloss.NewStyle().MarginLeft(2).Width(body).Foreground(t.muted).Render(it.text)
	case kindError:
		return lipgloss.NewStyle().MarginLeft(1).Width(body).Border(lipgloss.ThickBorder(), false, false, false, true).
			BorderForeground(t.danger).PaddingLeft(1).Foreground(t.danger).Render(it.text)
	case kindMeta:
		return lipgloss.NewStyle().MarginLeft(2).Foreground(t.subtle).Render(it.text)
	}
	return it.text
}

var toolIcons = map[string]string{
	"read": "📖", "write": "📝", "edit": "✏️", "bash": "⚡", "glob": "🔍", "grep": "🔎", "ls": "📂",
	"webfetch": "🌐", "rag_search": "📚", "db_query": "🗄", "db_schema": "🗄", "db_exec": "🗄",
	"email_search": "✉", "email_read": "✉", "email_send": "✉",
}

func (m *model) renderTool(it *item, body int) string {
	t := m.th
	icon := toolIcons[it.call.Name]
	if icon == "" {
		icon = "⚙"
	}
	status := m.spin.View()
	nameColor := t.accent
	switch {
	case !it.done:
	case it.denied:
		status, nameColor = t.fg(t.warning).Render("⊘"), t.warning
	case it.err != nil:
		status, nameColor = t.dangerS().Render("✗"), t.danger
	default:
		status = t.fg(t.success).Render("✓")
	}
	subject := strings.ReplaceAll(it.subject, "\n", " ⏎ ")
	head := "  " + status + " " + icon + " " + t.fg(nameColor).Bold(true).Render(it.call.Name)
	room := body - lipgloss.Width(head) - 2
	if room > 8 && subject != "" {
		if lipgloss.Width(subject) > room {
			r := []rune(subject)
			subject = string(r[:max(min(len(r), room-1), 0)]) + "…"
		}
		head += " " + t.fg(t.text).Render(subject)
	}
	if !it.done {
		return head
	}
	switch {
	case it.denied:
		return head + t.fg(t.warning).Render("  "+i18n.T("tui.denied"))
	case errors.Is(it.err, context.Canceled):
		return head + t.mutedS().Render("  "+i18n.T("tui.interrupted_short"))
	case it.err != nil:
		msg := it.err.Error()
		if len(msg) > 400 {
			msg = msg[:400] + "…"
		}
		return head + "\n" + lipgloss.NewStyle().MarginLeft(6).Width(body-6).Foreground(t.danger).Render(msg)
	}
	d := it.result.Display
	if (it.call.Name == "edit" || it.call.Name == "write") && d != "" {
		return head + "\n" + m.renderDiff(d, body-6)
	}
	if it.call.Name == "bash" {
		return head + "\n" + m.renderOutput(it.result.Output, body-6, 12)
	}
	if d != "" {
		return head + t.mutedS().Render("  · "+d)
	}
	return head
}

func (m *model) renderDiff(d string, w int) string {
	t := m.th
	lines := strings.Split(d, "\n")
	const maxLines = 24
	var out []string
	for i, l := range lines {
		if i == maxLines {
			out = append(out, t.mutedS().Render(i18n.T("tui.more_lines", len(lines)-maxLines)))
			break
		}
		if lipgloss.Width(l) > w {
			l = string([]rune(l)[:max(w-1, 1)]) + "…"
		}
		switch {
		case strings.HasPrefix(l, "+"):
			l = t.fg(t.success).Render(l)
		case strings.HasPrefix(l, "-"):
			l = t.fg(t.danger).Render(l)
		case strings.HasPrefix(l, "@@"):
			l = t.fg(t.secondary).Render(l)
		default:
			l = t.mutedS().Render(l)
		}
		out = append(out, l)
	}
	return lipgloss.NewStyle().MarginLeft(6).Render(strings.Join(out, "\n"))
}

func (m *model) renderOutput(s string, w, maxLines int) string {
	t := m.th
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > maxLines {
		lines = append([]string{i18n.T("tui.more_lines", len(lines)-maxLines)}, lines[len(lines)-maxLines:]...)
	}
	for i, l := range lines {
		if lipgloss.Width(l) > w {
			lines[i] = string([]rune(l)[:max(w-1, 1)]) + "…"
		}
	}
	return lipgloss.NewStyle().MarginLeft(6).Foreground(t.muted).Render(strings.Join(lines, "\n"))
}

// mdCache renders markdown with glamour for the current width.
type mdCache struct {
	th    theme
	width int
	r     *glamour.TermRenderer
}

func newMDCache(th theme) *mdCache { return &mdCache{th: th} }

func (c *mdCache) setWidth(w int) {
	if w == c.width && c.r != nil {
		return
	}
	c.width = max(w, 20)
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(c.th.glamourStyle()), glamour.WithWordWrap(c.width), glamour.WithEmoji())
	if err == nil {
		c.r = r
	}
}

func (c *mdCache) render(text string, streaming bool) string {
	if c.r == nil {
		c.setWidth(80)
	}
	src := text
	if streaming && strings.Count(src, "```")%2 == 1 {
		src += "\n```" // close an open code block so the partial text renders well
	}
	out, err := c.r.Render(src)
	if err != nil {
		return text
	}
	return strings.Trim(out, "\n")
}
