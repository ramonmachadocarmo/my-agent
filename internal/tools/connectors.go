package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"my-agent/internal/config"
	"my-agent/internal/connectors"
	"my-agent/internal/i18n"
)

// ConnectorTools returns the tools of the database and e-mail connectors.
// When there are several connectors of a kind, the model picks one with the
// "connector" argument.
func ConnectorTools(conns []config.ConnectorConfig) []*Tool {
	var dbs, mails []config.ConnectorConfig
	for _, c := range conns {
		switch {
		case c.Type == "database" && c.Database != nil:
			dbs = append(dbs, c)
		case c.Type == "email" && c.Email != nil:
			mails = append(mails, c)
		}
	}
	var out []*Tool
	if len(dbs) > 0 {
		out = append(out, dbTools(dbs)...)
	}
	if len(mails) > 0 {
		out = append(out, emailTools(mails)...)
	}
	return out
}

// withConnector adds the "connector" argument to a schema and describes the
// choices.
func withConnector(s map[string]any, conns []config.ConnectorConfig) map[string]any {
	names := make([]any, len(conns))
	var desc []string
	for i, c := range conns {
		names[i] = c.Name
		desc = append(desc, c.Name)
	}
	props := s["properties"].(map[string]any)
	props["connector"] = map[string]any{"type": "string", "enum": names, "description": "Connector to use: " + strings.Join(desc, ", ")}
	if len(conns) > 1 {
		req, _ := s["required"].([]string)
		s["required"] = append(req, "connector")
	}
	return s
}

func pick(conns []config.ConnectorConfig, args map[string]any) (config.ConnectorConfig, error) {
	name := str(args, "connector")
	if name == "" && len(conns) == 1 {
		return conns[0], nil
	}
	for _, c := range conns {
		if c.Name == name {
			return c, nil
		}
	}
	var names []string
	for _, c := range conns {
		names = append(names, c.Name)
	}
	return config.ConnectorConfig{}, fmt.Errorf("unknown connector %q; use one of: %s", name, strings.Join(names, ", "))
}

func connSubject(conns []config.ConnectorConfig, key string) func(map[string]any) string {
	return func(args map[string]any) string {
		c, _ := pick(conns, args)
		s := str(args, key)
		if len(s) > 200 {
			s = s[:200] + "..."
		}
		if c.Name == "" {
			return s
		}
		return c.Name + ": " + s
	}
}

func dbTools(conns []config.ConnectorConfig) []*Tool {
	var list []string
	anyWrite := false
	for _, c := range conns {
		mode := "read-only"
		if c.Database.AllowWrite {
			mode, anyWrite = "read-write", true
		}
		list = append(list, fmt.Sprintf("%s (%s, %s)", c.Name, c.Database.Driver, mode))
	}
	dbs := strings.Join(list, "; ")
	ts := []*Tool{
		{
			Name:        "db_schema",
			Description: "List the tables and columns of a database. Databases: " + dbs,
			Params:      withConnector(schema(), conns),
			Subject:     connSubject(conns, ""),
			Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
				c, err := pick(conns, args)
				if err != nil {
					return Result{}, err
				}
				s, err := connectors.Schema(ctx, *c.Database)
				return Result{Output: Truncate(s, 60<<10)}, err
			},
		},
		{
			Name:        "db_query",
			Description: "Run a read-only SQL query (SELECT) and return the rows. Use the database's SQL dialect. Databases: " + dbs,
			Params:      withConnector(schema("sql", "string:The SQL query"), conns),
			Subject:     connSubject(conns, "sql"),
			Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
				c, err := pick(conns, args)
				if err != nil {
					return Result{}, err
				}
				res, err := connectors.Query(ctx, *c.Database, str(args, "sql"))
				if err != nil {
					return Result{}, err
				}
				return Result{Output: Truncate(res.Text(), 60<<10), Display: i18n.N("tools.rows", len(res.Rows))}, nil
			},
		},
	}
	if anyWrite {
		ts = append(ts, &Tool{
			Name:        "db_exec",
			Description: "Run a SQL statement that changes data (INSERT, UPDATE, DELETE, DDL) on a read-write database. Databases: " + dbs,
			Params:      withConnector(schema("sql", "string:The SQL statement"), conns),
			Subject:     connSubject(conns, "sql"),
			Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
				c, err := pick(conns, args)
				if err != nil {
					return Result{}, err
				}
				n, err := connectors.Exec(ctx, *c.Database, str(args, "sql"))
				if err != nil {
					return Result{}, err
				}
				return Result{Output: fmt.Sprintf("%d rows affected", n)}, nil
			},
		})
	}
	return ts
}

func emailTools(conns []config.ConnectorConfig) []*Tool {
	var names []string
	var canSend []config.ConnectorConfig
	for _, c := range conns {
		names = append(names, c.Name+" ("+c.Email.Username+")")
		if c.Email.SMTPHost != "" {
			canSend = append(canSend, c)
		}
	}
	accounts := strings.Join(names, "; ")
	ts := []*Tool{
		{
			Name:        "email_search",
			Description: "List e-mail messages, newest first, optionally matching a text. Unread messages are marked with *. Accounts: " + accounts,
			Params: withConnector(schema("query?", "string:Text to search for (empty: all messages)", "folder?", "string:Mailbox folder (default INBOX)",
				"unread_only?", "boolean:Only unread messages", "limit?", "integer:Maximum messages (default 20)"), conns),
			Subject: connSubject(conns, "query"),
			Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
				c, err := pick(conns, args)
				if err != nil {
					return Result{}, err
				}
				msgs, err := connectors.SearchEmail(ctx, *c.Email, str(args, "folder"), str(args, "query"), boolean(args, "unread_only"), num(args, "limit", 20), false)
				if err != nil {
					return Result{}, err
				}
				if len(msgs) == 0 {
					return Result{Output: "no messages"}, nil
				}
				var sb strings.Builder
				for _, m := range msgs {
					sb.WriteString(m.Header() + "\n")
				}
				return Result{Output: sb.String(), Display: i18n.N("tools.messages", len(msgs))}, nil
			},
		},
		{
			Name:        "email_read",
			Description: "Read one e-mail message (headers and text body) by its uid, as listed by email_search.",
			Params:      withConnector(schema("uid", "integer:Message uid", "folder?", "string:Mailbox folder (default INBOX)"), conns),
			Subject:     connSubject(conns, "uid"),
			Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
				c, err := pick(conns, args)
				if err != nil {
					return Result{}, err
				}
				uid := num(args, "uid", 0)
				if uid <= 0 {
					return Result{}, errors.New("uid is required")
				}
				m, err := connectors.ReadEmail(ctx, *c.Email, str(args, "folder"), uint32(uid))
				if err != nil {
					return Result{}, err
				}
				out := fmt.Sprintf("From: %s\nTo: %s\nDate: %s\nSubject: %s\n\n%s", m.From, m.To, m.Date, m.Subject, m.Body)
				return Result{Output: Truncate(out, 60<<10), Display: m.Subject}, nil
			},
		},
	}
	if len(canSend) > 0 {
		ts = append(ts, &Tool{
			Name:        "email_send",
			Description: "Send a plain-text e-mail. Always show the user the recipients, subject and body before sending.",
			Params: withConnector(schema("to", "string:Recipients, comma separated", "subject", "string:Subject", "body", "string:Plain-text body",
				"cc?", "string:Carbon-copy recipients, comma separated"), canSend),
			Subject: func(args map[string]any) string { return str(args, "to") + ": " + str(args, "subject") },
			Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
				c, err := pick(canSend, args)
				if err != nil {
					return Result{}, err
				}
				to, cc := splitList(str(args, "to")), splitList(str(args, "cc"))
				if err := connectors.SendEmail(ctx, *c.Email, to, cc, str(args, "subject"), str(args, "body")); err != nil {
					return Result{}, err
				}
				return Result{Output: "sent to " + strings.Join(append(to, cc...), ", ")}, nil
			},
		})
	}
	return ts
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
