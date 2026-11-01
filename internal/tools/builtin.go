package tools

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"my-agent/internal/connectors"
	"my-agent/internal/i18n"
)

// Messages in this file are read by the model, so they stay in English.

const (
	maxRead      = 256 << 10
	maxReadLines = 2000
	maxOutput    = 30 << 10
	maxResults   = 200
)

// skipDirs are never searched.
var skipDirs = map[string]bool{".git": true, "node_modules": true, ".venv": true, "venv": true, "__pycache__": true, ".idea": true, ".vs": true, "dist": true, "target": true, ".next": true, "vendor": false}

// Builtins returns the built-in tools. rag_search is included only when env
// can open a knowledge base.
func Builtins(env *Env) []*Tool {
	ts := []*Tool{readTool(), writeTool(), editTool(), bashTool(env), globTool(), grepTool(), lsTool(), webfetchTool()}
	if env.Index != nil {
		ts = append(ts, ragTool())
	}
	for _, t := range ts {
		t.Source = "builtin"
	}
	return ts
}

func pathSubject(args map[string]any) string { return str(args, "path") }

func readTool() *Tool {
	return &Tool{
		Name:        "read",
		Description: "Read a text file. Returns numbered lines. Use offset (1-based line) and limit for large files.",
		Params:      schema("path", "string:File path, absolute or relative to the working directory", "offset?", "integer:First line to read (1-based)", "limit?", "integer:Maximum number of lines (default 2000)"),
		Subject:     pathSubject,
		Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
			p := env.Abs(str(args, "path"))
			fi, err := os.Stat(p)
			if err != nil {
				return Result{}, err
			}
			if fi.IsDir() {
				return Result{}, fmt.Errorf("%s is a directory; use the ls tool", p)
			}
			f, err := os.Open(p)
			if err != nil {
				return Result{}, err
			}
			defer f.Close()
			head := make([]byte, 8000)
			n, _ := io.ReadFull(f, head)
			if bytes.IndexByte(head[:n], 0) >= 0 {
				return Result{}, fmt.Errorf("%s looks like a binary file", p)
			}
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return Result{}, err
			}
			offset := max(num(args, "offset", 1), 1)
			limit := num(args, "limit", maxReadLines)
			if limit <= 0 || limit > maxReadLines {
				limit = maxReadLines
			}
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 64<<10), 4<<20)
			var sb strings.Builder
			line, shown, total := 0, 0, 0
			for sc.Scan() {
				line++
				total = line
				if line < offset || shown >= limit || sb.Len() > maxRead {
					continue
				}
				text := sc.Text()
				if len(text) > 2000 {
					text = text[:2000] + "..."
				}
				fmt.Fprintf(&sb, "%6d\t%s\n", line, text)
				shown++
			}
			if err := sc.Err(); err != nil {
				return Result{}, err
			}
			if shown == 0 {
				if total == 0 {
					return Result{Output: "(empty file)"}, nil
				}
				return Result{}, fmt.Errorf("offset %d is past the end of the file (%d lines)", offset, total)
			}
			if last := offset + shown - 1; last < total {
				fmt.Fprintf(&sb, "... (%d more lines; use offset=%d to continue)\n", total-last, last+1)
			}
			return Result{Output: sb.String(), Display: i18n.N("tools.lines", shown)}, nil
		},
	}
}

func writeTool() *Tool {
	return &Tool{
		Name:        "write",
		Description: "Create or overwrite a file with the given content. Parent directories are created. Prefer edit for changing existing files.",
		Params:      schema("path", "string:File path", "content", "string:The complete file content"),
		Subject:     pathSubject,
		Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
			p := env.Abs(str(args, "path"))
			content := str(args, "content")
			old, readErr := os.ReadFile(p)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return Result{}, err
			}
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				return Result{}, err
			}
			verb := "created"
			if readErr == nil {
				verb = "overwrote"
			}
			return Result{
				Output:  fmt.Sprintf("%s %s (%d bytes)", verb, env.Rel(p), len(content)),
				Display: Diff(string(old), content),
			}, nil
		},
	}
}

func editTool() *Tool {
	return &Tool{
		Name: "edit",
		Description: "Replace text in a file. old_string must match the file exactly (including indentation) and be unique unless replace_all is true. " +
			"Read the file first. With an empty old_string and a missing file, the file is created with new_string.",
		Params:  schema("path", "string:File path", "old_string", "string:Exact text to replace", "new_string", "string:Replacement text", "replace_all?", "boolean:Replace every occurrence"),
		Subject: pathSubject,
		Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
			p := env.Abs(str(args, "path"))
			oldS, newS := str(args, "old_string"), str(args, "new_string")
			data, err := os.ReadFile(p)
			if errors.Is(err, fs.ErrNotExist) && oldS == "" {
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					return Result{}, err
				}
				if err := os.WriteFile(p, []byte(newS), 0o644); err != nil {
					return Result{}, err
				}
				return Result{Output: "created " + env.Rel(p), Display: Diff("", newS)}, nil
			}
			if err != nil {
				return Result{}, err
			}
			if oldS == newS {
				return Result{}, errors.New("old_string and new_string are identical")
			}
			text := string(data)
			// tolerate CRLF files when the model sends LF
			crlf := strings.Contains(text, "\r\n") && !strings.Contains(oldS, "\r\n")
			if crlf {
				oldS = strings.ReplaceAll(oldS, "\n", "\r\n")
				newS = strings.ReplaceAll(newS, "\n", "\r\n")
			}
			n := strings.Count(text, oldS)
			switch {
			case oldS == "":
				return Result{}, errors.New("old_string is empty; use write to replace a whole file")
			case n == 0:
				return Result{}, errors.New("old_string was not found in the file; read the file and copy the text exactly")
			case n > 1 && !boolean(args, "replace_all"):
				return Result{}, fmt.Errorf("old_string matches %d places; add more context to make it unique or set replace_all", n)
			}
			var out string
			if boolean(args, "replace_all") {
				out = strings.ReplaceAll(text, oldS, newS)
			} else {
				out = strings.Replace(text, oldS, newS, 1)
			}
			if err := os.WriteFile(p, []byte(out), 0o644); err != nil {
				return Result{}, err
			}
			d := Diff(strings.ReplaceAll(oldS, "\r\n", "\n"), strings.ReplaceAll(newS, "\r\n", "\n"))
			return Result{Output: fmt.Sprintf("edited %s (%d replacement(s))", env.Rel(p), max(n, 1)), Display: d}, nil
		},
	}
}

func bashTool(env *Env) *Tool {
	shell := "sh"
	if len(env.Shell) > 0 {
		shell = filepath.Base(env.Shell[0])
	}
	return &Tool{
		Name: "bash",
		Description: fmt.Sprintf("Run a shell command (%s) in the working directory and return its combined output and exit code. "+
			"Use it for builds, tests, git and other CLI tools. Avoid interactive commands. Default timeout 120s (max 600).", shell),
		Params:  schema("command", "string:The command line to run", "timeout_seconds?", "integer:Timeout in seconds", "workdir?", "string:Directory to run in (default: working directory)"),
		Subject: func(args map[string]any) string { return str(args, "command") },
		Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
			command := str(args, "command")
			if strings.TrimSpace(command) == "" {
				return Result{}, errors.New("command is empty")
			}
			timeout := time.Duration(min(max(num(args, "timeout_seconds", 120), 1), 600)) * time.Second
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			sh := env.Shell
			if len(sh) == 0 {
				sh = []string{"/bin/sh", "-c"}
			}
			cmd := exec.CommandContext(ctx, sh[0], append(append([]string{}, sh[1:]...), command)...)
			cmd.Dir = env.Dir
			if wd := str(args, "workdir"); wd != "" {
				cmd.Dir = env.Abs(wd)
			}
			cmd.Stdin = nil
			var buf bytes.Buffer
			cmd.Stdout, cmd.Stderr = &buf, &buf
			cmd.WaitDelay = 2 * time.Second
			err := cmd.Run()
			out := buf.String()
			if len(out) > maxOutput { // keep the end: errors and summaries are there
				out = fmt.Sprintf("... [%d bytes omitted]\n", len(out)-maxOutput) + out[len(out)-maxOutput:]
			}
			code := 0
			var ee *exec.ExitError
			switch {
			case ctx.Err() == context.DeadlineExceeded:
				return Result{Output: out + fmt.Sprintf("\n[timed out after %s]", timeout)}, nil
			case errors.As(err, &ee):
				code = ee.ExitCode()
			case err != nil:
				return Result{}, err
			}
			if strings.TrimSpace(out) == "" {
				out = "(no output)"
			}
			return Result{Output: fmt.Sprintf("%s\n[exit code %d]", strings.TrimRight(out, "\n"), code)}, nil
		},
	}
}

func globTool() *Tool {
	return &Tool{
		Name:        "glob",
		Description: "Find files by name pattern, e.g. **/*.go or src/**/*.ts. Returns paths sorted by modification time (newest first).",
		Params:      schema("pattern", "string:Glob pattern; ** matches any number of directories", "path?", "string:Directory to search (default: working directory)"),
		Subject:     func(args map[string]any) string { return str(args, "pattern") },
		Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
			root := env.Abs(str(args, "path"))
			re, err := globRegexp(str(args, "pattern"))
			if err != nil {
				return Result{}, err
			}
			type hit struct {
				p string
				t time.Time
			}
			var hits []hit
			err = walk(ctx, root, func(p string, d fs.DirEntry) {
				rel, _ := filepath.Rel(root, p)
				if re.MatchString(filepath.ToSlash(rel)) {
					var t time.Time
					if fi, err := d.Info(); err == nil {
						t = fi.ModTime()
					}
					hits = append(hits, hit{p, t})
				}
			})
			if err != nil {
				return Result{}, err
			}
			sort.Slice(hits, func(i, j int) bool { return hits[i].t.After(hits[j].t) })
			var sb strings.Builder
			for i, h := range hits {
				if i == maxResults {
					fmt.Fprintf(&sb, "... %d more\n", len(hits)-maxResults)
					break
				}
				sb.WriteString(env.Rel(h.p) + "\n")
			}
			if len(hits) == 0 {
				return Result{Output: "no files found"}, nil
			}
			return Result{Output: sb.String(), Display: i18n.N("tools.files", len(hits))}, nil
		},
	}
}

// globRegexp converts a glob with ** to a regular expression over
// slash-separated relative paths. A pattern without a slash matches the
// file name at any depth.
func globRegexp(pattern string) (*regexp.Regexp, error) {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return nil, errors.New("pattern is empty")
	}
	if !strings.Contains(pattern, "/") {
		pattern = "**/" + pattern
	}
	var sb strings.Builder
	sb.WriteString("(?i)^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					sb.WriteString("(?:.*/)?")
				} else {
					sb.WriteString(".*")
				}
			} else {
				sb.WriteString("[^/]*")
			}
		case '?':
			sb.WriteString("[^/]")
		case '{':
			sb.WriteString("(?:")
		case '}':
			sb.WriteString(")")
		case ',':
			sb.WriteString("|")
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	sb.WriteString("$")
	return regexp.Compile(sb.String())
}

// walk visits the files under root, skipping hidden and dependency folders.
func walk(ctx context.Context, root string, visit func(p string, d fs.DirEntry)) error {
	count := 0
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return err
			}
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && (skipDirs[d.Name()] || (strings.HasPrefix(d.Name(), ".") && d.Name() != ".github")) {
				return filepath.SkipDir
			}
			return nil
		}
		if count++; count > 200000 {
			return filepath.SkipAll
		}
		visit(p, d)
		return nil
	})
}

func grepTool() *Tool {
	return &Tool{
		Name:        "grep",
		Description: "Search file contents with a regular expression (RE2 syntax). Returns file:line: text for each match.",
		Params:      schema("pattern", "string:Regular expression", "path?", "string:File or directory to search (default: working directory)", "include?", "string:Only files matching this glob, e.g. *.go or *.{ts,tsx}", "ignore_case?", "boolean:Case-insensitive search"),
		Subject:     func(args map[string]any) string { return str(args, "pattern") },
		Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
			pat := str(args, "pattern")
			if boolean(args, "ignore_case") {
				pat = "(?i)" + pat
			}
			re, err := regexp.Compile(pat)
			if err != nil {
				return Result{}, fmt.Errorf("invalid regular expression: %w", err)
			}
			var inc *regexp.Regexp
			if s := str(args, "include"); s != "" {
				if inc, err = globRegexp(s); err != nil {
					return Result{}, err
				}
			}
			root := env.Abs(str(args, "path"))
			var sb strings.Builder
			matches, files := 0, 0
			search := func(p string) {
				if matches >= maxResults {
					return
				}
				if inc != nil {
					rel, _ := filepath.Rel(root, p)
					if !inc.MatchString(filepath.ToSlash(rel)) && !inc.MatchString(filepath.Base(p)) {
						return
					}
				}
				data, err := os.ReadFile(p)
				if err != nil || len(data) > 4<<20 || bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
					return
				}
				found := false
				for i, line := range strings.Split(string(data), "\n") {
					if re.MatchString(line) {
						found = true
						line = strings.TrimRight(line, "\r")
						if len(line) > 300 {
							line = line[:300] + "..."
						}
						fmt.Fprintf(&sb, "%s:%d: %s\n", env.Rel(p), i+1, line)
						if matches++; matches >= maxResults {
							sb.WriteString("... (more matches omitted)\n")
							return
						}
					}
				}
				if found {
					files++
				}
			}
			if fi, err := os.Stat(root); err == nil && !fi.IsDir() {
				search(root)
			} else if err := walk(ctx, root, func(p string, _ fs.DirEntry) { search(p) }); err != nil {
				return Result{}, err
			}
			if matches == 0 {
				return Result{Output: "no matches"}, nil
			}
			return Result{Output: sb.String(), Display: i18n.N("tools.matches", matches) + " · " + i18n.N("tools.files", files)}, nil
		},
	}
}

func lsTool() *Tool {
	return &Tool{
		Name:        "ls",
		Description: "List a directory. Folders end with /.",
		Params:      schema("path?", "string:Directory (default: working directory)"),
		Subject:     pathSubject,
		Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
			p := env.Abs(str(args, "path"))
			entries, err := os.ReadDir(p)
			if err != nil {
				return Result{}, err
			}
			var sb strings.Builder
			for i, e := range entries {
				if i == 500 {
					fmt.Fprintf(&sb, "... %d more\n", len(entries)-500)
					break
				}
				name := e.Name()
				if e.IsDir() {
					name += "/"
				} else if fi, err := e.Info(); err == nil {
					name += fmt.Sprintf("  (%d bytes)", fi.Size())
				}
				sb.WriteString(name + "\n")
			}
			if len(entries) == 0 {
				return Result{Output: "(empty directory)"}, nil
			}
			return Result{Output: sb.String(), Display: i18n.N("tools.entries", len(entries))}, nil
		},
	}
}

func webfetchTool() *Tool {
	return &Tool{
		Name:        "webfetch",
		Description: "Fetch a web page or text resource over HTTP(S). HTML is converted to plain text.",
		Params:      schema("url", "string:http:// or https:// URL"),
		Subject:     func(args map[string]any) string { return str(args, "url") },
		Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
			u := str(args, "url")
			if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
				return Result{}, errors.New("url must start with http:// or https://")
			}
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
			if err != nil {
				return Result{}, err
			}
			req.Header.Set("User-Agent", "my-agent")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return Result{}, err
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
			if err != nil {
				return Result{}, err
			}
			if resp.StatusCode < 200 || resp.StatusCode > 299 {
				return Result{}, fmt.Errorf("GET %s: http %d", u, resp.StatusCode)
			}
			text := string(body)
			if strings.Contains(resp.Header.Get("Content-Type"), "html") {
				text = connectors.HTMLToText(text)
			}
			return Result{Output: Truncate(text, 100<<10), Display: i18n.N("tools.bytes", len(text))}, nil
		},
	}
}

func ragTool() *Tool {
	return &Tool{
		Name:        "rag_search",
		Description: "Search the user's knowledge base (documents, pages, e-mails and database rows ingested by connectors). Returns the most relevant passages with their sources.",
		Params:      schema("query", "string:What to look for"),
		Subject:     func(args map[string]any) string { return str(args, "query") },
		Run: func(ctx context.Context, env *Env, args map[string]any) (Result, error) {
			ix, err := env.Index()
			if err != nil {
				return Result{}, err
			}
			hits, err := ix.Search(ctx, str(args, "query"))
			if err != nil {
				return Result{}, err
			}
			if len(hits) == 0 {
				return Result{Output: "no relevant passages"}, nil
			}
			var sb strings.Builder
			for i, h := range hits {
				fmt.Fprintf(&sb, "[%d] source: %s (score %.2f)\n%s\n\n", i+1, h.Chunk.Source, h.Score, h.Chunk.Text)
			}
			return Result{Output: sb.String(), Display: i18n.N("tools.passages", len(hits))}, nil
		},
	}
}

// Diff is a compact line diff of two texts: removed lines start with "-",
// added ones with "+", with up to two lines of context around changes.
func Diff(a, b string) string {
	al, bl := splitLines(a), splitLines(b)
	// trim the common head and tail, then show the middle as -/+
	p := 0
	for p < len(al) && p < len(bl) && al[p] == bl[p] {
		p++
	}
	s := 0
	for s < len(al)-p && s < len(bl)-p && al[len(al)-1-s] == bl[len(bl)-1-s] {
		s++
	}
	var sb strings.Builder
	ctxStart := max(p-2, 0)
	if ctxStart > 0 {
		fmt.Fprintf(&sb, "@@ line %d @@\n", ctxStart+1)
	}
	for _, l := range al[ctxStart:p] {
		sb.WriteString("  " + l + "\n")
	}
	lines := 0
	for _, l := range al[p : len(al)-s] {
		if lines++; lines > 200 {
			break
		}
		sb.WriteString("- " + l + "\n")
	}
	for _, l := range bl[p : len(bl)-s] {
		if lines++; lines > 400 {
			sb.WriteString("  ...\n")
			break
		}
		sb.WriteString("+ " + l + "\n")
	}
	for _, l := range al[len(al)-s : min(len(al)-s+2, len(al))] {
		sb.WriteString("  " + l + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), "\n"), "\n")
}
