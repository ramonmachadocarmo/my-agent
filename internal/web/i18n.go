package web

import (
	"net/http"

	"my-agent/internal/i18n"
)

// getI18n serves the messages of the web UI. ?lang= picks the language (a
// browser's navigator.language works, e.g. "pt-BR" or "es-MX"); without it, or
// with an unsupported one, the server's current language is used.
func (s *Server) getI18n(w http.ResponseWriter, r *http.Request) {
	l, ok := i18n.Normalize(r.URL.Query().Get("lang"))
	if !ok {
		l = i18n.Current()
	}
	type language struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	langs := make([]language, 0, len(i18n.Supported))
	for _, x := range i18n.Supported {
		langs = append(langs, language{Code: string(x), Name: i18n.Names[x]})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"lang":      l,
		"languages": langs,
		"messages":  i18n.Messages(l, "web.ui."),
	})
}
