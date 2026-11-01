package i18n

// Messages of the interactive chat: banner, slash commands, help tree,
// argument hints and @file mentions.
func init() {
	add(map[string]m{
		// ---- banner and conversation ---------------------------------------------------
		"chat.banner_plain": {
			"%s ready (%s). Commands: /help /reset /sources /exit",
			"%s pronto (%s). Comandos: /help /reset /sources /exit",
			"%s listo (%s). Comandos: /help /reset /sources /exit",
		},
		"chat.rag_off": {"RAG off", "RAG desligado", "RAG desactivado"},
		"chat.rag_on":  {"RAG on · %d chunks", "RAG ligado · %d trechos", "RAG activado · %d fragmentos"},
		"chat.banner_hint": {
			"/ commands   @ attach file   /help   /exit",
			"/ comandos   @ anexa arquivo   /help   /exit",
			"/ comandos   @ adjunta archivo   /help   /exit",
		},
		"chat.you":             {"you", "você", "tú"},
		"chat.agent":           {"agent", "agente", "agente"},
		"chat.thinking":        {"thinking…", "pensando…", "pensando…"},
		"chat.interrupted":     {"⏹ interrupted", "⏹ interrompido", "⏹ interrumpido"},
		"chat.sources_title":   {"Sources:", "Fontes:", "Fuentes:"},
		"chat.history_cleared": {"history cleared", "histórico apagado", "historial borrado"},
		"chat.show_sources":    {"show sources: %v", "mostrar fontes: %v", "mostrar fuentes: %v"},
		"chat.model_hint": {
			"(change with /models select or /config chat model)",
			"(troque com /models select ou /config chat model)",
			"(cambie con /models select o /config chat model)",
		},
		"chat.using": {"using %s · %s", "usando %s · %s", "usando %s · %s"},
		"chat.applying_error": {
			"error applying the configuration: %v",
			"erro ao aplicar a configuração: %v",
			"error al aplicar la configuración: %v",
		},
		"chat.secret_visible": {
			"(warning: it will be visible in this terminal) ",
			"(atenção: ficará visível neste terminal) ",
			"(atención: será visible en este terminal) ",
		},

		// ---- errors of the command line inside the chat ----------------------------------
		"chat.err.type_command": {
			"type a command, for example /help",
			"digite um comando, por exemplo /help",
			"escriba un comando, por ejemplo /help",
		},
		"chat.err.ambiguous": {"%q is ambiguous: %s", "%q é ambíguo: %s", "%q es ambiguo: %s"},
		"chat.err.unknown_command": {
			"unknown command %q; type %s for the options",
			"comando desconhecido %q; digite %s para ver as opções",
			"comando desconocido %q; escriba %s para ver las opciones",
		},
		"chat.err.unknown_provider": {
			"provider %q does not exist (available: %s)",
			"o provedor %q não existe (disponíveis: %s)",
			"el proveedor %q no existe (disponibles: %s)",
		},

		// ---- help screen -------------------------------------------------------------------
		"chat.help.title": {"Commands", "Comandos", "Comandos"},
		"chat.help.subtitle": {
			"   / lists commands · Tab completes · ↑↓ chooses · @file attaches · Ctrl+C interrupts",
			"   / lista os comandos · Tab completa · ↑↓ escolhe · @arquivo anexa · Ctrl+C interrompe",
			"   / lista los comandos · Tab completa · ↑↓ elige · @archivo adjunta · Ctrl+C interrumpe",
		},
		"chat.help.shortcuts": {"Shortcuts:", "Atalhos:", "Atajos:"},
		"chat.help.attach":    {"Attach:", "Anexar:", "Adjuntar:"},
		"chat.help.attach_text": {
			"@path/to/file in a message sends the file's content along (Tab completes paths)",
			"@caminho/do/arquivo na mensagem envia o conteúdo do arquivo junto (Tab completa caminhos)",
			"@ruta/al/archivo en el mensaje envía el contenido del archivo (Tab completa rutas)",
		},
		"chat.help.tip": {"Tip:", "Dica:", "Consejo:"},
		"chat.help.tip_text": {
			"/help <command> details a branch, for example /help config chat",
			"/help <comando> detalha um ramo, por exemplo /help config chat",
			"/help <comando> detalla una rama, por ejemplo /help config chat",
		},

		// ---- argument hints (the usage column of the help) -------------------------------------
		"arg.name":           {"<name>", "<nome>", "<nombre>"},
		"arg.model":          {"<model>", "<modelo>", "<modelo>"},
		"arg.key":            {"<key>", "<chave>", "<clave>"},
		"arg.key_value":      {"<key> <value>", "<chave> <valor>", "<clave> <valor>"},
		"arg.query":          {"<query>", "<consulta>", "<consulta>"},
		"arg.source":         {"<source>", "<fonte>", "<fuente>"},
		"arg.ingest":         {"<folder|file|url>...", "<pasta|arquivo|url>...", "<carpeta|archivo|url>..."},
		"arg.text":           {"<text>", "<texto>", "<texto>"},
		"arg.file":           {"<file>", "<arquivo>", "<archivo>"},
		"arg.name_opt":       {"[name]", "[nome]", "[nombre]"},
		"arg.names_opt":      {"[name]...", "[nome]...", "[nombre]..."},
		"arg.command_opt":    {"[command]", "[comando]", "[comando]"},
		"arg.provider_field": {"<name> <field> [value]", "<nome> <campo> [valor]", "<nombre> <campo> [valor]"},
		"arg.connector_add":  {"--name N --type file|url ...", "--name N --type file|url ...", "--name N --type file|url ..."},
		"arg.embedder":       {"<local|provider>", "<local|provedor>", "<local|proveedor>"},
		"arg.lang":           {"<auto|en|pt-BR|es>", "<auto|en|pt-BR|es>", "<auto|en|pt-BR|es>"},

		// ---- command descriptions (the help tree and the autocomplete list) --------------------------
		"tree.config":      {"Configuration and parameters", "Configuração e parâmetros", "Configuración y parámetros"},
		"tree.config.chat": {"Agent", "Agente", "Agente"},
		"tree.config.rag":  {"Search and index", "Busca e índice", "Búsqueda e índice"},
		"tree.config.provider": {
			"keys, address, keep_alive, num_ctx... of a provider",
			"chaves, endereço, keep_alive, num_ctx... de um provedor",
			"claves, dirección, keep_alive, num_ctx... de un proveedor",
		},
		"tree.config.show":     {"shows the configuration (keys hidden)", "mostra a configuração (chaves ocultas)", "muestra la configuración (claves ocultas)"},
		"tree.config.get":      {"shows a value", "mostra um valor", "muestra un valor"},
		"tree.config.set":      {"changes and saves any value", "altera e salva qualquer valor", "cambia y guarda cualquier valor"},
		"tree.config.validate": {"checks the configuration", "confere a configuração", "comprueba la configuración"},
		"tree.config.path":     {"shows the configuration file in use", "mostra o arquivo de configuração em uso", "muestra el archivo de configuración en uso"},
		"tree.config.language": {
			"language of the messages (auto, en, pt-BR, es)",
			"idioma das mensagens (auto, en, pt-BR, es)",
			"idioma de los mensajes (auto, en, pt-BR, es)",
		},

		"tree.param.provider":      {"default provider (saved, switches now)", "provedor padrão (salva e troca agora)", "proveedor predeterminado (se guarda y cambia ahora)"},
		"tree.param.model":         {"model of the current provider (Ollama: downloads if missing)", "modelo do provedor atual (Ollama: baixa se faltar)", "modelo del proveedor actual (Ollama: descarga si falta)"},
		"tree.param.temperature":   {"creativity of the replies", "criatividade das respostas", "creatividad de las respuestas"},
		"tree.param.max_tokens":    {"maximum length of each reply", "tamanho máximo de cada resposta", "longitud máxima de cada respuesta"},
		"tree.param.prompt":        {"the agent's instructions (system prompt)", "instruções do agente (system prompt)", "instrucciones del agente (system prompt)"},
		"tree.param.rag":           {"use the knowledge base", "consultar a base de conhecimento", "consultar la base de conocimiento"},
		"tree.param.embedder":      {"who generates the embeddings", "quem gera os embeddings", "quién genera los embeddings"},
		"tree.param.top_k":         {"chunks used per question", "trechos usados por pergunta", "fragmentos usados por pregunta"},
		"tree.param.min_score":     {"minimum similarity", "similaridade mínima", "similitud mínima"},
		"tree.param.chunk_size":    {"size of the chunks (characters)", "tamanho dos trechos (caracteres)", "tamaño de los fragmentos (caracteres)"},
		"tree.param.chunk_overlap": {"overlap between chunks", "sobreposição entre trechos", "solapamiento entre fragmentos"},
		"tree.param.store":         {"index file", "arquivo do índice", "archivo del índice"},

		"tree.rag":        {"Knowledge base", "Base de conhecimento", "Base de conocimiento"},
		"tree.rag.ingest": {"indexes documents", "indexa documentos", "indexa documentos"},
		"tree.rag.search": {"shows what would be retrieved", "mostra o que seria recuperado", "muestra lo que se recuperaría"},
		"tree.rag.list":   {"indexed sources", "fontes indexadas", "fuentes indexadas"},
		"tree.rag.remove": {"removes a source", "remove uma fonte", "elimina una fuente"},
		"tree.rag.clear":  {"wipes the whole base", "apaga toda a base", "borra toda la base"},

		"tree.models":        {"Ollama models", "Modelos do Ollama", "Modelos de Ollama"},
		"tree.models.list":   {"installed models", "modelos instalados", "modelos instalados"},
		"tree.models.ps":     {"what is loaded and how much is on the GPU", "o que está carregado e quanto está na GPU", "lo que está cargado y cuánto está en la GPU"},
		"tree.models.select": {"choose from a menu (or type another and download it)", "escolher em um menu (ou digitar outro e baixar)", "elegir en un menú (o escribir otro y descargarlo)"},
		"tree.models.use":    {"uses the model, downloading it if missing", "usa o modelo, baixando se faltar", "usa el modelo, descargándolo si falta"},
		"tree.models.pull":   {"only downloads, does not switch", "só baixa, sem trocar", "solo descarga, sin cambiar"},

		"tree.providers":      {"Providers (Claude, Gemini, Ollama)", "Provedores (Claude, Gemini, Ollama)", "Proveedores (Claude, Gemini, Ollama)"},
		"tree.providers.list": {"providers and whether they have a key", "provedores e se têm chave", "proveedores y si tienen clave"},
		"tree.providers.test": {"tests the connection", "testa a conexão", "prueba la conexión"},
		"tree.providers.use":  {"switches provider in this conversation (not saved)", "troca de provedor nesta conversa (não salva)", "cambia de proveedor en esta conversación (no se guarda)"},

		"tree.connectors":        {"Data connectors", "Conectores de dados", "Conectores de datos"},
		"tree.connectors.list":   {"configured connectors", "conectores configurados", "conectores configurados"},
		"tree.connectors.add":    {"adds a connector", "adiciona um conector", "añade un conector"},
		"tree.connectors.remove": {"removes a connector", "remove um conector", "elimina un conector"},
		"tree.connectors.sync":   {"syncs (all, if no name)", "sincroniza (todos, se sem nome)", "sincroniza (todos, si no hay nombre)"},

		"tree.chat":         {"Conversation", "Conversa", "Conversación"},
		"tree.chat.reset":   {"erases the conversation history", "apaga o histórico da conversa", "borra el historial de la conversación"},
		"tree.chat.sources": {"toggles showing the RAG sources", "liga/desliga a exibição das fontes do RAG", "activa/desactiva mostrar las fuentes del RAG"},
		"tree.chat.model":   {"shows the provider and model in use", "mostra o provedor e o modelo em uso", "muestra el proveedor y el modelo en uso"},
		"tree.chat.clear":   {"clears the screen", "limpa a tela", "limpia la pantalla"},
		"tree.chat.exit":    {"leaves the chat", "sai do chat", "sale del chat"},
		"tree.help":         {"this help", "esta ajuda", "esta ayuda"},

		// ---- /config chat|rag <param> and /config provider ------------------------------------------------
		"chat.note.chunks": {
			"applies to new ingestions; re-ingest (/rag ingest) to apply it to what is already indexed",
			"vale para novas ingestões; reingira (/rag ingest) para aplicar ao que já está no índice",
			"vale para nuevas ingestas; vuelva a ingerir (/rag ingest) para aplicarlo a lo ya indexado",
		},
		"chat.note.embedder": {
			"the current index was built by another embedder: use /rag clear and re-ingest the documents",
			"o índice atual foi gerado por outro embedder: use /rag clear e reingira os documentos",
			"el índice actual fue generado por otro embedder: use /rag clear y vuelva a ingerir los documentos",
		},
		"chat.note.store": {
			"the index is now read from that file (documents already indexed stay in the previous file)",
			"o índice passa a ser lido desse arquivo (documentos já indexados ficam no arquivo anterior)",
			"el índice pasa a leerse de ese archivo (los documentos ya indexados quedan en el archivo anterior)",
		},
		"chat.provider.syntax": {
			"/config provider <name> <field> [value]   fields: %s",
			"/config provider <nome> <campo> [valor]   campos: %s",
			"/config provider <nombre> <campo> [valor]   campos: %s",
		},
		"chat.provider.unknown_field": {
			"field %q is unknown; fields: %s",
			"campo %q desconhecido; campos: %s",
			"campo %q desconocido; campos: %s",
		},
		"chat.provider.key_tip": {
			"tip: typing the key here leaves it in the terminal history; prefer /config provider %s api_key (no value, hidden input) or an environment variable",
			"dica: digitar a chave aqui a deixa no histórico do terminal; prefira /config provider %s api_key (sem valor, entrada oculta) ou uma variável de ambiente",
			"consejo: escribir la clave aquí la deja en el historial del terminal; prefiera /config provider %s api_key (sin valor, entrada oculta) o una variable de entorno",
		},
		"chat.provider.secret_prompt": {
			"API key for %s (not shown while typing): ",
			"API key de %s (não aparece ao digitar): ",
			"API key de %s (no se muestra al escribir): ",
		},
		"chat.provider.empty_secret": {
			"nothing typed; the key was not changed",
			"nada digitado; a chave não foi alterada",
			"no se escribió nada; la clave no se cambió",
		},
		"chat.provider.env_not_name": {
			"api_key_env is the NAME of an environment variable (e.g. GEMINI_API_KEY), not the key. To store the key use: /config provider %s api_key",
			"api_key_env é o NOME de uma variável de ambiente (ex.: GEMINI_API_KEY), não a chave. Para guardar a chave use: /config provider %s api_key",
			"api_key_env es el NOMBRE de una variable de entorno (p. ej.: GEMINI_API_KEY), no la clave. Para guardar la clave use: /config provider %s api_key",
		},
		"chat.provider.saved_hidden": {"%s saved (hidden)", "%s salva (oculta)", "%s guardada (oculta)"},
		"chat.language.set":          {"language set to %s", "idioma definido como %s", "idioma establecido: %s"},

		// ---- @file mentions ------------------------------------------------------------------------------------
		"mention.describe":       {"attached: %s (%s)", "anexado: %s (%s)", "adjunto: %s (%s)"},
		"mention.describe_trunc": {", only the first %s were sent", ", só os primeiros %s foram enviados", ", solo se enviaron los primeros %s"},
		"mention.header":         {"[Attached file: %s]", "[Arquivo anexado: %s]", "[Archivo adjunto: %s]"},
		"mention.truncated":      {"(content truncated)", "(conteúdo truncado)", "(contenido truncado)"},
		"mention.not_found":      {"@%s: file not found", "@%s: arquivo não encontrado", "@%s: archivo no encontrado"},
		"mention.folder": {
			"@%s is a folder; use /rag ingest %s to add the folder to the knowledge base",
			"@%s é uma pasta; use /rag ingest %s para adicionar a pasta à base de conhecimento",
			"@%s es una carpeta; use /rag ingest %s para añadir la carpeta a la base de conocimiento",
		},
		"mention.max_files": {
			"@%s ignored: at most %d files per message",
			"@%s ignorado: no máximo %d arquivos por mensagem",
			"@%s ignorado: como máximo %d archivos por mensaje",
		},
		"mention.err_binary": {
			"it is a binary file; only text can be attached",
			"é um arquivo binário; só texto pode ser anexado",
			"es un archivo binario; solo se puede adjuntar texto",
		},
		"mention.file_error": {"@%s: %v", "@%s: %v", "@%s: %v"},
		"suggest.folder":     {"folder", "pasta", "carpeta"},
	})
}
