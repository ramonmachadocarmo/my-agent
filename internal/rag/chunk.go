// Package rag implements chunking, embeddings, a JSON vector store and
// similarity search used to ground the agent in your own documents.
package rag

import (
	"strings"
	"unicode"
)

// ChunkText splits text into chunks of about size runes with the given
// overlap, preferring paragraph, line and word boundaries.
func ChunkText(text string, size, overlap int) []string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" || size <= 0 {
		return nil
	}
	if overlap < 0 || overlap >= size {
		overlap = 0
	}
	r := []rune(text)
	var chunks []string
	for start := 0; start < len(r); {
		end := start + size
		if end >= len(r) {
			end = len(r)
		} else {
			end = breakPoint(r, start, end)
		}
		if c := strings.TrimSpace(string(r[start:end])); c != "" {
			chunks = append(chunks, c)
		}
		if end >= len(r) {
			break
		}
		next := end - overlap
		if next <= start {
			next = end
		}
		for next < end && !unicode.IsSpace(r[next-1]) { // start overlap on a word boundary
			next++
		}
		start = next
	}
	return chunks
}

// breakPoint looks backwards from end (within the second half of the window)
// for a natural boundary.
func breakPoint(r []rune, start, end int) int {
	lo := start + (end-start)/2
	for _, sep := range [][]rune{{'\n', '\n'}, {'\n'}, {' '}} {
		for i := end - len(sep); i > lo; i-- {
			if matchAt(r, i, sep) {
				return i + len(sep)
			}
		}
	}
	return end
}

func matchAt(r []rune, i int, sep []rune) bool {
	for j, c := range sep {
		if r[i+j] != c {
			return false
		}
	}
	return true
}
