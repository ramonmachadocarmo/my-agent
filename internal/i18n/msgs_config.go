package i18n

func init() {
	add(map[string]m{
		"config.default_prompt": {
			"You are a helpful assistant. Answer concisely and say when you do not know.",
			"Você é um assistente prestativo. Responda de forma concisa e diga quando não souber.",
			"Eres un asistente útil. Responde de forma concisa y di cuando no lo sepas.",
		},
		"config.err.invalid": {"invalid config: %w", "configuração inválida: %w", "configuración inválida: %w"},
		"config.err.providers_required": {
			"providers: at least one provider is required",
			"providers: é necessário pelo menos um provedor",
			"providers: se necesita al menos un proveedor",
		},
		"config.err.provider_type": {
			"providers.%s.type: %q is not one of claude, gemini, ollama, openai",
			"providers.%s.type: %q não é claude, gemini, ollama nem openai",
			"providers.%s.type: %q no es claude, gemini, ollama ni openai",
		},
		"config.err.required":     {"%s: required", "%s: obrigatório", "%s: obligatorio"},
		"config.err.non_negative": {"%s: must be >= 0", "%s: deve ser >= 0", "%s: debe ser >= 0"},
		"config.err.positive":     {"%s: must be > 0", "%s: deve ser > 0", "%s: debe ser > 0"},
		"config.err.between": {
			"%s: must be between %v and %v",
			"%s: deve estar entre %v e %v",
			"%s: debe estar entre %v y %v",
		},
		"config.err.keep_alive": {
			"providers.%s.keep_alive: %q is not a duration like 30m, seconds, or -1",
			"providers.%s.keep_alive: %q não é uma duração como 30m, segundos ou -1",
			"providers.%s.keep_alive: %q no es una duración como 30m, segundos o -1",
		},
		"config.err.agent_provider": {
			"agent.provider: %q is not a configured provider",
			"agent.provider: %q não é um provedor configurado",
			"agent.provider: %q no es un proveedor configurado",
		},
		"config.err.embedder": {
			"rag.embedder: must be %q or a configured provider, got %q",
			"rag.embedder: deve ser %q ou um provedor configurado, mas recebi %q",
			"rag.embedder: debe ser %q o un proveedor configurado, pero se recibió %q",
		},
		"config.err.overlap": {
			"rag.chunk_overlap: must be >= 0 and < chunk_size",
			"rag.chunk_overlap: deve ser >= 0 e < chunk_size",
			"rag.chunk_overlap: debe ser >= 0 y < chunk_size",
		},
		"config.err.conn_dup": {
			"connectors[%d].name: duplicate %q",
			"connectors[%d].name: %q duplicado",
			"connectors[%d].name: %q duplicado",
		},
		"config.err.conn_path": {
			"connectors[%d].path: required for type file",
			"connectors[%d].path: obrigatório para o tipo file",
			"connectors[%d].path: obligatorio para el tipo file",
		},
		"config.err.conn_url": {
			"connectors[%d].url: must start with http:// or https://",
			"connectors[%d].url: deve começar com http:// ou https://",
			"connectors[%d].url: debe empezar con http:// o https://",
		},
		"config.err.conn_type": {
			"connectors[%d].type: %q is not one of file, url, database, email",
			"connectors[%d].type: %q não é file, url, database nem email",
			"connectors[%d].type: %q no es file, url, database ni email",
		},
		"config.err.language": {
			"language: %q is not supported (use auto, en, pt-BR or es)",
			"language: %q não é suportado (use auto, en, pt-BR ou es)",
			"language: %q no es compatible (use auto, en, pt-BR o es)",
		},
		"config.err.unknown_key": {"unknown key %q", "chave desconhecida %q", "clave desconocida %q"},
		"config.err.not_number": {
			"%s: %q is not a number",
			"%s: %q não é um número",
			"%s: %q no es un número",
		},
		"config.err.not_bool": {
			"%s: %q is not a boolean",
			"%s: %q não é um booleano (true/false)",
			"%s: %q no es un booleano (true/false)",
		},
		"config.err.not_settable": {
			"%s cannot be set directly (use the dedicated command)",
			"%s não pode ser alterada diretamente (use o comando específico)",
			"%s no se puede cambiar directamente (use el comando específico)",
		},
		"config.warn.api_key_env": {
			"providers.%[1]s.api_key_env is not a valid environment variable name; it looks like an API key pasted in the wrong field. " +
				"Put the key in providers.%[1]s.api_key, or set api_key_env to a variable name (for example GEMINI_API_KEY) and define that variable.",
			"providers.%[1]s.api_key_env não é um nome válido de variável de ambiente; parece uma chave de API colada no campo errado. " +
				"Coloque a chave em providers.%[1]s.api_key, ou use em api_key_env o nome de uma variável (por exemplo GEMINI_API_KEY) e defina essa variável.",
			"providers.%[1]s.api_key_env no es un nombre válido de variable de entorno; parece una clave de API pegada en el campo equivocado. " +
				"Ponga la clave en providers.%[1]s.api_key, o use en api_key_env el nombre de una variable (por ejemplo GEMINI_API_KEY) y defina esa variable.",
		},
	})
}
