// Package connectors loads documents from external sources (files,
// directories, web pages) so they can be ingested into the RAG index.
package connectors

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"my-agent/internal/config"
	"my-agent/internal/i18n"
	"my-agent/internal/rag"
)

const maxBytes = 5 << 20

// DefaultInclude are the file patterns used when a connector sets none.
var DefaultInclude = []string{"*.md", "*.txt", "*.rst", "*.go", "*.py", "*.js", "*.ts", "*.json", "*.yaml", "*.yml", "*.csv", "*.html"}

type Document struct {
	Source string
	Text   string
}

type Loader interface {
	Load(ctx context.Context) ([]Document, error)
}

// New builds the loader for a connector definition.
func New(c config.ConnectorConfig) (Loader, error) {
	switch c.Type {
	case "file":
		if c.Path == "" {
			return nil, i18n.Err("connectors.err.file_needs_path")
		}
		return FileLoader{Path: c.Path, Include: c.Include, Recursive: c.Recursive}, nil
	case "url":
		if c.URL == "" {
			return nil, i18n.Err("connectors.err.url_needs_url")
		}
		return URLLoader{URL: c.URL, Client: &http.Client{Timeout: 30 * time.Second}}, nil
	case "database":
		if c.Database == nil {
			return nil, i18n.Errorf("connectors.err.missing_section", c.Name, "database")
		}
		return DBLoader{Name: c.Name, Conn: *c.Database}, nil
	case "email":
		if c.Email == nil {
			return nil, i18n.Errorf("connectors.err.missing_section", c.Name, "email")
		}
		return EmailLoader{Name: c.Name, Conn: *c.Email}, nil
	}
	return nil, i18n.Errorf("connectors.err.unknown_type", c.Type)
}

// Stats summarises an ingestion run.
type Stats struct {
	Documents int
	Chunks    int
}

// Ingest loads every document and stores it in ix. Per-document failures do
// not stop the run; they are joined into the returned error.
func Ingest(ctx context.Context, ix *rag.Index, l Loader) (Stats, error) {
	docs, err := l.Load(ctx)
	if err != nil {
		return Stats{}, err
	}
	var st Stats
	var errs []error
	for _, d := range docs {
		n, err := ix.IngestText(ctx, d.Source, d.Text)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.Source, err))
			continue
		}
		st.Documents++
		st.Chunks += n
	}
	return st, errors.Join(errs...)
}

// FileLoader reads a file, or the matching files of a directory.
type FileLoader struct {
	Path      string
	Include   []string
	Recursive bool
}

func (f FileLoader) Load(ctx context.Context) ([]Document, error) {
	info, err := os.Stat(f.Path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		d, err := readDoc(f.Path)
		if err != nil {
			return nil, err
		}
		return []Document{d}, nil
	}
	include := f.Include
	if len(include) == 0 {
		include = DefaultInclude
	}
	var docs []Document
	err = filepath.WalkDir(f.Path, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if e.IsDir() {
			name := e.Name()
			if p != f.Path && (!f.Recursive || strings.HasPrefix(name, ".") || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !matchAny(include, e.Name()) {
			return nil
		}
		d, err := readDoc(p)
		if err != nil {
			return nil // unreadable, binary or oversized files are skipped
		}
		docs = append(docs, d)
		return nil
	})
	return docs, err
}

func matchAny(patterns []string, name string) bool {
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, name); ok {
			return true
		}
	}
	return false
}

func readDoc(path string) (Document, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Document{}, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return Document{}, err
	}
	if fi.Size() > maxBytes {
		return Document{}, i18n.Errorf("connectors.err.too_large", path, maxBytes)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return Document{}, err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return Document{}, i18n.Errorf("connectors.err.binary", path)
	}
	text := string(data)
	if ext := strings.ToLower(filepath.Ext(abs)); ext == ".html" || ext == ".htm" {
		text = HTMLToText(text)
	}
	return Document{Source: abs, Text: text}, nil
}

// URLLoader fetches a single web page or text resource.
type URLLoader struct {
	URL    string
	Client *http.Client
}

func (u URLLoader) Load(ctx context.Context) ([]Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.URL, nil)
	if err != nil {
		return nil, err
	}
	client := u.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("GET %s: http %d", u.URL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, err
	}
	text := string(body)
	if strings.Contains(resp.Header.Get("Content-Type"), "html") {
		text = HTMLToText(text)
	}
	return []Document{{Source: u.URL, Text: text}}, nil
}

var (
	reDrop  = regexp.MustCompile(`(?is)<(script|style|noscript)[^>]*>.*?</(script|style|noscript)>`)
	reBlock = regexp.MustCompile(`(?i)</?(p|div|br|li|ul|ol|h[1-6]|tr|table|section|article)[^>]*>`)
	reTag   = regexp.MustCompile(`(?s)<[^>]*>`)
	reSpace = regexp.MustCompile(`[ \t]+`)
	reLines = regexp.MustCompile(`\n\s*\n+`)
)

// HTMLToText strips markup, keeping block structure as line breaks.
func HTMLToText(s string) string {
	s = reDrop.ReplaceAllString(s, " ")
	s = reBlock.ReplaceAllString(s, "\n")
	s = reTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = reSpace.ReplaceAllString(s, " ")
	s = reLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
