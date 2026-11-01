# my-agent

[![Sponsor](https://img.shields.io/static/v1?label=Sponsor&message=%E2%9D%A4&logo=GitHub&color=%23fe8e86)](https://github.com/sponsors/ramonmachadocarmo)

[English](README.md) · **Português (Brasil)** · [Español](README.es.md)

Agente de IA para o terminal, no estilo do **opencode**, escrito em **Go** — só que com tudo configurável: digite `my-agent` e abre uma interface em tela cheia onde o agente lê e edita arquivos, roda comandos, consulta bancos de dados e e-mail, usa servidores **MCP** e a sua base de conhecimento (**RAG**). Conecta com **Claude**, **Gemini**, **Ollama** e qualquer API **compatível com OpenAI** (OpenAI, OpenRouter, Groq, DeepSeek, Mistral, xAI, LM Studio…), com **várias chaves por provedor**. Disponível em **inglês, português (Brasil) e espanhol**.

- **Interface interativa** (TUI) com sessões salvas, troca de agente com `Tab`, seletor de modelos, paleta de comandos e confirmação de permissões.
- **Agentes** configuráveis (`build`, `plan`, `chat` e os seus), cada um com prompt, modelo, ferramentas e permissões próprios.
- **Ferramentas**: `read`, `write`, `edit`, `bash`, `glob`, `grep`, `ls`, `webfetch`, `rag_search`, mais as de **banco de dados**, **e-mail** e **MCP**.
- **Permissões** `allow` / `ask` / `deny` por ferramenta, com regras por comando (`"bash:git status*": "allow"`).
- **Várias chaves de API** por provedor, com rodízio automático quando uma é recusada ou atinge o limite, e novas tentativas em sobrecarga (503/529).
- Um único binário (a interface web continua embutida); funciona offline com Ollama.

## Instalação

Os instaladores colocam o `my-agent` no PATH e configuram o autocompletar (Tab) nos terminais, sem precisar de administrador. Depois de instalar, **abra um terminal novo** e digite `my-agent`. Rodar de novo atualiza.

### A partir do código-fonte (este repositório)

Precisa do [Go 1.26+](https://go.dev/dl). Na **raiz do repositório**, o instalador compila e instala:

```powershell
# Windows (PowerShell) — ou dê dois cliques em scripts\install.cmd
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\install.ps1
```
```bash
# Linux e macOS
sh scripts/install.sh
```

### A partir do pacote de release (sem Go)

Gere os pacotes com `.\scripts\package.ps1` (ou `scripts/package.sh`); eles ficam em `dist/`, um por sistema (`windows`/`linux`/`darwin`, `amd64`/`arm64`). Extraia o do seu sistema e rode o instalador **de dentro da pasta extraída**:

```powershell
# Windows — ou dê dois cliques em install.cmd
Expand-Archive my-agent-<versão>-windows-amd64.zip -DestinationPath .; cd my-agent-<versão>-windows-amd64
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1
```
```bash
# Linux e macOS
tar -xzf my-agent-<versão>-linux-amd64.tar.gz && cd my-agent-<versão>-linux-amd64
sh install.sh
```

### O que o instalador faz

| | Windows (`install.ps1`) | Linux e macOS (`install.sh`) |
|---|---|---|
| Binário | `%LOCALAPPDATA%\Programs\my-agent\my-agent.exe` | `~/.local/bin/my-agent` (`--system`: `/usr/local/bin`, com sudo) |
| PATH | PATH do usuário (vale para PowerShell, cmd, Windows Terminal e Git Bash) | `~/.bashrc`, `~/.zshrc`, `~/.profile`, fish, pwsh |
| Autocompletar | perfis do PowerShell 5.1 e 7 e `~/.bashrc` do Git Bash | bash, zsh, fish e pwsh |
| Remover | `-Uninstall` (`-Purge` apaga também configuração e sessões) | `--uninstall` (`--purge` idem) |
| Outras opções | `-InstallDir PASTA`, `-NoShell`, `-Yes` | `--bin-dir PASTA`, `--no-shell` |

- Os perfis ganham um bloco entre `# >>> my-agent >>>` e `# <<< my-agent <<<`; reinstalar substitui o bloco e desinstalar o remove.
- Se o PowerShell estiver bloqueando scripts de perfil, o instalador pergunta se pode liberar para o seu usuário (`RemoteSigned`).
- No macOS, o instalador remove a quarentena do binário baixado.
- Só compilar, sem instalar: `.\scripts\build.ps1` ou `scripts/build.sh` (gera `bin/`).

## Início rápido

```bash
my-agent                           # abre o agente interativo na pasta atual
```

Na primeira execução, se não houver configuração, o programa cria uma em `%APPDATA%\my-agent\my-agent.json` (Windows) ou `~/.config/my-agent/my-agent.json`. Dentro da interface:

1. `/connect` → escolha o provedor (claude, gemini, openai, openrouter, groq, ollama…) e cole a chave (a digitação fica oculta).
2. `ctrl+o` → escolha o modelo (a lista vem de todos os provedores).
3. Peça o que precisa: *"rode os testes e corrija o que falhar"*. Quando o agente quiser editar um arquivo ou rodar um comando, você confirma com `y` (uma vez), `a` (sempre nesta sessão) ou `n` (negar).

```bash
my-agent --continue                # retoma a última sessão desta pasta
my-agent --agent plan              # abre com o agente somente leitura
my-agent --model gemini/gemini-2.5-pro
my-agent run "resuma o README"     # sem interface: imprime a resposta (para scripts)
my-agent run --yes "formate o código com gofmt"   # permite o que pediria confirmação
```

## Interface interativa

```
  ┃ rode os testes e corrija o que falhar

  ✓ ⚡ bash go test ./...
  ✓ 📖 read internal/app/app.go · 104 linhas
  ✓ ✏️ edit internal/app/app.go
      - return nil
      + return err
  Corrigi o retorno de erro em app.go; os testes passam.
  build · claude/claude-sonnet-5 · 12.4s · 8.1k↑ 312↓
╭──────────────────────────────────────────────────────────────╮
│ Pergunte qualquer coisa… (/ comandos, @ arquivos, tab …)     │
╰──────────────────────────────────────────────────────────────╯
  BUILD  claude/claude-sonnet-5   mcp 2/2        tab agentes · ctrl+o modelos · ctrl+p comandos
```

| Tecla | Ação |
|---|---|
| `enter` / `alt+enter` ou `ctrl+j` | envia / nova linha |
| `tab` / `shift+tab` | próximo / anterior agente |
| `ctrl+o` | seletor de modelos (todos os provedores) |
| `ctrl+p` | paleta de comandos |
| `ctrl+s` / `ctrl+n` | sessões salvas / nova sessão |
| `↑` `↓` | mensagens anteriores (ou navega no autocompletar) |
| `pgup` `pgdn`, roda do mouse | rola a conversa |
| `esc` | interrompe a resposta |
| `ctrl+c` | interrompe; com a linha vazia, sai |

Comandos (`/` abre o autocompletar; `/help` lista tudo):

| Comando | O que faz |
|---|---|
| `/new`, `/sessions`, `/export [arquivo]` | nova sessão, retomar/apagar (`ctrl+x`) sessões, salvar a conversa em Markdown |
| `/models [provedor/modelo]`, `/agents [nome]` | troca de modelo ou de agente |
| `/connect [preset]`, `/keys [provedor]` | adiciona provedor e chave; lista chaves ou adiciona mais uma |
| `/tools`, `/mcp`, `/connectors` | ferramentas do agente e permissões; estado dos servidores MCP; conectores |
| `/sync <conector\|all>`, `/ingest <caminho\|url>`, `/rag` | alimenta e consulta a base de conhecimento |
| `/config`, `/config get\|set CHAVE [VALOR]`, `/reload` | abre a configuração no editor (`$EDITOR`, ou Bloco de Notas) e recarrega; lê/grava uma chave |

`@caminho` numa mensagem anexa o arquivo (texto até 100 KB; `Tab` completa caminhos). Se existir `AGENTS.md` (ou `CLAUDE.md`) na pasta, ele entra nas instruções do agente. As sessões ficam em `%LOCALAPPDATA%\my-agent\sessions` (ou `~/.local/share/my-agent/sessions`; mude com `MY_AGENT_DATA`).

## Agentes

Três agentes vêm prontos, e você pode alterar qualquer campo deles ou criar outros na seção `agents`:

| Agente | Para quê |
|---|---|
| `build` (padrão) | acesso total: lê, edita, roda comandos |
| `plan` | somente leitura: investiga e propõe um plano (sem `write`, `edit`, `bash`, `db_exec`, `email_send`) |
| `chat` | conversa com a base de conhecimento, sem ferramentas |

```json
"default_agent": "build",
"agents": {
  "plan":   { "provider": "gemini", "model": "gemini-2.5-pro" },
  "review": {
    "description": "Revisa as mudanças sem tocar em arquivos",
    "provider": "claude",
    "instructions": "Rode git diff e aponte bugs, do mais grave ao menos grave.",
    "tools": { "write": false, "edit": false },
    "permission": { "bash": "deny", "bash:git diff*": "allow" },
    "color": "#f7768e"
  }
}
```

Campos (todos opcionais; o que faltar vem do agente embutido de mesmo nome e depois da seção `agent`): `description`, `system_prompt`, `instructions` (acrescentado ao prompt), `provider`, `model`, `temperature`, `max_tokens`, `use_rag`, `max_steps` (padrão 25), `tools` (nome ou glob → `true`/`false`), `permission`, `color`, `disabled`. `my-agent agents` lista os agentes.

## Ferramentas e permissões

| Ferramenta | Permissão padrão |
|---|---|
| `read`, `glob`, `grep`, `ls`, `webfetch`, `rag_search`, `db_query`, `db_schema`, `email_search`, `email_read` | `allow` |
| `write`, `edit`, `bash`, `db_exec`, `email_send`, `mcp_*` | `ask` |

Regras em `permission` (global) ou `agents.<nome>.permission`: a chave é o nome da ferramenta ou um glob; para o `bash`, `bash:<padrão>` casa com o comando. A regra mais específica vence.

```json
"permission": {
  "bash:git status*": "allow",
  "bash:go test*": "allow",
  "bash:rm *": "deny",
  "webfetch": "deny"
}
```

Uma ferramenta com `deny` nem é oferecida ao modelo. A resposta "sempre" (`a`) vale até o fim da sessão; para o `bash` ela vale para o mesmo programa e subcomando (`go test …`). O `bash` usa PowerShell no Windows e `/bin/sh` nos demais; troque com `"shell": "pwsh -NoProfile -Command"` (ou `"bash -c"`).

## Provedores e várias chaves

```bash
my-agent providers add openrouter --key sk-or-...      # a partir de um preset
my-agent providers add groq --key gsk_... --model llama-3.3-70b-versatile
my-agent providers add meu-vllm --preset openai --base-url http://gpu:8000/v1 --model qwen
my-agent providers add openrouter --key sk-or-segunda  # segunda chave: vai para api_keys
my-agent providers list
```

Presets: `claude`, `gemini`, `ollama`, `openai`, `openrouter`, `groq`, `deepseek`, `mistral`, `xai`, `together`, `lmstudio`. O tipo `openai` serve para qualquer API compatível (a `base_url` inclui a versão, ex.: `https://api.openai.com/v1`).

Cada provedor aceita várias chaves: `api_key`, a lista `api_keys` (itens `"env:NOME"` leem variáveis) e a variável de `api_key_env`. Se uma chave é recusada (401/403), fica sem crédito (402) ou atinge o limite (429), a próxima é usada na hora; a que funcionou vira a atual. Falhas temporárias (429, 5xx, 529) são repetidas após 1 s, 2 s e 4 s. `headers` acrescenta cabeçalhos HTTP e `models` lista modelos extras no seletor.

## Servidores MCP

```json
"mcp": {
  "github":   { "type": "local",  "command": ["npx", "-y", "@modelcontextprotocol/server-github"],
                "environment": { "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_TOKEN}" } },
  "context7": { "type": "remote", "url": "https://mcp.context7.com/mcp", "headers": { "Authorization": "Bearer ${CTX7_KEY}" } },
  "browser":  { "type": "local",  "command": ["npx", "-y", "@playwright/mcp@latest"], "enabled": false }
}
```

Servidores `local` rodam como subprocesso (stdio); `remote` usam Streamable HTTP. `${VAR}` em `environment` e `headers` lê variáveis de ambiente. As ferramentas aparecem como `mcp_<servidor>_<ferramenta>` (permissão padrão `ask`). Os servidores sobem em segundo plano ao abrir a interface (`mcp 2/2` na barra de status); `/mcp` ou `my-agent mcp` mostram o estado e as ferramentas.

## Conectores

Conectores `file` e `url` alimentam a base de conhecimento; `database` e `email` também viram **ferramentas do agente**:

```json
"connectors": [
  { "name": "docs", "type": "file", "path": "./docs", "include": ["*.md"], "recursive": true },
  { "name": "crm", "type": "database",
    "database": { "driver": "postgres", "dsn_env": "CRM_DATABASE_URL",
                  "query": "SELECT nome, email, notas FROM clientes" } },
  { "name": "app", "type": "database", "database": { "driver": "sqlite", "dsn": "./data/app.db", "allow_write": true } },
  { "name": "trabalho", "type": "email",
    "email": { "imap_host": "imap.gmail.com", "smtp_host": "smtp.gmail.com",
               "username": "eu@exemplo.com", "password_env": "MAIL_APP_PASSWORD" } }
]
```

- **database** (`sqlite`, `postgres`, `mysql`): ferramentas `db_schema` e `db_query` (sempre em transação somente leitura) e, com `allow_write: true`, `db_exec` (pede confirmação). `query` define as linhas que `/sync` ou `connectors sync` levam para o RAG. Use `dsn_env` para não gravar senha no arquivo.
- **email** (IMAP + SMTP): `email_search`, `email_read` e, com `smtp_host`, `email_send` (pede confirmação). IMAP na porta 993 (TLS) ou 143 (STARTTLS); SMTP em 587 (STARTTLS) ou 465 (TLS). No Gmail/Outlook use uma *senha de app*. `sync` ingere as últimas `limit` mensagens (padrão 50).
- **file** e **url**: veja [Fontes de RAG](#fontes-de-rag-file-e-url) abaixo.

Veja um arquivo completo em [docs/examples/my-agent.full.json](docs/examples/my-agent.full.json).

## Idioma

O programa fala **inglês (`en`)**, **português do Brasil (`pt-BR`)** e **espanhol (`es`)**: mensagens, erros, ajuda, validação da configuração, a interface web e o prompt padrão que o `init` escreve. Por padrão ele **detecta o idioma do sistema**.

```bash
my-agent --lang es chat                  # só nesta execução
MY_AGENT_LANG=pt-BR my-agent help        # via variável de ambiente
my-agent config set language es          # permanente (auto volta a detectar)
my-agent --lang es init                  # cria o agente já em espanhol
```

- Ordem de escolha: `--lang` → `$MY_AGENT_LANG` → `language` no arquivo de configuração → idioma do sistema → inglês. Apelidos valem (`pt`, `pt_BR`, `es-MX`…).
- **No chat:** `/config language es` (ou `/lang es`) troca na hora, inclusive a árvore de ajuda e o autocompletar.
- **Na interface interativa:** `/config set language es` troca na hora; `--lang` vale só para aquela execução.
- **Na interface web:** seletor de idioma no cabeçalho (grava `language`; `Automático` segue o navegador).
- Comandos, flags e chaves de configuração **não** são traduzidos (scripts e documentação valem em qualquer idioma), e as respostas do modelo seguem o `system_prompt` e o idioma em que você escreve.

### Respostas em streaming

A interface interativa, o `run`, o `chat`, o `ask` e a interface web mostram a resposta **conforme o modelo escreve**, em todos os provedores (Claude, Gemini, Ollama e compatíveis com OpenAI), inclusive quando o agente está usando ferramentas. Para interromper: `esc` na interface interativa, `Ctrl+C` no `chat` (no prompt, sai) e o botão **Parar** na web. Se um modelo grande demorar para carregar e estourar o tempo, aumente o limite de inatividade:

```powershell
my-agent config set providers.ollama.timeout_seconds 600
```

### Se o Ollama estiver lento

```powershell
my-agent models ps      # o que está carregado e quanto está na GPU vs CPU
```

A causa mais comum é o modelo **não caber na memória da GPU**: o Ollama divide o modelo entre GPU e CPU e a geração fica várias vezes mais lenta (`models ps` mostra, por exemplo, `40% GPU / 60% CPU`; o `chat` também avisa depois da primeira resposta). O que fazer, do mais efetivo ao menos:

1. **Use um modelo menor** ou mais quantizado (`my-agent models select`). Regra prática: o tamanho do modelo deve caber na VRAM livre, com folga para o contexto.
2. **Libere VRAM**: navegadores, launchers (Steam, GOG), overlays e o próprio Windows usam memória de GPU. Confira com `nvidia-smi`.
3. **Reduza o contexto**: `my-agent config set providers.ollama.num_ctx 2048` (menos VRAM para o cache de contexto).
4. No servidor do Ollama, ative `OLLAMA_FLASH_ATTENTION=1` e `OLLAMA_KV_CACHE_TYPE=q8_0` (reduz a memória do contexto; reinicie o Ollama).

Para evitar recarregar o modelo a cada pausa, o programa usa `providers.ollama.keep_alive` (padrão `30m`; `-1` mantém carregado) e pré-carrega o modelo ao abrir o `chat`.

### Chaves de API

O jeito mais simples é pela interface: `/connect` escolhe o provedor e pede a chave com a digitação oculta; `/keys <provedor>` acrescenta mais uma. Pela linha de comando:

```bash
my-agent providers add openrouter --key sk-or-...     # cria o provedor (preset) com a chave
my-agent providers add openrouter --key sk-or-outra   # a segunda vai para api_keys (rodízio)
my-agent providers list                               # mostra quantas chaves cada um tem
my-agent providers test                               # testa todos de verdade
```

Para não gravar a chave no arquivo, use variáveis de ambiente — o nome fica em `api_key_env` (e `"env:NOME"` em `api_keys`):

| Provedor | Variável padrão |
|---|---|
| Claude | `ANTHROPIC_API_KEY` |
| Gemini | `GEMINI_API_KEY` |
| OpenAI / OpenRouter / Groq / DeepSeek / Mistral / xAI / Together | `OPENAI_API_KEY`, `OPENROUTER_API_KEY`, `GROQ_API_KEY`, `DEEPSEEK_API_KEY`, `MISTRAL_API_KEY`, `XAI_API_KEY`, `TOGETHER_API_KEY` |

```powershell
[Environment]::SetEnvironmentVariable('ANTHROPIC_API_KEY', 'sk-ant-...', 'User')   # PowerShell, permanente
```
```bash
echo 'export ANTHROPIC_API_KEY=sk-ant-...' >> ~/.bashrc                           # bash (ou ~/.zshrc)
```

Chaves gravadas no arquivo ficam protegidas (permissão `0600`) e aparecem mascaradas em `config show`, `/config get` e na interface web. Ollama e LM Studio locais não precisam de chave: instale o Ollama e rode `ollama pull llama3.2` (chat) e `ollama pull nomic-embed-text` (embeddings).

## Comandos

| Comando | Descrição |
|---|---|
| *(sem comando)* `[--agent A] [--model P/M] [--continue] [--session ID]` | Abre o agente interativo (tela cheia) |
| `run [--agent A] [--model P/M] [--yes] MENSAGEM` | Executa o agente com ferramentas uma vez, sem interface |
| `agents` · `mcp` · `sessions [list \| remove ID]` | Lista agentes; conecta aos servidores MCP e lista as ferramentas; sessões salvas |
| `init [--force]` | Cria o arquivo de configuração padrão |
| `config show \| get KEY \| set KEY VALUE \| path \| validate` | Inspeciona e edita a configuração |
| `providers list \| test [nome] \| add NOME [--preset P] [--key K] [--model M] [--base-url URL]` | Lista providers / testa conectividade / adiciona provedor ou mais uma chave |
| `chat [--provider P] [--no-rag]` | Conversa interativa (`/` para comandos, `@arquivo` para anexar) |
| `ask [--provider P] [--no-rag] [--sources] PERGUNTA` | Pergunta única |
| `models list \| ps \| select \| use NOME \| pull NOME` | Modelos do Ollama: lista, mostra o que está carregado (GPU/CPU), escolhe (baixando se faltar) e configura. Flags: `--provider P`, `--embedding` |
| `rag ingest [--recursive=BOOL] [--include=*.md,*.txt] CAMINHO\|URL...` | Indexa arquivos, pastas ou páginas |
| `rag search [-k N] CONSULTA` | Busca na base |
| `rag list \| remove FONTE \| clear` | Gerencia a base |
| `connectors list \| add \| remove NOME \| sync [NOME...]` | Gerencia conectores (`add` cria `file` e `url`; `database` e `email` são configurados no JSON) |
| `serve [--addr HOST:PORT]` | Sobe a interface web |
| `completion bash\|zsh\|fish\|powershell` | Script de autocompletar |
| `version` | Versão |

Flags globais: `--config ARQUIVO` (ou `-c`) e `--lang IDIOMA`. Ordem de resolução da configuração: flag → `$MY_AGENT_CONFIG` → `./my-agent.json` → arquivo do usuário (`%APPDATA%\my-agent\my-agent.json` ou `~/.config/my-agent/my-agent.json`).
Códigos de saída: `0` ok, `1` erro de execução, `2` uso incorreto.

### Chat em linha (`my-agent chat`): comandos `/`, autocompletar e `@arquivo`

O `chat` é a conversa clássica, sem tela cheia e sem ferramentas (só RAG). Não precisa sair dele para configurar nada. Digite **`/`** e aparece a lista de comandos; **Tab** completa, **↑/↓** escolhem, **Enter** completa o item destacado e **Esc** fecha. Digite **`help`** (ou `?`, ou `/help`) para ver tudo em árvore numerada:

```
 1.    config       Configuração e parâmetros
    1.1   chat          Agente            provider · model · temperature · max-tokens · prompt · rag
    1.2   rag           Busca e índice    embedder · top-k · min-score · chunk-size · chunk-overlap · store
    1.3   provider <nome> <campo> [valor]
    1.4   language · show · get · set · validate · path
 2.    rag          Base de conhecimento (ingest · search · list · remove · clear)
 3.    models       Modelos do Ollama (list · ps · select · use · pull)
 4.    providers    list · test · use
 5.    connectors   list · add · remove · sync
 6.    chat         Conversa (reset · sources · model · clear · exit)
```

Exemplos, todos **sem sair da conversa** e valendo já na próxima mensagem (o histórico é mantido):

```text
/config chat temperature 0.7            # criatividade
/config chat prompt "Responda sempre em português, de forma objetiva."
/config chat model qwen2.5:7b           # no Ollama, baixa se faltar
/config chat rag off                    # liga/desliga a base de conhecimento
/config rag top-k 6
/config provider gemini api_key         # pede a chave com a digitação oculta
/config language es                     # troca o idioma na hora
/models select                          # menu de modelos instalados (+ "Outro…")
/providers use claude                   # troca de provedor só nesta conversa
/rag ingest ./docs                      # indexa e já usa na próxima pergunta
/help config chat                       # detalha um ramo da árvore
```

- Tudo o que existe na CLI (`config`, `rag`, `models`, `providers`, `connectors`) existe aqui com o mesmo nome.
- O Tab também completa **valores**: nomes de provedores, modelos instalados, chaves de configuração e fontes do RAG. Aceita prefixo único (`/he` = `/help`) e avisa quando é ambíguo.
- **`@arquivo`** em qualquer mensagem anexa o conteúdo do arquivo (texto, até 100 KB; até 5 por mensagem): `explique o erro em @logs/app.log`. Tab completa caminhos. Só o texto da pergunta vai para a busca no RAG.
- Enter envia; `Ctrl+C` interrompe a resposta (ou limpa a linha; com a linha vazia, sai); `Ctrl+D` sai; ↑/↓ percorrem o histórico de linhas.
- Em terminais sem modo interativo (pipe, mintty), os comandos e o `@arquivo` funcionam igual; só a lista de autocompletar não aparece.

### Modelos do Ollama (listar, baixar e configurar)

```powershell
my-agent models list                       # modelos instalados (marca o atual)
my-agent models select                     # menu: escolha um número, ou 0 / digite um nome novo
my-agent models use qwen2.5:7b             # usa; baixa antes se não estiver instalado
my-agent models use --embedding bge-m3     # define o modelo de embeddings
my-agent models pull mistral               # só baixa, sem alterar a configuração
```

Depois de escolher, o `select` **abre o chat direto** com o modelo configurado (`--no-chat` desativa). O chat usa cores, banner, spinner e markdown quando a saída é um terminal; em pipe ou com `NO_COLOR=1` sai em texto puro.

O `select` mostra os modelos instalados e a opção **0) Outro...**. Qualquer nome que não esteja instalado é baixado (com progresso) e gravado em `providers.<nome>.model` (ou `embedding_model` com `--embedding`). Coloque as flags **antes** do nome do modelo. Vale para providers do tipo `ollama` (`--provider` escolhe qual; o padrão é `agent.provider`).

Na interface web, o mesmo fica no card de cada provider Ollama na aba **Configuração**.

### Autocompletar

Os instaladores já configuram isto. Para fazer à mão (também há `completion zsh` e `completion fish`):

```bash
source <(my-agent completion bash)        # bash (coloque no ~/.bashrc)
```
```powershell
my-agent completion powershell | Out-String | Invoke-Expression   # PowerShell (coloque no $PROFILE)
```

## Configuração

> 📘 **Guia completo:** [docs/CONFIGURACAO.md](docs/CONFIGURACAO.md) ([English](docs/CONFIGURATION.md) · [Español](docs/CONFIGURACION.md)): todos os campos, valores aceitos, provedores, RAG, conectores, idioma, receitas prontas e erros comuns.

A configuração é **um arquivo JSON**. Qual arquivo vale: `--config` → `$MY_AGENT_CONFIG` → `./my-agent.json` da pasta atual → arquivo do usuário (`%APPDATA%\my-agent\my-agent.json` ou `~/.config/my-agent/my-agent.json`, criado na primeira execução). Assim um projeto pode ter sua própria configuração e o resto usa a global; `my-agent config path` mostra qual está em uso.

Formas de editar:

| Onde | Como |
|---|---|
| Interface interativa | `/config` abre o arquivo no editor (`$EDITOR`, ou Bloco de Notas) e recarrega ao fechar; `/config get\|set CHAVE [VALOR]`; `/connect`, `/keys`, `/models`, `/agents` |
| Linha de comando | `my-agent config show\|get\|set\|validate\|path`, `providers add`, `connectors add`, `models use` |
| Chat em linha | `/config chat …`, `/config rag …`, `/config provider …` |
| Interface web | `my-agent serve` → aba **Configuração** |
| À mão | edite o JSON e rode `my-agent config validate` (ou `/reload` na interface) |

```bash
my-agent config set agent.provider claude
my-agent config set providers.claude.model claude-sonnet-5
my-agent config set default_agent plan
my-agent config set rag.embedder ollama      # embeddings semânticos via Ollama
```

| Seção | Para que serve |
|---|---|
| `language` | Idioma das mensagens: `auto` (padrão), `en`, `pt-BR` ou `es` |
| `agent` | Valores base de todos os agentes: `name`, `system_prompt`, `provider`, `temperature` (0–2), `max_tokens`, `use_rag` |
| `default_agent`, `agents` | Agente inicial e definição dos agentes ([Agentes](#agentes)) |
| `providers.<nome>` | `type` (`claude`, `gemini`, `ollama`, `openai`), `model`, `base_url`, `api_key` / `api_key_env` / `api_keys`, `headers`, `models`, `embedding_model`, `timeout_seconds` e, no Ollama, `keep_alive` / `num_ctx` ([Provedores e várias chaves](#provedores-e-várias-chaves)) |
| `permission` | Regras globais `allow`/`ask`/`deny` ([Ferramentas e permissões](#ferramentas-e-permissões)) |
| `mcp` | Servidores MCP ([Servidores MCP](#servidores-mcp)) |
| `connectors` | `file`, `url`, `database`, `email` ([Conectores](#conectores)) |
| `rag` | `store_path`, `embedder` (`local` ou um provedor com embeddings), `chunk_size`, `chunk_overlap`, `top_k`, `min_score` ([RAG](#rag)) |
| `shell` | Comando da ferramenta `bash` (padrão: PowerShell no Windows, `/bin/sh` nos demais) |
| `theme` | Cores da interface: `dark` (padrão), `light` ou `auto` |
| `server.addr` | Endereço da interface web (padrão `127.0.0.1:8080`) |

`timeout_seconds`: com streaming é o tempo máximo **sem receber dados** (cobre o carregamento de modelos grandes; padrão 120, Ollama 300); sem streaming é o limite total. Todas as seções novas são opcionais: arquivos de versões antigas continuam válidos.


> O Claude não oferece API de embeddings. Para RAG use `local`, `ollama` ou `gemini` em `rag.embedder` — o Claude continua sendo o modelo de chat.

## RAG

1. Documentos são divididos em chunks (preferindo quebras de parágrafo/linha/palavra).
2. Cada chunk é vetorizado pelo embedder configurado e salvo em `rag.store_path`.
3. A cada pergunta, os `top_k` trechos mais similares (cosseno ≥ `min_score`) entram no prompt de sistema, numerados para citação `[n]`.

Notas:
- Reingerir a mesma fonte substitui os chunks antigos (idempotente).
- O índice guarda qual embedder o gerou. Trocar `rag.embedder` exige `rag clear` e nova ingestão (o programa avisa).
- O embedder `local` é lexical (palavras em comum). Para busca semântica use um modelo de embeddings.

## Fontes de RAG (file e url)

Definem fontes que você sincroniza sob demanda com `connectors sync` (ou `/sync` na interface).

```bash
my-agent connectors add --name docs --type file --path ./docs --include "*.md,*.txt"
my-agent connectors add --name site --type url  --url https://exemplo.com/sobre
my-agent connectors sync            # todos; ou: connectors sync docs
```

- `file`: arquivo ou diretório. Ignora binários, arquivos > 5 MB, diretórios ocultos e `node_modules`. `include` usa padrões glob (padrão: tipos comuns de texto e código). PDF e Office não são lidos: converta para texto antes.
- `url`: baixa uma página; HTML é convertido em texto.

## Interface web

`my-agent serve` abre a UI com abas **Chat**, **Configuração**, **RAG** e **Conectores** (editar parâmetros, testar providers, ingerir, buscar, sincronizar) e um **seletor de idioma** no cabeçalho.

Segurança: a UI **não tem autenticação** e escuta em `127.0.0.1` por padrão. Ela pode ler caminhos locais (ingestão) e usar suas chaves de API; não a exponha em rede sem um proxy com autenticação. Requisições que alteram estado exigem `Content-Type: application/json`.

## Estrutura

```
cmd/my-agent/          ponto de entrada
internal/config/       modelo, validação, get/set por chave
internal/i18n/         traduções (en, pt-BR, es), detecção do idioma
internal/provider/     Claude, Gemini, Ollama, OpenAI-compatível (tool calling, rodízio de chaves)
internal/rag/          chunking, embedders, store vetorial, index
internal/connectors/   file, url, database, email
internal/agent/        agente + sessão de conversa; loop com ferramentas e permissões
internal/tools/        ferramentas (arquivos, shell, busca, web, RAG, banco, e-mail)
internal/mcp/          cliente MCP (stdio e Streamable HTTP)
internal/session/      sessões salvas
internal/tui/          interface em tela cheia (Bubble Tea)
internal/app/          composição (usada por CLI e web)
internal/cli/          comandos e chat interativo
internal/web/          API HTTP + UI embutida
scripts/               build, install (Windows/Linux/macOS) e package (pacotes de release)
docs/                  guia de configuração (en, pt-BR, es) e examples/
```

## Desenvolvimento

```bash
go test ./...                       # testes unitários (sem rede: providers usam servidores httptest)
go test -cover ./...                # com cobertura (-race exige CGO_ENABLED=1)
scripts/build.sh                    # vet + testes + build em bin/ (Windows: .\scripts\build.ps1)
scripts/package.sh                  # pacotes de todas as plataformas em dist/ (Windows: .\scripts\package.ps1)
```

A compilação é Go puro (`CGO_ENABLED=0`), então os pacotes de Windows, Linux e macOS saem de qualquer uma das três plataformas. O instalador de Linux pode ser testado num container limpo:

```bash
docker run --rm -v "$PWD/dist:/dist" ubuntu:24.04 sh -c 'cd /tmp && tar -xzf /dist/my-agent-*-linux-amd64.tar.gz && sh my-agent-*/install.sh && ~/.local/bin/my-agent version'
```

### Traduções

Todo texto visível vive no catálogo `internal/i18n/msgs_*.go`, com as **três traduções lado a lado** em cada chave:

```go
"cli.rag.no_results": {"no results", "nenhum resultado", "sin resultados"},
```

- Use `i18n.T("chave", args...)` (ou `i18n.Errorf` para erros com `%w`); nunca escreva frases direto no código.
- `go test ./internal/i18n` garante que **nenhuma tradução falta**, que os marcadores (`%s`, `%d`) batem entre os idiomas e **acusa qualquer frase nova escrita direto no código** (exceção explícita: comentário `// i18n:ok`).
- A interface web usa as chaves `web.ui.*` (o navegador as busca em `GET /api/i18n`); um teste confere que a página só usa chaves que existem.
- Corrigir uma tradução é editar a linha do catálogo. Para incluir um idioma novo, acrescente uma coluna em `m` e um valor em `Lang`/`Supported`/`Names` (o compilador e os testes apontam o que falta).

Veja o [CHANGELOG](CHANGELOG.md) (em português) para o histórico de versões.
