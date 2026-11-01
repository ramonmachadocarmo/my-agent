# my-agent

[![Sponsor](https://img.shields.io/static/v1?label=Sponsor&message=%E2%9D%A4&logo=GitHub&color=%23fe8e86)](https://github.com/sponsors/ramonmachadocarmo)

[English](README.md) · [Português (Brasil)](README.pt-BR.md) · **Español**

Agente de IA para la terminal al estilo de **opencode**, escrito en **Go**, pero con todo configurable: escribe `my-agent` y se abre una interfaz de pantalla completa donde el agente lee y edita archivos, ejecuta comandos, consulta bases de datos y correo, usa servidores **MCP** y tu base de conocimiento (**RAG**). Se conecta con **Claude**, **Gemini**, **Ollama** y cualquier API **compatible con OpenAI** (OpenAI, OpenRouter, Groq, DeepSeek, Mistral, xAI, LM Studio…), con **varias claves por proveedor**. Disponible en **inglés, portugués (Brasil) y español**.

- **Interfaz interactiva** (TUI) con sesiones guardadas, cambio de agente con `Tab`, selector de modelos, paleta de comandos y confirmación de permisos.
- **Agentes** configurables (`build`, `plan`, `chat` y los tuyos), cada uno con su prompt, modelo, herramientas y permisos.
- **Herramientas**: `read`, `write`, `edit`, `bash`, `glob`, `grep`, `ls`, `webfetch`, `rag_search`, más las de **base de datos**, **correo** y **MCP**.
- **Permisos** `allow` / `ask` / `deny` por herramienta, con reglas por comando (`"bash:git status*": "allow"`).
- **Varias claves de API** por proveedor, con rotación automática cuando una es rechazada o alcanza el límite, y reintentos ante sobrecarga (503/529).
- Un único binario (la interfaz web sigue incluida); funciona sin conexión con Ollama.

## Instalación

Los instaladores ponen `my-agent` en el PATH y configuran el autocompletado (Tab) en tus terminales, sin administrador. Después de instalar, **abre una terminal nueva** y escribe `my-agent`. Ejecutarlos de nuevo actualiza.

### Desde el código fuente (este repositorio)

Necesita [Go 1.26+](https://go.dev/dl). En la **raíz del repositorio**, el instalador compila e instala:

```powershell
# Windows (PowerShell) — o haz doble clic en scripts\install.cmd
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\install.ps1
```
```bash
# Linux y macOS
sh scripts/install.sh
```

### Desde un paquete de release (sin Go)

Genera los paquetes con `.\scripts\package.ps1` (o `scripts/package.sh`); quedan en `dist/`, uno por sistema (`windows`/`linux`/`darwin`, `amd64`/`arm64`). Extrae el tuyo y ejecuta el instalador **desde dentro de la carpeta extraída**:

```powershell
# Windows — o haz doble clic en install.cmd
Expand-Archive my-agent-<versión>-windows-amd64.zip -DestinationPath .; cd my-agent-<versión>-windows-amd64
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1
```
```bash
# Linux y macOS
tar -xzf my-agent-<versión>-linux-amd64.tar.gz && cd my-agent-<versión>-linux-amd64
sh install.sh
```

### Qué hace el instalador

| | Windows (`install.ps1`) | Linux y macOS (`install.sh`) |
|---|---|---|
| Binario | `%LOCALAPPDATA%\Programs\my-agent\my-agent.exe` | `~/.local/bin/my-agent` (`--system`: `/usr/local/bin`, con sudo) |
| PATH | PATH del usuario (vale para PowerShell, cmd, Windows Terminal y Git Bash) | `~/.bashrc`, `~/.zshrc`, `~/.profile`, fish, pwsh |
| Autocompletado | perfiles de PowerShell 5.1 y 7 y `~/.bashrc` de Git Bash | bash, zsh, fish y pwsh |
| Eliminar | `-Uninstall` (`-Purge` borra también configuración y sesiones) | `--uninstall` (`--purge` igual) |
| Otras opciones | `-InstallDir CARPETA`, `-NoShell`, `-Yes` | `--bin-dir CARPETA`, `--no-shell` |

- Los perfiles reciben un bloque entre `# >>> my-agent >>>` y `# <<< my-agent <<<`; reinstalar lo reemplaza y desinstalar lo elimina.
- Si PowerShell bloquea los scripts de perfil, el instalador pregunta si puede permitirlos para tu usuario (`RemoteSigned`).
- En macOS el instalador quita la cuarentena del binario descargado.
- Solo compilar, sin instalar: `.\scripts\build.ps1` o `scripts/build.sh` (genera `bin/`).

## Inicio rápido

```bash
my-agent                           # abre el agente interactivo en la carpeta actual
```

En la primera ejecución, si no hay configuración, se crea una en `%APPDATA%\my-agent\my-agent.json` (Windows) o `~/.config/my-agent/my-agent.json`. Dentro de la interfaz:

1. `/connect` → elige el proveedor (claude, gemini, openai, openrouter, groq, ollama…) y pega la clave (no se muestra al escribir).
2. `ctrl+o` → elige el modelo (la lista viene de todos los proveedores).
3. Pide lo que necesites: *"ejecuta las pruebas y corrige lo que falle"*. Cuando el agente quiera editar un archivo o ejecutar un comando, respondes `y` (una vez), `a` (siempre en esta sesión) o `n` (denegar).

```bash
my-agent --continue                # reanuda la última sesión de esta carpeta
my-agent --agent plan              # empieza con el agente de solo lectura
my-agent --model gemini/gemini-2.5-pro
my-agent run "resume el README"    # sin interfaz: imprime la respuesta (para scripts)
my-agent run --yes "formatea el código con gofmt"   # permite lo que pediría confirmación
```

## Interfaz interactiva

```
  ┃ ejecuta las pruebas y corrige lo que falle

  ✓ ⚡ bash go test ./...
  ✓ 📖 read internal/app/app.go · 104 líneas
  ✓ ✏️ edit internal/app/app.go
      - return nil
      + return err
  Corregí el retorno de error en app.go; las pruebas pasan.
  build · claude/claude-sonnet-5 · 12.4s · 8.1k↑ 312↓
╭──────────────────────────────────────────────────────────────╮
│ Pregunte lo que quiera… (/ comandos, @ archivos, tab …)      │
╰──────────────────────────────────────────────────────────────╯
  BUILD  claude/claude-sonnet-5   mcp 2/2        tab agentes · ctrl+o modelos · ctrl+p comandos
```

| Tecla | Acción |
|---|---|
| `enter` / `alt+enter` o `ctrl+j` | enviar / nueva línea |
| `tab` / `shift+tab` | agente siguiente / anterior |
| `ctrl+o` | selector de modelos (todos los proveedores) |
| `ctrl+p` | paleta de comandos |
| `ctrl+s` / `ctrl+n` | sesiones guardadas / nueva sesión |
| `↑` `↓` | mensajes anteriores (o moverse en el autocompletado) |
| `pgup` `pgdn`, rueda del ratón | desplazar la conversación |
| `esc` | detener la respuesta |
| `ctrl+c` | detener; con la línea vacía, salir |

Comandos (`/` abre el autocompletado; `/help` lista todo):

| Comando | Qué hace |
|---|---|
| `/new`, `/sessions`, `/export [archivo]` | nueva sesión, reanudar/borrar (`ctrl+x`) sesiones, guardar la conversación en Markdown |
| `/models [proveedor/modelo]`, `/agents [nombre]` | cambiar de modelo o de agente |
| `/connect [preset]`, `/keys [proveedor]` | añadir proveedor y clave; listar claves o añadir otra |
| `/tools`, `/mcp`, `/connectors` | herramientas del agente y permisos; estado de los servidores MCP; conectores |
| `/sync <conector\|all>`, `/ingest <ruta\|url>`, `/rag` | alimentar y consultar la base de conocimiento |
| `/config`, `/config get\|set CLAVE [VALOR]`, `/reload` | abrir la configuración en el editor (`$EDITOR`, o el Bloc de notas) y recargar; leer/escribir una clave |

`@ruta` en un mensaje adjunta el archivo (texto hasta 100 KB; `Tab` completa rutas). Si la carpeta tiene un `AGENTS.md` (o `CLAUDE.md`), se añade a las instrucciones del agente. Las sesiones se guardan en `%LOCALAPPDATA%\my-agent\sessions` (o `~/.local/share/my-agent/sessions`; cámbialo con `MY_AGENT_DATA`).

## Agentes

Hay tres agentes incluidos; puedes cambiar cualquiera de sus campos o crear otros en la sección `agents`:

| Agente | Para qué |
|---|---|
| `build` (predeterminado) | acceso total: lee, edita, ejecuta comandos |
| `plan` | solo lectura: investiga y propone un plan (sin `write`, `edit`, `bash`, `db_exec`, `email_send`) |
| `chat` | conversación con la base de conocimiento, sin herramientas |

```json
"default_agent": "build",
"agents": {
  "plan":   { "provider": "gemini", "model": "gemini-2.5-pro" },
  "review": {
    "description": "Revisa los cambios sin tocar archivos",
    "provider": "claude",
    "instructions": "Ejecuta git diff y señala errores, del más grave al menos grave.",
    "tools": { "write": false, "edit": false },
    "permission": { "bash": "deny", "bash:git diff*": "allow" },
    "color": "#f7768e"
  }
}
```

Campos (todos opcionales; lo que falte viene del agente incluido del mismo nombre y luego de la sección `agent`): `description`, `system_prompt`, `instructions` (se añade al prompt), `provider`, `model`, `temperature`, `max_tokens`, `use_rag`, `max_steps` (predeterminado 25), `tools` (nombre o glob → `true`/`false`), `permission`, `color`, `disabled`. `my-agent agents` los lista.

## Herramientas y permisos

| Herramienta | Permiso predeterminado |
|---|---|
| `read`, `glob`, `grep`, `ls`, `webfetch`, `rag_search`, `db_query`, `db_schema`, `email_search`, `email_read` | `allow` |
| `write`, `edit`, `bash`, `db_exec`, `email_send`, `mcp_*` | `ask` |

Las reglas van en `permission` (global) o `agents.<nombre>.permission`: la clave es el nombre de la herramienta o un glob; para `bash`, `bash:<patrón>` coincide con el comando. Gana la regla más específica.

```json
"permission": {
  "bash:git status*": "allow",
  "bash:go test*": "allow",
  "bash:rm *": "deny",
  "webfetch": "deny"
}
```

Una herramienta con `deny` ni siquiera se ofrece al modelo. La respuesta "siempre" (`a`) vale hasta el final de la sesión; para `bash` vale para el mismo programa y subcomando (`go test …`). `bash` usa PowerShell en Windows y `/bin/sh` en los demás; cámbialo con `"shell": "pwsh -NoProfile -Command"` (o `"bash -c"`).

## Proveedores y varias claves

```bash
my-agent providers add openrouter --key sk-or-...      # desde un preset
my-agent providers add groq --key gsk_... --model llama-3.3-70b-versatile
my-agent providers add mi-vllm --preset openai --base-url http://gpu:8000/v1 --model qwen
my-agent providers add openrouter --key sk-or-segunda  # una segunda clave va a api_keys
my-agent providers list
```

Presets: `claude`, `gemini`, `ollama`, `openai`, `openrouter`, `groq`, `deepseek`, `mistral`, `xai`, `together`, `lmstudio`. El tipo `openai` sirve para cualquier API compatible (`base_url` incluye la versión, p. ej. `https://api.openai.com/v1`).

Cada proveedor acepta varias claves: `api_key`, la lista `api_keys` (los elementos `"env:NOMBRE"` leen variables) y la variable indicada en `api_key_env`. Si una clave es rechazada (401/403), se queda sin crédito (402) o alcanza el límite (429), se usa la siguiente al instante y la que funcionó pasa a ser la actual. Los fallos temporales (429, 5xx, 529) se reintentan tras 1 s, 2 s y 4 s. `headers` añade cabeceras HTTP y `models` lista modelos extra en el selector.

## Servidores MCP

```json
"mcp": {
  "github":   { "type": "local",  "command": ["npx", "-y", "@modelcontextprotocol/server-github"],
                "environment": { "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_TOKEN}" } },
  "context7": { "type": "remote", "url": "https://mcp.context7.com/mcp", "headers": { "Authorization": "Bearer ${CTX7_KEY}" } },
  "browser":  { "type": "local",  "command": ["npx", "-y", "@playwright/mcp@latest"], "enabled": false }
}
```

Los servidores `local` se ejecutan como subproceso (stdio); los `remote` usan Streamable HTTP. `${VAR}` en `environment` y `headers` lee variables de entorno. Las herramientas aparecen como `mcp_<servidor>_<herramienta>` (permiso predeterminado `ask`). Los servidores arrancan en segundo plano al abrir la interfaz (`mcp 2/2` en la barra de estado); `/mcp` o `my-agent mcp` muestran su estado y herramientas.

## Conectores

Los conectores `file` y `url` alimentan la base de conocimiento; `database` y `email` también se convierten en **herramientas del agente**:

```json
"connectors": [
  { "name": "docs", "type": "file", "path": "./docs", "include": ["*.md"], "recursive": true },
  { "name": "crm", "type": "database",
    "database": { "driver": "postgres", "dsn_env": "CRM_DATABASE_URL",
                  "query": "SELECT nombre, email, notas FROM clientes" } },
  { "name": "app", "type": "database", "database": { "driver": "sqlite", "dsn": "./data/app.db", "allow_write": true } },
  { "name": "trabajo", "type": "email",
    "email": { "imap_host": "imap.gmail.com", "smtp_host": "smtp.gmail.com",
               "username": "yo@ejemplo.com", "password_env": "MAIL_APP_PASSWORD" } }
]
```

- **database** (`sqlite`, `postgres`, `mysql`): herramientas `db_schema` y `db_query` (siempre en una transacción de solo lectura) y, con `allow_write: true`, `db_exec` (pide confirmación). `query` elige las filas que `/sync` o `connectors sync` llevan al RAG. Usa `dsn_env` para no guardar contraseñas en el archivo.
- **email** (IMAP + SMTP): `email_search`, `email_read` y, con `smtp_host`, `email_send` (pide confirmación). IMAP en el puerto 993 (TLS) o 143 (STARTTLS); SMTP en 587 (STARTTLS) o 465 (TLS). Con Gmail/Outlook usa una *contraseña de aplicación*. `sync` ingiere los últimos `limit` mensajes (predeterminado 50).
- **file** y **url**: consulta [Fuentes de RAG](#fuentes-de-rag-file-y-url) más abajo.

Hay un archivo completo en [docs/examples/my-agent.full.json](docs/examples/my-agent.full.json).

## Idioma

El programa habla **inglés (`en`)**, **portugués de Brasil (`pt-BR`)** y **español (`es`)**: mensajes, errores, ayuda, validación de la configuración, la interfaz web y el prompt por defecto que escribe `init`. Por defecto **detecta el idioma del sistema**.

```bash
my-agent --lang es chat                  # solo en esta ejecución
MY_AGENT_LANG=pt-BR my-agent help        # mediante variable de entorno
my-agent config set language es          # permanente (auto vuelve a detectar)
my-agent --lang es init                  # crea el agente ya en español
```

- Orden de elección: `--lang` → `$MY_AGENT_LANG` → `language` en el archivo de configuración → idioma del sistema → inglés. Los alias funcionan (`pt`, `pt_BR`, `es-MX`…).
- **En el chat:** `/config language es` (o `/lang es`) cambia al instante, incluido el árbol de ayuda y el autocompletado.
- **En la interfaz interactiva:** `/config set language es` cambia al instante; `--lang` vale solo para esa ejecución.
- **En la interfaz web:** selector de idioma en el encabezado (guarda `language`; `Automático` sigue al navegador).
- Los comandos, flags y claves de configuración **no** se traducen (los scripts y la documentación sirven en cualquier idioma), y las respuestas del modelo siguen el `system_prompt` y el idioma en que escribes.

### Respuestas en streaming

La interfaz interactiva, `run`, `chat`, `ask` y la interfaz web muestran la respuesta **a medida que el modelo la escribe**, con todos los proveedores (Claude, Gemini, Ollama y compatibles con OpenAI), también mientras el agente usa herramientas. Para detenerla: `esc` en la interfaz interactiva, `Ctrl+C` en `chat` (en el prompt, sale) y el botón **Detener** en la web. Si un modelo grande tarda en cargar y agota el tiempo, aumenta el límite de inactividad:

```powershell
my-agent config set providers.ollama.timeout_seconds 600
```

### Si Ollama va lento

```powershell
my-agent models ps      # qué está cargado y cuánto está en GPU frente a CPU
```

La causa más común es que el modelo **no cabe en la memoria de la GPU**: Ollama lo reparte entre GPU y CPU y la generación se vuelve varias veces más lenta (`models ps` muestra, por ejemplo, `40% GPU / 60% CPU`; `chat` también avisa tras la primera respuesta). Qué hacer, de más a menos efectivo:

1. **Usa un modelo más pequeño** o más cuantizado (`my-agent models select`). Regla práctica: el tamaño del modelo debe caber en la VRAM libre, con margen para el contexto.
2. **Libera VRAM**: navegadores, launchers (Steam, GOG), overlays y el propio Windows usan memoria de GPU. Compruébalo con `nvidia-smi`.
3. **Reduce el contexto**: `my-agent config set providers.ollama.num_ctx 2048` (menos VRAM para la caché de contexto).
4. En el servidor de Ollama, activa `OLLAMA_FLASH_ATTENTION=1` y `OLLAMA_KV_CACHE_TYPE=q8_0` (reduce la memoria del contexto; reinicia Ollama).

Para evitar recargar el modelo tras cada pausa, el programa usa `providers.ollama.keep_alive` (por defecto `30m`; `-1` lo mantiene cargado) y precarga el modelo al abrir `chat`.

### Claves de API

Lo más sencillo es desde la interfaz: `/connect` elige el proveedor y pide la clave sin mostrarla; `/keys <proveedor>` añade otra. Desde la línea de comandos:

```bash
my-agent providers add openrouter --key sk-or-...     # crea el proveedor (preset) con la clave
my-agent providers add openrouter --key sk-or-otra    # la segunda va a api_keys (rotación)
my-agent providers list                               # muestra cuántas claves tiene cada uno
my-agent providers test                               # los prueba todos de verdad
```

Para no guardar la clave en el archivo, usa variables de entorno — el nombre va en `api_key_env` (y `"env:NOMBRE"` en `api_keys`):

| Proveedor | Variable predeterminada |
|---|---|
| Claude | `ANTHROPIC_API_KEY` |
| Gemini | `GEMINI_API_KEY` |
| OpenAI / OpenRouter / Groq / DeepSeek / Mistral / xAI / Together | `OPENAI_API_KEY`, `OPENROUTER_API_KEY`, `GROQ_API_KEY`, `DEEPSEEK_API_KEY`, `MISTRAL_API_KEY`, `XAI_API_KEY`, `TOGETHER_API_KEY` |

```powershell
[Environment]::SetEnvironmentVariable('ANTHROPIC_API_KEY', 'sk-ant-...', 'User')   # PowerShell, permanente
```
```bash
echo 'export ANTHROPIC_API_KEY=sk-ant-...' >> ~/.bashrc                           # bash (o ~/.zshrc)
```

Las claves guardadas en el archivo quedan protegidas (permiso `0600`) y aparecen enmascaradas en `config show`, `/config get` y la interfaz web. Ollama y LM Studio locales no necesitan clave: instala Ollama y ejecuta `ollama pull llama3.2` (chat) y `ollama pull nomic-embed-text` (embeddings).

## Comandos

| Comando | Descripción |
|---|---|
| *(sin comando)* `[--agent A] [--model P/M] [--continue] [--session ID]` | Abre el agente interactivo (pantalla completa) |
| `run [--agent A] [--model P/M] [--yes] MENSAJE` | Ejecuta el agente con herramientas una vez, sin interfaz |
| `agents` · `mcp` · `sessions [list \| remove ID]` | Lista agentes; conecta con los servidores MCP y lista sus herramientas; sesiones guardadas |
| `init [--force]` | Crea el archivo de configuración por defecto |
| `config show \| get CLAVE \| set CLAVE VALOR \| path \| validate` | Inspecciona y edita la configuración |
| `providers list \| test [nombre] \| add NOMBRE [--preset P] [--key K] [--model M] [--base-url URL]` | Lista proveedores / prueba la conexión / añade un proveedor u otra clave |
| `chat [--provider P] [--no-rag]` | Conversación interactiva (`/` para comandos, `@archivo` para adjuntar) |
| `ask [--provider P] [--no-rag] [--sources] PREGUNTA` | Pregunta única |
| `models list \| ps \| select \| use NOMBRE \| pull NOMBRE` | Modelos de Ollama: lista, muestra lo cargado (GPU/CPU), elige (descargando si falta) y configura. Flags: `--provider P`, `--embedding` |
| `rag ingest [--recursive=BOOL] [--include=*.md,*.txt] RUTA\|URL...` | Indexa archivos, carpetas o páginas |
| `rag search [-k N] CONSULTA` | Busca en la base |
| `rag list \| remove FUENTE \| clear` | Gestiona la base |
| `connectors list \| add \| remove NOMBRE \| sync [NOMBRE...]` | Gestiona los conectores (`add` crea `file` y `url`; `database` y `email` se configuran en el JSON) |
| `serve [--addr HOST:PORT]` | Levanta la interfaz web |
| `completion bash\|zsh\|fish\|powershell` | Script de autocompletado |
| `version` | Versión |

Flags globales: `--config ARCHIVO` (o `-c`) y `--lang IDIOMA`. Orden de resolución de la configuración: flag → `$MY_AGENT_CONFIG` → `./my-agent.json` → archivo del usuario (`%APPDATA%\my-agent\my-agent.json` o `~/.config/my-agent/my-agent.json`).
Códigos de salida: `0` correcto, `1` error de ejecución, `2` uso incorrecto.

### Chat en línea (`my-agent chat`): comandos `/`, autocompletado y `@archivo`

El `chat` es la conversación clásica, sin pantalla completa y sin herramientas (solo RAG). No necesitas salir de él para configurar nada. Escribe **`/`** y aparece la lista de comandos; **Tab** completa, **↑/↓** eligen, **Enter** completa el elemento resaltado y **Esc** cierra. Escribe **`help`** (o `?`, o `/help`) para ver todo en un árbol numerado:

```
 1.    config       Configuración y parámetros
    1.1   chat          Agente            provider · model · temperature · max-tokens · prompt · rag
    1.2   rag           Búsqueda e índice embedder · top-k · min-score · chunk-size · chunk-overlap · store
    1.3   provider <nombre> <campo> [valor]
    1.4   language · show · get · set · validate · path
 2.    rag          Base de conocimiento (ingest · search · list · remove · clear)
 3.    models       Modelos de Ollama (list · ps · select · use · pull)
 4.    providers    list · test · use
 5.    connectors   list · add · remove · sync
 6.    chat         Conversación (reset · sources · model · clear · exit)
```

Ejemplos, todos **sin salir de la conversación** y vigentes desde el siguiente mensaje (el historial se mantiene):

```text
/config chat temperature 0.7            # creatividad
/config chat prompt "Responde siempre de forma concisa."
/config chat model qwen2.5:7b           # en Ollama, descarga si falta
/config chat rag off                    # activa/desactiva la base de conocimiento
/config rag top-k 6
/config provider gemini api_key         # pide la clave con entrada oculta
/config language es                     # cambia el idioma al instante
/models select                          # menú de modelos instalados (+ "Otro…")
/providers use claude                   # cambia de proveedor solo en esta conversación
/rag ingest ./docs                      # indexa y lo usa desde la siguiente pregunta
/help config chat                       # detalla una rama del árbol
```

- Todo lo que existe en la CLI (`config`, `rag`, `models`, `providers`, `connectors`) existe aquí con el mismo nombre.
- Tab también completa **valores**: nombres de proveedores, modelos instalados, claves de configuración y fuentes del RAG. Acepta un prefijo único (`/he` = `/help`) y avisa cuando es ambiguo.
- **`@archivo`** en cualquier mensaje adjunta el contenido del archivo (texto, hasta 100 KB; hasta 5 por mensaje): `explica el error en @logs/app.log`. Tab completa rutas. Solo el texto de la pregunta se usa en la búsqueda del RAG.
- Enter envía; `Ctrl+C` interrumpe la respuesta (o limpia la línea; con la línea vacía, sale); `Ctrl+D` sale; ↑/↓ recorren el historial de líneas.
- En terminales sin modo interactivo (pipe, mintty), los comandos y `@archivo` funcionan igual; solo no aparece la lista de autocompletado.

### Modelos de Ollama (listar, descargar y configurar)

```powershell
my-agent models list                       # modelos instalados (marca el actual)
my-agent models select                     # menú: elige un número, o 0 / escribe un nombre nuevo
my-agent models use qwen2.5:7b             # lo usa; descarga antes si no está instalado
my-agent models use --embedding bge-m3     # define el modelo de embeddings
my-agent models pull mistral               # solo descarga, sin cambiar la configuración
```

Tras elegir, `select` **abre el chat directamente** con el modelo configurado (`--no-chat` lo desactiva). El chat usa colores, banner, spinner y markdown cuando la salida es un terminal; en un pipe o con `NO_COLOR=1` sale en texto plano.

`select` muestra los modelos instalados y la opción **0) Otro...**. Cualquier nombre que no esté instalado se descarga (con progreso) y se guarda en `providers.<nombre>.model` (o `embedding_model` con `--embedding`). Pon los flags **antes** del nombre del modelo. Vale para proveedores de tipo `ollama` (`--provider` elige cuál; por defecto `agent.provider`).

En la interfaz web, lo mismo está en la tarjeta de cada proveedor Ollama, en la pestaña **Configuración**.

### Autocompletado

Los instaladores ya lo configuran. Para hacerlo a mano (también hay `completion zsh` y `completion fish`):

```bash
source <(my-agent completion bash)        # bash (ponlo en ~/.bashrc)
```
```powershell
my-agent completion powershell | Out-String | Invoke-Expression   # PowerShell (ponlo en $PROFILE)
```

## Configuración

> 📘 **Guía completa:** [docs/CONFIGURACION.md](docs/CONFIGURACION.md) ([English](docs/CONFIGURATION.md) · [Português](docs/CONFIGURACAO.md)): todos los campos, valores aceptados, proveedores, RAG, conectores, idioma, recetas listas y errores comunes.

La configuración es **un archivo JSON**. Qué archivo se usa: `--config` → `$MY_AGENT_CONFIG` → `./my-agent.json` de la carpeta actual → archivo del usuario (`%APPDATA%\my-agent\my-agent.json` o `~/.config/my-agent/my-agent.json`, creado en la primera ejecución). Así un proyecto puede tener su propia configuración y el resto usa la global; `my-agent config path` muestra cuál está en uso.

Formas de editarla:

| Dónde | Cómo |
|---|---|
| Interfaz interactiva | `/config` abre el archivo en el editor (`$EDITOR`, o el Bloc de notas) y lo recarga al cerrarlo; `/config get\|set CLAVE [VALOR]`; `/connect`, `/keys`, `/models`, `/agents` |
| Línea de comandos | `my-agent config show\|get\|set\|validate\|path`, `providers add`, `connectors add`, `models use` |
| Chat en línea | `/config chat …`, `/config rag …`, `/config provider …` |
| Interfaz web | `my-agent serve` → pestaña **Configuración** |
| A mano | edita el JSON y ejecuta `my-agent config validate` (o `/reload` en la interfaz) |

```bash
my-agent config set agent.provider claude
my-agent config set providers.claude.model claude-sonnet-5
my-agent config set default_agent plan
my-agent config set rag.embedder ollama      # embeddings semánticos con Ollama
```

| Sección | Para qué sirve |
|---|---|
| `language` | Idioma de los mensajes: `auto` (predeterminado), `en`, `pt-BR` o `es` |
| `agent` | Valores base de todos los agentes: `name`, `system_prompt`, `provider`, `temperature` (0–2), `max_tokens`, `use_rag` |
| `default_agent`, `agents` | Agente inicial y definición de los agentes ([Agentes](#agentes)) |
| `providers.<nombre>` | `type` (`claude`, `gemini`, `ollama`, `openai`), `model`, `base_url`, `api_key` / `api_key_env` / `api_keys`, `headers`, `models`, `embedding_model`, `timeout_seconds` y, en Ollama, `keep_alive` / `num_ctx` ([Proveedores y varias claves](#proveedores-y-varias-claves)) |
| `permission` | Reglas globales `allow`/`ask`/`deny` ([Herramientas y permisos](#herramientas-y-permisos)) |
| `mcp` | Servidores MCP ([Servidores MCP](#servidores-mcp)) |
| `connectors` | `file`, `url`, `database`, `email` ([Conectores](#conectores)) |
| `rag` | `store_path`, `embedder` (`local` o un proveedor con embeddings), `chunk_size`, `chunk_overlap`, `top_k`, `min_score` ([RAG](#rag)) |
| `shell` | Comando de la herramienta `bash` (predeterminado: PowerShell en Windows, `/bin/sh` en los demás) |
| `theme` | Colores de la interfaz: `dark` (predeterminado), `light` o `auto` |
| `server.addr` | Dirección de la interfaz web (predeterminado `127.0.0.1:8080`) |

`timeout_seconds`: con streaming es el tiempo máximo **sin recibir datos** (cubre la carga lenta de modelos grandes; predeterminado 120, Ollama 300); sin streaming es el límite total. Todas las secciones nuevas son opcionales: los archivos de versiones anteriores siguen siendo válidos.


> Claude no ofrece API de embeddings. Para RAG usa `local`, `ollama` o `gemini` en `rag.embedder`; Claude sigue siendo el modelo de chat.

## RAG

1. Los documentos se dividen en chunks (prefiriendo cortes de párrafo/línea/palabra).
2. Cada chunk lo vectoriza el embedder configurado y se guarda en `rag.store_path`.
3. En cada pregunta, los `top_k` fragmentos más similares (coseno ≥ `min_score`) entran en el prompt de sistema, numerados para citarlos como `[n]`.

Notas:
- Reingerir la misma fuente reemplaza los chunks antiguos (idempotente).
- El índice guarda qué embedder lo generó. Cambiar `rag.embedder` exige `rag clear` y una nueva ingesta (el programa avisa).
- El embedder `local` es léxico (palabras en común). Para búsqueda semántica usa un modelo de embeddings.

## Fuentes de RAG (file y url)

Definen fuentes que sincronizas bajo demanda con `connectors sync` (o `/sync` en la interfaz).

```bash
my-agent connectors add --name docs --type file --path ./docs --include "*.md,*.txt"
my-agent connectors add --name site --type url  --url https://ejemplo.com/acerca
my-agent connectors sync            # todos; o: connectors sync docs
```

- `file`: archivo o directorio. Ignora binarios, archivos > 5 MB, directorios ocultos y `node_modules`. `include` usa patrones glob (por defecto: tipos comunes de texto y código). PDF y Office no se leen: conviértelos a texto antes.
- `url`: descarga una página; el HTML se convierte en texto.

## Interfaz web

`my-agent serve` abre la interfaz con pestañas **Chat**, **Configuración**, **RAG** y **Conectores** (editar parámetros, probar proveedores, ingerir, buscar, sincronizar) y un **selector de idioma** en el encabezado.

Seguridad: la interfaz **no tiene autenticación** y escucha en `127.0.0.1` por defecto. Puede leer rutas locales (ingesta) y usar tus claves de API; no la expongas en red sin un proxy con autenticación. Las peticiones que modifican estado exigen `Content-Type: application/json`.

## Estructura

```
cmd/my-agent/          punto de entrada
internal/config/       modelo, validación, get/set por clave
internal/i18n/         traducciones (en, pt-BR, es), detección del idioma
internal/provider/     Claude, Gemini, Ollama, compatibles con OpenAI (tool calling, rotación de claves)
internal/rag/          chunking, embedders, almacén vectorial, índice
internal/connectors/   file, url, database, email
internal/agent/        agente + sesión de conversación; bucle con herramientas y permisos
internal/tools/        herramientas (archivos, shell, búsqueda, web, RAG, base de datos, correo)
internal/mcp/          cliente MCP (stdio y Streamable HTTP)
internal/session/      sesiones guardadas
internal/tui/          interfaz de pantalla completa (Bubble Tea)
internal/app/          composición (usada por CLI y web)
internal/cli/          comandos y chat interactivo
internal/web/          API HTTP + interfaz incrustada
scripts/               build, install (Windows/Linux/macOS) y package (paquetes de release)
docs/                  guía de configuración (en, pt-BR, es) y examples/
```

## Desarrollo

```bash
go test ./...                       # tests unitarios (sin red: los proveedores usan servidores httptest)
go test -cover ./...                # con cobertura (-race exige CGO_ENABLED=1)
scripts/build.sh                    # vet + tests + build en bin/ (Windows: .\scripts\build.ps1)
scripts/package.sh                  # paquetes de todas las plataformas en dist/ (Windows: .\scripts\package.ps1)
```

La compilación es Go puro (`CGO_ENABLED=0`), así que los paquetes de Windows, Linux y macOS se generan desde cualquiera de las tres. El instalador de Linux se puede probar en un contenedor limpio:

```bash
docker run --rm -v "$PWD/dist:/dist" ubuntu:24.04 sh -c 'cd /tmp && tar -xzf /dist/my-agent-*-linux-amd64.tar.gz && sh my-agent-*/install.sh && ~/.local/bin/my-agent version'
```

### Traducciones

Todo el texto visible vive en el catálogo `internal/i18n/msgs_*.go`, con las **tres traducciones lado a lado** en cada clave:

```go
"cli.rag.no_results": {"no results", "nenhum resultado", "sin resultados"},
```

- Usa `i18n.T("clave", args...)` (o `i18n.Errorf` para errores con `%w`); nunca escribas frases directamente en el código.
- `go test ./internal/i18n` garantiza que **no falta ninguna traducción**, que los marcadores (`%s`, `%d`) coinciden entre idiomas y **señala cualquier frase nueva escrita directamente en el código** (excepción explícita: un comentario `// i18n:ok`).
- La interfaz web usa las claves `web.ui.*` (el navegador las obtiene de `GET /api/i18n`); un test comprueba que la página solo usa claves que existen.
- Corregir una traducción es editar la línea del catálogo. Para añadir un idioma nuevo, agrega una columna a `m` y un valor en `Lang`/`Supported`/`Names` (el compilador y los tests señalan lo que falta).

Consulta el [CHANGELOG](CHANGELOG.md) (en portugués) para el historial de versiones.
