// Package tui is the full-screen interface of my-agent: a conversation with
// an agent that can use tools, with sessions, agents, models, connectors and
// the knowledge base one keystroke away.
package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"my-agent/internal/agent"
	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/i18n"
	"my-agent/internal/provider"
	"my-agent/internal/session"
)

// Options are the choices given on the command line.
type Options struct {
	Agent, Provider, Model string
	Session                string // session to resume; "last" = the most recent one here
	Prompt                 string // first message to send
}

// Run starts the interface and blocks until the user quits.
func Run(ctx context.Context, ws *app.Workspace, opts Options) error {
	dark := true
	switch ws.App.Cfg.Theme {
	case "light":
		dark = false
	case "auto":
		dark = lipgloss.HasDarkBackground()
	}
	m := newModel(ctx, ws, newTheme(dark), opts)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithContext(ctx))
	m.post = p.Send
	_, err := p.Run()
	if errors.Is(err, tea.ErrProgramKilled) {
		err = nil
	}
	m.saveSession()
	return err
}

// messages sent to the program from other goroutines
type (
	eventMsg struct{ e agent.Event }
	permMsg  struct {
		req   agent.PermissionRequest
		reply chan agent.Decision
	}
	doneMsg struct {
		added  []provider.Message
		usage  provider.Usage
		err    error
		took   time.Duration
		grants map[string]bool
	}
	noticeMsg struct {
		text  string
		isErr bool
	}
	modelsMsg struct{ choices []app.ModelChoice }
	reloadMsg struct{ err error }
	tickMsg   struct{}
	bgDoneMsg struct {
		text string
		err  error
	}
)

type model struct {
	ctx  context.Context
	ws   *app.Workspace
	post func(tea.Msg) // delivers messages from other goroutines (Program.Send)
	th   theme

	width, height int
	vp            viewport.Model
	input         textarea.Model
	spin          spinner.Model

	sel    app.Selection
	ra     config.ResolvedAgent
	grants map[string]bool
	sess   *session.Session
	items  []*item

	busy     bool
	busyText string
	cancel   context.CancelFunc
	started  time.Time
	usage    provider.Usage // of the current session

	perm   *permMsg
	dialog *picker
	prompt *promptDialog
	sugg   []suggestion
	suggI  int

	notice    string
	noticeErr bool
	noticeAt  time.Time

	history  []string // sent inputs, for ↑/↓
	histI    int
	md       *mdCache
	dirty    bool
	quitting bool
}

func newModel(ctx context.Context, ws *app.Workspace, th theme, opts Options) *model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.MaxHeight = 10
	ta.SetHeight(1)
	ta.Placeholder = i18n.T("tui.placeholder")
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = th.mutedS()
	ta.BlurredStyle = ta.FocusedStyle
	ta.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter", "ctrl+j"))
	ta.Focus()

	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(th.fg(th.accent)))
	m := &model{
		ctx: ctx, ws: ws, th: th, input: ta, spin: sp,
		sel:  app.Selection{Agent: opts.Agent, Provider: opts.Provider, Model: opts.Model},
		sess: session.New(ws.Dir),
		md:   newMDCache(th),
		vp:   viewport.New(80, 20),
	}
	m.vp.MouseWheelEnabled = true
	m.vp.MouseWheelDelta = 3
	if err := m.resolveAgent(); err != nil {
		m.addItem(&item{kind: kindError, text: err.Error()})
		m.sel = app.Selection{}
		_ = m.resolveAgent()
	}
	if opts.Session != "" {
		m.resume(opts.Session)
	}
	if strings.TrimSpace(opts.Prompt) != "" {
		m.input.SetValue(opts.Prompt)
	}
	return m
}

// resolveAgent refreshes the agent settings from the configuration.
func (m *model) resolveAgent() error {
	ra, err := m.ws.App.Cfg.ResolveAgent(m.sel.Agent)
	if err != nil {
		return err
	}
	m.sel.Agent = ra.Name
	m.ra = ra
	return nil
}

// provModel is the provider and model the next message will use.
func (m *model) provModel() (string, string) {
	prov, mod := m.ra.Provider, m.ra.Model
	if m.sel.Provider != "" {
		prov, mod = m.sel.Provider, m.sel.Model
	} else if m.sel.Model != "" {
		mod = m.sel.Model
	}
	if mod == "" {
		mod = m.ws.App.Cfg.Providers[prov].Model
	}
	return prov, mod
}

func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{textarea.Blink, m.spin.Tick, tick()}
	if strings.TrimSpace(m.input.Value()) != "" {
		cmds = append(cmds, func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} })
	}
	return tea.Batch(cmds...)
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

// ---- layout ----

func (m *model) layout() {
	w := max(m.width, 20)
	m.input.SetWidth(w - 4)
	lines := 0
	for _, l := range strings.Split(m.input.Value(), "\n") {
		lines += lipgloss.Width(l)/max(w-5, 1) + 1 // long lines wrap
	}
	m.input.SetHeight(min(max(lines, 1), 8))
	bottom := lipgloss.Height(m.bottomView())
	m.vp.Width = w
	m.vp.Height = max(m.height-bottom, 3)
	m.md.setWidth(w - 4)
	m.dirty = true
}

func (m *model) refresh() {
	atBottom := m.vp.AtBottom() || m.busy
	m.vp.SetContent(m.transcript())
	if atBottom {
		m.vp.GotoBottom()
	}
	m.dirty = false
}

// ---- update ----

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
	case tea.KeyMsg:
		if cmd, handled := m.handleKey(msg); handled {
			m.layout()
			m.refresh()
			return m, cmd
		}
	case tea.MouseMsg:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		if m.busy {
			m.dirty = true
		}
		cmds = append(cmds, cmd)
	case tickMsg:
		if m.notice != "" && time.Since(m.noticeAt) > 6*time.Second {
			m.notice = ""
		}
		cmds = append(cmds, tick())
	case eventMsg:
		m.onEvent(msg.e)
	case permMsg:
		m.perm = &msg
		m.layout()
	case doneMsg:
		m.onDone(msg)
	case noticeMsg:
		m.setNotice(msg.text, msg.isErr)
	case bgDoneMsg:
		m.busy, m.busyText = false, ""
		if msg.err != nil {
			m.addItem(&item{kind: kindError, text: msg.err.Error()})
		} else if msg.text != "" {
			m.addItem(&item{kind: kindInfo, text: msg.text})
		}
	case modelsMsg:
		m.openModelPicker(msg.choices)
	case editedMsg:
		if msg.err != nil {
			m.fail(msg.err)
		} else {
			cmds = append(cmds, m.reload())
		}
	case reloadMsg:
		if msg.err != nil {
			m.addItem(&item{kind: kindError, text: msg.err.Error()})
		} else {
			m.setNotice(i18n.T("tui.config_reloaded"), false)
		}
	}
	if m.dialog == nil && m.prompt == nil && m.perm == nil {
		var cmd tea.Cmd
		before := m.input.Value()
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
		if m.input.Value() != before {
			m.updateSuggestions()
			m.layout()
		}
	}
	if m.dirty {
		m.refresh()
	}
	return m, tea.Batch(cmds...)
}

func (m *model) setNotice(text string, isErr bool) {
	m.notice, m.noticeErr, m.noticeAt = text, isErr, time.Now()
}

func (m *model) handleKey(k tea.KeyMsg) (tea.Cmd, bool) {
	if m.perm != nil {
		return m.answerPermission(k), true
	}
	if m.prompt != nil {
		return m.prompt.update(m, k), true
	}
	if m.dialog != nil {
		return m.dialog.update(m, k), true
	}
	switch k.String() {
	case "ctrl+c":
		switch {
		case m.busy:
			m.interrupt()
		case m.input.Value() != "":
			m.input.Reset()
			m.sugg = nil
		default:
			return m.quit(), true
		}
		return nil, true
	case "ctrl+d":
		if m.input.Value() == "" {
			return m.quit(), true
		}
	case "esc":
		switch {
		case len(m.sugg) > 0:
			m.sugg = nil
		case m.busy:
			m.interrupt()
		}
		return nil, true
	case "ctrl+p":
		m.openCommandPalette()
		return nil, true
	case "ctrl+o":
		return m.loadModels(), true
	case "ctrl+n":
		m.newSession()
		return nil, true
	case "ctrl+s":
		m.openSessionPicker()
		return nil, true
	case "pgup":
		m.vp.HalfPageUp()
		return nil, true
	case "pgdown":
		m.vp.HalfPageDown()
		return nil, true
	case "ctrl+home":
		m.vp.GotoTop()
		return nil, true
	case "ctrl+end":
		m.vp.GotoBottom()
		return nil, true
	case "tab", "shift+tab":
		if len(m.sugg) > 0 && k.String() == "tab" {
			m.acceptSuggestion(false)
			return nil, true
		}
		if !m.busy {
			m.cycleAgent(k.String() == "tab")
		}
		return nil, true
	case "up", "down":
		if len(m.sugg) > 0 {
			if k.String() == "up" {
				m.suggI = (m.suggI - 1 + len(m.sugg)) % len(m.sugg)
			} else {
				m.suggI = (m.suggI + 1) % len(m.sugg)
			}
			return nil, true
		}
		if m.input.LineCount() <= 1 && len(m.history) > 0 {
			m.browseHistory(k.String() == "up")
			return nil, true
		}
	case "enter":
		if len(m.sugg) > 0 {
			if m.acceptSuggestion(true) {
				return m.submit(), true
			}
			return nil, true
		}
		return m.submit(), true
	}
	return nil, false
}

func (m *model) browseHistory(up bool) {
	if up {
		m.histI = max(m.histI-1, 0)
	} else {
		m.histI = min(m.histI+1, len(m.history))
	}
	if m.histI == len(m.history) {
		m.input.Reset()
	} else {
		m.input.SetValue(m.history[m.histI])
	}
}

func (m *model) quit() tea.Cmd {
	m.quitting = true
	if m.cancel != nil {
		m.cancel()
	}
	return tea.Quit
}

func (m *model) interrupt() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *model) cycleAgent(forward bool) {
	names := m.ws.App.Cfg.AgentNames()
	if len(names) == 0 {
		return
	}
	i := 0
	for j, n := range names {
		if n == m.ra.Name {
			i = j
		}
	}
	if forward {
		i = (i + 1) % len(names)
	} else {
		i = (i - 1 + len(names)) % len(names)
	}
	m.switchAgent(names[i])
}

func (m *model) switchAgent(name string) {
	prev := m.sel
	m.sel.Agent = name
	if err := m.resolveAgent(); err != nil {
		m.sel = prev
		m.setNotice(err.Error(), true)
		return
	}
	m.setNotice(i18n.T("tui.agent_switched", m.ra.Name, m.ra.Description), false)
}

// ---- running the agent ----

func (m *model) submit() tea.Cmd {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return nil
	}
	m.input.Reset()
	m.sugg = nil
	if len(m.history) == 0 || m.history[len(m.history)-1] != text {
		m.history = append(m.history, text)
	}
	m.histI = len(m.history)
	if strings.HasPrefix(text, "/") {
		return m.runCommand(text)
	}
	if m.busy {
		m.setNotice(i18n.T("tui.busy"), true)
		m.input.SetValue(text)
		return nil
	}
	return m.send(text)
}

func (m *model) send(text string) tea.Cmd {
	prov, mod := m.provModel()
	r, ra, err := m.ws.Runner(app.Selection{Agent: m.ra.Name, Provider: prov, Model: mod})
	if err != nil {
		m.addItem(&item{kind: kindError, text: err.Error()})
		return nil
	}
	m.ra = ra
	full, atts, warns := expandMentions(m.ws.Dir, text)
	m.addItem(&item{kind: kindUser, text: text, attachments: atts})
	for _, w := range warns {
		m.addItem(&item{kind: kindInfo, text: "⚠ " + w})
	}
	if m.sess.Title == "" {
		m.sess.Title = session.TitleFrom(text)
	}
	m.sess.Agent, m.sess.Provider, m.sess.Model = ra.Name, prov, mod

	r.SetGrants(m.grants)
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	m.busy, m.busyText, m.started = true, i18n.T("tui.thinking"), time.Now()
	post := m.post
	r.OnEvent = func(e agent.Event) { post(eventMsg{e}) }
	r.Approve = func(ctx context.Context, req agent.PermissionRequest) agent.Decision {
		ch := make(chan agent.Decision, 1)
		post(permMsg{req: req, reply: ch})
		select {
		case d := <-ch:
			return d
		case <-ctx.Done():
			return agent.Deny
		}
	}
	hist := append([]provider.Message{}, m.sess.Messages...)
	start := time.Now()
	return func() tea.Msg {
		added, usage, err := r.Run(ctx, hist, full, text)
		return doneMsg{added: added, usage: usage, err: err, took: time.Since(start), grants: r.Grants()}
	}
}

func (m *model) onEvent(e agent.Event) {
	switch e.Kind {
	case agent.EvText:
		last := m.lastItem()
		if last == nil || last.kind != kindAssistant || last.done {
			last = &item{kind: kindAssistant}
			m.addItem(last)
		}
		last.text += e.Text
		last.cache = ""
		m.busyText = i18n.T("tui.writing")
	case agent.EvToolStart:
		m.finishStreaming()
		m.addItem(&item{kind: kindTool, call: e.Call, subject: e.Subject})
		m.busyText = i18n.T("tui.running_tool", e.Call.Name)
	case agent.EvToolEnd:
		m.finishStreaming()
		it := m.toolItem(e.Call.ID)
		if it == nil {
			it = &item{kind: kindTool, call: e.Call, subject: e.Subject}
			m.addItem(it)
		}
		it.done, it.result, it.err, it.denied = true, e.Result, e.Err, e.Denied
		it.cache = ""
		m.busyText = i18n.T("tui.thinking")
	case agent.EvUsage:
		m.usage.Input += e.Usage.Input
		m.usage.Output += e.Usage.Output
		m.sess.Usage = m.usage
	case agent.EvStep:
		m.finishStreaming()
	}
	m.dirty = true
}

func (m *model) finishStreaming() {
	if last := m.lastItem(); last != nil && last.kind == kindAssistant {
		last.done = true
		last.cache = ""
	}
}

func (m *model) onDone(d doneMsg) {
	m.busy, m.cancel = false, nil
	m.perm = nil
	if d.grants != nil {
		m.grants = d.grants
	}
	m.finishStreaming()
	m.sess.Messages = append(m.sess.Messages, d.added...)
	for _, it := range m.items { // tools left running were interrupted
		if it.kind == kindTool && !it.done {
			it.done, it.err, it.cache = true, context.Canceled, ""
		}
	}
	switch {
	case d.err == nil:
	case agent.IsInterrupted(d.err):
		m.addItem(&item{kind: kindInfo, text: i18n.T("tui.interrupted")})
	case errors.Is(d.err, agent.ErrDenied):
	default:
		m.addItem(&item{kind: kindError, text: d.err.Error()})
	}
	prov, mod := m.provModel()
	meta := fmt.Sprintf("%s · %s/%s · %s", m.ra.Name, prov, mod, d.took.Round(100*time.Millisecond))
	if d.usage.Output > 0 {
		meta += " · " + tokens(d.usage.Input) + "↑ " + tokens(d.usage.Output) + "↓"
	}
	m.addItem(&item{kind: kindMeta, text: meta})
	m.saveSession()
	m.layout()
}

func (m *model) answerPermission(k tea.KeyMsg) tea.Cmd {
	var d agent.Decision
	switch strings.ToLower(k.String()) {
	case "y", "enter", "s":
		d = agent.Once
	case "a", "t":
		d = agent.Always
	case "n", "esc", "d":
		d = agent.Deny
	case "ctrl+c":
		d = agent.Deny
		m.interrupt()
	default:
		return nil
	}
	m.perm.reply <- d
	m.perm = nil
	return nil
}

func (m *model) saveSession() {
	if m.sess == nil || len(m.sess.Messages) == 0 {
		return
	}
	if err := m.ws.Sessions.Save(m.sess); err != nil {
		m.setNotice(err.Error(), true)
	}
}

func (m *model) newSession() {
	if m.busy {
		m.setNotice(i18n.T("tui.busy"), true)
		return
	}
	m.saveSession()
	m.sess = session.New(m.ws.Dir)
	m.items = nil
	m.usage = provider.Usage{}
	m.setNotice(i18n.T("tui.new_session"), false)
	m.dirty = true
}

// resume loads a saved session ("last" = the latest of this directory).
func (m *model) resume(id string) {
	if id == "last" {
		list, _ := m.ws.Sessions.List(m.ws.Dir)
		if len(list) == 0 || list[0].Dir != m.ws.Dir {
			return
		}
		id = list[0].ID
	}
	s, err := m.ws.Sessions.Load(id)
	if err != nil {
		m.addItem(&item{kind: kindError, text: err.Error()})
		return
	}
	m.saveSession()
	m.sess, m.usage = s, s.Usage
	m.items = itemsFromMessages(s.Messages)
	if s.Agent != "" {
		if _, err := m.ws.App.Cfg.ResolveAgent(s.Agent); err == nil {
			m.sel.Agent = s.Agent
			_ = m.resolveAgent()
		}
	}
	if _, ok := m.ws.App.Cfg.Providers[s.Provider]; ok && s.Model != "" {
		m.sel.Provider, m.sel.Model = s.Provider, s.Model
	}
	m.setNotice(i18n.T("tui.resumed", s.Title), false)
	m.dirty = true
}

// ---- view ----

func (m *model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		return ""
	}
	top := m.vp.View()
	if m.dialog != nil {
		top = lipgloss.Place(m.width, m.vp.Height, lipgloss.Center, lipgloss.Center, m.dialog.view(m))
	} else if m.prompt != nil {
		top = lipgloss.Place(m.width, m.vp.Height, lipgloss.Center, lipgloss.Center, m.prompt.view(m))
	} else if len(m.items) == 0 {
		top = lipgloss.Place(m.width, m.vp.Height, lipgloss.Center, lipgloss.Center, m.welcome())
	}
	return lipgloss.JoinVertical(lipgloss.Left, top, m.bottomView())
}

func (m *model) bottomView() string {
	var parts []string
	if m.perm != nil {
		parts = append(parts, m.permissionView())
	} else if len(m.sugg) > 0 {
		parts = append(parts, m.suggestionsView())
	}
	if m.busy && m.perm == nil {
		parts = append(parts, " "+m.spin.View()+" "+m.th.mutedS().Render(m.busyText+"  "+i18n.T("tui.esc_to_stop")))
	}
	color := m.th.agentColor(m.ra.Name, m.ra.Color)
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(color).Padding(0, 1).Width(max(m.width-2, 10))
	parts = append(parts, box.Render(m.input.View()), m.statusBar())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *model) statusBar() string {
	color := m.th.agentColor(m.ra.Name, m.ra.Color)
	prov, mod := m.provModel()
	left := m.th.badge(strings.ToUpper(m.ra.Name), color) + " " + m.th.fg(m.th.text).Render(prov+"/"+mod)
	if st := m.ws.MCP.Statuses(); len(st) > 0 {
		ok := 0
		for _, s := range st {
			if s.Err == nil && !s.Pending && !s.Disabled {
				ok++
			}
		}
		left += m.th.mutedS().Render(fmt.Sprintf("  mcp %d/%d", ok, len(st)))
	}
	var right string
	if m.notice != "" {
		st := m.th.fg(m.th.accent)
		if m.noticeErr {
			st = m.th.dangerS()
		}
		right = st.Render(m.notice)
	} else {
		if m.usage.Input+m.usage.Output > 0 {
			right = m.th.mutedS().Render(tokens(m.usage.Input)+"↑ "+tokens(m.usage.Output)+"↓  ") + ""
		}
		right += m.th.mutedS().Render(i18n.T("tui.hints"))
	}
	dir := shortDir(m.ws.Dir)
	space := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	mid := ""
	if space > lipgloss.Width(dir)+4 {
		mid = m.th.mutedS().Render("  " + dir)
		space -= lipgloss.Width(mid)
	}
	if space < 1 {
		right = ""
		space = max(m.width-lipgloss.Width(left)-lipgloss.Width(mid)-1, 1)
	}
	return " " + left + mid + strings.Repeat(" ", space) + right
}

func shortDir(d string) string {
	if h, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(h, d); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(filepath.Join("~", rel))
		}
	}
	return filepath.ToSlash(d)
}

func tokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprint(n)
}

const logo = `                                              __
  ____ ___  __  __      ____ _____ ____  ____  / /_
 / __ '__ \/ / / /_____/ __ '/ __ '/ _ \/ __ \/ __/
/ / / / / / /_/ /_____/ /_/ / /_/ /  __/ / / / /_
/_/ /_/ /_/\__, /      \__,_/\__, /\___/_/ /_/\__/
          /____/            /____/`

func (m *model) welcome() string {
	t := m.th
	var sb strings.Builder
	lines := strings.Split(logo, "\n")
	grad := []lipgloss.Color{t.accent, t.primary, t.primary, t.secondary, t.secondary, t.secondary}
	for i, l := range lines {
		sb.WriteString(t.fg(grad[i%len(grad)]).Bold(true).Render(l) + "\n")
	}
	sb.WriteString("\n")
	prov, mod := m.provModel()
	row := func(k, v string) {
		sb.WriteString(t.mutedS().Render(fmt.Sprintf("%-12s", k)) + t.fg(t.text).Render(v) + "\n")
	}
	row(i18n.T("tui.w.agent"), m.ra.Name+" — "+m.ra.Description)
	row(i18n.T("tui.w.model"), prov+"/"+mod)
	row(i18n.T("tui.w.dir"), shortDir(m.ws.Dir))
	row(i18n.T("tui.w.config"), m.ws.App.Path)
	sb.WriteString("\n")
	keys := [][2]string{
		{"enter", i18n.T("tui.k.send")}, {"alt+enter", i18n.T("tui.k.newline")}, {"tab", i18n.T("tui.k.agents")},
		{"ctrl+o", i18n.T("tui.k.models")}, {"ctrl+p", i18n.T("tui.k.commands")}, {"ctrl+s", i18n.T("tui.k.sessions")},
		{"/", i18n.T("tui.k.slash")}, {"@", i18n.T("tui.k.mention")}, {"esc", i18n.T("tui.k.stop")}, {"ctrl+c", i18n.T("tui.k.quit")},
	}
	for _, k := range keys {
		sb.WriteString(t.fg(t.accent).Render(fmt.Sprintf("%-12s", k[0])) + t.mutedS().Render(k[1]) + "\n")
	}
	s := strings.TrimRight(sb.String(), "\n")
	// a fixed width keeps the block left-aligned when it is centered
	return lipgloss.NewStyle().Width(lipgloss.Width(s)).Render(s)
}
