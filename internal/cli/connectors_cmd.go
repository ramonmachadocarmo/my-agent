package cli

import (
	"context"
	"fmt"
	"strings"

	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/i18n"
)

const connectorsSyntax = "connectors list|add|remove|sync"

func (e *env) cmdConnectors(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usageSyntax(connectorsSyntax)
	}
	a, err := e.app()
	if err != nil {
		return err
	}
	return e.connectorsCommand(ctx, a, args)
}

// connectorsCommand runs a `connectors` subcommand on a loaded app.
func (e *env) connectorsCommand(ctx context.Context, a *app.App, args []string) error {
	if len(args) == 0 {
		return usageSyntax(connectorsSyntax)
	}
	switch sub, rest := args[0], args[1:]; sub {
	case "list":
		for _, c := range a.Cfg.Connectors {
			target := c.Path
			if c.Type == "url" {
				target = c.URL
			}
			fmt.Fprintf(e.out, "%-16s %-5s %s\n", c.Name, c.Type, target)
		}
		if len(a.Cfg.Connectors) == 0 {
			fmt.Fprintln(e.out, i18n.T("cli.connectors.none"))
		}
		return nil
	case "add":
		fs := e.flags("connectors add")
		name := fs.String("name", "", i18n.T("cli.flag.conn_name"))
		typ := fs.String("type", "", i18n.T("cli.flag.conn_type"))
		path := fs.String("path", "", i18n.T("cli.flag.conn_path"))
		url := fs.String("url", "", i18n.T("cli.flag.conn_url"))
		include := fs.String("include", "", i18n.T("cli.flag.include_short"))
		recursive := fs.Bool("recursive", true, i18n.T("cli.flag.recursive"))
		if err := parse(fs, rest); err != nil {
			return err
		}
		c := config.ConnectorConfig{Name: *name, Type: *typ, Path: *path, URL: *url, Recursive: *recursive}
		if *include != "" {
			c.Include = strings.Split(*include, ",")
		}
		next := a.Cfg
		next.Connectors = append(append([]config.ConnectorConfig{}, a.Cfg.Connectors...), c)
		if err := next.Validate(); err != nil {
			return err
		}
		a.Cfg = next
		if err := a.Save(); err != nil {
			return err
		}
		fmt.Fprintln(e.out, i18n.T("cli.connectors.added", c.Name))
		return nil
	case "remove":
		if len(rest) != 1 {
			return usageSyntax("connectors remove NAME")
		}
		var kept []config.ConnectorConfig
		for _, c := range a.Cfg.Connectors {
			if c.Name != rest[0] {
				kept = append(kept, c)
			}
		}
		if len(kept) == len(a.Cfg.Connectors) {
			return i18n.Errorf("app.err.unknown_connector", rest[0])
		}
		a.Cfg.Connectors = append([]config.ConnectorConfig{}, kept...)
		if err := a.Save(); err != nil {
			return err
		}
		fmt.Fprintln(e.out, i18n.T("cli.connectors.removed", rest[0]))
		return nil
	case "sync":
		names := rest
		if len(names) == 0 {
			for _, c := range a.Cfg.Connectors {
				names = append(names, c.Name)
			}
		}
		var failed int
		for _, n := range names {
			st, err := a.SyncConnector(ctx, n)
			if err != nil {
				failed++
				fmt.Fprintln(e.out, i18n.T("cli.connectors.sync_failed", n, err))
				continue
			}
			fmt.Fprintln(e.out, i18n.T("cli.rag.ingested", n, st.Documents, st.Chunks))
		}
		if failed > 0 {
			return i18n.Errorf("cli.connectors.failed", failed)
		}
		return nil
	}
	return usageKey("cli.unknown_sub", "connectors", args[0])
}
