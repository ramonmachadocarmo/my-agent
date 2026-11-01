# Changelog

Todas as mudanças relevantes deste projeto são documentadas aqui.
Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/) e versionamento [SemVer](https://semver.org/lang/pt-BR/).

## [Unreleased]

### Adicionado
- **Instaladores** que deixam o comando `my-agent` pronto nos terminais, sem precisar de administrador:
  - Windows: `install.ps1` (e `install.cmd` para duplo clique). Instala em `%LOCALAPPDATA%\Programs\my-agent`, adiciona ao PATH do usuário (preservando o tipo do valor no registro e avisando os terminais abertos) e configura o autocompletar no PowerShell 5.1, no PowerShell 7 e no Git Bash. Oferece liberar scripts de perfil (`RemoteSigned`) quando a política os bloqueia.
  - Linux e macOS: `install.sh` para bash, zsh, fish, sh e pwsh. Instala em `~/.local/bin`, ou em `/usr/local/bin` com `--system` (via sudo). No macOS remove a quarentena do binário baixado.
  - Rodar de novo atualiza sem duplicar nada (blocos marcados nos perfis). `-Uninstall`/`--uninstall` remove tudo, e `-Purge`/`--purge` também apaga a configuração e as sessões.
  - Mensagens em inglês, português e espanhol.
- **Pacotes de release**: `scripts/package.ps1` e `scripts/package.sh` geram em `dist/` os pacotes de Windows, Linux e macOS (amd64 e arm64), cada um com o binário, o instalador e a documentação, mais o `SHA256SUMS`.
- `my-agent completion zsh` e `my-agent completion fish`.
- `config set` (e `/config set`) alcança os campos opcionais que ainda não estão no arquivo: `default_agent`, `shell`, `theme` e `agents.<nome>.<campo>`. O agente é criado se não existir, por exemplo `config set agents.review.model claude-sonnet-5`.
- Na interface interativa, `/config set language <idioma>` passa a valer na hora.

### Corrigido
- **Modelo se apresentando como outro assistente** (por exemplo, `qwen2.5:14b` dizendo "Meu nome é Claude"). O prompt de sistema do agente substitui o prompt padrão que muitos modelos locais trazem ("You are Qwen…"), e sem ele o modelo assumia outra identidade. Agora o prompt diz ao modelo o nome do assistente, o agente, o modelo e o provedor em uso, então "qual é o modelo?" tem resposta correta.
- Modelos locais que escrevem a chamada de ferramenta como texto JSON (puro, em bloco de código ou entre `<tool_call>`) têm a chamada reconhecida e executada, desde que o nome seja de uma ferramenta do agente.
- Modelo do Ollama não instalado: o erro agora explica o que fazer (escolher outro com `ctrl+o` ou `ollama pull <modelo>`). Na primeira execução, a configuração criada já usa um modelo instalado quando o Ollama está rodando, em vez de `llama3.2`.
- `install.ps1` falhava ao compilar a partir do código-fonte ("Não é possível indexar em um objeto do tipo MatchInfo"): no PowerShell os nomes de variáveis não diferenciam maiúsculas de minúsculas, e `$m` sobrescrevia a tabela de mensagens `$M`. Com isso, o README passa a separar a instalação pelo repositório (`.\scripts\install.ps1`) da instalação pelo pacote de release (`.\install.ps1`).
- `providers add NOME --key …` (com o nome antes das flags, como na documentação) era recusado como uso incorreto. Agora as duas ordens funcionam.
- As mensagens de erro de tipo inválido não citavam `openai` (provedores) nem `database`/`email` (conectores).

### Documentação
- README (en, pt-BR, es) com as seções atualizadas:
  - chaves de API (`/connect`, `providers add`, rodízio e variáveis padrão por provedor);
  - streaming;
  - idioma na interface;
  - configuração: quais arquivos valem, formas de editar e tabela por seção;
  - estrutura;
  - desenvolvimento, incluindo empacotamento e teste do instalador em container.
- Guias de configuração com:
  - a nova seção 2.0 (configurar pela interface);
  - regras atualizadas do `config set` e o comando `providers add`;
  - provedores compatíveis com OpenAI;
  - rodízio de chaves e novas tentativas;
  - receitas H–L: várias chaves, agente de revisão, GitHub via MCP, assistente de e-mail e CRM, LM Studio;
  - novos erros comuns.

## [0.7.0] - 2026-09-28

### Adicionado
- **Agente interativo em tela cheia, no estilo do opencode**: digitar `my-agent` (sem comando) abre uma TUI (Bubble Tea) com área de conversa rolável, markdown com realce de código, caixa de entrada multilinha, barra de status (agente, provedor/modelo, servidores MCP, tokens) e tela de boas-vindas. Teclas: `tab`/`shift+tab` troca de agente, `ctrl+o` modelos, `ctrl+p` paleta de comandos, `ctrl+s` sessões, `ctrl+n` nova sessão, `esc` interrompe. Autocompletar de `/comandos` (com valores) e de `@arquivos`.
- **Ferramentas com tool calling** em todos os provedores (streaming): `read`, `write`, `edit` (com diff na tela), `bash` (PowerShell no Windows, `/bin/sh` nos demais; configurável em `shell`), `glob`, `grep`, `ls`, `webfetch` e `rag_search`. O agente trabalha em várias etapas (`max_steps`) até concluir.
- **Permissões** `allow`/`ask`/`deny` por ferramenta (glob) e por comando do shell (`"bash:git status*": "allow"`), globais (`permission`) ou por agente. Na TUI, `y` permite uma vez, `a` sempre (na sessão), `n` nega e encerra a etapa.
- **Agentes** configuráveis (`agents`, `default_agent`): `build` (acesso total), `plan` (somente leitura) e `chat` (sem ferramentas) embutidos, e quantos outros quiser, cada um com prompt, `instructions`, provedor, modelo, temperatura, ferramentas, permissões, cor e `max_steps`. `AGENTS.md`/`CLAUDE.md` do projeto entram nas instruções.
- **Provedor `openai`** para qualquer API compatível (OpenAI, OpenRouter, Groq, DeepSeek, Mistral, xAI, Together, LM Studio, vLLM), com presets. `providers add NOME [--preset] [--key] [--model] [--base-url]` e `/connect` na TUI (chave digitada oculta).
- **Várias chaves por provedor** (`api_keys`, com `"env:NOME"`): rodízio automático em 401/402/403/429, mantendo a chave que funcionou. `/keys` lista e adiciona chaves. Também `headers` e `models` (modelos extras no seletor) por provedor.
- **Novas tentativas** automáticas em falhas temporárias (408, 429, 5xx, 529) após 1 s, 2 s e 4 s.
- **Servidores MCP** (`mcp`): locais (stdio) e remotos (Streamable HTTP), com `${VAR}` em variáveis e cabeçalhos; ferramentas expostas como `mcp_<servidor>_<ferramenta>`. `my-agent mcp` e `/mcp` mostram o estado.
- **Conector `database`** (SQLite, PostgreSQL, MySQL): ferramentas `db_schema`, `db_query` (sempre somente leitura) e `db_exec` (com `allow_write`); `query` alimenta o RAG no `sync`.
- **Conector `email`** (IMAP/SMTP): `email_search`, `email_read`, `email_send`; `sync` ingere as últimas mensagens no RAG.
- **Sessões salvas** (`%LOCALAPPDATA%\my-agent\sessions`, `~/.local/share/my-agent/sessions` ou `$MY_AGENT_DATA`): `--continue`, `--session ID`, `/sessions` (retomar/apagar), `/export` para Markdown, `my-agent sessions`.
- **`my-agent run`**: executa o agente com ferramentas sem interface (para scripts); `--yes` permite o que pediria confirmação.
- **Configuração global**: sem `./my-agent.json`, usa `%APPDATA%\my-agent\my-agent.json` / `~/.config/my-agent/my-agent.json`, criada automaticamente na primeira execução da TUI.
- Na TUI: `/config` abre a configuração no editor e recarrega; `/config get|set`; `/reload` reinicia os servidores MCP; `/sync`, `/ingest`, `/rag`, `/tools`, `/connectors`.
- `my-agent agents`, flags globais `--agent`, `--model provedor/modelo`, `--continue`, `--session`.
- `docs/examples/my-agent.full.json` com todas as seções novas; seção 11 nos guias de configuração.

### Alterado
- O projeto passa a ter dependências externas: Bubble Tea/Lip Gloss/Glamour (TUI), drivers de banco (modernc SQLite, pgx, go-sql-driver/mysql) e go-imap. Requer Go 1.26+.
- `my-agent` sem comando abre a TUI em um terminal; em pipe/script continua mostrando a ajuda.
- `config show` e a interface web também mascaram `api_keys`, `headers`, variáveis e cabeçalhos do MCP, `database.dsn` e `email.password`.

### Compatibilidade
- Arquivos de configuração antigos continuam válidos: todas as seções novas são opcionais. `chat`, `ask`, `serve` e a interface web funcionam como antes.

## [0.6.0] - 2026-09-21

### Adicionado (i18n)
- **Internacionalização completa em inglês, português (Brasil) e espanhol**: CLI, chat, ajuda, erros, validação da configuração, interface web e prompt padrão do `init`. Todo texto visível vive no catálogo `internal/i18n` (três traduções por chave, com plural e erros traduzidos que continuam funcionando com `errors.Is`).
- Escolha do idioma, em ordem: `--lang` → `MY_AGENT_LANG` → `language` na configuração → idioma do sistema (Windows e `LC_ALL`/`LANG`) → inglês. Aceita apelidos (`pt`, `pt_BR`, `es-MX`, `en_US.UTF-8`); `--lang` inválido sai com código 2 e valor inválido na variável de ambiente é ignorado.
- Novo campo `language` (`auto`, `en`, `pt-BR`, `es`); configurações antigas sem o campo continuam válidas (`auto`).
- No chat: `/config language <idioma>` (atalho `/lang`) troca na hora, incluindo a árvore de ajuda e o autocompletar; `on/off` também aceita `sim/não` e `sí/no`.
- Web: `GET /api/i18n?lang=`, seletor de idioma no cabeçalho e página sem textos fixos (`data-i18n`).
- `init` grava o prompt padrão no idioma escolhido.
- Testes: completude do catálogo, marcadores consistentes entre idiomas e uma verificação por AST que acusa frases escritas direto no código (exceção: `// i18n:ok`).
- Documentação em três idiomas: `README.md` (en), `README.pt-BR.md`, `README.es.md` e `docs/CONFIGURATION.md`, `docs/CONFIGURACAO.md`, `docs/CONFIGURACION.md`.

### Alterado
- `README.md` passa a ser em inglês; a versão em português está em `README.pt-BR.md`.

### Corrigido
- `PUT /api/config` gravava apelidos de idioma (`pt`) sem normalizar; agora salva a forma canônica (`pt-BR`).
- Colar a chave de API no campo `api_key_env` (que espera o *nome* da variável) fazia o provedor aparecer como "sem chave" sem explicação. Agora `providers list` e `config validate` mostram um aviso claro, e o valor **nunca** é exibido: `config show`/`get`, a interface web e as mensagens de erro o ocultam. A interface web preserva o valor ao salvar. O aviso não invalida a configuração, então o conserto (`config set providers.<n>.api_key_env NOME`) continua funcionando.

### Adicionado
- `docs/CONFIGURACAO.md`: guia de configuração com referência de todos os campos, formas de configurar (CLI, JSON e web), provedores, RAG, conectores, variáveis de ambiente, receitas e tabela de erros comuns.

## [0.5.0] - 2026-09-20

### Adicionado
- **Comandos dentro do chat** (`/`): tudo o que a CLI faz agora também funciona sem sair da conversa: `/config`, `/rag`, `/models`, `/providers`, `/connectors` e atalhos (`/reset`, `/sources`, `/model`, `/clear`, `/exit`). As mudanças valem já na próxima mensagem, **sem perder o histórico** (o agente é recriado a cada comando).
- **Ajuda numerada em árvore** (`/help`, `help` ou `?`): `1. config` → `1.1 chat`, `1.2 rag`, `1.3 provider`…; `/help config chat` detalha um ramo e digitar só `/config` (um grupo) mostra a ajuda dele. A ajuda, o autocompletar e a execução saem da mesma definição de comandos, então nunca ficam fora de sincronia.
- **Autocompletar com lista** ao digitar `/`: ↑/↓ escolhem, Tab completa, Enter completa o item destacado, Esc fecha; desce pela hierarquia (`/co` → `/config ` → `chat` → `temperature`). Completa também **valores**: provedores, modelos instalados no Ollama, chaves de configuração (`agent.temperature`…), fontes do RAG e conectores. Aceita prefixo único (`/he` = `/help`; se for ambíguo pede para desambiguar).
- **`/config chat …` e `/config rag …`**: provedor, modelo (Ollama baixa se faltar), temperatura, max-tokens, prompt, RAG on/off, embedder, top-k, min-score, chunk-size, chunk-overlap. Sem valor, mostra o atual. `/config provider <nome> <campo> [valor]` edita qualquer campo de um provedor, e `/config show|get|set|validate|path` cobre o resto.
- **Chave de API digitada de forma oculta**: `/config provider gemini api_key` (sem valor) pede a chave com a entrada mascarada (`*`), sem eco e fora do histórico. `api_key_env` recusa valores que não sejam nome de variável, sem exibi-los.
- **`@arquivo`**: em qualquer mensagem, `@caminho` anexa o conteúdo do arquivo de texto (até 100 KB por arquivo, 5 por mensagem) e o Tab completa caminhos. A busca no RAG usa só o texto da pergunta, não o arquivo anexado. Pastas, binários e arquivos inexistentes geram um aviso claro.
- Histórico de linhas com ↑/↓ e edição completa da linha (setas, Home/End, Ctrl+U/W/K, Delete, UTF-8). Linhas longas rolam na horizontal em vez de quebrar.
- Agente: `AskStreamQuery` / `SendStreamQuery` (consulta de recuperação separada do texto enviado).

### Alterado
- Os comandos `config`, `providers`, `rag`, `connectors` e `models` da CLI agora compartilham a mesma implementação com o chat (comportamento da CLI inalterado).
- Em terminais sem suporte a entrada "raw" (pipe, mintty) o chat continua funcionando em modo texto: todos os `/comandos` e `@arquivos` funcionam, só sem a lista de autocompletar.

### Corrigido
- Uma linha colada ou digitada rápido (chegando inteira numa só leitura) era executada sem ser desenhada, deixando o prompt vazio na tela.
- Enter sobre um item de autocompletar já digitado por inteiro só acrescentava um espaço em vez de executar.

## [0.4.0] - 2026-09-20

### Adicionado
- **Diagnóstico de desempenho do Ollama**: `models ps` mostra os modelos carregados, quanto de cada um está na GPU/CPU, o contexto e quando serão descarregados. Se o modelo estiver parcialmente na CPU (memória de GPU insuficiente, a causa mais comum de lentidão), avisa e sugere o que fazer.
- O `chat` avisa **uma vez**, após a primeira resposta, quando o modelo em uso está rodando na CPU.
- **Pré-carregamento**: ao abrir o `chat`, o modelo é carregado em segundo plano enquanto você digita a primeira mensagem.
- `providers.<ollama>.keep_alive` (padrão `30m`; o do Ollama é 5 min, o que causava recarga após pausas) e `providers.<ollama>.num_ctx` (limita a janela de contexto para economizar VRAM e manter modelos grandes 100% na GPU).
- Web: o card do Ollama mostra o que está carregado (com alerta de CPU) e permite editar `keep_alive` e `num_ctx`; a API `GET /api/providers/{nome}/models` inclui `running`.
- Autocompletar de bash e PowerShell para `models ps`.

### Compatibilidade
- Configurações antigas continuam válidas: sem `keep_alive`, o programa usa 30m.

## [0.3.0] - 2026-09-19

### Adicionado
- **Streaming de respostas**: o texto aparece conforme o modelo gera, em Claude (SSE), Gemini (SSE) e Ollama (NDJSON). Providers sem streaming continuam funcionando (a resposta chega inteira de uma vez).
- CLI: `chat` e `ask` exibem a resposta em tempo real. No chat estilizado o markdown é formatado enquanto chega (títulos, listas, **negrito**, `código` e blocos de código), mesmo que os pedaços quebrem no meio de um `**`.
- CLI: **Ctrl+C** interrompe só a resposta em andamento (o chat continua); no prompt, sai. A resposta interrompida não entra no histórico.
- Web: `POST /api/chat/stream` (NDJSON: `{"delta"}`… e `{"done"}` ou `{"error"}`); o chat mostra a resposta aos poucos e o botão **Enviar** vira **Parar** durante a geração. Fechar a conexão cancela a geração e o servidor não fica travado durante respostas longas.
- Agente: `AskStream` e `Session.SendStream`.

### Alterado
- `timeout_seconds` no streaming passa a ser um **timeout de inatividade** (tempo sem receber dados), não um limite total: não corta uma resposta que está fluindo e cobre o carregamento lento de modelos grandes. Nas chamadas sem streaming continua sendo o limite total.
- Padrão de `providers.ollama.timeout_seconds` subiu de 120 para 300. Configurações já existentes mantêm o valor salvo: `config set providers.ollama.timeout_seconds 300`.
- `providers test` (CLI e web) agora usa streaming, então não falha só porque o modelo demorou para carregar.

### Corrigido
- Erro `context deadline exceeded (Client.Timeout exceeded while awaiting headers)` com modelos grandes do Ollama que demoram para carregar.

## [0.2.0] - 2026-09-19

### Adicionado
- Gerenciamento de modelos do Ollama: lista os modelos instalados, permite digitar outro nome, baixa se estiver faltando (com progresso) e grava na configuração.
- CLI: `models list`, `models select` (menu interativo com opção "Outro..."), `models use [--embedding] NOME` e `models pull NOME`.
- Web: no card de cada provider Ollama, seletor de modelos instalados, opção "Outro… (digitar e baixar)", escolha entre modelo de chat ou de embeddings e botão "Baixar e usar" com barra de progresso.
- API: `GET /api/providers/{nome}/models` e `POST /api/providers/{nome}/models/pull` (resposta NDJSON com progresso).
- Autocompletar de bash e PowerShell para o comando `models`.
- `models select` agora abre o chat logo após configurar o modelo (use `--no-chat` para só configurar).
- Chat da CLI estilizado em terminais: banner com provider/modelo/RAG, cores, indicador animado de "thinking…", tempo de resposta, markdown (títulos, listas, **negrito**, `código` e blocos de código) e fontes do RAG. Em pipes/redirecionamento ou com `NO_COLOR` a saída continua em texto puro. No Windows ativa ANSI e UTF-8 no console automaticamente.
- Novos comandos no chat: `/help` e `/model`.
- Chat web: renderização de markdown (com escape de HTML), avatares, indicador de "digitando", tempo de resposta, chips de fontes e provider/modelo em uso.

### Corrigido
- Chunks de RAG com sobreposição não começam mais no meio de uma palavra.

## [0.1.0] - 2026-09-19

### Adicionado
- Configurador de agente em Go, sem dependências externas (apenas biblioteca padrão).
- Providers: **Claude** (Anthropic Messages API), **Gemini** (Generative Language API) e **Ollama** (local), com chat e embeddings (Gemini e Ollama).
- **RAG**: chunking com sobreposição respeitando parágrafos/palavras, store vetorial em arquivo JSON, busca por similaridade de cosseno, reingestão idempotente por fonte e proteção contra mistura de embedders.
- Embedder `local` offline (hashing de palavras) para funcionar sem rede nem chave de API.
- **Conectores**: `file` (arquivo ou diretório, filtros `include`, recursivo) e `url` (página web com limpeza de HTML).
- Configuração parametrizável em JSON com validação completa, edição por chave (`config set rag.top_k 6`), chaves de API via arquivo ou variável de ambiente e máscara de segredos na exibição.
- **CLI** (`init`, `config`, `providers`, `chat`, `ask`, `rag`, `connectors`, `serve`, `completion`, `version`) para bash e PowerShell, com scripts de completion.
- **Interface web** embutida no binário: chat, edição da configuração, gestão do RAG e dos conectores, teste de providers.
- Scripts de build/teste `scripts/build.sh` e `scripts/build.ps1`.
- Testes unitários para config, providers (servidores HTTP simulados), RAG, conectores, agente, CLI e API web.

[Unreleased]: #
[0.5.0]: #
[0.4.0]: #
[0.3.0]: #
[0.2.0]: #
[0.1.0]: #
