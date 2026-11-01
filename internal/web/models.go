package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"my-agent/internal/i18n"
	"my-agent/internal/provider"
)

func (s *Server) listModels(_ http.ResponseWriter, r *http.Request) (any, int, error) {
	name, mm, err := s.app.OllamaManager(r.PathValue("name"))
	if err != nil {
		return bad(err)
	}
	models, err := mm.ListModels(r.Context())
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	if models == nil {
		models = []provider.ModelInfo{}
	}
	// Which models are loaded (and whether they sit on the GPU) is a hint:
	// failing to get it must not fail the listing.
	running := []provider.RunningModel{}
	if rn, ok := mm.(provider.Runner); ok {
		if rm, err := rn.RunningModels(r.Context()); err == nil && rm != nil {
			running = rm
		}
	}
	pc := s.app.Cfg.Providers[name]
	return map[string]any{"models": models, "running": running, "model": pc.Model, "embedding_model": pc.EmbeddingModel}, 0, nil
}

// pullModel downloads a model (if missing) and optionally stores it in the
// config. The response is newline-delimited JSON: progress objects
// ({"status","total","completed"}) followed by {"done":true} or {"error":...}.
// It does not hold the server lock while downloading.
func (s *Server) pullModel(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	var req struct {
		Model string `json:"model"`
		SetAs string `json:"set_as"` // "model", "embedding" or "" (download only)
	}
	if err := decode(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	req.Model = strings.TrimSpace(req.Model)
	if req.Model == "" || (req.SetAs != "" && req.SetAs != "model" && req.SetAs != "embedding") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T("web.err.model_required")})
		return
	}
	s.mu.RLock()
	name, mm, err := s.app.OllamaManager(r.PathValue("name"))
	s.mu.RUnlock()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	enc := json.NewEncoder(w)
	flusher, _ := w.(http.Flusher)
	send := func(v any) {
		enc.Encode(v)
		if flusher != nil {
			flusher.Flush()
		}
	}
	fail := func(err error) { send(map[string]string{"error": err.Error()}) }

	pulled, err := provider.EnsureModel(r.Context(), mm, req.Model, provider.Throttle(func(p provider.PullProgress) { send(p) }))
	if err != nil {
		fail(err)
		return
	}
	if req.SetAs != "" {
		s.mu.Lock()
		err = s.app.SetModel(name, req.Model, req.SetAs == "embedding")
		s.mu.Unlock()
		if err != nil {
			fail(i18n.Errorf("web.err.saved_not_config", err.Error()))
			return
		}
	}
	send(map[string]any{"done": true, "model": req.Model, "downloaded": pulled, "configured": req.SetAs != ""})
}
