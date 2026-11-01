# my-agent

[![Sponsor](https://img.shields.io/static/v1?label=Sponsor&message=%E2%9D%A4&logo=GitHub&color=%23fe8e86)](https://github.com/sponsors/ramonmachadocarmo)

**English** · [Português (Brasil)](README.pt-BR.md) · [Español](README.es.md)

An AI agent for the terminal in the style of **opencode**, written in **Go**, with everything configurable: type `my-agent` and a full-screen interface opens where the agent reads and edits files, runs commands, queries databases and e-mail, uses **MCP** servers and your knowledge base (**RAG**). It talks to **Claude**, **Gemini**, **Ollama** and any **OpenAI-compatible** API (OpenAI, OpenRouter, Groq, DeepSeek, Mistral, xAI, LM Studio…), with **several keys per provider**. Available in **English, Brazilian Portuguese and Spanish**.

- **Interactive interface** (TUI) with saved sessions, agent switching with `Tab`, a model picker, a command palette and permission prompts.
- Configurable **agents** (`build`, `plan`, `chat` and your own), each with its own prompt, model, tools and permissions.
- **Tools**: `read`, `write`, `edit`, `bash`, `glob`, `grep`, `ls`, `webfetch`, `rag_search`, plus **database**, **e-mail** and **MCP** tools.
- `allow` / `ask` / `deny` **permissions** per tool, with per-command rules (`"bash:git status*": "allow"`).
- **Several API keys** per provider, rotated automatically when one is rejected or rate limited, and retries on overload (503/529).
- A single binary (the web UI is still embedded); works offline with Ollama.

## Install

The installers put `my-agent` on the PATH and set up tab completion in your terminals, with no administrator rights. After installing, **open a new terminal** and type `my-agent`. Running them again updates.

### From source (this repository)

Needs [Go 1.26+](https://go.dev/dl). From the **repository root**, the installer builds and installs:

```powershell
# Windows (PowerShell) — or double-click scripts\install.cmd
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\install.ps1
```
```bash
# Linux and macOS
sh scripts/install.sh
```

### From a release archive (no Go)

Build the archives with `.\scripts\package.ps1` (or `scripts/package.sh`); they go to `dist/`, one per system (`windows`/`linux`/`darwin`, `amd64`/`arm64`). Extract yours and run the installer **from inside the extracted folder**:

```powershell
# Windows — or double-click install.cmd
Expand-Archive my-agent-<version>-windows-amd64.zip -DestinationPath .; cd my-agent-<version>-windows-amd64
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1
```
```bash
# Linux and macOS
tar -xzf my-agent-<version>-linux-amd64.tar.gz && cd my-agent-<version>-linux-amd64
sh install.sh
```

### What the installer does

| | Windows (`install.ps1`) | Linux and macOS (`install.sh`) |
|---|---|---|
| Binary | `%LOCALAPPDATA%\Programs\my-agent\my-agent.exe` | `~/.local/bin/my-agent` (`--system`: `/usr/local/bin`, with sudo) |
| PATH | user PATH (works in PowerShell, cmd, Windows Terminal and Git Bash) | `~/.bashrc`, `~/.zshrc`, `~/.profile`, fish, pwsh |
| Completion | PowerShell 5.1 and 7 profiles and Git Bash's `~/.bashrc` | bash, zsh, fish and pwsh |
| Remove | `-Uninstall` (`-Purge` also deletes configuration and sessions) | `--uninstall` (`--purge` likewise) |
| Other options | `-InstallDir DIR`, `-NoShell`, `-Yes` | `--bin-dir DIR`, `--no-shell` |

- Profiles get a block between `# >>> my-agent >>>` and `# <<< my-agent <<<`; reinstalling replaces it and uninstalling removes it.
- If PowerShell blocks profile scripts, the installer asks whether to allow them for your user (`RemoteSigned`).
- On macOS the installer removes the quarantine flag of the downloaded binary.
- Build only, without installing: `.\scripts\build.ps1` or `scripts/build.sh` (writes `bin/`).

## Quick start

```bash
my-agent                           # opens the interactive agent in the current folder
```

On the first run, when there is no configuration, one is created at `%APPDATA%\my-agent\my-agent.json` (Windows) or `~/.config/my-agent/my-agent.json`. Inside the interface:

1. `/connect` → choose the provider (claude, gemini, openai, openrouter, groq, ollama…) and paste the key (typing is hidden).
2. `ctrl+o` → choose the model (the list comes from every provider).
3. Ask for what you need: *"run the tests and fix what fails"*. When the agent wants to edit a file or run a command, you answer `y` (once), `a` (always, this session) or `n` (deny).

```bash
my-agent --continue                # resumes the latest session of this folder
my-agent --agent plan              # starts with the read-only agent
my-agent --model gemini/gemini-2.5-pro
my-agent run "summarize the README"               # no interface: prints the reply (for scripts)
my-agent run --yes "format the code with gofmt"   # allows what would ask for permission
```

## Interactive interface

```
  ┃ run the tests and fix what fails

  ✓ ⚡ bash go test ./...
  ✓ 📖 read internal/app/app.go · 104 lines
  ✓ ✏️ edit internal/app/app.go
      - return nil
      + return err
  Fixed the error return in app.go; the tests pass.
  build · claude/claude-sonnet-5 · 12.4s · 8.1k↑ 312↓
╭──────────────────────────────────────────────────────────────╮
│ Ask anything… (/ commands, @ files, tab switches agent)      │
╰──────────────────────────────────────────────────────────────╯
  BUILD  claude/claude-sonnet-5   mcp 2/2         tab agents · ctrl+o models · ctrl+p commands
```

| Key | Action |
|---|---|
| `enter` / `alt+enter` or `ctrl+j` | send / new line |
| `tab` / `shift+tab` | next / previous agent |
| `ctrl+o` | model picker (every provider) |
| `ctrl+p` | command palette |
| `ctrl+s` / `ctrl+n` | saved sessions / new session |
| `↑` `↓` | previous messages (or move in the autocomplete list) |
| `pgup` `pgdn`, mouse wheel | scroll the conversation |
| `esc` | stop the reply |
| `ctrl+c` | stop; with an empty line, quit |

Commands (`/` opens the autocomplete; `/help` lists everything):

| Command | What it does |
|---|---|
| `/new`, `/sessions`, `/export [file]` | new session, resume/delete (`ctrl+x`) sessions, save the conversation as Markdown |
| `/models [provider/model]`, `/agents [name]` | switch model or agent |
| `/connect [preset]`, `/keys [provider]` | add a provider and key; list keys or add one more |
| `/tools`, `/mcp`, `/connectors` | the agent's tools and permissions; MCP server status; connectors |
| `/sync <connector\|all>`, `/ingest <path\|url>`, `/rag` | feed and inspect the knowledge base |
| `/config`, `/config get\|set KEY [VALUE]`, `/reload` | open the configuration in the editor (`$EDITOR`, or Notepad) and reload; read/write one key |

`@path` in a message attaches the file (text up to 100 KB; `Tab` completes paths). When the folder has an `AGENTS.md` (or `CLAUDE.md`), it is added to the agent's instructions. Sessions live in `%LOCALAPPDATA%\my-agent\sessions` (or `~/.local/share/my-agent/sessions`; change it with `MY_AGENT_DATA`).

## Agents

Three agents are built in; you can change any of their fields or add new agents in the `agents` section:

| Agent | Purpose |
|---|---|
| `build` (default) | full access: reads, edits, runs commands |
| `plan` | read-only: investigates and proposes a plan (no `write`, `edit`, `bash`, `db_exec`, `email_send`) |
| `chat` | conversation with the knowledge base, no tools |

```json
"default_agent": "build",
"agents": {
  "plan":   { "provider": "gemini", "model": "gemini-2.5-pro" },
  "review": {
    "description": "Reviews the changes without touching files",
    "provider": "claude",
    "instructions": "Run git diff and report bugs, most severe first.",
    "tools": { "write": false, "edit": false },
    "permission": { "bash": "deny", "bash:git diff*": "allow" },
    "color": "#f7768e"
  }
}
```

Fields (all optional; what is missing comes from the built-in agent of the same name, then from the `agent` section): `description`, `system_prompt`, `instructions` (appended to the prompt), `provider`, `model`, `temperature`, `max_tokens`, `use_rag`, `max_steps` (default 25), `tools` (name or glob → `true`/`false`), `permission`, `color`, `disabled`. `my-agent agents` lists them.

## Tools and permissions

| Tool | Default permission |
|---|---|
| `read`, `glob`, `grep`, `ls`, `webfetch`, `rag_search`, `db_query`, `db_schema`, `email_search`, `email_read` | `allow` |
| `write`, `edit`, `bash`, `db_exec`, `email_send`, `mcp_*` | `ask` |

Rules go in `permission` (global) or `agents.<name>.permission`: the key is a tool name or glob; for `bash`, `bash:<pattern>` matches the command line. The most specific rule wins.

```json
"permission": {
  "bash:git status*": "allow",
  "bash:go test*": "allow",
  "bash:rm *": "deny",
  "webfetch": "deny"
}
```

A tool set to `deny` is not even offered to the model. The "always" answer (`a`) lasts until the end of the session; for `bash` it covers the same program and subcommand (`go test …`). `bash` uses PowerShell on Windows and `/bin/sh` elsewhere; change it with `"shell": "pwsh -NoProfile -Command"` (or `"bash -c"`).

## Providers and several keys

```bash
my-agent providers add openrouter --key sk-or-...      # from a preset
my-agent providers add groq --key gsk_... --model llama-3.3-70b-versatile
my-agent providers add my-vllm --preset openai --base-url http://gpu:8000/v1 --model qwen
my-agent providers add openrouter --key sk-or-second   # a second key goes to api_keys
my-agent providers list
```

Presets: `claude`, `gemini`, `ollama`, `openai`, `openrouter`, `groq`, `deepseek`, `mistral`, `xai`, `together`, `lmstudio`. The `openai` type works with any compatible API (`base_url` includes the version, e.g. `https://api.openai.com/v1`).

Each provider accepts several keys: `api_key`, the `api_keys` list (`"env:NAME"` items read variables) and the variable named in `api_key_env`. When a key is rejected (401/403), out of credit (402) or rate limited (429), the next one is used right away and the one that worked becomes current. Temporary failures (429, 5xx, 529) are retried after 1 s, 2 s and 4 s. `headers` adds HTTP headers and `models` lists extra models in the picker.

## MCP servers

```json
"mcp": {
  "github":   { "type": "local",  "command": ["npx", "-y", "@modelcontextprotocol/server-github"],
                "environment": { "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_TOKEN}" } },
  "context7": { "type": "remote", "url": "https://mcp.context7.com/mcp", "headers": { "Authorization": "Bearer ${CTX7_KEY}" } },
  "browser":  { "type": "local",  "command": ["npx", "-y", "@playwright/mcp@latest"], "enabled": false }
}
```

`local` servers run as a subprocess (stdio); `remote` ones use Streamable HTTP. `${VAR}` in `environment` and `headers` reads environment variables. Tools show up as `mcp_<server>_<tool>` (default permission `ask`). Servers start in the background when the interface opens (`mcp 2/2` in the status bar); `/mcp` or `my-agent mcp` show their state and tools.

## Connectors

`file` and `url` connectors feed the knowledge base; `database` and `email` also become **agent tools**:

```json
"connectors": [
  { "name": "docs", "type": "file", "path": "./docs", "include": ["*.md"], "recursive": true },
  { "name": "crm", "type": "database",
    "database": { "driver": "postgres", "dsn_env": "CRM_DATABASE_URL",
                  "query": "SELECT name, email, notes FROM customers" } },
  { "name": "app", "type": "database", "database": { "driver": "sqlite", "dsn": "./data/app.db", "allow_write": true } },
  { "name": "work", "type": "email",
    "email": { "imap_host": "imap.gmail.com", "smtp_host": "smtp.gmail.com",
               "username": "me@example.com", "password_env": "MAIL_APP_PASSWORD" } }
]
```

- **database** (`sqlite`, `postgres`, `mysql`): tools `db_schema` and `db_query` (always in a read-only transaction) and, with `allow_write: true`, `db_exec` (asks first). `query` chooses the rows that `/sync` or `connectors sync` put into the RAG. Use `dsn_env` to keep passwords out of the file.
- **email** (IMAP + SMTP): `email_search`, `email_read` and, with `smtp_host`, `email_send` (asks first). IMAP on port 993 (TLS) or 143 (STARTTLS); SMTP on 587 (STARTTLS) or 465 (TLS). With Gmail/Outlook use an *app password*. `sync` ingests the latest `limit` messages (default 50).
- **file** and **url**: see [RAG sources](#rag-sources-file-and-url) below.

A complete file is in [docs/examples/my-agent.full.json](docs/examples/my-agent.full.json).

## Language

The program speaks **English (`en`)**, **Brazilian Portuguese (`pt-BR`)** and **Spanish (`es`)**: messages, errors, help, configuration validation, the web UI and the default prompt written by `init`. By default it **detects the system language**.

```bash
my-agent --lang es chat                  # this run only
MY_AGENT_LANG=pt-BR my-agent help        # via environment variable
my-agent config set language es          # permanent (auto goes back to detecting)
my-agent --lang es init                  # creates the agent already in Spanish
```

- Order of choice: `--lang` → `$MY_AGENT_LANG` → `language` in the config file → system language → English. Aliases work (`pt`, `pt_BR`, `es-MX`…).
- **In chat:** `/config language es` (or `/lang es`) switches instantly, including the help tree and autocomplete.
- **In the interactive interface:** `/config set language es` switches at once; `--lang` applies to that run only.
- **In the web UI:** language selector in the header (saves `language`; `Automatic` follows the browser).
- Commands, flags and configuration keys are **not** translated (scripts and docs work in any language), and model replies follow the `system_prompt` and the language you write in.

### Streaming replies

The interactive interface, `run`, `chat`, `ask` and the web UI show the reply **as the model writes it**, with every provider (Claude, Gemini, Ollama and OpenAI-compatible), also while the agent is using tools. To stop it: `esc` in the interactive interface, `Ctrl+C` in `chat` (at the prompt, it exits) and the **Stop** button on the web. If a large model takes long to load and times out, raise the idle limit:

```powershell
my-agent config set providers.ollama.timeout_seconds 600
```

### If Ollama is slow

```powershell
my-agent models ps      # what is loaded and how much sits on GPU vs CPU
```

The most common cause is the model **not fitting in GPU memory**: Ollama splits it between GPU and CPU and generation becomes several times slower (`models ps` shows, for example, `40% GPU / 60% CPU`; `chat` also warns after the first reply). What to do, most to least effective:

1. **Use a smaller** or more heavily quantized model (`my-agent models select`). Rule of thumb: the model size must fit in free VRAM, with room for the context.
2. **Free VRAM**: browsers, launchers (Steam, GOG), overlays and Windows itself use GPU memory. Check with `nvidia-smi`.
3. **Reduce the context**: `my-agent config set providers.ollama.num_ctx 2048` (less VRAM for the context cache).
4. On the Ollama server, enable `OLLAMA_FLASH_ATTENTION=1` and `OLLAMA_KV_CACHE_TYPE=q8_0` (shrinks context memory; restart Ollama).

To avoid reloading the model after every pause, the program uses `providers.ollama.keep_alive` (default `30m`; `-1` keeps it loaded) and preloads the model when `chat` opens.

### API keys

The easiest way is inside the interface: `/connect` picks the provider and asks for the key with hidden typing; `/keys <provider>` adds one more. From the command line:

```bash
my-agent providers add openrouter --key sk-or-...     # creates the provider (preset) with the key
my-agent providers add openrouter --key sk-or-other   # the second one goes to api_keys (rotation)
my-agent providers list                               # shows how many keys each one has
my-agent providers test                               # tests all of them for real
```

To keep keys out of the file, use environment variables — the name goes in `api_key_env` (and `"env:NAME"` in `api_keys`):

| Provider | Default variable |
|---|---|
| Claude | `ANTHROPIC_API_KEY` |
| Gemini | `GEMINI_API_KEY` |
| OpenAI / OpenRouter / Groq / DeepSeek / Mistral / xAI / Together | `OPENAI_API_KEY`, `OPENROUTER_API_KEY`, `GROQ_API_KEY`, `DEEPSEEK_API_KEY`, `MISTRAL_API_KEY`, `XAI_API_KEY`, `TOGETHER_API_KEY` |

```powershell
[Environment]::SetEnvironmentVariable('ANTHROPIC_API_KEY', 'sk-ant-...', 'User')   # PowerShell, permanent
```
```bash
echo 'export ANTHROPIC_API_KEY=sk-ant-...' >> ~/.bashrc                           # bash (or ~/.zshrc)
```

Keys saved in the file are protected (mode `0600`) and masked in `config show`, `/config get` and the web UI. Local Ollama and LM Studio need no key: install Ollama and run `ollama pull llama3.2` (chat) and `ollama pull nomic-embed-text` (embeddings).

## Commands

| Command | Description |
|---|---|
| *(no command)* `[--agent A] [--model P/M] [--continue] [--session ID]` | Open the interactive agent (full screen) |
| `run [--agent A] [--model P/M] [--yes] MESSAGE` | Run the agent with tools once, without the interface |
| `agents` · `mcp` · `sessions [list \| remove ID]` | List agents; connect to the MCP servers and list their tools; saved sessions |
| `init [--force]` | Creates the default configuration file |
| `config show \| get KEY \| set KEY VALUE \| path \| validate` | Inspect and edit the configuration |
| `providers list \| test [name] \| add NAME [--preset P] [--key K] [--model M] [--base-url URL]` | List providers / check connectivity / add a provider or one more key |
| `chat [--provider P] [--no-rag]` | Interactive conversation (`/` for commands, `@file` to attach) |
| `ask [--provider P] [--no-rag] [--sources] QUESTION` | Single question |
| `models list \| ps \| select \| use NAME \| pull NAME` | Ollama models: list, show what is loaded (GPU/CPU), choose (downloading if missing) and configure. Flags: `--provider P`, `--embedding` |
| `rag ingest [--recursive=BOOL] [--include=*.md,*.txt] PATH\|URL...` | Indexes files, folders or pages |
| `rag search [-k N] QUERY` | Searches the knowledge base |
| `rag list \| remove SOURCE \| clear` | Manages the knowledge base |
| `connectors list \| add \| remove NAME \| sync [NAME...]` | Manages connectors (`add` creates `file` and `url`; `database` and `email` are configured in the JSON) |
| `serve [--addr HOST:PORT]` | Starts the web UI |
| `completion bash\|zsh\|fish\|powershell` | Completion script |
| `version` | Version |

Global flags: `--config FILE` (or `-c`) and `--lang LANGUAGE`. Config resolution order: flag → `$MY_AGENT_CONFIG` → `./my-agent.json` → per-user file (`%APPDATA%\my-agent\my-agent.json` or `~/.config/my-agent/my-agent.json`).
Exit codes: `0` ok, `1` runtime error, `2` incorrect usage.

### Inline chat (`my-agent chat`): `/` commands, autocomplete and `@file`

`chat` is the classic conversation, without full screen and without tools (RAG only). You never need to leave it to configure anything. Type **`/`** and the command list appears; **Tab** completes, **↑/↓** choose, **Enter** completes the highlighted item and **Esc** closes it. Type **`help`** (or `?`, or `/help`) to see everything as a numbered tree:

```
 1.    config       Configuration and parameters
    1.1   chat          Agent             provider · model · temperature · max-tokens · prompt · rag
    1.2   rag           Search and index  embedder · top-k · min-score · chunk-size · chunk-overlap · store
    1.3   provider <name> <field> [value]
    1.4   language · show · get · set · validate · path
 2.    rag          Knowledge base (ingest · search · list · remove · clear)
 3.    models       Ollama models (list · ps · select · use · pull)
 4.    providers    list · test · use
 5.    connectors   list · add · remove · sync
 6.    chat         Conversation (reset · sources · model · clear · exit)
```

Examples, all **without leaving the conversation** and effective from the next message (history is kept):

```text
/config chat temperature 0.7            # creativity
/config chat prompt "Always answer concisely."
/config chat model qwen2.5:7b           # on Ollama, downloads if missing
/config chat rag off                    # turns the knowledge base on/off
/config rag top-k 6
/config provider gemini api_key         # asks for the key with hidden input
/config language es                     # switches language instantly
/models select                          # menu of installed models (+ "Other…")
/providers use claude                   # switches provider for this conversation only
/rag ingest ./docs                      # indexes and uses it from the next question
/help config chat                       # details one branch of the tree
```

- Everything the CLI has (`config`, `rag`, `models`, `providers`, `connectors`) exists here under the same name.
- Tab also completes **values**: provider names, installed models, configuration keys and RAG sources. A unique prefix is accepted (`/he` = `/help`) and ambiguity is reported.
- **`@file`** in any message attaches the file's content (text, up to 100 KB; up to 5 per message): `explain the error in @logs/app.log`. Tab completes paths. Only the text of the question is used for the RAG search.
- Enter sends; `Ctrl+C` interrupts the reply (or clears the line; on an empty line it exits); `Ctrl+D` exits; ↑/↓ walk the line history.
- On terminals without interactive mode (pipe, mintty) commands and `@file` work the same; only the autocomplete list is not shown.

### Ollama models (list, download and configure)

```powershell
my-agent models list                       # installed models (marks the current one)
my-agent models select                     # menu: pick a number, or 0 / type a new name
my-agent models use qwen2.5:7b             # uses it; downloads first if not installed
my-agent models use --embedding bge-m3     # sets the embeddings model
my-agent models pull mistral               # download only, config untouched
```

After you choose, `select` **opens the chat right away** with the configured model (`--no-chat` disables that). The chat uses colors, a banner, a spinner and markdown when the output is a terminal; in a pipe or with `NO_COLOR=1` it prints plain text.

`select` shows the installed models plus the **0) Other...** option. Any name that is not installed is downloaded (with progress) and stored in `providers.<name>.model` (or `embedding_model` with `--embedding`). Put flags **before** the model name. It applies to providers of type `ollama` (`--provider` picks which; the default is `agent.provider`).

In the web UI, the same lives on each Ollama provider's card in the **Configuration** tab.

### Autocomplete

The installers already set this up. To do it by hand (there are also `completion zsh` and `completion fish`):

```bash
source <(my-agent completion bash)        # bash (put it in ~/.bashrc)
```
```powershell
my-agent completion powershell | Out-String | Invoke-Expression   # PowerShell (put it in $PROFILE)
```

## Configuration

> 📘 **Full guide:** [docs/CONFIGURATION.md](docs/CONFIGURATION.md) ([Português](docs/CONFIGURACAO.md) · [Español](docs/CONFIGURACION.md)): every field, accepted values, providers, RAG, connectors, language, ready-made recipes and common errors.

The configuration is **one JSON file**. Which file is used: `--config` → `$MY_AGENT_CONFIG` → `./my-agent.json` in the current folder → per-user file (`%APPDATA%\my-agent\my-agent.json` or `~/.config/my-agent/my-agent.json`, created on the first run). So a project can have its own configuration while everything else uses the global one; `my-agent config path` shows which is in use.

Ways to edit it:

| Where | How |
|---|---|
| Interactive interface | `/config` opens the file in the editor (`$EDITOR`, or Notepad) and reloads it when you close it; `/config get\|set KEY [VALUE]`; `/connect`, `/keys`, `/models`, `/agents` |
| Command line | `my-agent config show\|get\|set\|validate\|path`, `providers add`, `connectors add`, `models use` |
| Inline chat | `/config chat …`, `/config rag …`, `/config provider …` |
| Web UI | `my-agent serve` → **Configuration** tab |
| By hand | edit the JSON and run `my-agent config validate` (or `/reload` in the interface) |

```bash
my-agent config set agent.provider claude
my-agent config set providers.claude.model claude-sonnet-5
my-agent config set default_agent plan
my-agent config set rag.embedder ollama      # semantic embeddings via Ollama
```

| Section | Purpose |
|---|---|
| `language` | Message language: `auto` (default), `en`, `pt-BR` or `es` |
| `agent` | Base values of every agent: `name`, `system_prompt`, `provider`, `temperature` (0–2), `max_tokens`, `use_rag` |
| `default_agent`, `agents` | Starting agent and agent definitions ([Agents](#agents)) |
| `providers.<name>` | `type` (`claude`, `gemini`, `ollama`, `openai`), `model`, `base_url`, `api_key` / `api_key_env` / `api_keys`, `headers`, `models`, `embedding_model`, `timeout_seconds` and, for Ollama, `keep_alive` / `num_ctx` ([Providers and several keys](#providers-and-several-keys)) |
| `permission` | Global `allow`/`ask`/`deny` rules ([Tools and permissions](#tools-and-permissions)) |
| `mcp` | MCP servers ([MCP servers](#mcp-servers)) |
| `connectors` | `file`, `url`, `database`, `email` ([Connectors](#connectors)) |
| `rag` | `store_path`, `embedder` (`local` or a provider with embeddings), `chunk_size`, `chunk_overlap`, `top_k`, `min_score` ([RAG](#rag)) |
| `shell` | Command of the `bash` tool (default: PowerShell on Windows, `/bin/sh` elsewhere) |
| `theme` | Interface colors: `dark` (default), `light` or `auto` |
| `server.addr` | Web UI address (default `127.0.0.1:8080`) |

`timeout_seconds`: with streaming it is the longest time **without receiving data** (covers slow loading of big models; default 120, Ollama 300); without streaming it is the total limit. Every new section is optional: files from older versions stay valid.


> Claude has no embeddings API. For RAG use `local`, `ollama` or `gemini` in `rag.embedder` — Claude remains the chat model.

## RAG

1. Documents are split into chunks (preferring paragraph/line/word breaks).
2. Each chunk is vectorized by the configured embedder and saved in `rag.store_path`.
3. On each question, the `top_k` most similar passages (cosine ≥ `min_score`) go into the system prompt, numbered for `[n]` citation.

Notes:
- Re-ingesting the same source replaces the old chunks (idempotent).
- The index records which embedder produced it. Changing `rag.embedder` requires `rag clear` and a new ingestion (the program warns you).
- The `local` embedder is lexical (shared words). For semantic search use an embeddings model.

## RAG sources (file and url)

They define sources that you sync on demand with `connectors sync` (or `/sync` in the interface).

```bash
my-agent connectors add --name docs --type file --path ./docs --include "*.md,*.txt"
my-agent connectors add --name site --type url  --url https://example.com/about
my-agent connectors sync            # all of them; or: connectors sync docs
```

- `file`: a file or directory. Skips binaries, files > 5 MB, hidden directories and `node_modules`. `include` uses glob patterns (default: common text and code types). PDF and Office files are not read: convert them to text first.
- `url`: downloads a page; HTML is converted to text.

## Web UI

`my-agent serve` opens the UI with **Chat**, **Configuration**, **RAG** and **Connectors** tabs (edit parameters, test providers, ingest, search, sync) and a **language selector** in the header.

Security: the UI has **no authentication** and listens on `127.0.0.1` by default. It can read local paths (ingestion) and use your API keys; do not expose it on a network without an authenticating proxy. Requests that change state require `Content-Type: application/json`.

## Structure

```
cmd/my-agent/          entry point
internal/config/       model, validation, get/set by key
internal/i18n/         translations (en, pt-BR, es), language detection
internal/provider/     Claude, Gemini, Ollama, OpenAI-compatible (tool calling, key rotation)
internal/rag/          chunking, embedders, vector store, index
internal/connectors/   file, url, database, email
internal/agent/        agent + conversation session; loop with tools and permissions
internal/tools/        tools (files, shell, search, web, RAG, database, e-mail)
internal/mcp/          MCP client (stdio and Streamable HTTP)
internal/session/      saved sessions
internal/tui/          full-screen interface (Bubble Tea)
internal/app/          composition (used by CLI and web)
internal/cli/          commands and interactive chat
internal/web/          HTTP API + embedded UI
scripts/               build, install (Windows/Linux/macOS) and package (release archives)
docs/                  configuration guide (en, pt-BR, es) and examples/
```

## Development

```bash
go test ./...                       # unit tests (no network: providers use httptest servers)
go test -cover ./...                # with coverage (-race needs CGO_ENABLED=1)
scripts/build.sh                    # vet + tests + build into bin/ (Windows: .\scripts\build.ps1)
scripts/package.sh                  # archives of every platform into dist/ (Windows: .\scripts\package.ps1)
```

The build is pure Go (`CGO_ENABLED=0`), so the Windows, Linux and macOS archives can be built from any of the three. The Linux installer can be tested in a clean container:

```bash
docker run --rm -v "$PWD/dist:/dist" ubuntu:24.04 sh -c 'cd /tmp && tar -xzf /dist/my-agent-*-linux-amd64.tar.gz && sh my-agent-*/install.sh && ~/.local/bin/my-agent version'
```

### Translations

All user-visible text lives in the catalog `internal/i18n/msgs_*.go`, with the **three translations side by side** on each key:

```go
"cli.rag.no_results": {"no results", "nenhum resultado", "sin resultados"},
```

- Use `i18n.T("key", args...)` (or `i18n.Errorf` for errors with `%w`); never write sentences directly in the code.
- `go test ./internal/i18n` guarantees that **no translation is missing**, that placeholders (`%s`, `%d`) match across languages, and it **flags any new sentence written directly in the code** (explicit exception: a `// i18n:ok` comment).
- The web UI uses the `web.ui.*` keys (the browser fetches them from `GET /api/i18n`); a test checks that the page only uses keys that exist.
- Fixing a translation means editing the catalog line. To add a new language, add a column to `m` and a value in `Lang`/`Supported`/`Names` (the compiler and the tests point out what is missing).

See the [CHANGELOG](CHANGELOG.md) (in Portuguese) for the version history.
