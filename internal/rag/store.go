package rag

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"my-agent/internal/i18n"
)

// ErrEmbedderMismatch means the store holds vectors from another embedder.
var ErrEmbedderMismatch = i18n.Err("rag.err.embedder_mismatch")

type Chunk struct {
	Source string    `json:"source"`
	Index  int       `json:"index"`
	Text   string    `json:"text"`
	Vector []float32 `json:"vector"`
}

type Result struct {
	Chunk Chunk   `json:"chunk"`
	Score float64 `json:"score"`
}

type SourceInfo struct {
	Source string `json:"source"`
	Chunks int    `json:"chunks"`
}

// Store is a JSON-file-backed vector store with brute-force cosine search.
type Store struct {
	mu         sync.RWMutex
	path       string
	embedderID string
	chunks     []Chunk
}

type storeFile struct {
	Embedder string  `json:"embedder"`
	Chunks   []Chunk `json:"chunks"`
}

// OpenStore loads the store at path; a missing file yields an empty store.
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var f storeFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, i18n.Errorf("rag.err.reading_store", path, err)
	}
	s.embedderID, s.chunks = f.Embedder, f.Chunks
	return s, nil
}

func (s *Store) save() error {
	data, err := json.Marshal(storeFile{Embedder: s.embedderID, Chunks: s.chunks})
	if err != nil {
		return err
	}
	if dir := filepath.Dir(s.path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Replace atomically swaps all chunks of source for the given ones.
func (s *Store) Replace(embedderID, source string, chunks []Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := make([]Chunk, 0, len(s.chunks)+len(chunks))
	for _, c := range s.chunks {
		if c.Source != source {
			kept = append(kept, c)
		}
	}
	if len(kept) > 0 && s.embedderID != embedderID {
		return i18n.Errorf("rag.err.mismatch_ingest", ErrEmbedderMismatch, s.embedderID, embedderID)
	}
	s.embedderID = embedderID
	s.chunks = append(kept, chunks...)
	return s.save()
}

// RemoveSource deletes a source and returns how many chunks were removed.
func (s *Store) RemoveSource(source string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.chunks[:0:0]
	for _, c := range s.chunks {
		if c.Source != source {
			kept = append(kept, c)
		}
	}
	removed := len(s.chunks) - len(kept)
	if removed == 0 {
		return 0, nil
	}
	s.chunks = kept
	if len(kept) == 0 {
		s.embedderID = ""
	}
	return removed, s.save()
}

// Clear removes everything.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chunks, s.embedderID = nil, ""
	return s.save()
}

func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.chunks)
}

func (s *Store) EmbedderID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.embedderID
}

// Sources lists sources with their chunk counts, sorted by name.
func (s *Store) Sources() []SourceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	counts := map[string]int{}
	for _, c := range s.chunks {
		counts[c.Source]++
	}
	out := make([]SourceInfo, 0, len(counts))
	for src, n := range counts {
		out = append(out, SourceInfo{src, n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}

// Search returns up to k chunks with cosine similarity >= minScore, best first.
func (s *Store) Search(query []float32, k int, minScore float64) []Result {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var res []Result
	for _, c := range s.chunks {
		if sc := Cosine(query, c.Vector); sc >= minScore {
			res = append(res, Result{Chunk: c, Score: sc})
		}
	}
	sort.SliceStable(res, func(i, j int) bool { return res[i].Score > res[j].Score })
	if k > 0 && len(res) > k {
		res = res[:k]
	}
	return res
}

// Cosine returns the cosine similarity of a and b (0 if lengths differ or a
// vector is zero).
func Cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
