package connectors

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"my-agent/internal/config"
	"my-agent/internal/i18n"
)

var (
	dbMu    sync.Mutex
	dbCache = map[string]*sql.DB{}
)

// OpenDB opens (once per DSN) the database of a connector.
func OpenDB(d config.DatabaseConn) (*sql.DB, error) {
	dsn := d.ResolveDSN()
	if dsn == "" {
		return nil, i18n.Err("connectors.err.db_no_dsn")
	}
	driver := map[string]string{"sqlite": "sqlite", "postgres": "pgx", "mysql": "mysql"}[d.Driver]
	if driver == "" {
		return nil, i18n.Errorf("connectors.err.unknown_type", d.Driver)
	}
	key := driver + "\x00" + dsn
	dbMu.Lock()
	defer dbMu.Unlock()
	if db, ok := dbCache[key]; ok {
		return db, nil
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetConnMaxIdleTime(5 * time.Minute)
	dbCache[key] = db
	return db, nil
}

// CloseDBs closes every database opened by the connectors.
func CloseDBs() {
	dbMu.Lock()
	defer dbMu.Unlock()
	for k, db := range dbCache {
		db.Close()
		delete(dbCache, k)
	}
}

// QueryResult is a table of rows.
type QueryResult struct {
	Columns   []string
	Rows      [][]string
	Truncated bool
}

// Text renders the result as a pipe-separated table.
func (r QueryResult) Text() string {
	var sb strings.Builder
	sb.WriteString(strings.Join(r.Columns, " | ") + "\n")
	for _, row := range r.Rows {
		sb.WriteString(strings.Join(row, " | ") + "\n")
	}
	fmt.Fprintf(&sb, "(%d rows", len(r.Rows))
	if r.Truncated {
		sb.WriteString(", truncated")
	}
	sb.WriteString(")")
	return sb.String()
}

// Query runs a read-only query: it executes inside a read-only transaction
// (PostgreSQL, MySQL) or on a query_only connection (SQLite), so it cannot
// change data even if the statement tries to.
func Query(ctx context.Context, d config.DatabaseConn, query string, args ...any) (QueryResult, error) {
	db, err := OpenDB(d)
	if err != nil {
		return QueryResult{}, err
	}
	maxRows := d.MaxRows
	if maxRows <= 0 {
		maxRows = 200
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return QueryResult{}, err
	}
	defer conn.Close()
	var rows *sql.Rows
	if d.Driver == "sqlite" {
		if _, err := conn.ExecContext(ctx, "PRAGMA query_only = 1"); err != nil {
			return QueryResult{}, err
		}
		defer conn.ExecContext(context.Background(), "PRAGMA query_only = 0")
		rows, err = conn.QueryContext(ctx, query, args...)
	} else {
		tx, terr := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if terr != nil {
			return QueryResult{}, terr
		}
		defer tx.Rollback()
		rows, err = tx.QueryContext(ctx, query, args...)
	}
	if err != nil {
		return QueryResult{}, err
	}
	defer rows.Close()
	return scanRows(rows, maxRows)
}

// Exec runs a statement that changes data. Only allowed when the connector
// sets allow_write.
func Exec(ctx context.Context, d config.DatabaseConn, stmt string) (int64, error) {
	if !d.AllowWrite {
		return 0, i18n.Err("connectors.err.db_read_only")
	}
	db, err := OpenDB(d)
	if err != nil {
		return 0, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if d.Driver == "sqlite" {
		if _, err := conn.ExecContext(ctx, "PRAGMA query_only = 0"); err != nil {
			return 0, err
		}
	}
	res, err := conn.ExecContext(ctx, stmt)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func scanRows(rows *sql.Rows, maxRows int) (QueryResult, error) {
	cols, err := rows.Columns()
	if err != nil {
		return QueryResult{}, err
	}
	res := QueryResult{Columns: cols}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if len(res.Rows) >= maxRows {
			res.Truncated = true
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			return res, err
		}
		row := make([]string, len(cols))
		for i, v := range vals {
			row[i] = cell(v)
		}
		res.Rows = append(res.Rows, row)
	}
	return res, rows.Err()
}

func cell(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case []byte:
		s := string(x)
		if len(s) > 500 {
			s = s[:500] + "..."
		}
		return s
	case time.Time:
		return x.Format(time.RFC3339)
	default:
		s := fmt.Sprint(x)
		if len(s) > 500 {
			s = s[:500] + "..."
		}
		return strings.ReplaceAll(s, "\n", " ")
	}
}

// Schema describes the tables and columns of the database.
func Schema(ctx context.Context, d config.DatabaseConn) (string, error) {
	var q string
	switch d.Driver {
	case "sqlite":
		q = `SELECT m.name, p.name, p.type FROM sqlite_master m JOIN pragma_table_info(m.name) p
		     WHERE m.type IN ('table','view') AND m.name NOT LIKE 'sqlite_%' ORDER BY m.name, p.cid`
	case "postgres":
		q = `SELECT table_schema || '.' || table_name, column_name, data_type FROM information_schema.columns
		     WHERE table_schema NOT IN ('pg_catalog','information_schema') ORDER BY table_schema, table_name, ordinal_position`
	case "mysql":
		q = `SELECT table_name, column_name, column_type FROM information_schema.columns
		     WHERE table_schema = DATABASE() ORDER BY table_name, ordinal_position`
	}
	d.MaxRows = 5000
	res, err := Query(ctx, d, q)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	last := ""
	for _, r := range res.Rows {
		if r[0] != last {
			if last != "" {
				sb.WriteString(")\n")
			}
			sb.WriteString(r[0] + " (")
			last = r[0]
		} else {
			sb.WriteString(", ")
		}
		sb.WriteString(r[1] + " " + r[2])
	}
	if last != "" {
		sb.WriteString(")\n")
	}
	if sb.Len() == 0 {
		return "(no tables)", nil
	}
	return sb.String(), nil
}

// DBLoader turns the rows of a query into documents for the knowledge base,
// 50 rows per document.
type DBLoader struct {
	Name string
	Conn config.DatabaseConn
}

func (l DBLoader) Load(ctx context.Context) ([]Document, error) {
	if strings.TrimSpace(l.Conn.Query) == "" {
		return nil, i18n.Errorf("connectors.err.db_no_query", l.Name)
	}
	c := l.Conn
	if c.MaxRows <= 0 {
		c.MaxRows = 10000
	}
	res, err := Query(ctx, c, c.Query)
	if err != nil {
		return nil, err
	}
	var docs []Document
	for start := 0; start < len(res.Rows); start += 50 {
		end := min(start+50, len(res.Rows))
		var sb strings.Builder
		for _, row := range res.Rows[start:end] {
			for i, col := range res.Columns {
				fmt.Fprintf(&sb, "%s: %s\n", col, row[i])
			}
			sb.WriteString("\n")
		}
		docs = append(docs, Document{Source: fmt.Sprintf("db:%s#%d-%d", l.Name, start+1, end), Text: sb.String()})
	}
	return docs, nil
}
