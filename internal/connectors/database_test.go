package connectors

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"my-agent/internal/config"
)

func sqliteConn(t *testing.T) config.DatabaseConn {
	t.Helper()
	d := config.DatabaseConn{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "t.db"), AllowWrite: true}
	t.Cleanup(CloseDBs) // registered after TempDir, so it runs before the folder is removed
	ctx := context.Background()
	for _, s := range []string{
		"CREATE TABLE people (id INTEGER PRIMARY KEY, name TEXT, city TEXT)",
		"INSERT INTO people (name, city) VALUES ('Ana', 'Recife'), ('Bruno', 'Natal')",
	} {
		if _, err := Exec(ctx, d, s); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func TestSQLiteQuerySchemaAndReadOnly(t *testing.T) {
	d := sqliteConn(t)
	ctx := context.Background()
	res, err := Query(ctx, d, "SELECT name, city FROM people ORDER BY id")
	if err != nil || len(res.Rows) != 2 || res.Rows[1][0] != "Bruno" {
		t.Fatalf("%+v %v", res, err)
	}
	if !strings.Contains(res.Text(), "name | city") {
		t.Fatalf("text: %q", res.Text())
	}
	s, err := Schema(ctx, d)
	if err != nil || !strings.Contains(s, "people (id INTEGER, name TEXT, city TEXT)") {
		t.Fatalf("schema %q %v", s, err)
	}
	// a query cannot write, even with a write statement
	if _, err := Query(ctx, d, "DELETE FROM people"); err == nil {
		t.Fatal("Query must be read-only")
	}
	res, _ = Query(ctx, d, "SELECT count(*) FROM people")
	if res.Rows[0][0] != "2" {
		t.Fatalf("rows were deleted: %v", res.Rows)
	}
	ro := d
	ro.AllowWrite = false
	if _, err := Exec(ctx, ro, "DELETE FROM people"); err == nil {
		t.Fatal("Exec must refuse a read-only connector")
	}
	// writing again after read-only queries on the same pool works
	if n, err := Exec(ctx, d, "UPDATE people SET city = 'Olinda' WHERE name = 'Ana'"); err != nil || n != 1 {
		t.Fatalf("exec: %d %v", n, err)
	}
}

func TestDBLoaderMakesDocuments(t *testing.T) {
	d := sqliteConn(t)
	d.Query = "SELECT name, city FROM people"
	docs, err := DBLoader{Name: "crm", Conn: d}.Load(context.Background())
	if err != nil || len(docs) != 1 || docs[0].Source != "db:crm#1-2" || !strings.Contains(docs[0].Text, "city: Natal") {
		t.Fatalf("%+v %v", docs, err)
	}
}
