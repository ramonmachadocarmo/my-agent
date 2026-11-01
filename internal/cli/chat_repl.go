package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"my-agent/internal/agent"
	"my-agent/internal/app"
	"my-agent/internal/i18n"
)

// chat is one interactive session: the conversation plus everything needed to
// run slash commands against the live configuration.
type chat struct {
	e    *env
	base context.Context // never cancelled by Ctrl+C: only the running work is
	ctx  context.Context // context of the command or reply being run
	a    *app.App
	rd   *bufio.Reader
	st   style
	tree []*node

	provName    string
	useRAG      bool
	showSources bool
	ag          *agent.Agent
	sess        *agent.Session

	warmKey        string // provider/model last warmed up
	checkedOffload bool

	term *termInput // line editor with autocomplete; nil = plain line reads

	mu         sync.Mutex
	cancelWork context.CancelFunc

	models modelCache
}

// runChat runs the interactive conversation. Replies stream in as they are
// generated. On a real terminal it is styled (colors, banner, spinner,
// markdown) and has a line editor with autocomplete for / commands and @files;
// when input or output is piped it is plain text. Ctrl+C stops the reply or
// command being run; at the prompt it exits.
func (e *env) runChat(ctx context.Context, a *app.App, rd *bufio.Reader, provName string, useRAG bool) error {
	c, err := e.newChat(ctx, a, rd, provName, useRAG)
	if err != nil {
		return err
	}
	if c.st.on {
		c.term = newTermInput(e.in, e.out, rd, c.st)
	}
	e.banner(c.st, a, c.ag, c.provName, c.model())
	c.warm()
	return c.loop()
}

func (e *env) newChat(ctx context.Context, a *app.App, rd *bufio.Reader, provName string, useRAG bool) (*chat, error) {
	ag, err := a.Agent(provName, useRAG)
	if err != nil {
		return nil, err
	}
	if provName == "" {
		provName = a.Cfg.Agent.Provider
	}
	c := &chat{
		e: e, base: context.WithoutCancel(ctx), a: a, rd: rd, st: newStyle(e.out),
		provName: provName, useRAG: useRAG, ag: ag,
	}
	c.ctx = c.base
	c.sess = &agent.Session{Agent: ag}
	c.tree = c.buildTree()
	return c, nil
}

func (c *chat) model() string { return c.a.Cfg.Providers[c.provName].Model }

// warm preloads the model in the background when the provider or model
// changed since the last time.
func (c *chat) warm() {
	key := c.provName + "/" + c.model()
	if key == c.warmKey {
		return
	}
	c.warmKey, c.checkedOffload = key, false
	warmUp(c.base, c.ag.Provider)
}

// rebuild recreates the agent from the current configuration, keeping the
// conversation. It runs after every command so config, model and knowledge
// base changes apply to the very next message.
func (c *chat) rebuild() error {
	ag, err := c.a.Agent(c.provName, c.useRAG)
	if err != nil {
		return err
	}
	c.ag, c.sess.Agent = ag, ag
	c.warm()
	return nil
}

// runWork runs fn with a context that Ctrl+C cancels (without ending the
// chat). interrupted reports whether fn failed because of that.
func (c *chat) runWork(fn func(ctx context.Context) error) (interrupted bool, err error) {
	ctx, cancel := context.WithCancel(c.base)
	c.mu.Lock()
	c.cancelWork = cancel
	c.mu.Unlock()
	prev := c.ctx
	c.ctx = ctx
	err = fn(ctx)
	c.mu.Lock()
	c.cancelWork = nil
	c.mu.Unlock()
	interrupted = err != nil && ctx.Err() != nil
	cancel()
	c.ctx = prev
	return interrupted, err
}

func (c *chat) loop() error {
	// Ctrl+C cancels the running work; with nothing running it exits. (In the
	// line editor Ctrl+C arrives as a key, not a signal.)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	go func() {
		for range sigc {
			c.mu.Lock()
			cancel := c.cancelWork
			c.mu.Unlock()
			if cancel == nil {
				fmt.Fprintln(c.e.out)
				os.Exit(130)
			}
			cancel()
		}
	}()
	defer func() { signal.Stop(sigc); close(sigc) }()

	for {
		line, err := c.readLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isCommandLine(line) {
			if err := c.command(line); errors.Is(err, errQuit) {
				return nil
			}
			continue
		}
		c.send(line)
	}
}

func isCommandLine(line string) bool {
	return strings.HasPrefix(line, "/") || line == "help" || line == "?"
}

// promptLabel is the styled prompt of the interactive editor, e.g. " you ❯ ".
func promptLabel() string { return " " + i18n.T("chat.you") + " ❯ " }

func (c *chat) readLine() (string, error) {
	if c.term != nil {
		fmt.Fprintln(c.e.out)
		return c.term.readLine(c.st.bold(c.st.cyan(promptLabel())), len([]rune(promptLabel())), c.suggest, false)
	}
	prompt := "you> "
	if c.st.on {
		prompt = "\n" + c.st.bold(c.st.cyan(promptLabel()))
	}
	fmt.Fprint(c.e.out, prompt)
	line, err := c.rd.ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		fmt.Fprintln(c.e.out)
		return "", err
	}
	return line, nil
}

// readSecret reads a value without echoing it when the terminal allows it.
func (c *chat) readSecret(prompt string) (string, error) {
	if c.term != nil {
		return c.term.readLine(prompt, len([]rune(prompt)), nil, true)
	}
	fmt.Fprint(c.e.out, prompt+i18n.T("chat.secret_visible"))
	line, err := c.rd.ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// command runs a slash command. Errors are reported here; only errQuit is
// returned.
func (c *chat) command(line string) error {
	if line == "help" || line == "?" || line == "/" {
		c.printHelp(nil)
		return nil
	}
	r, err := resolve(c.tree, splitArgs(strings.TrimPrefix(line, "/")))
	if err != nil {
		fmt.Fprintln(c.e.err, c.st.red(err.Error()))
		return nil
	}
	if r.node.run == nil {
		c.printHelp(r.path)
		return nil
	}
	interrupted, err := c.runWork(func(context.Context) error { return r.node.run(c, r.args) })
	switch {
	case errors.Is(err, errQuit):
		return errQuit
	case interrupted:
		fmt.Fprintln(c.e.out, c.st.dim(i18n.T("chat.interrupted")))
	case err != nil:
		var u usageError
		if errors.As(err, &u) {
			fmt.Fprintf(c.e.err, "%s\n", c.st.red(u.Error()))
		} else {
			fmt.Fprintf(c.e.err, "%s\n", c.st.red(i18n.T("cli.error", err)))
		}
	}
	if err := c.rebuild(); err != nil {
		fmt.Fprintf(c.e.err, "%s\n", c.st.red(i18n.T("chat.applying_error", err)))
	}
	return nil
}

// send sends a chat message, expanding @file mentions.
func (c *chat) send(line string) {
	msg, atts, warns := expandMentions(line)
	for _, w := range warns {
		fmt.Fprintf(c.e.out, "%s\n", c.st.yellow(i18n.T("cli.warning", w)))
	}
	for _, a := range atts {
		fmt.Fprintf(c.e.out, "%s\n", c.st.dim("📎 "+a.describe()))
	}

	w := &answerWriter{e: c.e, st: c.st, stop: c.e.spin(c.st, i18n.T("chat.thinking")), md: newMDStream(c.st, "   ")}
	start := time.Now()
	var ans agent.Answer
	interrupted, err := c.runWork(func(ctx context.Context) (err error) {
		ans, err = c.sess.SendStreamQuery(ctx, msg, line, w.delta)
		return err
	})
	w.stop()
	w.end()

	switch {
	case interrupted:
		fmt.Fprintln(c.e.out, c.st.dim("   "+i18n.T("chat.interrupted")))
	case err != nil:
		fmt.Fprintf(c.e.err, "%s\n", c.st.red(i18n.T("cli.error", err)))
	default:
		c.e.footer(c.st, ans, time.Since(start), c.showSources)
		if !c.checkedOffload {
			c.checkedOffload = true
			c.e.checkOffload(c.base, c.st, c.ag.Provider, c.provName, c.model())
		}
	}
}
