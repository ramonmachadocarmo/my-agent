package web

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"my-agent/internal/config"
	"my-agent/internal/i18n"
)

func withLanguage(t *testing.T, l i18n.Lang) {
	t.Helper()
	i18n.Set(l)
	t.Cleanup(func() { i18n.Set(i18n.EN) })
}

var (
	attrKeyRE = regexp.MustCompile(`data-i18n(?:-ph|-title)?="([^"]+)"`)
	callKeyRE = regexp.MustCompile(`\bt\(['"]([a-zA-Z0-9_.]+)['"]`)
)

// Every message key that the page asks for must exist in the catalog (and be
// one of the keys the server sends to the browser).
func TestThePageOnlyUsesKnownMessageKeys(t *testing.T) {
	page := string(indexHTML)
	keys := map[string]bool{}
	for _, m := range attrKeyRE.FindAllStringSubmatch(page, -1) {
		keys[m[1]] = true
	}
	for _, m := range callKeyRE.FindAllStringSubmatch(page, -1) {
		keys[m[1]] = true
	}
	if len(keys) < 60 {
		t.Fatalf("found only %d keys in the page; the extraction is broken", len(keys))
	}
	for k := range keys {
		if !i18n.Has(k) {
			t.Errorf("the page uses %q, which is not in the catalog", k)
		}
		if !strings.HasPrefix(k, "web.ui.") {
			t.Errorf("%q is used by the page but only web.ui.* keys are sent to the browser", k)
		}
	}
}

// The reverse: a web.ui.* message nobody uses is dead weight (or a forgotten
// hook-up), so it must be used by the page.
func TestEveryWebMessageIsUsedByThePage(t *testing.T) {
	page := string(indexHTML)
	for _, k := range i18n.Keys() {
		if strings.HasPrefix(k, "web.ui.") && !strings.Contains(page, k) {
			t.Errorf("%s is in the catalog but the page never uses it", k)
		}
	}
}

// The page must not hold user-visible text of its own: static text nodes are
// filled from the catalog. Anything that looks like a sentence between tags,
// outside the elements that carry data-i18n, is a leftover.
func TestThePageHasNoHardCodedSentences(t *testing.T) {
	body := string(indexHTML)
	body = body[strings.Index(body, "<body>"):strings.Index(body, "<script>")]
	tagRE := regexp.MustCompile(`<([a-z0-9]+)([^>]*)>([^<]+)</`)
	for _, m := range tagRE.FindAllStringSubmatch(body, -1) {
		attrs, text := m[2], strings.TrimSpace(m[3])
		if text == "" || strings.Contains(attrs, "data-i18n") || text == "file" || text == "url" { // connector types are values, not prose
			continue
		}
		// letters only, at least two words or a long word: a sentence
		if regexp.MustCompile(`[A-Za-zÀ-ú]{4,}`).MatchString(text) && !strings.HasPrefix(text, "My Agent") {
			t.Errorf("hard-coded text in the page: %q (in <%s%s>)", text, m[1], attrs)
		}
	}
	for _, m := range regexp.MustCompile(`placeholder="([^"]*)"`).FindAllStringSubmatch(body, -1) {
		if m[1] != "*.md,*.txt" { // a file pattern, not prose
			t.Errorf("hard-coded placeholder %q: use data-i18n-ph", m[1])
		}
	}
}

func getI18n(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	srv, _ := newServer(t)
	code, m, _ := call(t, srv, "GET", path, nil)
	return code, m
}

func TestI18nEndpointServesTheRequestedLanguage(t *testing.T) {
	for query, want := range map[string]struct{ lang, tabConfig string }{
		"?lang=en":    {"en", "Configuration"},
		"?lang=pt-BR": {"pt-BR", "Configuração"},
		"?lang=pt":    {"pt-BR", "Configuração"},
		"?lang=es":    {"es", "Configuración"},
		"?lang=es-MX": {"es", "Configuración"}, // what a browser sends
	} {
		code, m := getI18n(t, "/api/i18n"+query)
		msgs, _ := m["messages"].(map[string]any)
		if code != 200 || m["lang"] != want.lang || msgs["web.ui.tab.config"] != want.tabConfig {
			t.Errorf("%s: %d lang=%v config=%v", query, code, m["lang"], msgs["web.ui.tab.config"])
		}
	}
}

func TestI18nEndpointFallsBackToTheServerLanguage(t *testing.T) {
	withLanguage(t, i18n.ES)
	for _, q := range []string{"", "?lang=", "?lang=fr", "?lang=klingon"} {
		_, m := getI18n(t, "/api/i18n"+q)
		msgs, _ := m["messages"].(map[string]any)
		if m["lang"] != "es" || msgs["web.ui.tab.config"] != "Configuración" {
			t.Errorf("%q should use the server language: %v", q, m["lang"])
		}
	}
}

func TestI18nEndpointContent(t *testing.T) {
	_, m := getI18n(t, "/api/i18n?lang=en")
	langs, _ := m["languages"].([]any)
	if len(langs) != 3 {
		t.Fatalf("languages: %v", langs)
	}
	first := langs[1].(map[string]any)
	if first["code"] != "pt-BR" || first["name"] != "Português (Brasil)" {
		t.Errorf("language list: %v", langs)
	}
	msgs := m["messages"].(map[string]any)
	for k := range msgs {
		if !strings.HasPrefix(k, "web.ui.") {
			t.Errorf("only web.ui.* messages go to the browser, got %q", k)
		}
	}
	if len(msgs) != len(i18n.Messages(i18n.EN, "web.ui.")) {
		t.Errorf("every web.ui.* message must be delivered: %d", len(msgs))
	}
	// templates keep their placeholders for the browser to fill in
	if !strings.Contains(msgs["web.ui.cfg.test_ok"].(string), "%s") {
		t.Errorf("placeholders must be left for the client: %v", msgs["web.ui.cfg.test_ok"])
	}
}

func TestChangingTheLanguageInTheConfigChangesServerMessages(t *testing.T) {
	withLanguage(t, i18n.EN)
	srv, a := newServer(t)

	code, m, _ := call(t, srv, "POST", "/api/chat", map[string]any{"message": "  "})
	if code != 400 || m["error"] != "message is required" {
		t.Fatalf("english: %d %v", code, m)
	}

	var cfg config.Config
	_, _, raw := call(t, srv, "GET", "/api/config", nil)
	json.Unmarshal(raw, &cfg)
	cfg.Language = "es"
	if code, _, _ := call(t, srv, "PUT", "/api/config", cfg); code != 200 {
		t.Fatalf("PUT language: %d", code)
	}
	if a.Cfg.Language != "es" {
		t.Fatalf("language not stored: %q", a.Cfg.Language)
	}
	saved, _ := config.Load(a.Path)
	if saved.Language != "es" {
		t.Fatalf("language not persisted: %q", saved.Language)
	}
	code, m, _ = call(t, srv, "POST", "/api/chat", map[string]any{"message": "  "})
	if code != 400 || m["error"] != "el mensaje es obligatorio" {
		t.Fatalf("spanish: %d %v", code, m)
	}

	cfg.Language = "pt" // an alias is stored canonically
	call(t, srv, "PUT", "/api/config", cfg)
	if a.Cfg.Language != "pt-BR" {
		t.Errorf("alias should be canonicalised, got %q", a.Cfg.Language)
	}
	_, m, _ = call(t, srv, "POST", "/api/chat", map[string]any{"message": "  "})
	if m["error"] != "a mensagem é obrigatória" {
		t.Fatalf("portuguese: %v", m)
	}

	cfg.Language = "klingon"
	if code, m, _ := call(t, srv, "PUT", "/api/config", cfg); code != 400 || !strings.Contains(m["error"].(string), "klingon") {
		t.Fatalf("an unsupported language must be rejected: %d %v", code, m)
	}
}

func TestServerErrorsInEachLanguage(t *testing.T) {
	want := map[i18n.Lang]string{
		i18n.EN:   "content-type must be application/json",
		i18n.PtBR: "content-type deve ser application/json",
		i18n.ES:   "content-type debe ser application/json",
	}
	for l, msg := range want {
		withLanguage(t, l)
		srv, _ := newServer(t)
		code, m, _ := call(t, srv, "POST", "/api/rag/clear", nil)
		_ = code
		resp := postText(t, srv.URL+"/api/rag/clear")
		if resp != msg {
			t.Errorf("%s: %q, want %q (%v)", l, resp, msg, m)
		}
	}
}

// postText posts a non-JSON body and returns the server's error message.
func postText(t *testing.T, url string) string {
	t.Helper()
	resp, err := httpPostText(url)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func httpPostText(url string) (string, error) {
	resp, err := http.Post(url, "text/plain", strings.NewReader("x=1"))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var m map[string]string
	json.NewDecoder(resp.Body).Decode(&m)
	return m["error"], nil
}
