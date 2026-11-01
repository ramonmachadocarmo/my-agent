package i18n

// Messages of the agents, tools, connectors (MCP, database, e-mail) and the
// configuration sections that support them.
func init() {
	add(map[string]m{
		// config validation
		"config.err.unknown_agent": {
			"unknown agent %q",
			"agente desconhecido %q",
			"agente desconocido %q",
		},
		"config.err.agent_provider_of": {
			"agents.%s.provider: provider %q is not defined in providers",
			"agents.%s.provider: o provedor %q não está definido em providers",
			"agents.%s.provider: el proveedor %q no está definido en providers",
		},
		"config.err.permission": {
			"%s: invalid permission %q (use allow, ask or deny)",
			"%s: permissão inválida %q (use allow, ask ou deny)",
			"%s: permiso no válido %q (use allow, ask o deny)",
		},
		"config.err.mcp_url": {
			"mcp.%s.url must start with http:// or https://",
			"mcp.%s.url deve começar com http:// ou https://",
			"mcp.%s.url debe empezar con http:// o https://",
		},
		"config.err.mcp_type": {
			"mcp.%s.type: unknown type %q (use local or remote)",
			"mcp.%s.type: tipo desconhecido %q (use local ou remote)",
			"mcp.%s.type: tipo desconocido %q (use local o remote)",
		},
		"config.err.theme": {
			"theme: unknown value %q (use auto, dark or light)",
			"theme: valor desconhecido %q (use auto, dark ou light)",
			"theme: valor desconocido %q (use auto, dark o light)",
		},
		"config.err.db_driver": {
			"connectors[%d].database.driver: unknown driver %q (use sqlite, postgres or mysql)",
			"connectors[%d].database.driver: driver desconhecido %q (use sqlite, postgres ou mysql)",
			"connectors[%d].database.driver: driver desconocido %q (use sqlite, postgres o mysql)",
		},
		// command line
		"cli.usage.tui": {
			"open the interactive agent (full screen)",
			"abre o agente interativo (tela cheia)",
			"abre el agente interactivo (pantalla completa)",
		},
		"cli.usage.run": {
			"run the agent with tools once and print the reply",
			"executa o agente com ferramentas uma vez e imprime a resposta",
			"ejecuta el agente con herramientas una vez e imprime la respuesta",
		},
		"cli.usage.agents":   {"list the agents", "lista os agentes", "lista los agentes"},
		"cli.usage.mcp":      {"connect to the MCP servers and list their tools", "conecta aos servidores MCP e lista suas ferramentas", "conecta con los servidores MCP y lista sus herramientas"},
		"cli.usage.sessions": {"list or remove saved sessions", "lista ou remove sessões salvas", "lista o elimina sesiones guardadas"},
		"cli.flag.agent":     {"agent to use (build, plan, chat or one of yours)", "agente a usar (build, plan, chat ou um seu)", "agente a usar (build, plan, chat o uno suyo)"},
		"cli.flag.model":     {"model to use, as provider/model", "modelo a usar, como provedor/modelo", "modelo a usar, como proveedor/modelo"},
		"cli.flag.session":   {"resume this session", "retoma esta sessão", "reanuda esta sesión"},
		"cli.flag.continue":  {"resume the latest session of this folder", "retoma a última sessão desta pasta", "reanuda la última sesión de esta carpeta"},
		"cli.flag.yes":       {"allow every tool call that would ask for permission", "permite toda chamada de ferramenta que pediria permissão", "permite toda llamada a herramienta que pediría permiso"},
		"cli.flag.preset":    {"known provider to copy the settings from", "provedor conhecido de onde copiar as configurações", "proveedor conocido del que copiar la configuración"},
		"cli.flag.key":       {"API key (added to the provider's keys)", "chave de API (adicionada às chaves do provedor)", "clave de API (añadida a las claves del proveedor)"},
		"cli.flag.base_url":  {"base URL of the API", "URL base da API", "URL base de la API"},
		"cli.providers.keys_set": {
			"%d keys, rotated",
			"%d chaves, em rodízio",
			"%d claves, en rotación",
		},
		"cli.tui.created_config": {
			"created the configuration %s (edit it with /config or /connect)",
			"configuração criada em %s (edite com /config ou /connect)",
			"configuración creada en %s (edítela con /config o /connect)",
		},
		"cli.tui.no_terminal": {
			"the interactive interface needs a terminal; use \"my-agent run MESSAGE\" in scripts",
			"a interface interativa precisa de um terminal; use \"my-agent run MENSAGEM\" em scripts",
			"la interfaz interactiva necesita una terminal; use \"my-agent run MENSAJE\" en scripts",
		},
		"cli.run.denied": {
			"denied %s: it needs permission (use --yes to allow)",
			"%s negado: precisa de permissão (use --yes para permitir)",
			"%s denegado: necesita permiso (use --yes para permitir)",
		},
		"cli.mcp.failed": {
			"%d MCP server(s) failed",
			"%d servidor(es) MCP falharam",
			"%d servidor(es) MCP fallaron",
		},
		"cli.sessions.none": {"no saved sessions", "nenhuma sessão salva", "ninguna sesión guardada"},

		"provider.err.ollama_no_model": {
			"%s: the model %q is not installed in Ollama. Choose an installed one with ctrl+o (or \"my-agent models select\"), or download it: ollama pull %s",
			"%s: o modelo %q não está instalado no Ollama. Escolha um instalado com ctrl+o (ou \"my-agent models select\"), ou baixe: ollama pull %s",
			"%s: el modelo %q no está instalado en Ollama. Elija uno instalado con ctrl+o (o \"my-agent models select\"), o descárguelo: ollama pull %s",
		},
		"cli.tui.ollama_model": {
			"using the Ollama model %s (installed on this machine)",
			"usando o modelo do Ollama %s (instalado nesta máquina)",
			"usando el modelo de Ollama %s (instalado en este equipo)",
		},

		// agent loop
		"agent.err.denied": {
			"tool call denied",
			"chamada de ferramenta negada",
			"llamada a herramienta denegada",
		},
		"agent.err.max_steps": {
			"the agent reached its step limit (max_steps); send a message to continue",
			"o agente atingiu o limite de passos (max_steps); envie uma mensagem para continuar",
			"el agente alcanzó su límite de pasos (max_steps); envíe un mensaje para continuar",
		},

		// connectors
		"connectors.err.missing_section": {
			"connector %s: the %q section is missing",
			"conector %s: falta a seção %q",
			"conector %s: falta la sección %q",
		},
		"connectors.err.db_no_dsn": {
			"database connector has no DSN (set dsn or dsn_env)",
			"o conector de banco não tem DSN (defina dsn ou dsn_env)",
			"el conector de base de datos no tiene DSN (defina dsn o dsn_env)",
		},
		"connectors.err.db_read_only": {
			"this database connector is read-only (set allow_write to true to change data)",
			"este conector de banco é somente leitura (defina allow_write como true para alterar dados)",
			"este conector de base de datos es de solo lectura (defina allow_write en true para cambiar datos)",
		},
		"connectors.err.db_no_query": {
			"connector %s: set database.query to choose the rows to ingest",
			"conector %s: defina database.query para escolher as linhas a ingerir",
			"conector %s: defina database.query para elegir las filas a ingerir",
		},
		"connectors.err.no_imap": {
			"e-mail connector has no imap_host",
			"o conector de e-mail não tem imap_host",
			"el conector de correo no tiene imap_host",
		},
		"connectors.err.no_smtp": {
			"e-mail connector has no smtp_host",
			"o conector de e-mail não tem smtp_host",
			"el conector de correo no tiene smtp_host",
		},
		"connectors.err.no_recipient": {
			"no recipient given",
			"nenhum destinatário informado",
			"no se indicó ningún destinatario",
		},
		"connectors.err.no_message": {
			"message uid %d not found",
			"mensagem uid %d não encontrada",
			"mensaje uid %d no encontrado",
		},
		"config.agent.builtin.build": {
			"Full access: reads, edits files and runs commands",
			"Acesso total: lê, edita arquivos e executa comandos",
			"Acceso total: lee, edita archivos y ejecuta comandos",
		},
		"config.agent.builtin.plan": {
			"Read-only: analyzes and plans without changing anything",
			"Somente leitura: analisa e planeja sem alterar nada",
			"Solo lectura: analiza y planifica sin cambiar nada",
		},
		"config.agent.builtin.chat": {
			"Conversation with the knowledge base, no tools",
			"Conversa com a base de conhecimento, sem ferramentas",
			"Conversación con la base de conocimiento, sin herramientas",
		},
	})
}
