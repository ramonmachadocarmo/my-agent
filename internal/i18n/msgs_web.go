package i18n

// Messages of the web UI. Keys starting with "web.ui." are sent to the browser
// (GET /api/i18n); "web.err." keys are used by the server for its own errors.
func init() {
	add(map[string]m{
		// ---- server errors --------------------------------------------------------
		"web.err.message_required": {"message is required", "a mensagem é obrigatória", "el mensaje es obligatorio"},
		"web.err.model_required": {
			"model is required and set_as must be model, embedding or empty",
			"o modelo é obrigatório e set_as deve ser model, embedding ou vazio",
			"el modelo es obligatorio y set_as debe ser model, embedding o vacío",
		},
		"web.err.saved_not_config": {
			"model downloaded but config was not saved: %s",
			"modelo baixado, mas a configuração não foi salva: %s",
			"modelo descargado, pero la configuración no se guardó: %s",
		},
		"web.err.content_type": {
			"content-type must be application/json",
			"content-type deve ser application/json",
			"content-type debe ser application/json",
		},
		"web.err.bad_body": {
			"invalid request body: %w",
			"corpo da requisição inválido: %w",
			"cuerpo de la solicitud no válido: %w",
		},
		"web.err.name_required": {
			"name is required with text",
			"o nome é obrigatório junto com o texto",
			"el nombre es obligatorio junto con el texto",
		},
		"web.err.provide": {
			"provide text+name, path or url",
			"informe texto+nome, caminho ou url",
			"indique texto+nombre, ruta o url",
		},

		// ---- header, tabs ------------------------------------------------------------
		"web.ui.tab.chat":   {"Chat", "Chat", "Chat"},
		"web.ui.tab.config": {"Configuration", "Configuração", "Configuración"},
		"web.ui.tab.rag":    {"RAG", "RAG", "RAG"},
		"web.ui.tab.conn":   {"Connectors", "Conectores", "Conectores"},
		"web.ui.lang":       {"Language", "Idioma", "Idioma"},
		"web.ui.lang_auto":  {"Automatic", "Automático", "Automático"},

		// ---- chat ----------------------------------------------------------------------------
		"web.ui.chat.provider": {"Provider", "Provedor", "Proveedor"},
		"web.ui.chat.use_rag":  {"Use RAG", "Usar RAG", "Usar RAG"},
		"web.ui.chat.reset":    {"New conversation", "Nova conversa", "Nueva conversación"},
		"web.ui.chat.placeholder": {
			"Type your message (Enter sends, Shift+Enter adds a line)",
			"Digite sua mensagem (Enter envia, Shift+Enter quebra linha)",
			"Escriba su mensaje (Enter envía, Shift+Enter añade una línea)",
		},
		"web.ui.chat.send": {"Send", "Enviar", "Enviar"},
		"web.ui.chat.stop": {"Stop", "Parar", "Detener"},
		"web.ui.chat.empty": {
			"✦ Start a conversation. With RAG on, the agent consults your knowledge base.",
			"✦ Comece uma conversa. Com o RAG ligado, o agente consulta a sua base de conhecimento.",
			"✦ Empiece una conversación. Con el RAG activado, el agente consulta su base de conocimiento.",
		},
		"web.ui.chat.avatar_user": {"Y", "V", "T"},
		"web.ui.chat.interrupted": {"⏹ interrupted", "⏹ interrompido", "⏹ interrumpido"},
		"web.ui.chat.cut":         {"reply interrupted", "resposta interrompida", "respuesta interrumpida"},

		// ---- configuration ---------------------------------------------------------------------
		"web.ui.cfg.save":  {"Save configuration", "Salvar configuração", "Guardar configuración"},
		"web.ui.cfg.saved": {"Configuration saved", "Configuração salva", "Configuración guardada"},
		"web.ui.cfg.note": {
			"Saved API keys are shown masked; leave the masked value to keep it. Alternatively use the environment variable named in api_key_env.",
			"Chaves de API salvas aparecem mascaradas; deixe o valor mascarado para mantê-lo. Alternativamente use a variável de ambiente indicada em api_key_env.",
			"Las claves de API guardadas aparecen enmascaradas; deje el valor enmascarado para conservarlo. También puede usar la variable de entorno indicada en api_key_env.",
		},
		"web.ui.cfg.agent":            {"Agent", "Agente", "Agente"},
		"web.ui.cfg.name":             {"Name", "Nome", "Nombre"},
		"web.ui.cfg.default_provider": {"Default provider", "Provedor padrão", "Proveedor predeterminado"},
		"web.ui.cfg.temperature":      {"Temperature", "Temperatura", "Temperatura"},
		"web.ui.cfg.max_tokens":       {"Max tokens", "Máx. de tokens", "Máx. de tokens"},
		"web.ui.cfg.use_rag_default":  {"Use RAG by default", "Usar RAG por padrão", "Usar RAG por defecto"},
		"web.ui.cfg.system_prompt":    {"System prompt", "System prompt", "System prompt"},
		"web.ui.cfg.provider_card":    {"Provider: %s", "Provedor: %s", "Proveedor: %s"},
		"web.ui.cfg.test":             {"Test", "Testar", "Probar"},
		"web.ui.cfg.test_ok":          {"%s: OK (%d ms)", "%s: OK (%d ms)", "%s: OK (%d ms)"},
		"web.ui.cfg.rag":              {"RAG", "RAG", "RAG"},
		"web.ui.cfg.store_path":       {"Index file", "Arquivo do índice", "Archivo del índice"},
		"web.ui.cfg.embedder":         {"Embedder", "Embedder", "Embedder"},
		"web.ui.cfg.chunk_size":       {"Chunk size", "Tamanho do trecho", "Tamaño del fragmento"},
		"web.ui.cfg.chunk_overlap":    {"Overlap", "Sobreposição", "Solapamiento"},
		"web.ui.cfg.top_k":            {"Top K", "Top K", "Top K"},
		"web.ui.cfg.min_score":        {"Minimum score", "Score mínimo", "Puntuación mínima"},
		"web.ui.cfg.server":           {"Server", "Servidor", "Servidor"},
		"web.ui.cfg.addr":             {"Address (needs restart)", "Endereço (requer reinício)", "Dirección (requiere reinicio)"},

		// ---- Ollama models in the provider card ------------------------------------------------------
		"web.ui.models.installed": {"Models installed in Ollama", "Modelos instalados no Ollama", "Modelos instalados en Ollama"},
		"web.ui.models.use_as":    {"Use as", "Usar como", "Usar como"},
		"web.ui.models.as_chat":   {"Chat model", "Modelo de chat", "Modelo de chat"},
		"web.ui.models.as_embed":  {"Embedding model", "Modelo de embeddings", "Modelo de embeddings"},
		"web.ui.models.name": {
			"Model name (e.g. llama3.2, qwen2.5:7b)",
			"Nome do modelo (ex.: llama3.2, qwen2.5:7b)",
			"Nombre del modelo (p. ej.: llama3.2, qwen2.5:7b)",
		},
		"web.ui.models.pull": {"Download and use", "Baixar e usar", "Descargar y usar"},
		"web.ui.models.hint": {
			"If the model is not installed it is downloaded and the configuration is saved automatically.",
			"Se o modelo não estiver instalado, ele é baixado e a configuração é salva automaticamente.",
			"Si el modelo no está instalado, se descarga y la configuración se guarda automáticamente.",
		},
		"web.ui.models.loading":       {"Loading…", "Carregando…", "Cargando…"},
		"web.ui.models.unavailable":   {"Ollama unavailable — type a name", "Ollama indisponível — digitar nome", "Ollama no disponible — escriba un nombre"},
		"web.ui.models.list_failed":   {"Could not list the models: %s", "Não foi possível listar os modelos: %s", "No se pudieron listar los modelos: %s"},
		"web.ui.models.other_option":  {"Other… (type and download)", "Outro… (digitar e baixar)", "Otro… (escribir y descargar)"},
		"web.ui.models.starting":      {"Starting…", "Iniciando…", "Iniciando…"},
		"web.ui.models.need_name":     {"Enter the model name", "Informe o nome do modelo", "Indique el nombre del modelo"},
		"web.ui.models.cut":           {"Download interrupted", "Download interrompido", "Descarga interrumpida"},
		"web.ui.models.downloaded":    {"%s downloaded and configured", "%s baixado e configurado", "%s descargado y configurado"},
		"web.ui.models.was_installed": {"%s already installed; configured", "%s já instalado; configurado", "%s ya instalado; configurado"},
		"web.ui.models.loaded":        {"%s loaded · 100%% GPU · %s", "%s carregado · 100%% GPU · %s", "%s cargado · 100%% GPU · %s"},
		"web.ui.models.cpu": {
			"⚠ %s runs %d%% on the CPU (low GPU memory): slow replies. Close apps that use the GPU, pick a smaller model or lower num_ctx.",
			"⚠ %s roda %d%% na CPU (pouca memória de GPU): respostas lentas. Feche apps que usam a GPU, escolha um modelo menor ou reduza num_ctx.",
			"⚠ %s se ejecuta %d%% en la CPU (poca memoria de GPU): respuestas lentas. Cierre las apps que usan la GPU, elija un modelo más pequeño o reduzca num_ctx.",
		},
		"web.ui.models.none_loaded": {
			"No model loaded right now (it loads on the first message).",
			"Nenhum modelo carregado agora (carrega na primeira mensagem).",
			"Ningún modelo cargado ahora (se carga con el primer mensaje).",
		},

		// ---- rag ---------------------------------------------------------------------------------------------
		"web.ui.rag.title":         {"Knowledge base", "Base de conhecimento", "Base de conocimiento"},
		"web.ui.rag.info":          {"— %d chunks, embedder: %s", "— %d trechos, embedder: %s", "— %d fragmentos, embedder: %s"},
		"web.ui.rag.no_embedder":   {"(none)", "(nenhum)", "(ninguno)"},
		"web.ui.rag.th_source":     {"Source", "Fonte", "Fuente"},
		"web.ui.rag.th_chunks":     {"Chunks", "Trechos", "Fragmentos"},
		"web.ui.rag.remove":        {"Remove", "Remover", "Eliminar"},
		"web.ui.rag.empty":         {"Empty", "Vazio", "Vacío"},
		"web.ui.rag.clear":         {"Clear all", "Limpar tudo", "Vaciar todo"},
		"web.ui.rag.confirm_clear": {"Clear the whole knowledge base?", "Limpar toda a base?", "¿Vaciar toda la base?"},
		"web.ui.rag.ingest":        {"Ingest", "Ingerir", "Ingerir"},
		"web.ui.rag.target":        {"File/folder on the server or URL", "Arquivo/pasta no servidor ou URL", "Archivo/carpeta en el servidor o URL"},
		"web.ui.rag.target_ph":     {"C:\\docs  or  https://example.com/page", "C:\\docs  ou  https://exemplo.com/pagina", "C:\\docs  o  https://ejemplo.com/pagina"},
		"web.ui.rag.paste":         {"Or paste some text", "Ou cole um texto", "O pegue un texto"},
		"web.ui.rag.source_name":   {"Source name", "Nome da fonte", "Nombre de la fuente"},
		"web.ui.rag.source_ph":     {"notes.txt", "notas.txt", "notas.txt"},
		"web.ui.rag.ingest_text":   {"Ingest text", "Ingerir texto", "Ingerir texto"},
		"web.ui.rag.ingested":      {"%d document(s), %d chunk(s)", "%d documento(s), %d trecho(s)", "%d documento(s), %d fragmento(s)"},
		"web.ui.rag.ingested_text": {"%d chunk(s)", "%d trecho(s)", "%d fragmento(s)"},
		"web.ui.rag.search":        {"Search", "Buscar", "Buscar"},
		"web.ui.rag.query":         {"Query", "Consulta", "Consulta"},
		"web.ui.rag.no_results":    {"No results", "Sem resultados", "Sin resultados"},

		// ---- connectors ------------------------------------------------------------------------------------------
		"web.ui.conn.title":     {"Connectors", "Conectores", "Conectores"},
		"web.ui.conn.new":       {"New connector", "Novo conector", "Nuevo conector"},
		"web.ui.conn.name":      {"Name", "Nome", "Nombre"},
		"web.ui.conn.type":      {"Type", "Tipo", "Tipo"},
		"web.ui.conn.path":      {"Path (file)", "Caminho (file)", "Ruta (file)"},
		"web.ui.conn.url":       {"URL (url)", "URL (url)", "URL (url)"},
		"web.ui.conn.include":   {"Include (patterns, comma)", "Include (padrões, vírgula)", "Include (patrones, coma)"},
		"web.ui.conn.recursive": {"Recursive", "Recursivo", "Recursivo"},
		"web.ui.conn.add":       {"Add", "Adicionar", "Añadir"},
		"web.ui.conn.th_target": {"Target", "Destino", "Destino"},
		"web.ui.conn.sync":      {"Sync", "Sincronizar", "Sincronizar"},
		"web.ui.conn.remove":    {"Remove", "Remover", "Eliminar"},
		"web.ui.conn.none":      {"No connectors", "Nenhum conector", "Ningún conector"},
		"web.ui.conn.added":     {"Connector added", "Conector adicionado", "Conector añadido"},
		"web.ui.conn.synced":    {"%s: %d doc(s), %d chunk(s)", "%s: %d doc(s), %d trecho(s)", "%s: %d doc(s), %d fragmento(s)"},
	})
}
