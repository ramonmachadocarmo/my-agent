// Package web serves the visual configuration and chat UI plus its JSON API.
package web

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/i18n"
	"my-agent/internal/provider"
)

//go:embed static/index.html
var indexHTML []byte

// Server exposes an App over HTTP. Reads take the read lock; anything that
// changes config or the knowledge base takes the write lock.
type Server struct {
	mu  sync.RWMutex
	app *app.App
}

func New(a *app.App) *Server { return &Server{app: a} }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
	mux.HandleFunc("GET /api/i18n", s.getI18n)
	mux.HandleFunc("GET /api/config", s.read(s.getConfig))
	mux.HandleFunc("PUT /api/config", s.write(s.putConfig))
	mux.HandleFunc("GET /api/providers", s.read(s.listProviders))
	mux.HandleFunc("POST /api/providers/{name}/test", s.read(s.testProvider))
	mux.HandleFunc("GET /api/providers/{name}/models", s.read(s.listModels))
	mux.HandleFunc("POST /api/providers/{name}/models/pull", s.pullModel)
	mux.HandleFunc("POST /api/chat", s.read(s.chat))
	mux.HandleFunc("POST /api/chat/stream", s.chatStream)
	mux.HandleFunc("GET /api/rag", s.read(s.ragStatus))
	mux.HandleFunc("POST /api/rag/ingest", s.write(s.ragIngest))
	mux.HandleFunc("POST /api/rag/search", s.read(s.ragSearch))
	mux.HandleFunc("POST /api/rag/remove", s.write(s.ragRemove))
	mux.HandleFunc("POST /api/rag/clear", s.write(s.ragClear))
	mux.HandleFunc("POST /api/connectors", s.write(s.addConnector))
	mux.HandleFunc("DELETE /api/connectors/{name}", s.write(s.removeConnector))
	mux.HandleFunc("POST /api/connectors/{name}/sync", s.write(s.syncConnector))
	return mux
}

type handler func(w http.ResponseWriter, r *http.Request) (any, int, error)

func (s *Server) read(h handler) http.HandlerFunc  { return s.wrap(h, s.mu.RLocker()) }
func (s *Server) write(h handler) http.HandlerFunc { return s.wrap(h, &s.mu) }

func (s *Server) wrap(h handler, l sync.Locker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireJSON(w, r) {
			return
		}
		l.Lock()
		v, status, err := h(w, r)
		l.Unlock()
		if err != nil {
			if status == 0 {
				status = http.StatusInternalServerError
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
}

// requireJSON rejects state-changing requests that are not JSON, which blocks
// cross-site form posts.
func requireJSON(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodDelete && r.ContentLength != 0 &&
		!strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": i18n.T("web.err.content_type")})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func decode(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 10<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return i18n.Errorf("web.err.bad_body", err)
	}
	return nil
}

func bad(err error) (any, int, error) { return nil, http.StatusBadRequest, err }

func (s *Server) getConfig(http.ResponseWriter, *http.Request) (any, int, error) {
	return s.app.Cfg.Masked(), 0, nil
}

func (s *Server) putConfig(_ http.ResponseWriter, r *http.Request) (any, int, error) {
	var next config.Config
	if err := decode(r, &next); err != nil {
		return bad(err)
	}
	// A masked key means "unchanged".
	for name, p := range next.Providers {
		old := s.app.Cfg.Providers[name]
		if p.APIKey == config.MaskedKey {
			p.APIKey = old.APIKey
		}
		if p.APIKeyEnv == config.MaskedKey { // hidden because it held a secret
			p.APIKeyEnv = old.APIKeyEnv
		}
		next.Providers[name] = p
	}
	if next.Connectors == nil {
		next.Connectors = []config.ConnectorConfig{}
	}
	if i18n.Valid(next.Language) {
		next.Language = i18n.Canonical(next.Language) // "pt" is stored as "pt-BR", "" as "auto"
	}
	if err := next.Validate(); err != nil {
		return bad(err)
	}
	prev := s.app.Cfg
	s.app.Cfg = next
	if err := s.app.Save(); err != nil {
		s.app.Cfg = prev
		return nil, 0, err
	}
	// A language chosen in the UI applies to the server's own messages too.
	i18n.Set(i18n.Detect(next.Language))
	return s.app.Cfg.Masked(), 0, nil
}

func (s *Server) listProviders(http.ResponseWriter, *http.Request) (any, int, error) {
	type info struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Model   string `json:"model"`
		HasKey  bool   `json:"has_key"`
		Default bool   `json:"default"`
	}
	var out []info
	for n, p := range s.app.Cfg.Providers {
		out = append(out, info{n, p.Type, p.Model, p.HasKey(), n == s.app.Cfg.Agent.Provider})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, 0, nil
}

func (s *Server) testProvider(_ http.ResponseWriter, r *http.Request) (any, int, error) {
	p, err := s.app.Provider(r.PathValue("name"))
	if err != nil {
		return bad(err)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	start := time.Now()
	if _, err := provider.ChatStream(ctx, p, provider.ChatRequest{
		Messages:  []provider.Message{{Role: "user", Content: "Reply with the single word: ok"}},
		MaxTokens: 16,
	}, nil); err != nil {
		return nil, http.StatusBadGateway, err
	}
	return map[string]any{"ok": true, "millis": time.Since(start).Milliseconds()}, 0, nil
}

func (s *Server) chat(_ http.ResponseWriter, r *http.Request) (any, int, error) {
	var req chatRequest
	if err := decode(r, &req); err != nil {
		return bad(err)
	}
	if err := req.validate(); err != nil {
		return bad(err)
	}
	useRAG := req.useRAG()
	ag, err := s.app.Agent(req.Provider, useRAG)
	if err != nil {
		return bad(err)
	}
	ans, err := ag.Ask(r.Context(), req.History, req.Message)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	return ans, 0, nil
}

func (s *Server) ragStatus(http.ResponseWriter, *http.Request) (any, int, error) {
	ix, err := s.app.Index()
	if err != nil {
		return nil, 0, err
	}
	srcs := ix.Store.Sources()
	return map[string]any{"sources": srcs, "chunks": ix.Store.Len(), "embedder": ix.Store.EmbedderID()}, 0, nil
}

func (s *Server) ragIngest(_ http.ResponseWriter, r *http.Request) (any, int, error) {
	var req struct {
		Name string `json:"name"`
		Text string `json:"text"`
		Path string `json:"path"`
		URL  string `json:"url"`
	}
	if err := decode(r, &req); err != nil {
		return bad(err)
	}
	switch {
	case req.Text != "":
		if req.Name == "" {
			return bad(i18n.Err("web.err.name_required"))
		}
		ix, err := s.app.Index()
		if err != nil {
			return nil, 0, err
		}
		n, err := ix.IngestText(r.Context(), req.Name, req.Text)
		if err != nil {
			return nil, 0, err
		}
		return map[string]int{"documents": 1, "chunks": n}, 0, nil
	case req.Path != "" || req.URL != "":
		cn := config.ConnectorConfig{Type: "file", Path: req.Path, Recursive: true}
		if req.URL != "" {
			cn = config.ConnectorConfig{Type: "url", URL: req.URL}
		}
		st, err := s.app.IngestConnector(r.Context(), cn)
		if err != nil {
			return nil, 0, err
		}
		return map[string]int{"documents": st.Documents, "chunks": st.Chunks}, 0, nil
	}
	return bad(i18n.Err("web.err.provide"))
}

func (s *Server) ragSearch(_ http.ResponseWriter, r *http.Request) (any, int, error) {
	var req struct {
		Query string `json:"query"`
		K     int    `json:"k"`
	}
	if err := decode(r, &req); err != nil {
		return bad(err)
	}
	ix, err := s.app.Index()
	if err != nil {
		return nil, 0, err
	}
	if req.K > 0 {
		ix.TopK = req.K
	}
	res, err := ix.Search(r.Context(), req.Query)
	if err != nil {
		return nil, 0, err
	}
	return res, 0, nil
}

func (s *Server) ragRemove(_ http.ResponseWriter, r *http.Request) (any, int, error) {
	var req struct {
		Source string `json:"source"`
	}
	if err := decode(r, &req); err != nil {
		return bad(err)
	}
	ix, err := s.app.Index()
	if err != nil {
		return nil, 0, err
	}
	n, err := ix.Store.RemoveSource(req.Source)
	return map[string]int{"removed": n}, 0, err
}

func (s *Server) ragClear(http.ResponseWriter, *http.Request) (any, int, error) {
	ix, err := s.app.Index()
	if err != nil {
		return nil, 0, err
	}
	return map[string]bool{"ok": true}, 0, ix.Store.Clear()
}

func (s *Server) addConnector(_ http.ResponseWriter, r *http.Request) (any, int, error) {
	var c config.ConnectorConfig
	if err := decode(r, &c); err != nil {
		return bad(err)
	}
	next := s.app.Cfg
	next.Connectors = append(append([]config.ConnectorConfig{}, next.Connectors...), c)
	if err := next.Validate(); err != nil {
		return bad(err)
	}
	prev := s.app.Cfg
	s.app.Cfg = next
	if err := s.app.Save(); err != nil {
		s.app.Cfg = prev
		return nil, 0, err
	}
	return c, 0, nil
}

func (s *Server) removeConnector(_ http.ResponseWriter, r *http.Request) (any, int, error) {
	name := r.PathValue("name")
	var kept []config.ConnectorConfig
	for _, c := range s.app.Cfg.Connectors {
		if c.Name != name {
			kept = append(kept, c)
		}
	}
	if len(kept) == len(s.app.Cfg.Connectors) {
		return nil, http.StatusNotFound, i18n.Errorf("app.err.unknown_connector", name)
	}
	prev := s.app.Cfg.Connectors
	s.app.Cfg.Connectors = append([]config.ConnectorConfig{}, kept...)
	if err := s.app.Save(); err != nil {
		s.app.Cfg.Connectors = prev
		return nil, 0, err
	}
	return map[string]bool{"ok": true}, 0, nil
}

func (s *Server) syncConnector(_ http.ResponseWriter, r *http.Request) (any, int, error) {
	name := r.PathValue("name")
	if _, ok := s.app.Cfg.FindConnector(name); !ok {
		return nil, http.StatusNotFound, i18n.Errorf("app.err.unknown_connector", name)
	}
	st, err := s.app.SyncConnector(r.Context(), name)
	if err != nil {
		return nil, 0, err
	}
	return map[string]int{"documents": st.Documents, "chunks": st.Chunks}, 0, nil
}
