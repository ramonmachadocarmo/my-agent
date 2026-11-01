package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/i18n"
)

const configSyntax = "config show|get KEY|set KEY VALUE|path|validate"

func (e *env) cmdConfig(args []string) error {
	if len(args) == 0 {
		return usageSyntax(configSyntax)
	}
	if args[0] == "path" { // works even before the file exists
		fmt.Fprintln(e.out, e.cfgPath)
		return nil
	}
	a, err := e.app()
	if err != nil {
		return err
	}
	return e.configCommand(a, args)
}

// configCommand runs a `config` subcommand on a loaded app. The CLI and the
// in-chat /config command share it.
func (e *env) configCommand(a *app.App, args []string) error {
	if len(args) == 0 {
		return usageSyntax(configSyntax)
	}
	switch args[0] {
	case "path":
		fmt.Fprintln(e.out, a.Path)
		return nil
	case "validate":
		if err := a.Cfg.Validate(); err != nil {
			return err
		}
		fmt.Fprintln(e.out, i18n.T("cli.config.valid"))
		e.printWarnings(a.Cfg)
		return nil
	case "show":
		return e.printJSON(a.Cfg.Masked())
	case "get":
		if len(args) != 2 {
			return usageSyntax("config get KEY")
		}
		v, err := config.Get(a.Cfg.Masked(), args[1])
		if err != nil {
			return err
		}
		if s, ok := v.(string); ok {
			fmt.Fprintln(e.out, s)
			return nil
		}
		return e.printJSON(v)
	case "set":
		if len(args) != 3 {
			return usageSyntax("config set KEY VALUE")
		}
		if err := setConfig(a, args[1], args[2]); err != nil {
			return err
		}
		fmt.Fprintln(e.out, i18n.T("cli.config.updated", args[1]))
		return nil
	}
	return usageKey("cli.unknown_sub", "config", args[0])
}

// setConfig validates and saves one key. Nothing changes if it is invalid.
// Changing the language takes effect immediately.
func setConfig(a *app.App, key, value string) error {
	next, err := config.Set(a.Cfg, key, value)
	if err != nil {
		return err
	}
	prev := a.Cfg
	a.Cfg = next
	if err := a.Save(); err != nil {
		a.Cfg = prev
		return err
	}
	if key == "language" {
		i18n.Set(i18n.Detect(next.Language))
	}
	return nil
}

func (e *env) printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(e.out, string(b))
	return nil
}

const providersSyntax = "providers list|test|add [NAME]"

func (e *env) cmdProviders(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usageSyntax(providersSyntax)
	}
	a, err := e.app()
	if err != nil {
		return err
	}
	return e.providersCommand(ctx, a, args)
}

// providersCommand runs a `providers` subcommand on a loaded app.
func (e *env) providersCommand(ctx context.Context, a *app.App, args []string) error {
	if len(args) == 0 {
		return usageSyntax(providersSyntax)
	}
	names := make([]string, 0, len(a.Cfg.Providers))
	for n := range a.Cfg.Providers {
		names = append(names, n)
	}
	sort.Strings(names)
	switch args[0] {
	case "add":
		return e.providersAdd(a, args[1:])
	case "list":
		for _, n := range names {
			p := a.Cfg.Providers[n]
			mark := " "
			if n == a.Cfg.Agent.Provider {
				mark = "*"
			}
			key := i18n.T("cli.providers.no_key")
			if n := len(p.ResolveAPIKeys()); n > 1 {
				key = i18n.T("cli.providers.keys_set", n)
			} else if n == 1 {
				key = i18n.T("cli.providers.key_set")
			} else if p.HasKey() {
				key = i18n.T("cli.providers.no_key_needed")
			}
			fmt.Fprintf(e.out, "%s %-10s type=%-7s model=%-22s %s (%s)\n", mark, n, p.Type, p.Model, p.BaseURL, key)
		}
		e.printWarnings(a.Cfg)
		return nil
	case "test":
		if len(args) > 1 {
			names = args[1:]
		}
		failed := 0
		for _, n := range names {
			start := time.Now()
			err := e.testProvider(ctx, a.Provider, n)
			if err != nil {
				failed++
				fmt.Fprintf(e.out, "FAIL %-10s %v\n", n, err)
				continue
			}
			fmt.Fprintf(e.out, "OK   %-10s %s\n", n, time.Since(start).Round(time.Millisecond))
		}
		if failed > 0 {
			return i18n.Errorf("cli.providers.failed", failed)
		}
		return nil
	}
	return usageKey("cli.unknown_sub", "providers", args[0])
}

func (e *env) printWarnings(c config.Config) {
	for _, w := range c.Warnings() {
		fmt.Fprintf(e.out, "\n%s\n", i18n.T("cli.warning", w))
	}
}
