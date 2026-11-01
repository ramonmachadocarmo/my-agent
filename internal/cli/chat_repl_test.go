package cli

import (
	"strings"
	"testing"

	"my-agent/internal/config"
)

// chatRun runs `my-agent chat` with the given lines typed, returning
// stdout, stderr and the fake provider.
func chatRun(t *testing.T, lines ...string) (out, errOut string, cfgPath string, fp *fakeProvider) {
	t.Helper()
	cfg, fp := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	stdin := strings.Join(lines, "\n") + "\n/exit\n"
	code, o, e := run(t, stdin, "-config", cfg, "chat", "--no-rag")
	if code != 0 {
		t.Fatalf("chat exited %d\nstdout: %s\nstderr: %s", code, o, e)
	}
	return o, e, cfg, fp
}

func TestChatConfigChangesApplyToTheNextMessage(t *testing.T) {
	out, errOut, cfg, fp := chatRun(t,
		"/config chat temperature 0.9",
		"/config chat max-tokens 321",
		`/config chat prompt "Você é um pirata"`,
		"oi",
	)
	if errOut != "" {
		t.Fatalf("unexpected errors: %s", errOut)
	}
	if fp.last.Temperature != 0.9 || fp.last.MaxTokens != 321 || fp.last.System != "Você é um pirata" {
		t.Fatalf("the new settings must reach the very next request: %+v", fp.last)
	}
	for _, want := range []string{"agent.temperature = 0.9", "agent.max_tokens = 321", "agent.system_prompt = Você é um pirata"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing confirmation %q:\n%s", want, out)
		}
	}
	saved, err := config.Load(cfg)
	if err != nil || saved.Agent.Temperature != 0.9 || saved.Agent.MaxTokens != 321 || saved.Agent.SystemPrompt != "Você é um pirata" {
		t.Fatalf("changes must be saved to the file: %+v %v", saved.Agent, err)
	}
}

func TestChatPromptWithoutQuotesTakesTheWholeSentence(t *testing.T) {
	_, _, _, fp := chatRun(t, "/config chat prompt Responda sempre em português, com don't e tudo", "oi")
	if fp.last.System != "Responda sempre em português, com don't e tudo" {
		t.Fatalf("system = %q", fp.last.System)
	}
}

func TestChatShowsCurrentValueWhenNoneGiven(t *testing.T) {
	out, _, _, _ := chatRun(t, "/config chat temperature", "/config rag top-k", "/config chat rag", "/config chat model")
	for _, want := range []string{"agent.temperature = 0.2", "rag.top_k = 4", "agent.use_rag = true", "providers.ollama.model = llama3.2"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestChatInvalidValuesAreRejectedAndNothingChanges(t *testing.T) {
	_, errOut, cfg, fp := chatRun(t,
		"/config chat temperature 9",   // out of range
		"/config chat temperature abc", // not a number
		"/config chat rag maybe",       // not on/off
		"/config chat provider ghost",  // unknown provider
		"/config rag top-k 0",          // invalid
		"/config chat max-tokens 1 2",  // too many arguments
		"oi",                           // the chat is still alive afterwards
	)
	for _, want := range []string{"agent.temperature", "abc", "on|off", `provider "ghost" does not exist`, "rag.top_k", "usage:"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr is missing %q:\n%s", want, errOut)
		}
	}
	saved, _ := config.Load(cfg)
	def := config.Default()
	if saved.Agent.Temperature != def.Agent.Temperature || saved.RAG.TopK != def.RAG.TopK || saved.Agent.Provider != def.Agent.Provider {
		t.Fatalf("invalid changes must not be saved: %+v %+v", saved.Agent, saved.RAG)
	}
	if fp.last.Temperature != def.Agent.Temperature {
		t.Fatalf("the next message must still use the old settings, got %v", fp.last.Temperature)
	}
}

func TestChatSwitchProviders(t *testing.T) {
	out, _, cfg, _ := chatRun(t,
		"/providers use gemini", "/model",
		"/config chat provider claude", "/model",
		"/config chat model claude-opus-5", "/model",
	)
	for _, want := range []string{"using gemini · gemini-2.5-flash", "gemini · gemini-2.5-flash", "claude · claude-sonnet-5", "providers.claude.model = claude-opus-5", "claude · claude-opus-5"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	saved, _ := config.Load(cfg)
	if saved.Agent.Provider != "claude" || saved.Providers["claude"].Model != "claude-opus-5" {
		t.Fatalf("/config chat provider/model persist, /providers use does not: %+v", saved.Agent)
	}
}

func TestChatProvidersUseIsNotPersisted(t *testing.T) {
	_, _, cfg, _ := chatRun(t, "/providers use gemini")
	if saved, _ := config.Load(cfg); saved.Agent.Provider != "ollama" {
		t.Fatalf("/providers use is for this conversation only, saved provider = %q", saved.Agent.Provider)
	}
}

func TestChatModelsUseDownloadsAndSwitches(t *testing.T) {
	out, errOut, cfg, fp := chatRun(t, "/models use qwen2.5:7b", "/model", "/models list")
	if errOut != "" {
		t.Fatalf("errors: %s", errOut)
	}
	if len(fp.pulled) != 1 || fp.pulled[0] != "qwen2.5:7b" {
		t.Fatalf("pulled = %v", fp.pulled)
	}
	for _, want := range []string{"downloaded qwen2.5:7b", "providers.ollama.model = qwen2.5:7b", "ollama · qwen2.5:7b", "qwen2.5:7b:latest"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if saved, _ := config.Load(cfg); saved.Providers["ollama"].Model != "qwen2.5:7b" {
		t.Fatalf("model not saved: %q", saved.Providers["ollama"].Model)
	}
}

func TestChatModelsSelectReadsTheChoiceFromTheSameInput(t *testing.T) {
	cfg, fp := setup(t)
	fp.models = nil
	mustRun(t, "", "-config", cfg, "init")
	// after "/models select" the next line is the answer to the menu, not a chat message
	code, out, errOut := run(t, "/models select\nphi4\n/model\n/exit\n", "-config", cfg, "chat", "--no-rag")
	if code != 0 || errOut != "" {
		t.Fatalf("%d %s", code, errOut)
	}
	if !strings.Contains(out, "No models installed") || !strings.Contains(out, "ollama · phi4") {
		t.Fatalf("out:\n%s", out)
	}
	if fp.last.Messages != nil {
		t.Fatalf("the menu answer must not be sent to the model: %+v", fp.last)
	}
}

func TestChatRagIngestIsUsableInTheSameConversation(t *testing.T) {
	cfg, fp := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	docs := t.TempDir()
	writeFile(t, docs+"/pinguins.md", "Pinguins-imperador vivem na Antártida e chocam os ovos nos pés.")
	q := "onde vivem os pinguins-imperador?"
	stdin := strings.Join([]string{
		q, // 0: nothing indexed yet
		"/rag ingest " + docs,
		q, // 1: same conversation, now grounded
		"/config chat rag off",
		q, // 2: RAG switched off again
		"/config chat rag on",
		q, // 3: and on
		"/exit",
	}, "\n") + "\n"
	code, out, errOut := run(t, stdin, "-config", cfg, "chat")
	if code != 0 || errOut != "" {
		t.Fatalf("%d %s", code, errOut)
	}
	if !strings.Contains(out, "1 document(s)") || len(fp.all) != 4 {
		t.Fatalf("requests=%d\n%s", len(fp.all), out)
	}
	grounded := func(i int) bool { return strings.Contains(fp.all[i].System, "Antártida") }
	if grounded(0) || !grounded(1) || grounded(2) || !grounded(3) {
		t.Fatalf("context per message: %v %v %v %v (want false true false true)", grounded(0), grounded(1), grounded(2), grounded(3))
	}
}

func TestChatRagCommandsAndConnectors(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	docs := t.TempDir()
	writeFile(t, docs+"/a.md", "conteúdo sobre baleias azuis")
	stdin := strings.Join([]string{
		"/rag ingest " + docs,
		"/rag list",
		"/rag search baleias",
		"/connectors add --name kb --type file --path " + docs,
		"/connectors list",
		"/connectors sync kb",
		"/rag clear",
		"/rag list",
		"/connectors remove kb",
		"/exit",
	}, "\n") + "\n"
	code, out, errOut := run(t, stdin, "-config", cfg, "chat", "--no-rag")
	if code != 0 || errOut != "" {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, want := range []string{"1 source(s)", "baleias azuis", `kb`, "kb: 1 document(s)", "0 source(s)", `connector "kb" removed`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestChatConfigSubcommands(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	stdin := strings.Join([]string{
		"/config path", "/config get rag.top_k", "/config set rag.top_k 9", "/config get rag.top_k",
		"/config validate", "/config show",
		"/config provider ollama", "/config provider ollama num_ctx", "/config provider ollama num_ctx 2048", "/config provider ollama num_ctx",
		"/config provider ollama keep_alive 1h",
		"/exit",
	}, "\n") + "\n"
	code, out, errOut := run(t, stdin, "-config", cfg, "chat", "--no-rag")
	if code != 0 || errOut != "" {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, want := range []string{cfg, "you> 4\n", "rag.top_k updated", "you> 9\n", "config is valid", `"agent"`, `"embedding_model": "nomic-embed-text"`,
		"providers.ollama.num_ctx = 0", "providers.ollama.num_ctx = 2048", "providers.ollama.keep_alive = 1h"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	saved, _ := config.Load(cfg)
	if saved.RAG.TopK != 9 || saved.Providers["ollama"].NumCtx != 2048 || saved.Providers["ollama"].KeepAlive != "1h" {
		t.Fatalf("not saved: %+v %+v", saved.RAG, saved.Providers["ollama"])
	}
}

func TestChatNeverEchoesOrStoresAKeyPastedIntoApiKeyEnv(t *testing.T) {
	const pasted = "AQ.Ab8RN6-not-a-var-name"
	out, errOut, cfg, _ := chatRun(t, "/config provider gemini api_key_env "+pasted)
	if strings.Contains(out+errOut, pasted) {
		t.Fatalf("the value must never be printed:\nstdout: %s\nstderr: %s", out, errOut)
	}
	if !strings.Contains(errOut, "NAME of an environment variable") {
		t.Fatalf("the error should explain the mistake:\n%s", errOut)
	}
	if saved, _ := config.Load(cfg); saved.Providers["gemini"].APIKeyEnv != "GEMINI_API_KEY" {
		t.Fatalf("the invalid value must not be saved: %q", saved.Providers["gemini"].APIKeyEnv)
	}
	// a proper variable name is accepted
	out, _, cfg, _ = chatRun(t, "/config provider gemini api_key_env MINHA_CHAVE")
	if !strings.Contains(out, "providers.gemini.api_key_env = MINHA_CHAVE") {
		t.Fatalf("%s", out)
	}
	if saved, _ := config.Load(cfg); saved.Providers["gemini"].APIKeyEnv != "MINHA_CHAVE" {
		t.Fatal("not saved")
	}
}

func TestChatApiKeyPromptDoesNotEchoTheSecret(t *testing.T) {
	const secret = "TOPSECRET-123-xyz"
	out, errOut, cfg, _ := chatRun(t,
		"/config provider gemini api_key", // no value: asks for it
		secret,
		"/config show",
		"/config get providers.gemini.api_key",
		"/config provider gemini",
	)
	if strings.Contains(out+errOut, secret) {
		t.Fatalf("the key leaked into the output:\nstdout: %s\nstderr: %s", out, errOut)
	}
	if !strings.Contains(out, "providers.gemini.api_key saved (hidden)") || !strings.Contains(out, config.MaskedKey) {
		t.Fatalf("expected a confirmation without the value and masked displays:\n%s", out)
	}
	if saved, _ := config.Load(cfg); saved.Providers["gemini"].APIKey != secret {
		t.Fatal("the key must be saved to the config")
	}
}

func TestChatInlineApiKeyWarnsAboutHistory(t *testing.T) {
	out, _, cfg, _ := chatRun(t, "/config provider gemini api_key SEGREDO-INLINE")
	if !strings.Contains(out, "terminal history") || strings.Contains(out, "SEGREDO-INLINE") {
		t.Fatalf("should warn and not echo:\n%s", out)
	}
	if saved, _ := config.Load(cfg); saved.Providers["gemini"].APIKey != "SEGREDO-INLINE" {
		t.Fatal("still saved")
	}
}

func TestChatSlashCommandErrorsDoNotEndTheChat(t *testing.T) {
	out, errOut, _, fp := chatRun(t, "/naoexiste", "/co", "/rag", "/rag remove", "/rag frob", "/models use", "olá depois dos erros")
	for _, want := range []string{"unknown command", "ambiguous"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr is missing %q:\n%s", want, errOut)
		}
	}
	if !strings.Contains(out, "1.    config") && !strings.Contains(out, "2.    rag") && !strings.Contains(out, "rag") {
		t.Errorf("/rag alone should print the branch help:\n%s", out)
	}
	if !strings.Contains(out, "agent> echo: olá depois dos erros") || fp.last.Messages[len(fp.last.Messages)-1].Content != "olá depois dos erros" {
		t.Fatalf("the chat must keep working:\n%s", out)
	}
}

func TestChatHelpVariants(t *testing.T) {
	for _, in := range []string{"help", "?", "/", "/help", "/he"} {
		out, _, _, _ := chatRun(t, in)
		if !strings.Contains(out, "Commands") || !strings.Contains(out, "1.1   chat") {
			t.Errorf("%q should print the help:\n%s", in, out)
		}
	}
	out, _, _, _ := chatRun(t, "/config", "/help config chat")
	if strings.Count(out, "temperature") < 2 {
		t.Errorf("/config and /help config chat must print branches:\n%s", out)
	}
}

func TestChatExitCommands(t *testing.T) {
	for _, in := range []string{"/exit", "/quit", "/chat exit"} {
		cfg, _ := setup(t)
		mustRun(t, "", "-config", cfg, "init")
		code, out, _ := run(t, in+"\nsecond\n", "-config", cfg, "chat", "--no-rag")
		if code != 0 || strings.Contains(out, "echo: second") {
			t.Errorf("%q must end the chat before the next line:\n%s", in, out)
		}
	}
}

func TestChatMentionsAttachFilesButSearchOnlyTheQuestion(t *testing.T) {
	inDir(t, t.TempDir())
	writeFile(t, "notas.txt", "O código secreto é ABACAXI-42.")
	out, errOut, _, fp := chatRun(t, "qual é o código em @notas.txt ?", "e @nao-existe.txt?")
	sent := fp.last.Messages
	// second message: missing file, warned about, but the message is still sent
	if !strings.Contains(errOut, "") || !strings.Contains(out, "warning: @nao-existe.txt: file not found") {
		t.Fatalf("expected a warning for the missing file:\n%s", out)
	}
	if got := sent[len(sent)-1].Content; got != "e @nao-existe.txt?" {
		t.Fatalf("second message = %q", got)
	}
	// first message carried the file, and the history keeps it for follow-ups
	first := sent[0].Content
	for _, want := range []string{"qual é o código em @notas.txt ?", "[Attached file: notas.txt]", "ABACAXI-42"} {
		if !strings.Contains(first, want) {
			t.Errorf("first message is missing %q:\n%s", want, first)
		}
	}
	if !strings.Contains(out, "📎 attached: notas.txt") {
		t.Errorf("the user should see what was attached:\n%s", out)
	}
}

func TestChatCommandsSurviveAcrossTheConversationHistory(t *testing.T) {
	_, _, _, fp := chatRun(t, "primeira", "/config chat temperature 0.7", "segunda")
	if n := len(fp.last.Messages); n != 3 { // primeira, resposta, segunda
		t.Fatalf("changing settings must keep the conversation (%d messages sent)", n)
	}
	_, _, _, fp = chatRun(t, "primeira", "/reset", "segunda")
	if n := len(fp.last.Messages); n != 1 {
		t.Fatalf("/reset clears it (%d messages sent)", n)
	}
}

func TestChatShortcutsAndGroupedConversationCommands(t *testing.T) {
	out, _, _, _ := chatRun(t, "/sources", "/chat sources", "/chat model", "/clear")
	if strings.Count(out, "show sources:") != 2 || !strings.Contains(out, "ollama · llama3.2") {
		t.Fatalf("%s", out)
	}
}

func TestChatRagEmbedderChangeWarnsWhenTheIndexIsStale(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	docs := t.TempDir()
	writeFile(t, docs+"/a.md", "algum conteúdo")
	stdin := "/rag ingest " + docs + "\n/config rag embedder gemini\n/config rag chunk-size 500\n/exit\n"
	code, out, errOut := run(t, stdin, "-config", cfg, "chat", "--no-rag")
	if code != 0 || errOut != "" {
		t.Fatalf("%d %s", code, errOut)
	}
	if !strings.Contains(out, "note: the current index was built by another embedder") || !strings.Contains(out, "note: applies to new ingestions") {
		t.Fatalf("expected notes about side effects:\n%s", out)
	}
}
