package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"my-agent/internal/i18n"
)

func inLanguage(t *testing.T, l i18n.Lang) {
	t.Helper()
	i18n.Set(l)
	t.Cleanup(func() { i18n.Set(i18n.EN) })
}

func TestLanguageSetting(t *testing.T) {
	if Default().Language != "auto" {
		t.Fatalf("default language: %q", Default().Language)
	}
	for in, want := range map[string]string{"": "auto", "AUTO": "auto", "pt": "pt-BR", "pt_br": "pt-BR", "es-MX": "es", "EN": "en", "en_US.UTF-8": "en"} {
		c := Default()
		c.Language = in
		if err := c.Validate(); err != nil {
			t.Errorf("%q should be valid: %v", in, err)
		}
		next, err := Set(Default(), "language", in)
		if err != nil || next.Language != want {
			t.Errorf("Set(language, %q) = %q, %v; want %q", in, next.Language, err, want)
		}
	}
	c := Default()
	c.Language = "klingon"
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "language") || !strings.Contains(err.Error(), "klingon") {
		t.Fatalf("an unsupported language must be reported: %v", err)
	}
}

func TestParseCanonicalisesLanguageAndAcceptsOldFiles(t *testing.T) {
	base := `{"agent":{"name":"a","system_prompt":"s","provider":"ollama","temperature":0.2,"max_tokens":100,"use_rag":true},
	"providers":{"ollama":{"type":"ollama","model":"m","base_url":"http://x","api_key":"","api_key_env":"","embedding_model":"","timeout_seconds":120}},
	"rag":{"store_path":"d","embedder":"local","chunk_size":100,"chunk_overlap":10,"top_k":3,"min_score":0.1},
	"connectors":[],"server":{"addr":"127.0.0.1:1"}%s}`
	old, err := Parse([]byte(strings.Replace(base, "%s", "", 1)))
	if err != nil || old.Language != "auto" {
		t.Fatalf("a config with no language means auto: %q %v", old.Language, err)
	}
	pt, err := Parse([]byte(strings.Replace(base, "%s", `,"language":"pt"`, 1)))
	if err != nil || pt.Language != "pt-BR" {
		t.Fatalf("%q %v", pt.Language, err)
	}
	if _, err := Parse([]byte(strings.Replace(base, "%s", `,"language":"fr"`, 1))); err == nil {
		t.Fatal("unsupported language must be rejected")
	}
}

func TestPeekLanguage(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if got := PeekLanguage(write("a.json", `{"language":"es","agent":{"broken":true}}`)); got != "es" {
		t.Errorf("must read the language even from an otherwise invalid config: %q", got)
	}
	if got := PeekLanguage(write("b.json", `{"agent":{}}`)); got != "" {
		t.Errorf("%q", got)
	}
	for _, bad := range []string{filepath.Join(dir, "missing.json"), write("c.json", `not json`)} {
		if got := PeekLanguage(bad); got != "" {
			t.Errorf("%s: %q", bad, got)
		}
	}
}

func TestValidationMessagesInEachLanguage(t *testing.T) {
	cases := map[i18n.Lang][]string{
		i18n.EN:   {"agent.temperature: must be between 0 and 2", "rag.top_k: must be > 0", "connectors[0].path: required for type file"},
		i18n.PtBR: {"agent.temperature: deve estar entre 0 e 2", "rag.top_k: deve ser > 0", "connectors[0].path: obrigatório para o tipo file"},
		i18n.ES:   {"agent.temperature: debe estar entre 0 y 2", "rag.top_k: debe ser > 0", "connectors[0].path: obligatorio para el tipo file"},
	}
	for lang, wants := range cases {
		inLanguage(t, lang)
		c := Default()
		c.Agent.Temperature = 5
		c.RAG.TopK = 0
		c.Connectors = []ConnectorConfig{{Name: "a", Type: "file"}}
		err := c.Validate()
		if err == nil {
			t.Fatal("expected errors")
		}
		for _, w := range wants {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: missing %q in:\n%v", lang, w, err)
			}
		}
	}
}

func TestSetErrorsInEachLanguage(t *testing.T) {
	for lang, want := range map[i18n.Lang]string{i18n.EN: "unknown key", i18n.PtBR: "chave desconhecida", i18n.ES: "clave desconocida"} {
		inLanguage(t, lang)
		if _, err := Set(Default(), "nope.key", "1"); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", lang, err)
		}
	}
	for lang, want := range map[i18n.Lang]string{i18n.EN: "is not a number", i18n.PtBR: "não é um número", i18n.ES: "no es un número"} {
		inLanguage(t, lang)
		if _, err := Set(Default(), "rag.top_k", "abc"); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", lang, err)
		}
	}
}

func TestMisplacedKeyWarningInEachLanguage(t *testing.T) {
	for lang, want := range map[i18n.Lang]string{
		i18n.EN:   "it looks like an API key pasted in the wrong field",
		i18n.PtBR: "parece uma chave de API colada no campo errado",
		i18n.ES:   "parece una clave de API pegada en el campo equivocado",
	} {
		inLanguage(t, lang)
		c := Default()
		g := c.Providers["gemini"]
		g.APIKeyEnv = "AQ.secret-value"
		c.Providers["gemini"] = g
		w := c.Warnings()
		if len(w) != 1 || !strings.Contains(w[0], want) || !strings.Contains(w[0], "providers.gemini.api_key") {
			t.Errorf("%s: %v", lang, w)
		}
		if strings.Contains(w[0], "secret-value") {
			t.Errorf("%s: the warning must never contain the value", lang)
		}
	}
}
