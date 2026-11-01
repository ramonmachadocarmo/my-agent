package rag

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// Embedder turns text into vectors. ID identifies the embedding space; a
// store only accepts vectors from a single ID.
type Embedder interface {
	ID() string
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// FuncEmbedder adapts a function (for example a provider's Embed) to Embedder.
type FuncEmbedder struct {
	Name string
	Fn   func(ctx context.Context, texts []string) ([][]float32, error)
}

func (f FuncEmbedder) ID() string { return f.Name }
func (f FuncEmbedder) Embed(ctx context.Context, t []string) ([][]float32, error) {
	return f.Fn(ctx, t)
}

// LocalDim is the dimensionality of the offline hashing embedder.
const LocalDim = 512

// LocalEmbedder is an offline bag-of-words hashing embedder. It needs no
// network or API key and gives lexical (keyword) similarity, which is enough
// for small corpora and for tests. Use a model embedder for semantic search.
type LocalEmbedder struct{}

func (LocalEmbedder) ID() string { return "local-hash-512" }

func (LocalEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = hashVector(t)
	}
	return out, nil
}

func hashVector(text string) []float32 {
	v := make([]float32, LocalDim)
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, w := range words {
		h := fnv.New32a()
		h.Write([]byte(w))
		v[h.Sum32()%LocalDim]++
	}
	for i, x := range v { // sublinear term frequency
		if x > 0 {
			v[i] = 1 + float32(math.Log(float64(x)))
		}
	}
	return v
}
