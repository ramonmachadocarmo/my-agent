package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"my-agent/internal/i18n"
	"my-agent/internal/provider"
)

type chatRequest struct {
	Message  string             `json:"message"`
	Provider string             `json:"provider"`
	UseRAG   *bool              `json:"use_rag"`
	History  []provider.Message `json:"history"`
}

func (r chatRequest) validate() error {
	if strings.TrimSpace(r.Message) == "" {
		return i18n.Err("web.err.message_required")
	}
	return nil
}

func (r chatRequest) useRAG() bool { return r.UseRAG == nil || *r.UseRAG }

// chatStream answers like /api/chat but streams the reply as
// newline-delimited JSON: {"delta":"..."} lines while the model writes,
// then {"done":true,"text":"...","sources":[...]} or {"error":"..."}.
// The server lock is held only while the agent is built, never while the
// (possibly long) generation runs. Closing the connection stops generation.
func (s *Server) chatStream(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	fail := func(status int, err error) {
		writeJSON(w, status, map[string]string{"error": err.Error()})
	}
	var req chatRequest
	if err := decode(r, &req); err != nil {
		fail(http.StatusBadRequest, err)
		return
	}
	if err := req.validate(); err != nil {
		fail(http.StatusBadRequest, err)
		return
	}
	s.mu.RLock()
	ag, err := s.app.Agent(req.Provider, req.useRAG())
	s.mu.RUnlock()
	if err != nil {
		fail(http.StatusBadRequest, err)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	enc := json.NewEncoder(w)
	flusher, _ := w.(http.Flusher)
	send := func(v any) {
		enc.Encode(v)
		if flusher != nil {
			flusher.Flush()
		}
	}
	ans, err := ag.AskStream(r.Context(), req.History, req.Message, func(d string) {
		send(map[string]string{"delta": d})
	})
	if err != nil {
		send(map[string]string{"error": err.Error()})
		return
	}
	send(map[string]any{"done": true, "text": ans.Text, "sources": ans.Sources})
}
