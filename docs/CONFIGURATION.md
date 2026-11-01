# Configuration guide

**English** · [Português (Brasil)](CONFIGURACAO.md) · [Español](CONFIGURACION.md)

Everything in my-agent is controlled by **a single JSON file**. This guide shows what can be configured, what each field does, the accepted values and ready-made recipes.

- [1. Where the file lives](#1-where-the-file-lives)
- [2. Ways to configure](#2-ways-to-configure)
- [3. Field reference](#3-field-reference)
  - [language](#language) · [agent](#agent) · [providers](#providers) · [rag](#rag) · [connectors](#connectors) · [server](#server)
- [4. Providers in detail](#4-providers-in-detail)
- [5. RAG in detail](#5-rag-in-detail)
- [6. Connectors in detail](#6-connectors-in-detail)
- [7. Environment variables](#7-environment-variables)
- [8. Language](#8-language)
- [9. Recipes](#9-recipes)
- [10. Validation and common errors](#10-validation-and-common-errors)
- [11. Agents, tools, MCP and data connectors](#11-agents-tools-mcp-and-data-connectors)

---

## 1. Where the file lives

The program looks for the configuration in this order:

| Priority | Source | Example |
|---|---|---|
| 1 | `--config` flag (or `-c`) | `my-agent --config C:\agents\sales.json chat` |
| 2 | `MY_AGENT_CONFIG` environment variable | `$env:MY_AGENT_CONFIG = 'C:\agents\sales.json'` |
| 3 | `my-agent.json` in the current folder | — |
| 4 | Per-user file: `%APPDATA%\my-agent\my-agent.json` (Windows) or `~/.config/my-agent/my-agent.json` | created automatically the first time you run `my-agent` without a configuration |

This lets you have **several agents**, each with its own file (prompt, model and knowledge base).

```powershell
my-agent init                         # creates my-agent.json with the default values
my-agent --config sales.json init     # creates another agent
my-agent config path                  # shows which file is in use
```

Details:
- The file is saved with `0600` permission (it may hold API keys).
- **Relative** paths inside the file (such as `rag.store_path`) are resolved from the **folder of the configuration file**, not from the folder where you run the command.
- The JSON is **strict**: unknown fields are rejected (no silent typos). Comments are not allowed.
- Files created by older versions stay valid; missing new fields take the default value described below.

---

## 2. Ways to configure

### 2.0 In the interactive interface (`my-agent`)

| I want to | Command |
|---|---|
| Add a provider and its key | `/connect` (list of presets; the key is typed hidden) |
| Add one more key to a provider | `/keys <provider>`; `/keys` lists how many keys each one has |
| Change this session's model | `ctrl+o` or `/models provider/model` |
| Switch agents | `tab` / `shift+tab` or `/agents <name>` |
| Edit the whole file | `/config` (opens it in `$EDITOR`, or Notepad, and reloads it when closed) |
| Read or write one key | `/config get rag.top_k`, `/config set agents.plan.model gemini-2.5-pro` |
| Apply edits made elsewhere | `/reload` (also restarts the MCP servers) |

`/connect` and `/keys` write to the file; the model choice (`ctrl+o`) lasts for the session. To make it the default: `/config set agents.build.model <model>` (or `providers.<name>.model`).


### 2.1 Command line (`config`)

```powershell
my-agent config show                       # full config (API keys masked)
my-agent config get rag.top_k              # one value
my-agent config set rag.top_k 6            # changes and saves
my-agent config set agent.use_rag false    # booleans: true/false
my-agent config set agent.temperature 0.7  # numbers
my-agent config validate                   # checks the file
```

`set` uses a **dotted key** (`section.field` or `providers.<name>.field`) and converts the text to the field's type. Rules:

- It only changes **simple values** (text, number, boolean). Lists (`connectors`) have their own commands (`connectors add/remove`).
- It creates the optional fields `default_agent`, `shell`, `theme` and `agents.<name>.<field>` (the agent is created when missing). New providers: `providers add`. Lists and maps (`api_keys`, `mcp`, `permission`, database/e-mail `connectors`): edit the JSON.
- The result is validated **before** saving: if it would be invalid, nothing is written and the error explains why.

Specific commands that also change the configuration:

| Command | What it changes |
|---|---|
| `my-agent models select` / `models use NAME` | `providers.<ollama>.model` (downloads the model if missing) |
| `my-agent models use --embedding NAME` | `providers.<ollama>.embedding_model` |
| `my-agent connectors add/remove` | the `connectors` list |
| `my-agent providers add NAME [--preset P] [--key K] [--model M] [--base-url URL]` | creates `providers.<name>` from a preset, or adds a key (`api_key`, then `api_keys`) |

### 2.1.1 Inside the chat (without leaving the conversation)

Everything the CLI configures can also be done **inside `my-agent chat`**, with commands that start with `/`. Type `/` to see the list (Tab completes, ↑/↓ choose) or `help` for the numbered help. Changes are **saved to the file** and apply **to the next message**, keeping the history.

| I want to change | In the chat | JSON key |
|---|---|---|
| Default provider | `/config chat provider claude` | `agent.provider` |
| Model of the current provider | `/config chat model qwen2.5:7b` | `providers.<n>.model` |
| Temperature | `/config chat temperature 0.7` | `agent.temperature` |
| Reply length | `/config chat max-tokens 2048` | `agent.max_tokens` |
| Agent instructions | `/config chat prompt "You are..."` | `agent.system_prompt` |
| RAG on/off | `/config chat rag on` / `off` | `agent.use_rag` |
| Embedder | `/config rag embedder ollama` | `rag.embedder` |
| Chunks per question | `/config rag top-k 6` | `rag.top_k` |
| Minimum similarity | `/config rag min-score 0.2` | `rag.min_score` |
| Chunk size / overlap | `/config rag chunk-size 1000`, `chunk-overlap 150` | `rag.chunk_size`, `rag.chunk_overlap` |
| A provider's field | `/config provider gemini keep_alive 1h` | `providers.<n>.<field>` |
| API key | `/config provider gemini api_key` (hidden input) | `providers.<n>.api_key` |
| Language | `/config language es` | `language` |
| Any other key | `/config set <key> <value>` | any scalar |

- Without a value, the command **shows the current one** (`/config chat temperature` → `agent.temperature = 0.2`).
- `/config show`, `get`, `validate` and `path` work as in the CLI. Invalid values are refused and **nothing is written**.
- `/config provider <name> <field>`: fields `model`, `base_url`, `api_key`, `api_key_env`, `embedding_model`, `timeout_seconds`, `keep_alive`, `num_ctx`.
- **API key**: use `/config provider <name> api_key` **with no value**: the key is asked for with hidden typing (`*`), so it appears neither on screen nor in the terminal history. Typing it on the line itself works, but the program warns that it would stay in the history. `api_key_env` only accepts a variable **name** and never shows a rejected value.
- `/providers use <name>` switches provider **for this conversation only** (not saved); `/config chat provider <name>` saves.
- Changed `rag.embedder`? The chat warns that the old index was built by another embedder (`/rag clear` and re-ingest). Changed `chunk-size`/`overlap`? It applies to the next ingestions.
- The same goes for `/rag ingest|search|list|remove|clear`, `/models list|ps|select|use|pull` and `/connectors list|add|remove|sync`: same names and options as the CLI.

**Attaching files:** `@path` in any message sends the file's content along (text, up to 100 KB, at most 5 per message). Tab completes paths. Folders and binaries produce a warning; for a whole folder use `/rag ingest`.

### 2.2 Editing the JSON

Open the file in any editor (in the interactive interface: `/config`). It is the way to lists and maps — complete `agents`, `permission`, `mcp`, `api_keys`, `headers`, database and e-mail connectors — and to reorder things. An example with everything: [examples/my-agent.full.json](examples/my-agent.full.json). Run `my-agent config validate` after editing.

### 2.3 Web interface

```powershell
my-agent serve            # http://127.0.0.1:8080
```

In the **Configuration** tab each section becomes a form. **Save configuration** validates and writes the file. Saved API keys show masked (`********`): leave it that way to **keep them**; clear the value to **remove them**. Each provider has a **Test** button, and Ollama providers have the model selector (list, download and use). The **language selector** in the header changes the interface language and stores it in `language`.

> Changes to the server address (`server.addr`) only apply after restarting `serve`.

---

## 3. Field reference

### Complete file with the default values

```json
{
  "language": "auto",
  "agent": {
    "name": "my-agent",
    "system_prompt": "You are a helpful assistant. Answer concisely and say when you do not know.",
    "provider": "ollama",
    "temperature": 0.2,
    "max_tokens": 1024,
    "use_rag": true
  },
  "providers": {
    "claude": {
      "type": "claude",
      "model": "claude-sonnet-5",
      "base_url": "https://api.anthropic.com",
      "api_key": "",
      "api_key_env": "ANTHROPIC_API_KEY",
      "embedding_model": "",
      "timeout_seconds": 120,
      "keep_alive": "",
      "num_ctx": 0
    },
    "gemini": {
      "type": "gemini",
      "model": "gemini-2.5-flash",
      "base_url": "https://generativelanguage.googleapis.com",
      "api_key": "",
      "api_key_env": "GEMINI_API_KEY",
      "embedding_model": "gemini-embedding-001",
      "timeout_seconds": 120,
      "keep_alive": "",
      "num_ctx": 0
    },
    "ollama": {
      "type": "ollama",
      "model": "llama3.2",
      "base_url": "http://localhost:11434",
      "api_key": "",
      "api_key_env": "",
      "embedding_model": "nomic-embed-text",
      "timeout_seconds": 300,
      "keep_alive": "30m",
      "num_ctx": 0
    }
  },
  "rag": {
    "store_path": "data/rag-store.json",
    "embedder": "local",
    "chunk_size": 800,
    "chunk_overlap": 100,
    "top_k": 4,
    "min_score": 0.05
  },
  "connectors": [],
  "server": {
    "addr": "127.0.0.1:8080"
  }
}
```

### language

| Field | Type | Default | Rule | What it is for |
|---|---|---|---|---|
| `language` | text | `auto` | `auto`, `en`, `pt-BR` or `es` (aliases such as `pt`, `pt_BR`, `es-MX` are stored as `pt-BR` / `es`) | Language of the messages, help and web interface. `auto` follows the system. See [Language](#8-language) |

### agent

The agent's identity and behavior.

| Field | Type | Default | Rule | What it is for |
|---|---|---|---|---|
| `name` | text | `my-agent` | — | Name shown in the chat and the interface |
| `system_prompt` | text | (concise assistant) | — | Fixed instructions sent in every conversation: role, tone, language, rules. `init` writes it in the selected language |
| `provider` | text | `ollama` | must exist in `providers` | Provider used by default (`--provider` switches only for that run) |
| `temperature` | number | `0.2` | 0 to 2 | Creativity: near 0 = consistent replies; near 1+ = more varied |
| `max_tokens` | integer | `1024` | > 0 | Maximum length of each reply (in tokens) |
| `use_rag` | boolean | `true` | — | Consults the knowledge base by default (`--no-rag` turns it off per run) |

Tips for the `system_prompt`:
- State the language and tone you want (`Always answer in English, concisely.`). The reply language comes from this prompt and from what the user writes, not from the `language` setting.
- With RAG, ask it to cite sources and to admit when the context does not cover the question. The program already appends the retrieved chunks numbered as `[1]`, `[2]` and the instruction to cite them.

### providers

A map of **name → provider**. The name is free (`claude`, `gemini`, `ollama`, `ollama-server`…); the **type** decides how to talk to it. You can have several providers of the same type.

| Field | Type | Default | Rule | What it is for |
|---|---|---|---|---|
| `type` | text | — | `claude`, `gemini`, `ollama` or `openai` | Selects the API (`openai` = any compatible API: OpenAI, OpenRouter, Groq, DeepSeek, Mistral, xAI, LM Studio, vLLM…) |
| `model` | text | by type | required | Chat model |
| `base_url` | text | by type | required | API address (change it to use a proxy or a remote Ollama) |
| `api_key` | text | empty | — | Literal key. **Takes priority** over `api_key_env` |
| `api_key_env` | text | by type | a variable name (letters, digits and `_`) | The **name** of the environment variable that holds the key (recommended). It is not the key: if you paste the key here the program warns you and hides it when displaying |
| `embedding_model` | text | by type | — | Embeddings model (Gemini and Ollama) |
| `timeout_seconds` | integer | 120 (Ollama: 300) | ≥ 0 | See "Timeouts" below |
| `keep_alive` | text | `30m` on Ollama | duration (`30m`, `1h`), seconds or `-1` | **Ollama only.** How long the model stays in memory after use. Empty = `30m`; `-1` = forever |
| `num_ctx` | integer | `0` | ≥ 0 | **Ollama only.** Context window. `0` = Ollama's default. Smaller = less VRAM |
| `api_keys` | list of text | empty | — | Extra keys, used in **rotation**: when one is rejected (401/403), out of credit (402) or rate limited (429), the next one is used. `"env:NAME"` reads the key from a variable |
| `headers` | map | empty | — | Extra HTTP headers (e.g. `X-Title` for OpenRouter) |
| `models` | list of text | empty | — | Extra models offered in the interface's picker (`ctrl+o`) |

`keep_alive` and `num_ctx` appear on every provider in the file, but **only Ollama uses them**.

### rag

How the knowledge base is built and queried. See [section 5](#5-rag-in-detail).

| Field | Type | Default | Rule | What it is for |
|---|---|---|---|---|
| `store_path` | text | `data/rag-store.json` | required | Index file (relative to the config file) |
| `embedder` | text | `local` | `local` or a provider name | Who generates the vectors |
| `chunk_size` | integer | `800` | > 0 | Size of each chunk, in **characters** |
| `chunk_overlap` | integer | `100` | ≥ 0 and < `chunk_size` | Characters repeated between neighboring chunks |
| `top_k` | integer | `4` | > 0 | How many chunks go into the prompt |
| `min_score` | number | `0.05` | -1 to 1 | Minimum (cosine) similarity for a chunk to count |

### connectors

List of syncable data sources. Each item:

| Field | Type | Rule | What it is for |
|---|---|---|---|
| `name` | text | required, unique | Identifies the connector |
| `type` | text | `file`, `url`, `database` or `email` | Kind of source (`database` and `email`: see [section 11](#11-agents-tools-mcp-and-data-connectors)) |
| `path` | text | required if `file` | File or folder |
| `url` | text | required if `url`; starts with `http://` or `https://` | Page to download |
| `include` | list of text | — | File patterns (`*.md`); empty = common types |
| `recursive` | boolean | — | Descends into subfolders |
| `database` / `email` | object | required for those types | Settings of the database or e-mail account |

Manage them from the CLI (`connectors add/remove/list/sync`) or the **Connectors** tab.

### server

| Field | Type | Default | What it is for |
|---|---|---|---|
| `addr` | text | `127.0.0.1:8080` | Address of the web interface. `--addr` on `serve` overrides it |

---

## 4. Providers in detail

### Claude (Anthropic)

- Key: `ANTHROPIC_API_KEY` (or `api_key`).
- Model: any valid API ID, e.g. `claude-sonnet-5`.
- **It has no embeddings API.** It can be the **chat** model, but `rag.embedder` must be another one (`local`, `ollama` or `gemini`).

### Gemini (Google)

- Key: `GEMINI_API_KEY` (or `api_key`).
- Chat: `model` (e.g. `gemini-2.5-flash`). Embeddings: `embedding_model` (e.g. `gemini-embedding-001`).

### Ollama (local or remote)

- No key. `base_url` defaults to `http://localhost:11434`; point it at another machine if you prefer.
- Model management from the program itself:

  ```powershell
  my-agent models list                    # installed
  my-agent models select                  # menu: choose or type another (downloads if missing) and opens the chat
  my-agent models use qwen2.5:7b          # uses it, downloading if needed
  my-agent models use --embedding bge-m3  # sets the embeddings model
  my-agent models ps                      # what is loaded and how much is on the GPU
  ```
- **Performance** (the model must fit in the free VRAM; otherwise part of it runs on the CPU and gets slow):
  - `keep_alive`: keeps the model loaded between messages (default `30m`; Ollama's own is 5 min).
  - `num_ctx`: lower it (`2048`) to save VRAM.
  - Use `models ps` to see `100% GPU` or `40% GPU / 60% CPU`.

### Several and extra providers

Each provider's name is up to you, and there may be several of the same type. The quickest way is to start from a preset:

```powershell
my-agent providers add openrouter --key sk-or-...
my-agent providers add ollama-server --preset ollama --base-url http://192.168.0.10:11434 --model qwen2.5:14b
my-agent providers add my-vllm --preset openai --base-url http://gpu:8000/v1 --model qwen2.5-coder
```

Presets: `claude`, `gemini`, `ollama`, `openai`, `openrouter`, `groq`, `deepseek`, `mistral`, `xai`, `together`, `lmstudio` (also in `/connect`). Then: `my-agent config set agent.provider ollama-server`, `my-agent --model ollama-server/qwen2.5:14b` or `ctrl+o` in the interface.

### OpenAI-compatible (`type: "openai"`)

Works with any service or server that speaks the Chat Completions API: OpenAI, OpenRouter, Groq, DeepSeek, Mistral, xAI, Together, LM Studio, vLLM, llama.cpp server…

- `base_url` includes the version: `https://api.openai.com/v1`, `https://openrouter.ai/api/v1`, `http://localhost:1234/v1`.
- Servers on `localhost`/`127.0.0.1` need no key.
- `embedding_model` uses `/embeddings` (e.g. `text-embedding-3-small`), so the provider can also be the `rag.embedder`.
- `headers` adds headers (OpenRouter uses `HTTP-Referer` and `X-Title` to identify the app).
- The model picker lists what the server's `/models` returns; `models` adds fixed names.
- The model must support *tool calling* for the `build`/`plan` agents; without it, use the `chat` agent.

### API keys — how to provide them

| Way | How | Note |
|---|---|---|
| In the interface | `/connect` or `/keys <provider>` | Hidden typing; saved in the file |
| Environment variable (recommended) | Set the variable named in `api_key_env` | The key is not stored in a file |
| Several keys | `api_keys: ["sk-2", "env:OTHER_VAR"]` or `providers add NAME --key ...` again | Automatic rotation (below) |
| Key in the file | `my-agent config set providers.claude.api_key sk-ant-...` | File with mode `0600`; masked in `config show` and in the interface |

```powershell
# PowerShell — this window only
$env:ANTHROPIC_API_KEY = 'sk-ant-...'
# PowerShell — permanent for your user (open a new terminal afterwards)
[Environment]::SetEnvironmentVariable('ANTHROPIC_API_KEY', 'sk-ant-...', 'User')
```
```bash
# bash / zsh
echo 'export ANTHROPIC_API_KEY=sk-ant-...' >> ~/.bashrc
```

**Rotation and retries.** Keys are used in this order: `api_key`, `api_keys`, the variable in `api_key_env` (duplicates are ignored). When the current key is rejected (401/403), out of credit (402) or rate limited (429), the next one is tried at once, and the one that works stays current until the program exits. If every key fails with a temporary error (408, 429, 5xx, 529 — "model overloaded"), the call is repeated after 1 s, 2 s and 4 s before the error is shown.

`my-agent providers list` shows how many keys each provider has; `my-agent providers test [name]` makes a real test call.


### Timeouts

`timeout_seconds` has two meanings:

- **With streaming (interactive interface, `run`, `chat`, `ask`, web):** maximum time **without receiving any data**. A reply that is flowing is never cut off; the limit covers the slow loading of big models. If a big model takes long to start, raise it: `my-agent config set providers.ollama.timeout_seconds 600`.
- **Other calls (no streaming, such as embeddings and listings):** total time of the request.

`0` uses 120 s.

---

## 5. RAG in detail

**What it is:** the agent consults your documents before answering. Flow: documents are split into chunks → each chunk becomes a vector (embedding) → for each question, the `top_k` most similar chunks go into the prompt.

### Choosing the `embedder`

| Value | Needs | Quality | When to use |
|---|---|---|---|
| `local` | nothing (offline) | **Lexical**: compares shared words | Quick start, few documents, tests |
| `ollama` | a downloaded embeddings model (`nomic-embed-text`) | **Semantic** | Real use, private data, no cost |
| `gemini` | `GEMINI_API_KEY` | **Semantic** | If you already use Gemini |
| another provider's name | that it has an `embedding_model` | Semantic | Extra providers |

> Claude **cannot** be an embedder. If you set `"embedder": "claude"` the file passes validation, but ingestion fails saying the provider does not support embeddings.

**Changing the embedder invalidates the index.** The index remembers who built it; if you change it the program refuses to mix them and asks for `my-agent rag clear` and a new ingestion.

```powershell
my-agent config set rag.embedder ollama
my-agent rag clear
my-agent rag ingest .\docs
```

### Tuning quality

| Symptom | Adjustment |
|---|---|
| Answers ignore relevant chunks | Raise `top_k` (6–8) and/or lower `min_score` |
| Too many irrelevant chunks | Raise `min_score` (0.2–0.4 with a semantic embedder) and/or lower `top_k` |
| Chunks cut ideas in half | Raise `chunk_size` (1000–1500) and `chunk_overlap` (150–200) |
| Chunks too generic | Lower `chunk_size` (400–600) |
| Prompt too big / slow | Lower `top_k` and `chunk_size` |

After changing `chunk_size` or `chunk_overlap`, **re-ingest** the documents for it to apply to what is already in the index. The `local` embedder's `min_score` tends to be low (0.05–0.3); semantic embedders' is higher.

### Operation

```powershell
my-agent rag ingest .\docs .\manuals\guide.md https://site.com/faq       # folders, files, URLs
my-agent rag search "how do I cancel an order"                           # see what would be retrieved
my-agent rag list                                                        # sources and chunk counts
my-agent rag remove C:\docs\old.md                                       # removes one source
my-agent rag clear                                                       # wipes everything
my-agent ask --sources "question"                                        # shows the sources used
```

Re-ingesting the same source **replaces** its old chunks (it does not duplicate).

---

## 6. Connectors in detail

A connector is a **saved source** that you sync whenever you want (`connectors sync`), instead of repeating the path on every `rag ingest`.

### file

```powershell
my-agent connectors add --name docs --type file --path C:\docs --include "*.md,*.txt" --recursive=true
```

- Points to a **file** (the `include` filter is ignored) or a **folder**.
- `include` uses glob patterns applied to the **file name**. Default when empty: `*.md *.txt *.rst *.go *.py *.js *.ts *.json *.yaml *.yml *.csv *.html`.
- `recursive`: descends into subfolders. Hidden folders (`.git`…) and `node_modules` are always skipped.
- Skipped: **binary** files, files **larger than 5 MB** and unreadable files.
- HTML is converted to text.
- **PDF and Office files (`.pdf`, `.docx`, `.xlsx`) are not read**: they are binary and therefore skipped. Convert them to text or Markdown first (for example by exporting as `.txt`) and ingest the result.

### url

```powershell
my-agent connectors add --name faq --type url --url https://example.com/faq
```

- Downloads **one page** (30 s timeout, 5 MB limit). HTML becomes text without scripts/styles.
- For several sites, create one connector per URL.

### Syncing

```powershell
my-agent connectors sync          # all
my-agent connectors sync docs faq # only these
my-agent connectors list
my-agent connectors remove docs
```

Syncing **does not delete** files that disappeared from the folder: use `rag remove` for them.

---

## 7. Environment variables

| Variable | Effect |
|---|---|
| `MY_AGENT_CONFIG` | Path of the configuration file (the `--config` flag has priority) |
| `MY_AGENT_LANG` | Language for this run: `en`, `pt-BR`, `es` (or `auto`). Overrides the config's `language`; `--lang` overrides it |
| `ANTHROPIC_API_KEY` | Claude key (default name in `api_key_env`) |
| `GEMINI_API_KEY` | Gemini key (default name in `api_key_env`) |
| `NO_COLOR` | Any value turns off colors and decoration in the CLI chat (plain-text output) |
| `MY_AGENT_DATA` | Folder of the saved sessions (default `%LOCALAPPDATA%\my-agent` or `~/.local/share/my-agent`) |
| `OPENAI_API_KEY`, `OPENROUTER_API_KEY`, `GROQ_API_KEY`… | Default `api_key_env` names of providers created by `providers add` / `/connect` |

The names `ANTHROPIC_API_KEY`/`GEMINI_API_KEY` are only defaults: change `api_key_env` to use another variable.

---

## 8. Language

my-agent speaks **English**, **Portuguese (Brazil)** and **Spanish**: messages, errors, the `/help` tree, flag descriptions, configuration validation, the web interface and the default prompt written by `init`.

### How the language is chosen

The first of these that names a supported language wins:

| Priority | Source | Example |
|---|---|---|
| 1 | `--lang` flag | `my-agent --lang es chat` |
| 2 | `MY_AGENT_LANG` environment variable | `$env:MY_AGENT_LANG = 'pt-BR'` |
| 3 | `language` in the config file | `"language": "es"` |
| 4 | The system's language (Windows display language, or `LC_ALL`, `LC_MESSAGES`, `LANGUAGE`, `LANG` on Linux/macOS) | — |
| 5 | English | — |

`auto` (the default) means "use the system's language". Aliases are accepted: `pt`, `pt_BR`, `pt-PT` → Portuguese (Brazil); `es-MX`, `es_AR`… → Spanish; `en-GB`, `en_US.UTF-8`… → English. An unsupported `--lang` fails with an error; an unsupported environment value is ignored.

### Changing it

| Where | How |
|---|---|
| For one run | `my-agent --lang pt-BR ...` or `MY_AGENT_LANG=es my-agent ...` |
| Permanently (CLI) | `my-agent config set language es` (`auto` goes back to detecting) |
| Inside the chat | `/config language pt-BR` (or `/lang pt-BR`): applies immediately, including the help tree and the autocomplete list |
| Web interface | the selector in the header (saves `language`); `Automatic` follows the browser's language |
| At creation | `my-agent --lang es init` stores `"language": "es"` and writes the prompt in Spanish |

### What is not translated

- **Commands, flags, configuration keys and JSON values** stay as typed (`config set rag.top_k`, `--no-rag`, `keep_alive`…), so scripts and documentation work in any language.
- **The model's replies:** they follow your `system_prompt` and the language you write in. `language` does not translate what the model says.
- Status text coming from Ollama itself while downloading (`pulling manifest`, …) and the instruction that is appended to the prompt together with the retrieved chunks (it is written for the model, in English).
- The existing agent's `system_prompt`: `init` writes the prompt in the chosen language, but a file that already exists keeps its text. Change it with `/config chat prompt "..."`.

---

## 9. Recipes

### A. 100% local and private (Ollama)

```powershell
my-agent init
my-agent models select                        # choose/download the chat model
my-agent models use --embedding nomic-embed-text
my-agent config set rag.embedder ollama
my-agent config set rag.min_score 0.25
my-agent rag ingest .\docs
my-agent chat
```

### B. Claude for chat + local embeddings

```powershell
$env:ANTHROPIC_API_KEY = 'sk-ant-...'
my-agent config set agent.provider claude
my-agent config set rag.embedder ollama       # or gemini; Claude does not generate embeddings
my-agent providers test claude
```

### C. Everything on Gemini

```powershell
$env:GEMINI_API_KEY = '...'
my-agent config set agent.provider gemini
my-agent config set rag.embedder gemini
```

### D. An agent with its own personality and rules

```powershell
my-agent config set agent.name "Legal Assistant"
my-agent config set agent.system_prompt "You are a legal assistant. Answer in English, cite sources as [n] and say clearly when the context does not cover the question."
my-agent config set agent.temperature 0.1
```

### E. Big model on a small GPU

```powershell
my-agent models ps                             # check: 100% GPU?
my-agent config set providers.ollama.num_ctx 2048
my-agent config set providers.ollama.keep_alive 1h
```
If it still shows CPU: close programs that use the GPU or pick a smaller model (`models select`).

### F. Several agents, each with its own file

```powershell
my-agent --config sales.json init
my-agent --config sales.json config set agent.name "Sales"
my-agent --config sales.json config set rag.store_path "data/sales.json"
my-agent --config sales.json chat
```

Each file has its own `rag.store_path`, so the knowledge bases do not mix.

### G. Web interface reachable only from your machine (default) or on another port

```powershell
my-agent serve --addr 127.0.0.1:9000
```

> The interface has **no authentication**. Keep `127.0.0.1`. Do not use `0.0.0.0` without an authenticating proxy in front: whoever reaches it can read local paths (ingestion) and use your keys.

### H. OpenRouter and Groq with two keys each

```powershell
my-agent providers add openrouter --key sk-or-personal
my-agent providers add openrouter --key sk-or-work         # goes to api_keys: used when the first hits its limit
my-agent providers add groq --key gsk_... --model llama-3.3-70b-versatile
my-agent --model openrouter/anthropic/claude-sonnet-4.5
```

### I. Review agent that only reads and only runs `git`

```json
"agents": {
  "review": {
    "description": "Reviews the changes",
    "instructions": "Run git diff and report bugs and risks, most severe first. Change nothing.",
    "tools": { "write": false, "edit": false },
    "permission": { "bash": "deny", "bash:git diff*": "allow", "bash:git log*": "allow", "bash:git status*": "allow" },
    "color": "#f7768e"
  }
}
```

In the interface, `tab` to `REVIEW` and ask *"review my changes"*.

### J. GitHub through MCP

```json
"mcp": {
  "github": { "type": "local", "command": ["npx", "-y", "@modelcontextprotocol/server-github"],
              "environment": { "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_TOKEN}" } }
},
"permission": { "mcp_github_list_*": "allow", "mcp_github_get_*": "allow" }
```

Reads are allowed; creating issues, PRs etc. still asks first (`mcp_*` is `ask`). Check it with `my-agent mcp`.

### K. E-mail and CRM assistant

```json
"connectors": [
  { "name": "mail", "type": "email", "email": { "imap_host": "imap.gmail.com", "smtp_host": "smtp.gmail.com", "username": "me@example.com", "password_env": "MAIL_APP_PASSWORD" } },
  { "name": "crm", "type": "database", "database": { "driver": "postgres", "dsn_env": "CRM_DATABASE_URL" } }
],
"agents": {
  "secretary": {
    "description": "E-mail and customers",
    "instructions": "Help with e-mail and customer data. Always show the draft and wait for confirmation before sending.",
    "tools": { "*": false, "email_*": true, "db_query": true, "db_schema": true, "rag_search": true }
  }
}
```

Example: *"which unread e-mails come from customers whose contract ends this month?"*.

### L. Everything local with LM Studio

```powershell
my-agent providers add lmstudio --model qwen2.5-coder-14b-instruct     # base_url http://localhost:1234/v1, no key
my-agent config set agents.build.provider lmstudio
```


---

## 10. Validation and common errors

`config validate`, `config set` and the interface's Save button report **all** problems at once, with the field path. The messages below are shown in English; in the other languages the text after the field path is translated.

| Message | Cause | Fix |
|---|---|---|
| `agent.provider: "x" is not a configured provider` | The default provider does not exist in `providers` | Create the provider or use an existing one |
| `providers.<n>.type: "x" is not one of claude, gemini, ollama, openai` | Invalid type | Use one of the four (`openai` for compatible APIs) |
| `connectors[i].type: "x" is not one of file, url, database, email` | Invalid connector type | Use one of the four |
| `providers.<n>.model: required` / `base_url: required` | Required field empty | Fill it in |
| `providers.<n>.keep_alive: "..." is not a duration` | Invalid format | Use `30m`, `1h`, seconds (`600`) or `-1` |
| `rag.embedder: must be "local" or a configured provider` | Unknown embedder | Use `local` or a provider's name |
| `rag.chunk_overlap: must be >= 0 and < chunk_size` | Overlap larger than the chunk | Lower the overlap |
| `agent.temperature: must be between 0 and 2` | Out of range | Adjust |
| `connectors[i].path: required for type file` | `file` connector without a path | Provide `--path` |
| `language: "x" is not supported (use auto, en, pt-BR or es)` | Unsupported language | Use `auto`, `en`, `pt-BR` or `es` |
| `invalid config: json: unknown field "x"` | Field misspelled in the JSON | Fix the name |
| `provider "claude": missing API key (set providers.claude.api_key or env ANTHROPIC_API_KEY)` | No key | Define the environment variable or `api_key` |
| `warning: providers.<n>.api_key_env is not a valid environment variable name` | You pasted the **key** in `api_key_env`, which expects a variable **name** | `my-agent config set providers.<n>.api_key_env GEMINI_API_KEY` and define the variable; or put the key in `api_key`. Since the key was exposed in the file, consider generating a new one |
| `store was built with a different embedder` | You changed `rag.embedder` with a full index | `rag clear` and re-ingest |
| `provider does not support embeddings` | Embedder points to Claude (or a provider without `embedding_model`) | Use `local`, `ollama` or `gemini` |
| `no data received for Ns` | The model stayed silent beyond `timeout_seconds` | Raise `timeout_seconds`; check `models ps` |
| `config ... not found; run my-agent init first` | No file at the resolved path | `my-agent init` or use `--config` |
| `model 'x' not found` (Ollama) | Model not downloaded | `my-agent models use x` |
| `unknown agent "x"` | `--agent`, `default_agent` or `/agents` with a name that does not exist (or is `disabled`) | `my-agent agents` lists the agents |
| `agents.<n>.provider: provider "x" is not defined in providers` | An agent points to a missing provider | Create it (`providers add`) or fix the name |
| `permission.<rule>: invalid permission "x" (use allow, ask or deny)` | Wrong value in a rule | `allow`, `ask` or `deny` |
| `mcp.<n>.type: unknown type` / `mcp.<n>.url must start with http://` / `mcp.<n>.command: required` | Incomplete MCP server | `local` needs `command`; `remote` needs `url` |
| `connectors[i].database.driver: unknown driver` / `database.dsn: required` | Incomplete database connector | `sqlite`, `postgres` or `mysql`, plus `dsn` or `dsn_env` |
| `http 429` / `http 503` after a few attempts | Rate limit or overload on every key, even after the retries | Wait, add another key (`/keys`) or switch models (`ctrl+o`) |
| `the agent reached its step limit (max_steps)` | The task needed more calls than the agent allows | Send "continue", or raise `agents.<n>.max_steps` |
| `denied %s: it needs permission (use --yes to allow)` (in `run`) | Without the interface, what would ask is denied | `my-agent run --yes …` or an `allow` rule |
| `the interactive interface needs a terminal` | `my-agent tui` in a pipe/script | Use `my-agent run MESSAGE` |

Tip: `my-agent config path` shows which file is being read — a lot of "configuration that does not stick" is another file in the current folder.

---

## 11. Agents, tools, MCP and data connectors

These sections control the interactive agent (`my-agent` with no command) and `my-agent run`. All of them are optional: a file without them works with the built-in agents and the default permissions. Complete example: [examples/my-agent.full.json](examples/my-agent.full.json).

### default_agent and agents

| Field | Type | Default | Purpose |
|---|---|---|---|
| `default_agent` | text | `build` | Agent used when the interface opens (`--agent` overrides it) |
| `agents.<name>` | object | — | Creates an agent or changes a built-in one (`build`, `plan`, `chat`) |

Fields of each agent (all optional; what is missing comes from the built-in agent of the same name, then from the `agent` section):

| Field | Type | Purpose |
|---|---|---|
| `description` | text | Shown in the agent picker and in `my-agent agents` |
| `system_prompt` | text | Replaces `agent.system_prompt` |
| `instructions` | text | Appended to the prompt (`plan` uses it to change nothing) |
| `provider`, `model` | text | Provider (a name in `providers`) and model; without `model`, the provider's one |
| `temperature`, `max_tokens` | number | Generation parameters |
| `use_rag` | boolean | Automatic knowledge base search before each message |
| `max_steps` | integer | Model calls per message (default 25) |
| `tools` | map name/glob → boolean | Turns tools on/off: `{"*": false, "read": true}` |
| `permission` | map name/glob → `allow`\|`ask`\|`deny` | Rules for this agent only (override the global ones) |
| `color` | text `#rrggbb` | The agent's color in the interface |
| `disabled` | boolean | Hides the agent |

`Tab` switches agents in the interface. In the project folder, an `AGENTS.md` (or `CLAUDE.md`) is appended to the instructions.

### permission

Map of rule → `allow` (runs), `ask` (asks you) or `deny` (blocks; the tool is not even offered). The key is a tool name or glob (`mcp_*`, `email_*`); for `bash`, `bash:<pattern>` matches the command line (`bash:git status*`). The most specific rule wins; `agents.<name>.permission` overrides the global rules.

Defaults: everything `allow`, except `write`, `edit`, `bash`, `db_exec`, `email_send` and `mcp_*`, which are `ask`. In `my-agent run`, what would ask is denied unless you pass `--yes`.

### Tools

| Tool | What it does |
|---|---|
| `read` | Reads a text file with line numbers (`offset`, `limit`) |
| `write` | Creates or overwrites a file |
| `edit` | Replaces an exact snippet (handles CRLF files) |
| `bash` | Runs a shell command (`shell`; PowerShell on Windows, `/bin/sh` elsewhere by default), 120 s timeout (max 600) |
| `glob`, `grep`, `ls` | Find files by pattern (`**/*.go`), by content (regex) and list folders; skip `.git`, `node_modules` and the like |
| `webfetch` | Downloads a page (HTML becomes text) |
| `rag_search` | Searches the knowledge base (only offered when it has content) |
| `db_schema`, `db_query`, `db_exec` | `database` connectors |
| `email_search`, `email_read`, `email_send` | `email` connectors |
| `mcp_<server>_<tool>` | Tools of the MCP servers |

### mcp

Map of **name → server**:

| Field | Type | Purpose |
|---|---|---|
| `type` | `local` or `remote` | `local`: subprocess (stdio). `remote`: Streamable HTTP |
| `command` | list of text | **local**: program and arguments (`["npx", "-y", "package"]`) |
| `environment` | map | **local**: extra variables; `${VAR}` reads from your environment |
| `url` | text | **remote**: server address (`https://…/mcp`) |
| `headers` | map | **remote**: headers (e.g. `Authorization`); `${VAR}` works |
| `enabled` | boolean | `false` turns it off without deleting it (default `true`) |
| `timeout_seconds` | integer | Deadline of the initial connection (default 30) |

`my-agent mcp` connects to all of them and lists their tools; in the interface, `/mcp` shows their state and `/reload` restarts them after you edit the configuration.

### `database` connector

```json
{ "name": "crm", "type": "database", "database": { "driver": "postgres", "dsn_env": "CRM_DATABASE_URL" } }
```

| Field | Type | Purpose |
|---|---|---|
| `driver` | `sqlite`, `postgres` or `mysql` | Database (SQLite needs nothing installed) |
| `dsn` / `dsn_env` | text | Connection, or the name of the variable holding it. SQLite: file path. Postgres: `postgres://user:pass@host:5432/db`. MySQL: `user:pass@tcp(host:3306)/db` |
| `allow_write` | boolean | Offers `db_exec` (default `false`: read-only) |
| `query` | text | Rows put into the RAG by `sync` (50 rows per document) |
| `max_rows` | integer | Maximum rows per query (default 200) |

`db_query` always runs in a **read-only** transaction (Postgres/MySQL) or with `query_only` (SQLite): even a `DELETE` sent by mistake changes nothing.

### `email` connector

```json
{ "name": "work", "type": "email",
  "email": { "imap_host": "imap.gmail.com", "smtp_host": "smtp.gmail.com", "username": "me@example.com", "password_env": "MAIL_APP_PASSWORD" } }
```

| Field | Type | Default | Purpose |
|---|---|---|---|
| `imap_host`, `imap_port` | text, integer | —, 993 | Reading (993 = TLS; 143 = STARTTLS) |
| `smtp_host`, `smtp_port` | text, integer | —, 587 | Sending (587 = STARTTLS; 465 = TLS). Without `smtp_host` there is no `email_send` |
| `username` | text | — | User (usually the address) |
| `password` / `password_env` | text | — | Password, or the name of the variable holding it. Gmail/Outlook: use an *app password* |
| `from` | text | `username` | Sender |
| `folder` | text | `INBOX` | Default folder |
| `limit` | integer | 50 | Messages put into the RAG by `sync` |

### shell and theme

| Field | Default | Purpose |
|---|---|---|
| `shell` | PowerShell (Windows) / `/bin/sh -c` | Command used by `bash`; the agent's command is the last argument. E.g. `"pwsh -NoProfile -Command"`, `"bash -c"` |
| `theme` | `dark` | Interface colors: `dark`, `light` or `auto` (detects the terminal background) |

### Secrets

`config show`, `/config get` and the web UI mask `api_key`, `api_keys`, `headers`, `mcp.*.environment`, `mcp.*.headers`, `database.dsn` and `email.password`. Prefer the `*_env` / `env:NAME` / `${VAR}` forms so secrets never go into the file.
