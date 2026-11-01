package cli

import (
	"os"
	"strings"
	"testing"

	"my-agent/internal/config"
	"my-agent/internal/i18n"
)

// One phrase per language that only that language contains.
var phrase = map[string]struct{ help, cleared, shortcuts string }{
	"en":    {"Usage: my-agent", "history cleared", "Shortcuts:"},
	"pt-BR": {"Uso: my-agent", "histórico apagado", "Atalhos:"},
	"es":    {"Uso: my-agent", "historial borrado", "Atajos:"},
}

func TestHelpScreenInEachLanguage(t *testing.T) {
	desc := map[string]string{
		"en":    "create a default config file",
		"pt-BR": "cria um arquivo de configuração padrão",
		"es":    "crea un archivo de configuración predeterminado",
	}
	for lang, want := range desc {
		cfg, _ := setup(t)
		out := mustRun(t, "", "--lang", lang, "-config", cfg, "help")
		if !strings.Contains(out, want) || !strings.Contains(out, phrase[lang].help) {
			t.Errorf("%s: help is missing %q / %q:\n%s", lang, want, phrase[lang].help, out)
		}
		// the command syntax itself is never translated
		for _, syntax := range []string{"init [--force]", "config show|get|set|path|validate", "rag ingest|search|list|remove|clear"} {
			if !strings.Contains(out, syntax) {
				t.Errorf("%s: syntax %q must stay as typed", lang, syntax)
			}
		}
		for other, w := range desc {
			if other != lang && strings.Contains(out, w) {
				t.Errorf("%s: contains the %s text %q", lang, other, w)
			}
		}
	}
}

func TestLanguagePriority(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	mustRun(t, "", "-config", cfg, "config", "set", "language", "pt-BR")
	validate := func(env string, args ...string) string {
		t.Setenv(i18n.EnvVar, env)
		return mustRun(t, "", append([]string{"-config", cfg}, append(args, "config", "validate")...)...)
	}
	// config only
	if got := validate(""); !strings.Contains(got, "a configuração é válida") {
		t.Errorf("config language: %q", got)
	}
	// $MY_AGENT_LANG beats the config
	if got := validate("es"); !strings.Contains(got, "la configuración es válida") {
		t.Errorf("env over config: %q", got)
	}
	// --lang beats both
	if got := validate("es", "--lang", "en"); !strings.Contains(got, "config is valid") {
		t.Errorf("flag over env: %q", got)
	}
	// "auto" and junk in the env fall through to the next source
	if got := validate("auto"); !strings.Contains(got, "a configuração é válida") {
		t.Errorf("auto in env: %q", got)
	}
	if got := validate("klingon"); !strings.Contains(got, "a configuração é válida") {
		t.Errorf("unsupported env value: %q", got)
	}
}

func TestUnsupportedLangFlag(t *testing.T) {
	cfg, _ := setup(t)
	code, _, errOut := run(t, "", "--lang", "klingon", "-config", cfg, "help")
	if code != 2 || !strings.Contains(errOut, "unsupported language") || !strings.Contains(errOut, "klingon") {
		t.Fatalf("%d %q", code, errOut)
	}
	// the error itself is shown in the language of the environment
	t.Setenv(i18n.EnvVar, "es")
	code, _, errOut = run(t, "", "--lang", "klingon", "help")
	if code != 2 || !strings.Contains(errOut, "idioma no compatible") {
		t.Fatalf("%d %q", code, errOut)
	}
}

func TestAliasesAreAccepted(t *testing.T) {
	for _, in := range []string{"pt", "pt_BR", "PT-br", "pt-PT"} {
		cfg, _ := setup(t)
		if out := mustRun(t, "", "--lang", in, "-config", cfg, "help"); !strings.Contains(out, "Arquivo de configuração") {
			t.Errorf("--lang %s should select Portuguese", in)
		}
	}
	cfg, _ := setup(t)
	if out := mustRun(t, "", "--lang", "es-MX", "-config", cfg, "help"); !strings.Contains(out, "Archivo de configuración") {
		t.Error("--lang es-MX should select Spanish")
	}
}

func TestInitCreatesTheDefaultPromptInTheChosenLanguage(t *testing.T) {
	cases := map[string]struct{ language, prompt string }{
		"en":    {"en", "You are a helpful assistant."},
		"pt-BR": {"pt-BR", "Você é um assistente prestativo."},
		"es":    {"es", "Eres un asistente útil."},
	}
	for lang, want := range cases {
		cfg, _ := setup(t)
		out := mustRun(t, "", "--lang", lang, "-config", cfg, "init")
		saved, err := config.Load(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if saved.Language != want.language || !strings.HasPrefix(saved.Agent.SystemPrompt, want.prompt) {
			t.Errorf("%s: language=%q prompt=%q", lang, saved.Language, saved.Agent.SystemPrompt)
		}
		created := map[string]string{"en": "created", "pt-BR": "criado", "es": "creado"}[lang]
		if !strings.Contains(out, created) {
			t.Errorf("%s: init output %q should contain %q", lang, out, created)
		}
	}
	// without --lang the language stays "auto" (detected every run)
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	if saved, _ := config.Load(cfg); saved.Language != "auto" {
		t.Errorf("default language setting: %q", saved.Language)
	}
}

func TestDefaultPromptsMatchTheEnglishDefaultConfig(t *testing.T) {
	if got := config.DefaultPrompt(i18n.EN); got != config.Default().Agent.SystemPrompt {
		t.Fatalf("Default() and the English catalog text drifted apart:\n%q\n%q", got, config.Default().Agent.SystemPrompt)
	}
}

func TestConfigSetLanguage(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	out := mustRun(t, "", "-config", cfg, "config", "set", "language", "pt")
	if !strings.Contains(out, "language atualizado") { // the change takes effect immediately: this very message is already in the new language
		t.Errorf("%q", out)
	}
	if saved, _ := config.Load(cfg); saved.Language != "pt-BR" {
		t.Fatalf("alias must be stored canonically: %q", saved.Language)
	}
	// the next run (no env override) speaks Portuguese
	t.Setenv(i18n.EnvVar, "")
	if out := mustRun(t, "", "-config", cfg, "config", "set", "rag.top_k", "5"); !strings.Contains(out, "rag.top_k atualizado") {
		t.Errorf("%q", out)
	}
	if code, _, errOut := run(t, "", "-config", cfg, "config", "set", "language", "klingon"); code != 1 || !strings.Contains(errOut, "klingon") {
		t.Fatalf("invalid language: %d %q", code, errOut)
	}
	if saved, _ := config.Load(cfg); saved.Language != "pt-BR" {
		t.Fatalf("an invalid value must not be saved: %q", saved.Language)
	}
	// "auto" is accepted
	mustRun(t, "", "-config", cfg, "config", "set", "language", "auto")
	if saved, _ := config.Load(cfg); saved.Language != "auto" {
		t.Fatalf("%q", saved.Language)
	}
}

func TestOldConfigWithoutLanguageStillWorks(t *testing.T) {
	cfg, _ := setup(t)
	c := config.Default()
	raw := strings.Replace(mustMarshal(t, c), `"language": "auto",`, "", 1)
	if strings.Contains(raw, `"language"`) {
		t.Fatal("test setup: language should be removed")
	}
	if err := os.WriteFile(cfg, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if out := mustRun(t, "", "-config", cfg, "config", "validate"); !strings.Contains(out, "config is valid") {
		t.Fatalf("a config written before the language existed must load: %q", out)
	}
}

func TestCommandOutputInEachLanguage(t *testing.T) {
	type check struct {
		args []string
		want map[string]string
	}
	checks := []check{
		{[]string{"rag", "list"}, map[string]string{
			"en": "0 source(s), 0 chunk(s)", "pt-BR": "0 fonte(s), 0 trecho(s)", "es": "0 fuente(s), 0 fragmento(s)"}},
		{[]string{"rag", "search", "x"}, map[string]string{"en": "no results", "pt-BR": "nenhum resultado", "es": "sin resultados"}},
		{[]string{"connectors", "list"}, map[string]string{
			"en": "no connectors configured", "pt-BR": "nenhum conector configurado", "es": "ningún conector configurado"}},
		{[]string{"config", "validate"}, map[string]string{
			"en": "config is valid", "pt-BR": "a configuração é válida", "es": "la configuración es válida"}},
		{[]string{"providers", "list"}, map[string]string{
			"en": "no key", "pt-BR": "sem chave", "es": "sin clave"}},
	}
	for lang := range phrase {
		for _, c := range checks {
			cfg, _ := setup(t)
			mustRun(t, "", "-config", cfg, "init")
			out := mustRun(t, "", append([]string{"--lang", lang, "-config", cfg}, c.args...)...)
			if !strings.Contains(out, c.want[lang]) {
				t.Errorf("%s %v: want %q in:\n%s", lang, c.args, c.want[lang], out)
			}
		}
	}
}

func TestErrorsInEachLanguage(t *testing.T) {
	for lang, want := range map[string]string{
		"en":    "not found; run `my-agent init` first",
		"pt-BR": "não encontrada; rode `my-agent init` primeiro",
		"es":    "no encontrada; ejecute `my-agent init` primero",
	} {
		cfg, _ := setup(t)
		code, _, errOut := run(t, "", "--lang", lang, "-config", cfg, "ask", "hi")
		if code != 1 || !strings.Contains(errOut, want) {
			t.Errorf("%s: %d %q", lang, code, errOut)
		}
	}
	// unknown command and usage errors
	for lang, want := range map[string]string{"en": "unknown command", "pt-BR": "comando desconhecido", "es": "comando desconocido"} {
		cfg, _ := setup(t)
		if code, _, e := run(t, "", "--lang", lang, "-config", cfg, "bogus"); code != 2 || !strings.Contains(e, want) {
			t.Errorf("%s: %d %q", lang, code, e)
		}
	}
	for lang, want := range map[string]string{"en": "usage:", "pt-BR": "uso:", "es": "uso:"} {
		cfg, _ := setup(t)
		mustRun(t, "", "-config", cfg, "init")
		if code, _, e := run(t, "", "--lang", lang, "-config", cfg, "rag", "remove"); code != 2 || !strings.Contains(e, want) {
			t.Errorf("%s: %d %q", lang, code, e)
		}
	}
	// errors that come from the config validation
	for lang, want := range map[string]string{
		"en":    "rag.top_k: must be > 0",
		"pt-BR": "rag.top_k: deve ser > 0",
		"es":    "rag.top_k: debe ser > 0",
	} {
		cfg, _ := setup(t)
		mustRun(t, "", "-config", cfg, "init")
		if code, _, e := run(t, "", "--lang", lang, "-config", cfg, "config", "set", "rag.top_k", "0"); code != 1 || !strings.Contains(e, want) {
			t.Errorf("%s: %d %q", lang, code, e)
		}
	}
}

func TestFlagDescriptionsAreTranslated(t *testing.T) {
	for lang, want := range map[string]string{
		"en": "number of results", "pt-BR": "número de resultados", "es": "número de resultados"} {
		cfg, _ := setup(t)
		mustRun(t, "", "-config", cfg, "init")
		_, _, errOut := run(t, "", "--lang", lang, "-config", cfg, "rag", "search", "-h")
		if !strings.Contains(errOut, want) {
			t.Errorf("%s: %q", lang, errOut)
		}
	}
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	_, _, errOut := run(t, "", "--lang", "pt-BR", "-config", cfg, "rag", "ingest", "-h")
	if !strings.Contains(errOut, "entra em subpastas") {
		t.Errorf("%q", errOut)
	}
}

func TestOfflineChatHelpAndMessagesInEachLanguage(t *testing.T) {
	for lang, want := range phrase {
		cfg, _ := setup(t)
		mustRun(t, "", "-config", cfg, "init")
		out := mustRun(t, "/reset\n/help\n/exit\n", "--lang", lang, "-config", cfg, "chat", "--no-rag")
		if !strings.Contains(out, want.cleared) || !strings.Contains(out, want.shortcuts) {
			t.Errorf("%s: chat output is missing %q / %q:\n%s", lang, want.cleared, want.shortcuts, out)
		}
	}
}

func TestChatCommandDescriptionsFollowTheLanguage(t *testing.T) {
	for lang, want := range map[string]string{
		"en":    "erases the conversation history",
		"pt-BR": "apaga o histórico da conversa",
		"es":    "borra el historial de la conversación",
	} {
		cfg, _ := setup(t)
		mustRun(t, "", "-config", cfg, "init")
		out := mustRun(t, "/help chat\n/exit\n", "--lang", lang, "-config", cfg, "chat", "--no-rag")
		if !strings.Contains(out, want) {
			t.Errorf("%s: %s", lang, out)
		}
	}
}

func TestSwitchLanguageInsideTheChat(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	stdin := strings.Join([]string{
		"/reset",                 // english
		"/config language pt-BR", // → portuguese, saved
		"/reset",                 // histórico apagado
		"/help chat",             // the tree is redrawn in Portuguese
		"/lang es",               // the /lang shortcut
		"/reset",                 // historial borrado
		"/config language",       // shows the current setting
		"/config language en",
		"/reset", // english again
		"/exit",
	}, "\n") + "\n"
	code, out, errOut := run(t, stdin, "-config", cfg, "chat", "--no-rag")
	if code != 0 || errOut != "" {
		t.Fatalf("%d %s", code, errOut)
	}
	order := []string{
		"history cleared",
		"idioma definido como pt-BR", "histórico apagado", "apaga o histórico da conversa",
		"idioma establecido: es", "historial borrado",
		"language = es (es)",
		"language set to en", "history cleared",
	}
	pos := 0
	for _, w := range order {
		i := strings.Index(out[pos:], w)
		if i < 0 {
			t.Fatalf("expected %q (in this order) after position %d:\n%s", w, pos, out)
		}
		pos += i + len(w)
	}
	if saved, _ := config.Load(cfg); saved.Language != "en" {
		t.Fatalf("the last choice must be saved: %q", saved.Language)
	}
}

func TestInvalidLanguageInsideTheChatChangesNothing(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	_, out, errOut := run(t, "/config language klingon\n/reset\n/exit\n", "-config", cfg, "chat", "--no-rag")
	if !strings.Contains(errOut, "klingon") || !strings.Contains(out, "history cleared") {
		t.Fatalf("stdout=%q stderr=%q", out, errOut)
	}
	if saved, _ := config.Load(cfg); saved.Language != "auto" {
		t.Fatalf("%q", saved.Language)
	}
}

func TestYesNoWordsOfEveryLanguageWorkForOnOff(t *testing.T) {
	for _, word := range []string{"on", "true", "yes", "sim", "sí", "si"} {
		if v, ok := parseOnOff(word); !ok || v != "true" {
			t.Errorf("%q should be true", word)
		}
	}
	for _, word := range []string{"off", "false", "no", "não", "nao"} {
		if v, ok := parseOnOff(word); !ok || v != "false" {
			t.Errorf("%q should be false", word)
		}
	}
	for _, word := range []string{"maybe", "talvez", "quizás", ""} {
		if _, ok := parseOnOff(word); ok {
			t.Errorf("%q must not be accepted", word)
		}
	}
}

func TestMentionsMessagesAndTheHeaderSentToTheModelFollowTheLanguage(t *testing.T) {
	for lang, want := range map[string]struct{ header, warn string }{
		"en":    {"[Attached file: n.txt]", "file not found"},
		"pt-BR": {"[Arquivo anexado: n.txt]", "arquivo não encontrado"},
		"es":    {"[Archivo adjunto: n.txt]", "archivo no encontrado"},
	} {
		dir := t.TempDir()
		inDir(t, dir)
		writeFile(t, "n.txt", "conteúdo")
		cfg, fp := setup(t)
		mustRun(t, "", "-config", cfg, "init")
		out := mustRun(t, "veja @n.txt e @nada.md\n/exit\n", "--lang", lang, "-config", cfg, "chat", "--no-rag")
		sent := fp.last.Messages[len(fp.last.Messages)-1].Content
		if !strings.Contains(sent, want.header) || !strings.Contains(out, want.warn) {
			t.Errorf("%s: header %q / warning %q\nsent: %s\nout: %s", lang, want.header, want.warn, sent, out)
		}
	}
}

func mustMarshal(t *testing.T, c config.Config) string {
	t.Helper()
	dir := t.TempDir()
	p := dir + "/c.json"
	if err := c.Save(p); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
