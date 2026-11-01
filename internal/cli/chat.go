package cli

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"my-agent/internal/agent"
	"my-agent/internal/app"
	"my-agent/internal/i18n"
	"my-agent/internal/provider"
	"my-agent/internal/rag"
)

func (e *env) testProvider(ctx context.Context, get func(string) (provider.Provider, error), name string) error {
	p, err := get(name)
	if err != nil {
		return err
	}
	// Streaming has only an idle timeout, so a slow first load of a big local
	// model is not cut off after a fixed number of seconds.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	_, err = provider.ChatStream(ctx, p, provider.ChatRequest{
		Messages:  []provider.Message{{Role: "user", Content: "Reply with the single word: ok"}},
		MaxTokens: 16,
	}, nil)
	return err
}

func (e *env) printSources(res []rag.Result) {
	for i, r := range res {
		fmt.Fprintf(e.out, "  [%d] %s (score %.2f)\n", i+1, r.Chunk.Source, r.Score)
	}
}

func (e *env) cmdAsk(ctx context.Context, args []string) error {
	fs := e.flags("ask")
	prov := fs.String("provider", "", i18n.T("cli.flag.provider"))
	noRAG := fs.Bool("no-rag", false, i18n.T("cli.flag.no_rag"))
	sources := fs.Bool("sources", false, i18n.T("cli.flag.sources"))
	if err := parse(fs, args); err != nil {
		return err
	}
	q := joinArgs(fs.Args())
	if q == "" {
		return usageSyntax("ask [flags] QUESTION...")
	}
	a, err := e.app()
	if err != nil {
		return err
	}
	ag, err := a.Agent(*prov, !*noRAG)
	if err != nil {
		return err
	}
	wrote := false
	ans, err := ag.AskStream(ctx, nil, q, func(d string) {
		wrote = true
		fmt.Fprint(e.out, d) // raw text as it arrives: script friendly
	})
	if wrote {
		fmt.Fprintln(e.out)
	}
	if err != nil {
		return err
	}
	if *sources && len(ans.Sources) > 0 {
		fmt.Fprintln(e.out, "\n"+i18n.T("chat.sources_title"))
		e.printSources(ans.Sources)
	}
	return nil
}

func (e *env) cmdChat(ctx context.Context, args []string) error {
	fs := e.flags("chat")
	prov := fs.String("provider", "", i18n.T("cli.flag.provider"))
	noRAG := fs.Bool("no-rag", false, i18n.T("cli.flag.no_rag"))
	if err := parse(fs, args); err != nil {
		return err
	}
	a, err := e.app()
	if err != nil {
		return err
	}
	return e.runChat(ctx, a, bufio.NewReader(e.in), *prov, !*noRAG)
}

// answerWriter prints a reply as it streams in: it stops the spinner and
// prints the agent header on the first chunk, then renders each chunk.
type answerWriter struct {
	e       *env
	st      style
	stop    func()
	md      *mdStream
	started bool
}

func (w *answerWriter) begin() {
	if w.started {
		return
	}
	w.started = true
	w.stop()
	if w.st.on {
		fmt.Fprintf(w.e.out, "\n %s\n   ", w.st.bold(w.st.green("✦ "+i18n.T("chat.agent"))))
	} else {
		fmt.Fprint(w.e.out, "agent> ")
	}
}

func (w *answerWriter) delta(d string) {
	w.begin()
	fmt.Fprint(w.e.out, w.md.Write(d))
}

func (w *answerWriter) end() {
	if !w.started {
		if w.st.on {
			return // nothing was printed (error before the first token)
		}
		w.begin()
	}
	fmt.Fprintln(w.e.out, w.md.Flush())
}

func (e *env) banner(st style, a *app.App, ag *agent.Agent, provName, model string) {
	if !st.on {
		fmt.Fprintln(e.out, i18n.T("chat.banner_plain", a.Cfg.Agent.Name, ag.Provider.Name()))
		return
	}
	rag := i18n.T("chat.rag_off")
	if ag.Index != nil {
		rag = i18n.T("chat.rag_on", ag.Index.Store.Len())
	}
	title := "✦ " + a.Cfg.Agent.Name
	info := provName + " · " + model
	hint := i18n.T("chat.banner_hint")
	fmt.Fprintln(e.out, st.box([][2]string{
		{title, st.bold(st.cyan(title))},
		{info + "   " + rag, st.green(info) + "   " + st.dim(rag)},
		{hint, st.dim(hint)},
	}))
}

// footer prints what follows a finished reply: timing and, on request, the
// sources used.
func (e *env) footer(st style, ans agent.Answer, took time.Duration, showSources bool) {
	if !st.on {
		if showSources {
			e.printSources(ans.Sources)
		}
		return
	}
	meta := took.Round(100 * time.Millisecond).String()
	fmt.Fprintf(e.out, "   %s\n", st.dim(meta))
	if showSources {
		for i, r := range ans.Sources {
			fmt.Fprintf(e.out, "   %s\n", st.dim(fmt.Sprintf("↳ [%d] %s (%.2f)", i+1, r.Chunk.Source, r.Score)))
		}
	}
}
