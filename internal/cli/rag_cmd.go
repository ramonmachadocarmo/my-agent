package cli

import (
	"context"
	"fmt"
	"strings"

	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/i18n"
)

const ragSyntax = "rag ingest|search|list|remove|clear"

func (e *env) cmdRAG(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usageSyntax(ragSyntax)
	}
	a, err := e.app()
	if err != nil {
		return err
	}
	return e.ragCommand(ctx, a, args)
}

// ragCommand runs a `rag` subcommand on a loaded app (CLI and in-chat /rag).
func (e *env) ragCommand(ctx context.Context, a *app.App, args []string) error {
	if len(args) == 0 {
		return usageSyntax(ragSyntax)
	}
	switch sub, rest := args[0], args[1:]; sub {
	case "ingest":
		fs := e.flags("rag ingest")
		recursive := fs.Bool("recursive", true, i18n.T("cli.flag.recursive"))
		include := fs.String("include", "", i18n.T("cli.flag.include"))
		if err := parse(fs, rest); err != nil {
			return err
		}
		if fs.NArg() == 0 {
			return usageSyntax("rag ingest [--recursive=BOOL] [--include=*.md,*.txt] PATH_OR_URL...")
		}
		var patterns []string
		if *include != "" {
			patterns = strings.Split(*include, ",")
		}
		for _, target := range fs.Args() {
			cn := config.ConnectorConfig{Type: "file", Path: target, Include: patterns, Recursive: *recursive}
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
				cn = config.ConnectorConfig{Type: "url", URL: target}
			}
			st, err := a.IngestConnector(ctx, cn)
			fmt.Fprintln(e.out, i18n.T("cli.rag.ingested", target, st.Documents, st.Chunks))
			if err != nil {
				return err
			}
		}
		return nil
	case "search":
		fs := e.flags("rag search")
		topK := fs.Int("k", 0, i18n.T("cli.flag.k"))
		if err := parse(fs, rest); err != nil {
			return err
		}
		q := joinArgs(fs.Args())
		if q == "" {
			return usageSyntax("rag search [-k N] QUERY...")
		}
		ix, err := a.Index()
		if err != nil {
			return err
		}
		if *topK > 0 {
			ix.TopK = *topK
		}
		res, err := ix.Search(ctx, q)
		if err != nil {
			return err
		}
		if len(res) == 0 {
			fmt.Fprintln(e.out, i18n.T("cli.rag.no_results"))
			return nil
		}
		for i, r := range res {
			fmt.Fprintf(e.out, "[%d] score=%.3f %s#%d\n%s\n\n", i+1, r.Score, r.Chunk.Source, r.Chunk.Index, r.Chunk.Text)
		}
		return nil
	case "list":
		ix, err := a.Index()
		if err != nil {
			return err
		}
		srcs := ix.Store.Sources()
		for _, s := range srcs {
			fmt.Fprintf(e.out, "%5d  %s\n", s.Chunks, s.Source)
		}
		fmt.Fprintln(e.out, i18n.T("cli.rag.stats", len(srcs), ix.Store.Len(), orNone(ix.Store.EmbedderID())))
		return nil
	case "remove":
		if len(rest) != 1 {
			return usageSyntax("rag remove SOURCE")
		}
		ix, err := a.Index()
		if err != nil {
			return err
		}
		n, err := ix.Store.RemoveSource(rest[0])
		if err != nil {
			return err
		}
		fmt.Fprintln(e.out, i18n.T("cli.rag.removed", n))
		return nil
	case "clear":
		ix, err := a.Index()
		if err != nil {
			return err
		}
		if err := ix.Store.Clear(); err != nil {
			return err
		}
		fmt.Fprintln(e.out, i18n.T("cli.rag.cleared"))
		return nil
	}
	return usageKey("cli.unknown_sub", "rag", args[0])
}

func orNone(s string) string {
	if s == "" {
		return i18n.T("cli.none")
	}
	return s
}
