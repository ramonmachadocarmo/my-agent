package i18n

// Messages of the internal packages: providers, app, connectors, rag, agent.
func init() {
	add(map[string]m{
		"common.err.model_empty": {"model name is empty", "o nome do modelo está vazio", "el nombre del modelo está vacío"},

		// providers
		"provider.err.no_embeddings": {
			"provider does not support embeddings",
			"o provedor não suporta embeddings",
			"el proveedor no admite embeddings",
		},
		"provider.err.unknown_type": {
			"provider %q: unknown type %q",
			"provedor %q: tipo desconhecido %q",
			"proveedor %q: tipo desconocido %q",
		},
		"provider.err.missing_key": {
			"provider %q: missing API key (set %s)",
			"provedor %q: falta a chave de API (defina %s)",
			"proveedor %q: falta la clave de API (defina %s)",
		},
		"provider.hint.key_or_env": {
			"providers.%s.api_key or env %s",
			"providers.%s.api_key ou a variável de ambiente %s",
			"providers.%s.api_key o la variable de entorno %s",
		},
		"provider.hint.key": {"providers.%s.api_key", "providers.%s.api_key", "providers.%s.api_key"},
		"provider.hint.env_not_name": {
			"providers.%[1]s.api_key; providers.%[1]s.api_key_env must be a variable NAME, not a key",
			"providers.%[1]s.api_key; providers.%[1]s.api_key_env deve ser o NOME de uma variável, não a chave",
			"providers.%[1]s.api_key; providers.%[1]s.api_key_env debe ser el NOMBRE de una variable, no la clave",
		},
		"provider.err.empty_response": {"%s: empty response", "%s: resposta vazia", "%s: respuesta vacía"},
		"provider.err.no_candidates": {
			"%s: no candidates in response",
			"%s: a resposta não trouxe candidatos",
			"%s: la respuesta no trajo candidatos",
		},
		"provider.err.bad_stream_event": {
			"%s: bad stream event: %w",
			"%s: evento de streaming inválido: %w",
			"%s: evento de streaming no válido: %w",
		},
		"provider.err.bad_stream_line": {
			"%s: bad stream line: %w",
			"%s: linha de streaming inválida: %w",
			"%s: línea de streaming no válida: %w",
		},
		"provider.err.embed_count": {
			"%s: expected %d embeddings, got %d",
			"%s: esperava %d embeddings, recebi %d",
			"%s: se esperaban %d embeddings, se recibieron %d",
		},
		"provider.err.decoding": {
			"%s: decoding response: %w",
			"%s: erro ao ler a resposta: %w",
			"%s: error al leer la respuesta: %w",
		},
		"provider.err.idle": {
			"no data received for %s (raise providers.%s.timeout_seconds for slow models)",
			"nenhum dado recebido por %s (aumente providers.%s.timeout_seconds para modelos lentos)",
			"no se recibieron datos durante %s (aumente providers.%s.timeout_seconds para modelos lentos)",
		},
		"provider.err.pull": {
			"%s: pull %s: %s",
			"%s: baixando %s: %s",
			"%s: descargando %s: %s",
		},
		"provider.err.pull_bad": {
			"%s: bad pull response: %w",
			"%s: resposta de download inválida: %w",
			"%s: respuesta de descarga no válida: %w",
		},
		"provider.err.pull_interrupted": {
			"%s: download of %s was interrupted",
			"%s: o download de %s foi interrompido",
			"%s: la descarga de %s se interrumpió",
		},

		// app
		"app.err.unknown_provider":  {"unknown provider %q", "provedor desconhecido %q", "proveedor desconocido %q"},
		"app.err.unknown_connector": {"unknown connector %q", "conector desconhecido %q", "conector desconocido %q"},
		"app.err.not_ollama": {
			"provider %q is type %s; model management is only available for ollama providers (use --provider)",
			"o provedor %q é do tipo %s; o gerenciamento de modelos só existe para provedores ollama (use --provider)",
			"el proveedor %q es de tipo %s; la gestión de modelos solo existe para proveedores ollama (use --provider)",
		},
		"app.err.no_model_mgmt": {
			"provider %q does not support model management",
			"o provedor %q não suporta gerenciamento de modelos",
			"el proveedor %q no admite la gestión de modelos",
		},

		// connectors
		"connectors.err.file_needs_path": {
			"file connector needs a path",
			"o conector file precisa de um caminho",
			"el conector file necesita una ruta",
		},
		"connectors.err.url_needs_url": {
			"url connector needs a url",
			"o conector url precisa de uma URL",
			"el conector url necesita una URL",
		},
		"connectors.err.unknown_type": {
			"unknown connector type %q",
			"tipo de conector desconhecido %q",
			"tipo de conector desconocido %q",
		},
		"connectors.err.too_large": {
			"%s: larger than %d bytes",
			"%s: maior que %d bytes",
			"%s: mayor que %d bytes",
		},
		"connectors.err.binary": {"%s: binary file", "%s: arquivo binário", "%s: archivo binario"},

		// rag
		"rag.err.embedder_mismatch": {
			"store was built with a different embedder",
			"o índice foi gerado por outro embedder",
			"el índice fue generado por otro embedder",
		},
		"rag.err.mismatch_search": {
			"%w (%s vs %s); run `rag clear` and re-ingest",
			"%w (%s vs %s); rode `rag clear` e reingira os documentos",
			"%w (%s vs %s); ejecute `rag clear` y vuelva a ingerir los documentos",
		},
		"rag.err.mismatch_ingest": {
			"%w (%s vs %s); run `rag clear` first",
			"%w (%s vs %s); rode `rag clear` primeiro",
			"%w (%s vs %s); ejecute `rag clear` primero",
		},
		"rag.err.vectors_query": {
			"embedder returned %d vectors for 1 query",
			"o embedder devolveu %d vetores para 1 consulta",
			"el embedder devolvió %d vectores para 1 consulta",
		},
		"rag.err.vectors_texts": {
			"embedder returned %d vectors for %d texts",
			"o embedder devolveu %d vetores para %d textos",
			"el embedder devolvió %d vectores para %d textos",
		},
		"rag.err.reading_store": {
			"reading store %s: %w",
			"erro ao ler o índice %s: %w",
			"error al leer el índice %s: %w",
		},

		// agent
		"agent.err.rag_search": {"rag search: %w", "busca no RAG: %w", "búsqueda en el RAG: %w"},
	})
}
