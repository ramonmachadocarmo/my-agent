package i18n

// Messages of the command line (everything except the interactive chat).
func init() {
	add(map[string]m{
		// ---- usage screen -------------------------------------------------
		"cli.usage.header": {
			"my-agent - configurable AI agent for the terminal (Claude, Gemini, Ollama, OpenAI-compatible) with tools, MCP, connectors and RAG",
			"my-agent - agente de IA configurável para o terminal (Claude, Gemini, Ollama, compatíveis com OpenAI) com ferramentas, MCP, conectores e RAG",
			"my-agent - agente de IA configurable para la terminal (Claude, Gemini, Ollama, compatibles con OpenAI) con herramientas, MCP, conectores y RAG",
		},
		"cli.usage.synopsis": {
			"Usage: my-agent [--config FILE] [--lang LANG] [<command> [args]]   (no command: interactive agent)",
			"Uso: my-agent [--config ARQUIVO] [--lang IDIOMA] [<comando> [args]]   (sem comando: agente interativo)",
			"Uso: my-agent [--config ARCHIVO] [--lang IDIOMA] [<comando> [args]]   (sin comando: agente interactivo)",
		},
		"cli.usage.commands": {"Commands:", "Comandos:", "Comandos:"},
		"cli.usage.init": {
			"create a default config file",
			"cria um arquivo de configuração padrão",
			"crea un archivo de configuración predeterminado",
		},
		"cli.usage.config": {
			"inspect and edit the configuration",
			"vê e edita a configuração",
			"ve y edita la configuración",
		},
		"cli.usage.providers": {
			"list providers / check connectivity",
			"lista os provedores / testa a conexão",
			"lista los proveedores / prueba la conexión",
		},
		"cli.usage.chat": {
			"interactive chat (type / for commands, @file to attach)",
			"chat interativo (digite / para comandos, @arquivo para anexar)",
			"chat interactivo (escriba / para comandos, @archivo para adjuntar)",
		},
		"cli.usage.rag": {
			"manage the knowledge base",
			"gerencia a base de conhecimento",
			"gestiona la base de conocimiento",
		},
		"cli.usage.models": {
			"manage Ollama models (download, configure, GPU status)",
			"gerencia modelos do Ollama (baixar, configurar, status da GPU)",
			"gestiona modelos de Ollama (descargar, configurar, estado de la GPU)",
		},
		"cli.usage.connectors": {
			"manage data connectors",
			"gerencia conectores de dados",
			"gestiona conectores de datos",
		},
		"cli.usage.serve": {"start the web UI", "inicia a interface web", "inicia la interfaz web"},
		"cli.usage.completion": {
			"print a shell completion script",
			"imprime um script de autocompletar",
			"imprime un script de autocompletado",
		},
		"cli.usage.config_file": {
			"Config file: --config, else $MY_AGENT_CONFIG, else ./my-agent.json, else the per-user file (AppData/Roaming/my-agent or ~/.config/my-agent)",
			"Arquivo de configuração: --config, senão $MY_AGENT_CONFIG, senão ./my-agent.json, senão o arquivo do usuário (AppData/Roaming/my-agent ou ~/.config/my-agent)",
			"Archivo de configuración: --config, si no $MY_AGENT_CONFIG, si no ./my-agent.json, si no el archivo del usuario (AppData/Roaming/my-agent o ~/.config/my-agent)",
		},
		"cli.usage.language": {
			"Language: --lang, else $MY_AGENT_LANG, else \"language\" in the config, else the system's (en, pt-BR, es)",
			"Idioma: --lang, senão $MY_AGENT_LANG, senão \"language\" da configuração, senão o do sistema (en, pt-BR, es)",
			"Idioma: --lang, si no $MY_AGENT_LANG, si no \"language\" de la configuración, si no el del sistema (en, pt-BR, es)",
		},

		// ---- common ---------------------------------------------------------
		"cli.usage":       {"usage: %s", "uso: %s", "uso: %s"},
		"cli.error":       {"error: %v", "erro: %v", "error: %v"},
		"cli.warning":     {"warning: %s", "aviso: %s", "aviso: %s"},
		"cli.note":        {"note: %s", "nota: %s", "nota: %s"},
		"cli.none":        {"(none)", "(nenhum)", "(ninguno)"},
		"cli.unknown_sub": {"unknown %s subcommand %q", "subcomando de %s desconhecido %q", "subcomando de %s desconocido %q"},
		"cli.unknown_command": {
			"unknown command %q (try `my-agent help`)",
			"comando desconhecido %q (tente `my-agent help`)",
			"comando desconocido %q (pruebe `my-agent help`)",
		},
		"cli.config_not_found": {
			"config %s not found; run `my-agent init` first",
			"configuração %s não encontrada; rode `my-agent init` primeiro",
			"configuración %s no encontrada; ejecute `my-agent init` primero",
		},

		// ---- flags ------------------------------------------------------------
		"cli.flag.lang": {
			"language: auto, en, pt-BR or es",
			"idioma: auto, en, pt-BR ou es",
			"idioma: auto, en, pt-BR o es",
		},
		"cli.flag.force": {
			"overwrite an existing config",
			"sobrescreve uma configuração existente",
			"sobrescribe una configuración existente",
		},
		"cli.flag.recursive": {"descend into directories", "entra em subpastas", "entra en subcarpetas"},
		"cli.flag.include": {
			"comma-separated file patterns (default: common text/code files)",
			"padrões de arquivo separados por vírgula (padrão: arquivos comuns de texto/código)",
			"patrones de archivo separados por comas (por defecto: archivos comunes de texto/código)",
		},
		"cli.flag.include_short": {
			"comma-separated file patterns",
			"padrões de arquivo separados por vírgula",
			"patrones de archivo separados por comas",
		},
		"cli.flag.k": {
			"number of results (default: rag.top_k)",
			"número de resultados (padrão: rag.top_k)",
			"número de resultados (por defecto: rag.top_k)",
		},
		"cli.flag.provider": {
			"provider name (default: agent.provider)",
			"nome do provedor (padrão: agent.provider)",
			"nombre del proveedor (por defecto: agent.provider)",
		},
		"cli.flag.ollama_provider": {
			"ollama provider name (default: agent.provider)",
			"nome do provedor ollama (padrão: agent.provider)",
			"nombre del proveedor ollama (por defecto: agent.provider)",
		},
		"cli.flag.no_rag": {
			"disable retrieval",
			"desliga a busca na base de conhecimento",
			"desactiva la búsqueda en la base de conocimiento",
		},
		"cli.flag.sources":   {"print retrieved sources", "mostra as fontes recuperadas", "muestra las fuentes recuperadas"},
		"cli.flag.conn_name": {"connector name (required)", "nome do conector (obrigatório)", "nombre del conector (obligatorio)"},
		"cli.flag.conn_type": {"file or url (required)", "file ou url (obrigatório)", "file o url (obligatorio)"},
		"cli.flag.conn_path": {"file or directory (type file)", "arquivo ou pasta (tipo file)", "archivo o carpeta (tipo file)"},
		"cli.flag.conn_url":  {"page URL (type url)", "URL da página (tipo url)", "URL de la página (tipo url)"},
		"cli.flag.embedding": {
			"configure the embedding model instead of the chat model",
			"configura o modelo de embeddings em vez do modelo de chat",
			"configura el modelo de embeddings en lugar del modelo de chat",
		},
		"cli.flag.no_chat": {
			"select: do not start the chat afterwards",
			"select: não abre o chat depois",
			"select: no abre el chat después",
		},
		"cli.flag.addr": {
			"listen address (default: server.addr)",
			"endereço de escuta (padrão: server.addr)",
			"dirección de escucha (por defecto: server.addr)",
		},

		// ---- init / config / providers ---------------------------------------------
		"cli.init.exists": {
			"%s already exists (use --force to overwrite)",
			"%s já existe (use --force para sobrescrever)",
			"%s ya existe (use --force para sobrescribir)",
		},
		"cli.init.created":            {"created %s", "%s criado", "%s creado"},
		"cli.config.valid":            {"config is valid", "a configuração é válida", "la configuración es válida"},
		"cli.config.updated":          {"%s updated", "%s atualizado", "%s actualizado"},
		"cli.providers.no_key":        {"no key", "sem chave", "sin clave"},
		"cli.providers.key_set":       {"key set", "chave definida", "clave definida"},
		"cli.providers.no_key_needed": {"no key needed", "não precisa de chave", "no necesita clave"},
		"cli.providers.failed": {
			"%d provider(s) failed",
			"falha em %d provedor(es)",
			"fallo en %d proveedor(es)",
		},

		// ---- rag ---------------------------------------------------------------------
		"cli.rag.ingested": {
			"%s: %d document(s), %d chunk(s)",
			"%s: %d documento(s), %d trecho(s)",
			"%s: %d documento(s), %d fragmento(s)",
		},
		"cli.rag.no_results": {"no results", "nenhum resultado", "sin resultados"},
		"cli.rag.stats": {
			"%d source(s), %d chunk(s), embedder=%s",
			"%d fonte(s), %d trecho(s), embedder=%s",
			"%d fuente(s), %d fragmento(s), embedder=%s",
		},
		"cli.rag.removed": {"removed %d chunk(s)", "%d trecho(s) removido(s)", "%d fragmento(s) eliminado(s)"},
		"cli.rag.cleared": {"knowledge base cleared", "base de conhecimento limpa", "base de conocimiento vaciada"},

		// ---- connectors ---------------------------------------------------------------
		"cli.connectors.none":        {"no connectors configured", "nenhum conector configurado", "ningún conector configurado"},
		"cli.connectors.added":       {"connector %q added", "conector %q adicionado", "conector %q añadido"},
		"cli.connectors.removed":     {"connector %q removed", "conector %q removido", "conector %q eliminado"},
		"cli.connectors.sync_failed": {"%s: FAILED: %v", "%s: FALHOU: %v", "%s: FALLÓ: %v"},
		"cli.connectors.failed": {
			"%d connector(s) failed",
			"falha em %d conector(es)",
			"fallo en %d conector(es)",
		},

		// ---- serve / completion ----------------------------------------------------------
		"cli.serve.listening": {
			"web UI on http://%s (Ctrl+C to stop)",
			"interface web em http://%s (Ctrl+C para parar)",
			"interfaz web en http://%s (Ctrl+C para detener)",
		},
		"cli.completion.unsupported": {
			"unsupported shell %q (use bash or powershell)",
			"shell não suportado %q (use bash ou powershell)",
			"shell no compatible %q (use bash o powershell)",
		},

		// ---- models --------------------------------------------------------------------------
		"cli.models.none_installed": {
			"no models installed (try `models select`)",
			"nenhum modelo instalado (tente `models select`)",
			"ningún modelo instalado (pruebe `models select`)",
		},
		"cli.models.downloaded": {"downloaded %s", "%s baixado", "%s descargado"},
		"cli.models.installed":  {"%s is already installed", "%s já está instalado", "%s ya está instalado"},
		"cli.models.pick_none": {
			"No models installed. Type a model name to download (e.g. llama3.2): ",
			"Nenhum modelo instalado. Digite o nome de um modelo para baixar (ex.: llama3.2): ",
			"Ningún modelo instalado. Escriba el nombre de un modelo para descargar (p. ej.: llama3.2): ",
		},
		"cli.models.pick_title": {"Installed models:", "Modelos instalados:", "Modelos instalados:"},
		"cli.models.current":    {"(current)", "(atual)", "(actual)"},
		"cli.models.other_option": {
			"Other... (type a name; it is downloaded if missing)",
			"Outro... (digite um nome; ele é baixado se faltar)",
			"Otro... (escriba un nombre; se descarga si falta)",
		},
		"cli.models.pick_prompt": {
			"Choose a number or type a model name: ",
			"Escolha um número ou digite o nome de um modelo: ",
			"Elija un número o escriba el nombre de un modelo: ",
		},
		"cli.models.name_prompt": {
			"Model name (e.g. llama3.2, qwen2.5:7b): ",
			"Nome do modelo (ex.: llama3.2, qwen2.5:7b): ",
			"Nombre del modelo (p. ej.: llama3.2, qwen2.5:7b): ",
		},
		"cli.models.bad_choice":   {"%d is not a valid choice", "%d não é uma opção válida", "%d no es una opción válida"},
		"cli.models.no_selection": {"no model selected", "nenhum modelo escolhido", "ningún modelo seleccionado"},
		"cli.ps.none": {
			"no model is loaded right now (it loads on the first request, or run `my-agent chat`)",
			"nenhum modelo está carregado agora (ele carrega na primeira requisição, ou rode `my-agent chat`)",
			"ningún modelo está cargado ahora (se carga con la primera solicitud, o ejecute `my-agent chat`)",
		},
		"cli.ps.header": {
			"NAME|SIZE|PROCESSOR|CONTEXT|UNLOADS IN",
			"NOME|TAMANHO|PROCESSADOR|CONTEXTO|DESCARREGA EM",
			"NOMBRE|TAMAÑO|PROCESADOR|CONTEXTO|SE DESCARGA EN",
		},
		"cli.ps.never": {"never", "nunca", "nunca"},
		"cli.ps.now":   {"now", "agora", "ahora"},
		"cli.ps.cannot_report": {
			"provider %q cannot report running models",
			"o provedor %q não informa os modelos em execução",
			"el proveedor %q no informa los modelos en ejecución",
		},
		"cli.ps.warning": {
			"warning: %s runs %d%% on the CPU because GPU memory is short, so replies are slow.\n" +
				"  Close apps that use the GPU (browsers, launchers, overlays), pick a smaller model\n" +
				"  (`my-agent models select`), or lower `providers.%s.num_ctx`.",
			"aviso: %s roda %d%% na CPU porque falta memória de GPU, então as respostas ficam lentas.\n" +
				"  Feche apps que usam a GPU (navegadores, launchers, overlays), escolha um modelo menor\n" +
				"  (`my-agent models select`) ou reduza `providers.%s.num_ctx`.",
			"aviso: %s se ejecuta %d%% en la CPU porque falta memoria de GPU, así que las respuestas son lentas.\n" +
				"  Cierre las apps que usan la GPU (navegadores, launchers, overlays), elija un modelo más pequeño\n" +
				"  (`my-agent models select`) o reduzca `providers.%s.num_ctx`.",
		},
	})
}
