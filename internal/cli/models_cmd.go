package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"my-agent/internal/app"
	"my-agent/internal/i18n"
	"my-agent/internal/provider"
)

const modelsSyntax = "models list|ps|select|use|pull [flags] [NAME]"

func (e *env) cmdModels(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usageSyntax(modelsSyntax)
	}
	a, err := e.app()
	if err != nil {
		return err
	}
	return e.modelsCommand(ctx, a, args, bufio.NewReader(e.in), modelsOpts{chatAfterSelect: true})
}

// modelsOpts adapts modelsCommand to where it is called from.
type modelsOpts struct {
	defProvider     string // used when --provider is not given (default: agent.provider)
	chatAfterSelect bool   // `select` opens the chat afterwards (CLI only: already inside it otherwise)
}

// modelsCommand runs a `models` subcommand on a loaded app. rd supplies the
// answers to interactive prompts (`select`).
func (e *env) modelsCommand(ctx context.Context, a *app.App, args []string, rd *bufio.Reader, o modelsOpts) error {
	if len(args) == 0 {
		return usageSyntax(modelsSyntax)
	}
	sub, rest := args[0], args[1:]
	fs := e.flags("models " + sub)
	prov := fs.String("provider", o.defProvider, i18n.T("cli.flag.ollama_provider"))
	embedding := fs.Bool("embedding", false, i18n.T("cli.flag.embedding"))
	noChat := fs.Bool("no-chat", false, i18n.T("cli.flag.no_chat"))
	if err := parse(fs, rest); err != nil {
		return err
	}
	name, mm, err := a.OllamaManager(*prov)
	if err != nil {
		return err
	}
	switch sub {
	case "list":
		models, err := mm.ListModels(ctx)
		if err != nil {
			return err
		}
		pc := a.Cfg.Providers[name]
		for _, m := range models {
			var tags []string
			if provider.ModelInstalled([]provider.ModelInfo{m}, pc.Model) {
				tags = append(tags, "chat")
			}
			if pc.EmbeddingModel != "" && provider.ModelInstalled([]provider.ModelInfo{m}, pc.EmbeddingModel) {
				tags = append(tags, "embedding")
			}
			mark := ""
			if len(tags) > 0 {
				mark = "  <- " + strings.Join(tags, ", ")
			}
			fmt.Fprintf(e.out, "%-32s %9s%s\n", m.Name, formatBytes(m.Size), mark)
		}
		if len(models) == 0 {
			fmt.Fprintln(e.out, i18n.T("cli.models.none_installed"))
		}
		return nil
	case "ps":
		return e.modelsPS(ctx, a, name, mm)
	case "pull":
		if fs.NArg() != 1 {
			return usageSyntax("models pull [--provider P] NAME")
		}
		_, err := e.ensure(ctx, mm, fs.Arg(0))
		return err
	case "use":
		if fs.NArg() != 1 {
			return usageSyntax("models use [--provider P] [--embedding] NAME")
		}
		return e.use(ctx, a, name, mm, fs.Arg(0), *embedding)
	case "select":
		models, err := mm.ListModels(ctx)
		if err != nil {
			return err
		}
		pc := a.Cfg.Providers[name]
		current := pc.Model
		if *embedding {
			current = pc.EmbeddingModel
		}
		chosen, err := e.pick(rd, models, current)
		if err != nil {
			return err
		}
		if err := e.use(ctx, a, name, mm, chosen, *embedding); err != nil {
			return err
		}
		if *noChat || !o.chatAfterSelect {
			return nil
		}
		fmt.Fprintln(e.out)
		return e.runChat(ctx, a, rd, name, true)
	}
	return usageKey("cli.unknown_sub", "models", sub)
}

// use makes sure the model is installed (downloading it if needed) and
// stores it in the config.
func (e *env) use(ctx context.Context, a *app.App, provName string, mm provider.ModelManager, model string, embedding bool) error {
	if _, err := e.ensure(ctx, mm, model); err != nil {
		return err
	}
	if err := a.SetModel(provName, model, embedding); err != nil {
		return err
	}
	key := "model"
	if embedding {
		key = "embedding_model"
	}
	fmt.Fprintf(e.out, "providers.%s.%s = %s\n", provName, key, strings.TrimSpace(model))
	return nil
}

func (e *env) ensure(ctx context.Context, mm provider.ModelManager, model string) (bool, error) {
	show, finish := e.progressPrinter()
	pulled, err := provider.EnsureModel(ctx, mm, model, show)
	finish()
	if err != nil {
		return false, err
	}
	if pulled {
		fmt.Fprintln(e.out, i18n.T("cli.models.downloaded", strings.TrimSpace(model)))
	} else {
		fmt.Fprintln(e.out, i18n.T("cli.models.installed", strings.TrimSpace(model)))
	}
	return pulled, nil
}

// pick lists installed models plus an "other" option and returns the chosen
// (or typed) model name.
func (e *env) pick(rd *bufio.Reader, models []provider.ModelInfo, current string) (string, error) {
	if len(models) == 0 {
		fmt.Fprint(e.out, i18n.T("cli.models.pick_none"))
	} else {
		fmt.Fprintln(e.out, i18n.T("cli.models.pick_title"))
		for i, m := range models {
			mark := ""
			if current != "" && provider.ModelInstalled([]provider.ModelInfo{m}, current) {
				mark = "  " + i18n.T("cli.models.current")
			}
			fmt.Fprintf(e.out, "  %2d) %-30s %9s%s\n", i+1, m.Name, formatBytes(m.Size), mark)
		}
		fmt.Fprintf(e.out, "   0) %s\n", i18n.T("cli.models.other_option"))
		fmt.Fprint(e.out, i18n.T("cli.models.pick_prompt"))
	}
	line, err := readLine(rd)
	if err != nil {
		return "", err
	}
	if n, err := strconv.Atoi(line); err == nil {
		switch {
		case n >= 1 && n <= len(models):
			return models[n-1].Name, nil
		case n == 0 && len(models) > 0:
			fmt.Fprint(e.out, i18n.T("cli.models.name_prompt"))
			return readLine(rd)
		default:
			return "", i18n.Errorf("cli.models.bad_choice", n)
		}
	}
	return line, nil
}

func readLine(rd *bufio.Reader) (string, error) {
	s, err := rd.ReadString('\n')
	s = strings.TrimSpace(s)
	if s == "" {
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return "", i18n.Err("cli.models.no_selection")
	}
	return s, nil
}

// progressPrinter returns a throttled progress callback that redraws one
// line, and a finish func that ends the line.
func (e *env) progressPrinter() (func(provider.PullProgress), func()) {
	wrote := false
	show := provider.Throttle(func(p provider.PullProgress) {
		wrote = true
		line := p.Status
		if len(line) > 30 {
			line = line[:30]
		}
		if pct := p.Percent(); pct >= 0 {
			line = fmt.Sprintf("%-30s %3d%% (%s / %s)", line, pct, formatBytes(p.Completed), formatBytes(p.Total))
		}
		fmt.Fprintf(e.out, "\r%-72s", line)
	})
	return show, func() {
		if wrote {
			fmt.Fprintln(e.out)
		}
	}
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
