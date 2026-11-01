package cli

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"my-agent/internal/i18n"
)

const (
	maxAttachBytes = 100 << 10 // per file
	maxAttachFiles = 5
)

// attachment is a file attached to a message with @path.
type attachment struct {
	path      string
	size      int64
	truncated bool
	content   string
}

func (a attachment) describe() string {
	s := i18n.T("mention.describe", a.path, formatBytes(a.size))
	if a.truncated {
		s += i18n.T("mention.describe_trunc", formatBytes(maxAttachBytes))
	}
	return s
}

// A mention starts at the beginning, after whitespace, or after an opening
// bracket or quote, so "(@a.md)" works but "ramon@gmail.com" does not.
var mentionRE = regexp.MustCompile(`(?:^|[\s(\["'])@(\S+)`)

// expandMentions finds @path words in text and, for each existing text file,
// appends its content to the message so the model can read it. Words that are
// not files (an @username, an email) are left alone silently; words that look
// like paths but cannot be attached produce a warning.
func expandMentions(text string) (msg string, atts []attachment, warns []string) {
	seen := map[string]bool{}
	for _, m := range mentionRE.FindAllStringSubmatch(text, -1) {
		word := m[1]
		path, info, ok := statMention(word)
		if !ok {
			if looksLikePath(word) {
				warns = append(warns, i18n.T("mention.not_found", strings.TrimRight(word, ".,;:!?")))
			}
			continue
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		switch {
		case info.IsDir():
			warns = append(warns, i18n.T("mention.folder", path, path))
		case len(atts) >= maxAttachFiles:
			warns = append(warns, i18n.T("mention.max_files", path, maxAttachFiles))
		default:
			a, err := readAttachment(path, info.Size())
			if err != nil {
				warns = append(warns, i18n.T("mention.file_error", path, err))
				continue
			}
			atts = append(atts, a)
		}
	}
	if len(atts) == 0 {
		return text, nil, warns
	}
	var sb strings.Builder
	sb.WriteString(text)
	for _, a := range atts {
		fmt.Fprintf(&sb, "\n\n%s\n```\n%s\n```", i18n.T("mention.header", a.path), strings.TrimRight(a.content, "\n"))
		if a.truncated {
			sb.WriteString("\n" + i18n.T("mention.truncated"))
		}
	}
	return sb.String(), atts, warns
}

// statMention resolves a mention to an existing path, forgiving punctuation
// that follows it in prose ("see @main.go, then...").
func statMention(word string) (string, os.FileInfo, bool) {
	for _, cand := range []string{word, strings.TrimRight(word, ".,;:!?)]}\"'")} {
		if cand == "" {
			continue
		}
		if info, err := os.Stat(cand); err == nil {
			return cand, info, true
		}
	}
	return "", nil, false
}

func looksLikePath(word string) bool {
	return strings.ContainsAny(word, `/\`) || strings.Contains(strings.TrimRight(word, ".,;:!?"), ".")
}

func readAttachment(path string, size int64) (attachment, error) {
	f, err := os.Open(path)
	if err != nil {
		return attachment{}, err
	}
	defer f.Close()
	buf := make([]byte, maxAttachBytes+1)
	n, _ := readFull(f, buf)
	data := buf[:n]
	if bytes.IndexByte(data[:min(len(data), 8192)], 0) >= 0 {
		return attachment{}, i18n.Err("mention.err_binary")
	}
	a := attachment{path: path, size: size}
	if len(data) > maxAttachBytes {
		data, a.truncated = data[:maxAttachBytes], true
	}
	a.content = string(data)
	return a, nil
}

func readFull(f *os.File, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := f.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
