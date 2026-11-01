// Package cli implements the my-agent command line. It works the same in
// bash and PowerShell; only the optional `completion` output differs.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/i18n"
)

// Version is overridden at build time with -ldflags "-X my-agent/internal/cli.Version=...".
var Version = "0.7.0"

// loadApp is a seam for tests.
var loadApp = app.Load

type env struct {
	in       io.Reader
	out, err io.Writer
	cfgPath  string
	langFlag string // --lang, kept so that `init` can store an explicit choice
}

// usageText is the help screen, in the current language. The command syntax
// stays as typed; only the descriptions are translated.
func usageText() string {
	var sb strings.Builder
	row := func(syntax, key string) {
		if key == "" {
			fmt.Fprintf(&sb, "  %s\n", syntax)
			return
		}
		fmt.Fprintf(&sb, "  %-35s %s\n", syntax, i18n.T(key))
	}
	fmt.Fprintf(&sb, "%s\n\n%s\n\n%s\n", i18n.T("cli.usage.header"), i18n.T("cli.usage.synopsis"), i18n.T("cli.usage.commands"))
	row("(none) [--agent A] [--model P/M] [--continue]", "cli.usage.tui")
	row("run [--agent A] [--model P/M] [--yes] MESSAGE...", "cli.usage.run")
	row("agents [list]", "cli.usage.agents")
	row("mcp [list]", "cli.usage.mcp")
	row("sessions [list|remove ID]", "cli.usage.sessions")
	row("init [--force]", "cli.usage.init")
	row("config show|get|set|path|validate", "cli.usage.config")
	row("providers list|test|add [name]", "cli.usage.providers")
	row("chat [--provider P] [--no-rag]", "cli.usage.chat")
	row("ask [--provider P] [--no-rag] [--sources] QUESTION...", "")
	row("rag ingest|search|list|remove|clear", "cli.usage.rag")
	row("models list|ps|select|use|pull", "cli.usage.models")
	row("connectors list|add|remove|sync", "cli.usage.connectors")
	row("serve [--addr HOST:PORT]", "cli.usage.serve")
	row("completion bash|zsh|fish|powershell", "cli.usage.completion")
	row("version", "")
	fmt.Fprintf(&sb, "\n%s\n%s\n", i18n.T("cli.usage.config_file"), i18n.T("cli.usage.language"))
	return sb.String()
}

// Run executes the CLI and returns the process exit code.
func Run(args []string, in io.Reader, out, errw io.Writer) int {
	e := &env{in: in, out: out, err: errw}
	i18n.Set(i18n.Detect(os.Getenv(i18n.EnvVar))) // so even flag errors are translated
	fs := flag.NewFlagSet("my-agent", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&e.cfgPath, "config", config.DefaultPath(), "")
	fs.StringVar(&e.cfgPath, "c", config.DefaultPath(), "")
	fs.StringVar(&e.langFlag, "lang", "", i18n.T("cli.flag.lang"))
	var to tuiOptions
	fs.StringVar(&to.agent, "agent", "", i18n.T("cli.flag.agent"))
	fs.StringVar(&to.model, "model", "", i18n.T("cli.flag.model"))
	fs.StringVar(&to.model, "m", "", i18n.T("cli.flag.model"))
	fs.StringVar(&to.session, "session", "", i18n.T("cli.flag.session"))
	fs.BoolVar(&to.cont, "continue", false, i18n.T("cli.flag.continue"))
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(errw, "%s\n\n%s", i18n.T("cli.error", err), usageText())
		return 2
	}
	if !i18n.Valid(e.langFlag) {
		fmt.Fprintln(errw, i18n.T("cli.error", i18n.Err("lang.unsupported", e.langFlag)))
		return 2
	}
	// --lang, then $MY_AGENT_LANG, then the config's "language", then the system's
	i18n.Set(i18n.Detect(e.langFlag, os.Getenv(i18n.EnvVar), config.PeekLanguage(e.cfgPath)))
	explicitConfig := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "config" || f.Name == "c" {
			explicitConfig = true
		}
	})
	explicitConfig = explicitConfig || os.Getenv(config.EnvConfigPath) != ""
	rest := fs.Args()
	if len(rest) == 0 {
		if !isTerminal(in) || !isTerminal(out) { // scripts and pipes get the help, as always
			fmt.Fprint(out, usageText())
			return 0
		}
		// the interface handles Ctrl+C itself: no signal context here
		if err := e.cmdTUI(context.Background(), to, explicitConfig, ""); err != nil {
			fmt.Fprintln(errw, i18n.T("cli.error", err))
			return 1
		}
		return 0
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	switch cmd, cargs := rest[0], rest[1:]; cmd {
	case "tui":
		stop()
		err = e.cmdTUI(context.Background(), to, explicitConfig, joinArgs(cargs))
	case "run":
		err = e.cmdRun(ctx, cargs)
	case "agents":
		err = e.cmdAgents(cargs)
	case "mcp":
		err = e.cmdMCP(ctx, cargs)
	case "sessions":
		err = e.cmdSessions(cargs)
	case "help", "-h", "--help":
		fmt.Fprint(out, usageText())
	case "version", "--version":
		fmt.Fprintln(out, "my-agent", Version)
	case "init":
		err = e.cmdInit(cargs)
	case "config":
		err = e.cmdConfig(cargs)
	case "providers":
		err = e.cmdProviders(ctx, cargs)
	case "chat":
		err = e.cmdChat(ctx, cargs)
	case "ask":
		err = e.cmdAsk(ctx, cargs)
	case "rag":
		err = e.cmdRAG(ctx, cargs)
	case "models":
		err = e.cmdModels(ctx, cargs)
	case "connectors":
		err = e.cmdConnectors(ctx, cargs)
	case "serve":
		err = e.cmdServe(ctx, cargs)
	case "completion":
		err = e.cmdCompletion(cargs)
	default:
		err = usageKey("cli.unknown_command", cmd)
	}
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return 0
	case errors.As(err, new(usageError)):
		fmt.Fprintln(errw, i18n.T("cli.error", err))
		return 2
	default:
		fmt.Fprintln(errw, i18n.T("cli.error", err))
		return 1
	}
}

type usageError struct{ msg string }

func (u usageError) Error() string { return u.msg }

// usageKey is a usage error whose message comes from the catalog.
func usageKey(key string, a ...any) error { return usageError{i18n.T(key, a...)} }

// usageSyntax is the "usage: ..." error for a command's syntax.
func usageSyntax(syntax string) error { return usageKey("cli.usage", syntax) }

// flags builds a FlagSet that reports errors as usage errors.
func (e *env) flags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(e.err)
	return fs
}

func parse(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return usageError{err.Error()}
	}
	return nil
}

func (e *env) app() (*app.App, error) {
	a, err := loadApp(e.cfgPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, i18n.Errorf("cli.config_not_found", e.cfgPath)
	}
	return a, err
}

func (e *env) cmdInit(args []string) error {
	fs := e.flags("init")
	force := fs.Bool("force", false, i18n.T("cli.flag.force"))
	if err := parse(fs, args); err != nil {
		return err
	}
	if _, err := os.Stat(e.cfgPath); err == nil && !*force {
		return i18n.Errorf("cli.init.exists", e.cfgPath)
	}
	cfg := config.Default()
	cfg.Agent.SystemPrompt = config.DefaultPrompt(i18n.Current()) // the default prompt in the user's language
	if e.langFlag != "" {
		cfg.Language = i18n.Canonical(e.langFlag) // an explicit --lang is remembered
	}
	if err := cfg.Save(e.cfgPath); err != nil {
		return err
	}
	fmt.Fprintln(e.out, i18n.T("cli.init.created", e.cfgPath))
	return nil
}

func joinArgs(a []string) string { return strings.TrimSpace(strings.Join(a, " ")) }
