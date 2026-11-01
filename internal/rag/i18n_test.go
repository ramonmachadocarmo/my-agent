package rag

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"my-agent/internal/i18n"
)

func TestEmbedderMismatchIsLocalizedAndStillMatchesWithErrorsIs(t *testing.T) {
	defer i18n.Set(i18n.EN)
	for lang, want := range map[i18n.Lang]string{
		i18n.EN:   "run `rag clear` and re-ingest",
		i18n.PtBR: "rode `rag clear` e reingira os documentos",
		i18n.ES:   "ejecute `rag clear` y vuelva a ingerir los documentos",
	} {
		i18n.Set(lang)
		ix := newIndex(t)
		ix.IngestText(context.Background(), "a", "some text here")
		other := &Index{Store: ix.Store, Embedder: FuncEmbedder{Name: "other", Fn: LocalEmbedder{}.Embed}, ChunkSize: 100, TopK: 1}
		_, err := other.Search(context.Background(), "text")
		if !errors.Is(err, ErrEmbedderMismatch) || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", lang, err)
		}
	}
	_ = filepath.Join
}
