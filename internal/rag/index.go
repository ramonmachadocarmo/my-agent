package rag

import (
	"context"

	"my-agent/internal/i18n"
)

const embedBatch = 32

// Index ties a Store to an Embedder and chunking settings.
type Index struct {
	Store     *Store
	Embedder  Embedder
	ChunkSize int
	Overlap   int
	TopK      int
	MinScore  float64
}

// IngestText chunks, embeds and stores text under source, replacing any
// previous version. It returns the number of chunks stored. Embedding happens
// before the store is touched, so a failure leaves existing data intact.
func (ix *Index) IngestText(ctx context.Context, source, text string) (int, error) {
	texts := ChunkText(text, ix.ChunkSize, ix.Overlap)
	vecs, err := ix.embedAll(ctx, texts)
	if err != nil {
		return 0, err
	}
	chunks := make([]Chunk, len(texts))
	for i, t := range texts {
		chunks[i] = Chunk{Source: source, Index: i, Text: t, Vector: vecs[i]}
	}
	if err := ix.Store.Replace(ix.Embedder.ID(), source, chunks); err != nil {
		return 0, err
	}
	return len(chunks), nil
}

// Search embeds query and returns the top matches. An empty store returns nil
// without calling the embedder.
func (ix *Index) Search(ctx context.Context, query string) ([]Result, error) {
	if ix.Store.Len() == 0 {
		return nil, nil
	}
	if id := ix.Store.EmbedderID(); id != ix.Embedder.ID() {
		return nil, i18n.Errorf("rag.err.mismatch_search", ErrEmbedderMismatch, id, ix.Embedder.ID())
	}
	vecs, err := ix.Embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vecs) != 1 {
		return nil, i18n.Errorf("rag.err.vectors_query", len(vecs))
	}
	return ix.Store.Search(vecs[0], ix.TopK, ix.MinScore), nil
}

func (ix *Index) embedAll(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for i := 0; i < len(texts); i += embedBatch {
		batch := texts[i:min(i+embedBatch, len(texts))]
		vecs, err := ix.Embedder.Embed(ctx, batch)
		if err != nil {
			return nil, err
		}
		if len(vecs) != len(batch) {
			return nil, i18n.Errorf("rag.err.vectors_texts", len(vecs), len(batch))
		}
		out = append(out, vecs...)
	}
	return out, nil
}
