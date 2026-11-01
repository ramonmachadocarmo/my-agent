# Guía de configuración

[English](CONFIGURATION.md) · [Português (Brasil)](CONFIGURACAO.md) · **Español**

Todo en my-agent se controla con **un único archivo JSON**. Esta guía muestra qué se puede configurar, qué hace cada campo, los valores aceptados y recetas listas para usar.

- [1. Dónde está el archivo](#1-dónde-está-el-archivo)
- [2. Formas de configurar](#2-formas-de-configurar)
- [3. Referencia de campos](#3-referencia-de-campos)
  - [language](#language) · [agent](#agent) · [providers](#providers) · [rag](#rag) · [connectors](#connectors) · [server](#server)
- [4. Proveedores en detalle](#4-proveedores-en-detalle)
- [5. RAG en detalle](#5-rag-en-detalle)
- [6. Conectores en detalle](#6-conectores-en-detalle)
- [7. Variables de entorno](#7-variables-de-entorno)
- [8. Idioma](#8-idioma)
- [9. Recetas](#9-recetas)
- [10. Validación y errores comunes](#10-validación-y-errores-comunes)
- [11. Agentes, herramientas, MCP y conectores de datos](#11-agentes-herramientas-mcp-y-conectores-de-datos)

---

## 1. Dónde está el archivo

El programa busca la configuración en este orden:

| Prioridad | Origen | Ejemplo |
|---|---|---|
| 1 | Flag `--config` (o `-c`) | `my-agent --config C:\agentes\ventas.json chat` |
| 2 | Variable de entorno `MY_AGENT_CONFIG` | `$env:MY_AGENT_CONFIG = 'C:\agentes\ventas.json'` |
| 3 | `my-agent.json` en la carpeta actual | — |
| 4 | Archivo del usuario: `%APPDATA%\my-agent\my-agent.json` (Windows) o `~/.config/my-agent/my-agent.json` | se crea automáticamente la primera vez que ejecutas `my-agent` sin configuración |

Esto permite tener **varios agentes**, cada uno con su archivo (prompt, modelo y base de conocimiento propios).

```powershell
my-agent init                         # crea my-agent.json con los valores predeterminados
my-agent --config ventas.json init    # crea otro agente
my-agent config path                  # muestra qué archivo se está usando
```

Detalles:
- El archivo se guarda con permiso `0600` (puede contener claves de API).
- Las rutas **relativas** dentro del archivo (como `rag.store_path`) se resuelven desde la **carpeta del archivo de configuración**, no desde la carpeta donde ejecuta el comando.
- El JSON es **estricto**: los campos desconocidos se rechazan (evita errores de escritura silenciosos). No se permiten comentarios.
- Los archivos creados por versiones anteriores siguen siendo válidos; los campos nuevos ausentes toman el valor predeterminado descrito abajo.

---

## 2. Formas de configurar

### 2.0 En la interfaz interactiva (`my-agent`)

| Quiero | Comando |
|---|---|
| Añadir un proveedor y su clave | `/connect` (lista de presets; la clave se escribe oculta) |
| Añadir otra clave a un proveedor | `/keys <proveedor>`; `/keys` lista cuántas claves tiene cada uno |
| Cambiar el modelo de esta sesión | `ctrl+o` o `/models proveedor/modelo` |
| Cambiar de agente | `tab` / `shift+tab` o `/agents <nombre>` |
| Editar el archivo completo | `/config` (lo abre en `$EDITOR`, o en el Bloc de notas, y lo recarga al cerrarlo) |
| Leer o escribir una clave | `/config get rag.top_k`, `/config set agents.plan.model gemini-2.5-pro` |
| Aplicar cambios hechos fuera | `/reload` (también reinicia los servidores MCP) |

`/connect` y `/keys` escriben en el archivo; la elección de modelo (`ctrl+o`) vale para la sesión. Para dejarla como predeterminada: `/config set agents.build.model <modelo>` (o `providers.<nombre>.model`).


### 2.1 Línea de comandos (`config`)

```powershell
my-agent config show                       # configuración completa (claves de API enmascaradas)
my-agent config get rag.top_k              # un valor
my-agent config set rag.top_k 6            # cambia y guarda
my-agent config set agent.use_rag false    # booleanos: true/false
my-agent config set agent.temperature 0.7  # números
my-agent config validate                   # comprueba el archivo
```

`set` usa una **clave con puntos** (`sección.campo` o `providers.<nombre>.campo`) y convierte el texto al tipo del campo. Reglas:

- Solo cambia **valores simples** (texto, número, booleano). Las listas (`connectors`) tienen comandos propios (`connectors add/remove`).
- Crea los campos opcionales `default_agent`, `shell`, `theme` y `agents.<nombre>.<campo>` (el agente se crea si no existe). Proveedores nuevos: `providers add`. Listas y mapas (`api_keys`, `mcp`, `permission`, `connectors` de base de datos/correo): edite el JSON.
- El resultado se valida **antes** de guardar: si queda inválido, no se escribe nada y el error explica el motivo.

Comandos específicos que también cambian la configuración:

| Comando | Qué cambia |
|---|---|
| `my-agent models select` / `models use NOMBRE` | `providers.<ollama>.model` (descarga el modelo si falta) |
| `my-agent models use --embedding NOMBRE` | `providers.<ollama>.embedding_model` |
| `my-agent connectors add/remove` | la lista `connectors` |
| `my-agent providers add NOMBRE [--preset P] [--key K] [--model M] [--base-url URL]` | crea `providers.<nombre>` a partir de un preset, o añade una clave (`api_key`, luego `api_keys`) |

### 2.1.1 Dentro del chat (sin salir de la conversación)

Todo lo que la CLI configura también se puede hacer **dentro de `my-agent chat`**, con comandos que empiezan por `/`. Escriba `/` para ver la lista (Tab completa, ↑/↓ eligen) o `help` para la ayuda numerada. Los cambios se **guardan en el archivo** y valen **para el siguiente mensaje**, manteniendo el historial.

| Quiero cambiar | En el chat | Clave en el JSON |
|---|---|---|
| Proveedor predeterminado | `/config chat provider claude` | `agent.provider` |
| Modelo del proveedor actual | `/config chat model qwen2.5:7b` | `providers.<n>.model` |
| Temperatura | `/config chat temperature 0.7` | `agent.temperature` |
| Longitud de la respuesta | `/config chat max-tokens 2048` | `agent.max_tokens` |
| Instrucciones del agente | `/config chat prompt "Eres..."` | `agent.system_prompt` |
| RAG activado/desactivado | `/config chat rag on` / `off` | `agent.use_rag` |
| Embedder | `/config rag embedder ollama` | `rag.embedder` |
| Fragmentos por pregunta | `/config rag top-k 6` | `rag.top_k` |
| Similitud mínima | `/config rag min-score 0.2` | `rag.min_score` |
| Tamaño / solapamiento | `/config rag chunk-size 1000`, `chunk-overlap 150` | `rag.chunk_size`, `rag.chunk_overlap` |
| Campo de un proveedor | `/config provider gemini keep_alive 1h` | `providers.<n>.<campo>` |
| Clave de API | `/config provider gemini api_key` (entrada oculta) | `providers.<n>.api_key` |
| Idioma | `/config language es` | `language` |
| Cualquier otra clave | `/config set <clave> <valor>` | cualquier escalar |

- Sin valor, el comando **muestra el actual** (`/config chat temperature` → `agent.temperature = 0.2`).
- `/config show`, `get`, `validate` y `path` funcionan como en la CLI. Los valores inválidos se rechazan y **no se escribe nada**.
- `/config provider <nombre> <campo>`: campos `model`, `base_url`, `api_key`, `api_key_env`, `embedding_model`, `timeout_seconds`, `keep_alive`, `num_ctx`.
- **Clave de API**: use `/config provider <nombre> api_key` **sin valor**: la clave se pide con la escritura oculta (`*`), no aparece en pantalla ni en el historial del terminal. Escribirla en la propia línea funciona, pero el programa avisa de que quedaría en el historial. `api_key_env` solo acepta el **nombre** de una variable y nunca muestra el valor rechazado.
- `/providers use <nombre>` cambia de proveedor **solo en esta conversación** (no guarda); `/config chat provider <nombre>` guarda.
- ¿Cambió `rag.embedder`? El chat avisa de que el índice anterior fue generado por otro embedder (`/rag clear` y vuelva a ingerir). ¿Cambió `chunk-size`/`overlap`? Vale para las próximas ingestas.
- Lo mismo vale para `/rag ingest|search|list|remove|clear`, `/models list|ps|select|use|pull` y `/connectors list|add|remove|sync`: mismos nombres y opciones que la CLI.

**Adjuntar archivos:** `@ruta` en cualquier mensaje envía el contenido del archivo (texto, hasta 100 KB, máximo 5 por mensaje). Tab completa rutas. Las carpetas y los binarios generan un aviso; para una carpeta entera use `/rag ingest`.

### 2.2 Editando el JSON

Abra el archivo en cualquier editor (en la interfaz interactiva: `/config`). Es el camino para listas y mapas — `agents` completos, `permission`, `mcp`, `api_keys`, `headers`, conectores de base de datos y correo — y para reordenar cosas. Un ejemplo con todo: [examples/my-agent.full.json](examples/my-agent.full.json). Ejecute `my-agent config validate` después de editar.

### 2.3 Interfaz web

```powershell
my-agent serve            # http://127.0.0.1:8080
```

En la pestaña **Configuración** cada sección se convierte en un formulario. **Guardar configuración** valida y escribe el archivo. Las claves de API ya guardadas aparecen enmascaradas (`********`): déjelas así para **conservarlas**; borre el valor para **eliminarlas**. Cada proveedor tiene el botón **Probar**, y los proveedores Ollama tienen el selector de modelos (listar, descargar y usar). El **selector de idioma** del encabezado cambia el idioma de la interfaz y lo guarda en `language`.

> Los cambios en la dirección del servidor (`server.addr`) solo valen tras reiniciar `serve`.

---

## 3. Referencia de campos

### Archivo completo con los valores predeterminados

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

> Con `my-agent --lang es init` el `system_prompt` se escribe en español (`Eres un asistente útil. Responde de forma concisa y di cuando no lo sepas.`) y `language` queda como `"es"`.

### language

| Campo | Tipo | Predeterminado | Regla | Para qué sirve |
|---|---|---|---|---|
| `language` | texto | `auto` | `auto`, `en`, `pt-BR` o `es` (los alias como `pt`, `pt_BR`, `es-MX` se guardan como `pt-BR` / `es`) | Idioma de los mensajes, de la ayuda y de la interfaz web. `auto` sigue el idioma del sistema. Vea [Idioma](#8-idioma) |

### agent

Identidad y comportamiento del agente.

| Campo | Tipo | Predeterminado | Regla | Para qué sirve |
|---|---|---|---|---|
| `name` | texto | `my-agent` | — | Nombre que se muestra en el chat y en la interfaz |
| `system_prompt` | texto | (asistente conciso) | — | Instrucciones fijas enviadas en toda conversación: rol, tono, idioma, reglas. `init` lo escribe en el idioma elegido |
| `provider` | texto | `ollama` | debe existir en `providers` | Proveedor usado por defecto (`--provider` cambia solo esa ejecución) |
| `temperature` | número | `0.2` | 0 a 2 | Creatividad: cerca de 0 = respuestas consistentes; cerca de 1+ = más variadas |
| `max_tokens` | entero | `1024` | > 0 | Longitud máxima de cada respuesta (en tokens) |
| `use_rag` | booleano | `true` | — | Consulta la base de conocimiento por defecto (`--no-rag` la desactiva por ejecución) |

Consejos para el `system_prompt`:
- Indique el idioma y el tono deseados (`Responde siempre en español, de forma concisa.`). El idioma de las respuestas sale de este prompt y de lo que escribe el usuario, no del ajuste `language`.
- Con RAG, pida que cite las fuentes y que admita cuando el contexto no cubre la pregunta. El programa ya añade al prompt los fragmentos recuperados numerados como `[1]`, `[2]` y la instrucción de citarlos.

### providers

Un mapa de **nombre → proveedor**. El nombre es libre (`claude`, `gemini`, `ollama`, `ollama-servidor`…); el **tipo** define cómo conversar. Puede tener varios proveedores del mismo tipo.

| Campo | Tipo | Predeterminado | Regla | Para qué sirve |
|---|---|---|---|---|
| `type` | texto | — | `claude`, `gemini`, `ollama` u `openai` | Define la API usada (`openai` = cualquier API compatible: OpenAI, OpenRouter, Groq, DeepSeek, Mistral, xAI, LM Studio, vLLM…) |
| `model` | texto | según el tipo | obligatorio | Modelo de chat |
| `base_url` | texto | según el tipo | obligatorio | Dirección de la API (cámbiela para usar un proxy o un Ollama remoto) |
| `api_key` | texto | vacío | — | Clave literal. **Tiene prioridad** sobre `api_key_env` |
| `api_key_env` | texto | según el tipo | nombre de variable (letras, números y `_`) | **Nombre** de la variable de entorno que contiene la clave (recomendado). No es la clave: si pega la clave aquí, el programa avisa y la oculta al mostrarla |
| `embedding_model` | texto | según el tipo | — | Modelo de embeddings (Gemini y Ollama) |
| `timeout_seconds` | entero | 120 (Ollama: 300) | ≥ 0 | Vea "Timeouts" abajo |
| `keep_alive` | texto | `30m` en Ollama | duración (`30m`, `1h`), segundos o `-1` | **Solo Ollama.** Cuánto tiempo queda el modelo en memoria tras el uso. Vacío = `30m`; `-1` = siempre |
| `num_ctx` | entero | `0` | ≥ 0 | **Solo Ollama.** Ventana de contexto. `0` = el valor predeterminado de Ollama. Menor = menos VRAM |
| `api_keys` | lista de textos | vacío | — | Claves extra, usadas en **rotación**: cuando una es rechazada (401/403), se queda sin crédito (402) o alcanza el límite (429), se usa la siguiente. `"env:NOMBRE"` lee la clave de una variable |
| `headers` | mapa | vacío | — | Cabeceras HTTP extra (p. ej. `X-Title` en OpenRouter) |
| `models` | lista de textos | vacío | — | Modelos extra ofrecidos en el selector de la interfaz (`ctrl+o`) |

`keep_alive` y `num_ctx` aparecen en todos los proveedores del archivo, pero **solo Ollama los usa**.

### rag

Cómo se construye y se consulta la base de conocimiento. Vea la [sección 5](#5-rag-en-detalle).

| Campo | Tipo | Predeterminado | Regla | Para qué sirve |
|---|---|---|---|---|
| `store_path` | texto | `data/rag-store.json` | obligatorio | Archivo del índice (relativo al archivo de configuración) |
| `embedder` | texto | `local` | `local` o el nombre de un proveedor | Quién genera los vectores |
| `chunk_size` | entero | `800` | > 0 | Tamaño de cada fragmento, en **caracteres** |
| `chunk_overlap` | entero | `100` | ≥ 0 y < `chunk_size` | Caracteres repetidos entre fragmentos vecinos |
| `top_k` | entero | `4` | > 0 | Cuántos fragmentos entran en el prompt |
| `min_score` | número | `0.05` | -1 a 1 | Similitud mínima (coseno) para que un fragmento cuente |

### connectors

Lista de fuentes de datos sincronizables. Cada elemento:

| Campo | Tipo | Regla | Para qué sirve |
|---|---|---|---|
| `name` | texto | obligatorio, único | Identifica el conector |
| `type` | texto | `file`, `url`, `database` o `email` | Tipo de fuente (`database` y `email`: consulta la [sección 11](#11-agentes-herramientas-mcp-y-conectores-de-datos)) |
| `path` | texto | obligatorio si `file` | Archivo o carpeta |
| `url` | texto | obligatorio si `url`; empieza por `http://` o `https://` | Página que se descarga |
| `include` | lista de textos | — | Patrones de archivo (`*.md`); vacío = tipos comunes |
| `recursive` | booleano | — | Entra en las subcarpetas |
| `database` / `email` | objeto | obligatorio en esos tipos | Configuración de la base de datos o de la cuenta de correo |

Adminístrelos desde la CLI (`connectors add/remove/list/sync`) o desde la pestaña **Conectores**.

### server

| Campo | Tipo | Predeterminado | Para qué sirve |
|---|---|---|---|
| `addr` | texto | `127.0.0.1:8080` | Dirección de la interfaz web. `--addr` en `serve` la sobrescribe |

---

## 4. Proveedores en detalle

### Claude (Anthropic)

- Clave: `ANTHROPIC_API_KEY` (o `api_key`).
- Modelo: cualquier ID válido de la API, p. ej.: `claude-sonnet-5`.
- **No tiene API de embeddings.** Puede ser el modelo de **chat**, pero `rag.embedder` debe ser otro (`local`, `ollama` o `gemini`).

### Gemini (Google)

- Clave: `GEMINI_API_KEY` (o `api_key`).
- Chat: `model` (p. ej.: `gemini-2.5-flash`). Embeddings: `embedding_model` (p. ej.: `gemini-embedding-001`).

### Ollama (local o remoto)

- Sin clave. `base_url` por defecto `http://localhost:11434`; apunte a otra máquina si lo prefiere.
- Gestión de modelos desde el propio programa:

  ```powershell
  my-agent models list                    # instalados
  my-agent models select                  # menú: elegir o escribir otro (descarga si falta) y abre el chat
  my-agent models use qwen2.5:7b          # lo usa, descargándolo si hace falta
  my-agent models use --embedding bge-m3  # define el modelo de embeddings
  my-agent models ps                      # qué está cargado y cuánto está en la GPU
  ```
- **Rendimiento** (el modelo debe caber en la VRAM libre; si no, parte se ejecuta en la CPU y se vuelve lento):
  - `keep_alive`: mantiene el modelo cargado entre mensajes (por defecto `30m`; el de Ollama es 5 min).
  - `num_ctx`: redúzcalo (`2048`) para ahorrar VRAM.
  - Use `models ps` para ver `100% GPU` o `40% GPU / 60% CPU`.

### Varios proveedores y proveedores extra

El nombre de cada proveedor es libre, y puede haber varios del mismo tipo. Lo más rápido es partir de un preset:

```powershell
my-agent providers add openrouter --key sk-or-...
my-agent providers add ollama-servidor --preset ollama --base-url http://192.168.0.10:11434 --model qwen2.5:14b
my-agent providers add mi-vllm --preset openai --base-url http://gpu:8000/v1 --model qwen2.5-coder
```

Presets: `claude`, `gemini`, `ollama`, `openai`, `openrouter`, `groq`, `deepseek`, `mistral`, `xai`, `together`, `lmstudio` (también en `/connect`). Después: `my-agent config set agent.provider ollama-servidor`, `my-agent --model ollama-servidor/qwen2.5:14b` o `ctrl+o` en la interfaz.

### Compatibles con OpenAI (`type: "openai"`)

Sirve para cualquier servicio o servidor que hable la API Chat Completions: OpenAI, OpenRouter, Groq, DeepSeek, Mistral, xAI, Together, LM Studio, vLLM, llama.cpp server…

- `base_url` incluye la versión: `https://api.openai.com/v1`, `https://openrouter.ai/api/v1`, `http://localhost:1234/v1`.
- Los servidores en `localhost`/`127.0.0.1` no necesitan clave.
- `embedding_model` usa `/embeddings` (p. ej. `text-embedding-3-small`), así que el proveedor también puede ser el `rag.embedder`.
- `headers` añade cabeceras (OpenRouter usa `HTTP-Referer` y `X-Title` para identificar la aplicación).
- El selector de modelos lista lo que devuelve `/models` del servidor; `models` añade nombres fijos.
- El modelo debe admitir *tool calling* para los agentes `build`/`plan`; si no, use el agente `chat`.

### Claves de API — cómo proporcionarlas

| Forma | Cómo | Observación |
|---|---|---|
| En la interfaz | `/connect` o `/keys <proveedor>` | Escritura oculta; se guarda en el archivo |
| Variable de entorno (recomendado) | Defina la variable indicada en `api_key_env` | La clave no queda en un archivo |
| Varias claves | `api_keys: ["sk-2", "env:OTRA_VAR"]` o `providers add NOMBRE --key ...` otra vez | Rotación automática (abajo) |
| Clave en el archivo | `my-agent config set providers.claude.api_key sk-ant-...` | Archivo con permiso `0600`; enmascarada en `config show` y en la interfaz |

```powershell
# PowerShell — solo esta ventana
$env:ANTHROPIC_API_KEY = 'sk-ant-...'
# PowerShell — permanente para su usuario (abra una terminal nueva después)
[Environment]::SetEnvironmentVariable('ANTHROPIC_API_KEY', 'sk-ant-...', 'User')
```
```bash
# bash / zsh
echo 'export ANTHROPIC_API_KEY=sk-ant-...' >> ~/.bashrc
```

**Rotación y reintentos.** Las claves se usan en este orden: `api_key`, `api_keys`, la variable de `api_key_env` (las repetidas se ignoran). Cuando la clave actual es rechazada (401/403), se queda sin crédito (402) o alcanza el límite (429), se prueba la siguiente al instante, y la que funciona queda como actual hasta que el programa se cierra. Si todas fallan con un error temporal (408, 429, 5xx, 529 — "modelo sobrecargado"), la llamada se repite tras 1 s, 2 s y 4 s antes de mostrar el error.

`my-agent providers list` muestra cuántas claves tiene cada proveedor; `my-agent providers test [nombre]` hace una llamada real de prueba.


### Timeouts

`timeout_seconds` tiene dos significados:

- **Con streaming (interfaz interactiva, `run`, `chat`, `ask`, web):** tiempo máximo **sin recibir ningún dato**. Una respuesta que está fluyendo nunca se corta; el límite cubre la carga lenta de modelos grandes. Si un modelo grande tarda en empezar, súbalo: `my-agent config set providers.ollama.timeout_seconds 600`.
- **Demás llamadas (sin streaming, como embeddings y listados):** tiempo total de la solicitud.

`0` usa 120 s.

---

## 5. RAG en detalle

**Qué es:** el agente consulta sus documentos antes de responder. Flujo: los documentos se dividen en fragmentos → cada fragmento se convierte en un vector (embedding) → en cada pregunta, los `top_k` fragmentos más parecidos entran en el prompt.

### Eligiendo el `embedder`

| Valor | Necesita | Calidad | Cuándo usarlo |
|---|---|---|---|
| `local` | nada (sin conexión) | **Léxica**: compara palabras en común | Empezar rápido, pocos documentos, pruebas |
| `ollama` | modelo de embeddings descargado (`nomic-embed-text`) | **Semántica** | Uso real, datos privados, sin coste |
| `gemini` | `GEMINI_API_KEY` | **Semántica** | Si ya usa Gemini |
| nombre de otro proveedor | que tenga `embedding_model` | Semántica | Proveedores extra |

> Claude **no** sirve como embedder. Si configura `"embedder": "claude"` el archivo pasa la validación, pero la ingesta falla diciendo que el proveedor no admite embeddings.

**Cambiar el embedder invalida el índice.** El índice guarda quién lo generó; si lo cambia, el programa se niega a mezclarlos y pide: `my-agent rag clear` y volver a ingerir.

```powershell
my-agent config set rag.embedder ollama
my-agent rag clear
my-agent rag ingest .\docs
```

### Ajustando la calidad

| Síntoma | Ajuste |
|---|---|
| Las respuestas ignoran fragmentos relevantes | Suba `top_k` (6–8) y/o baje `min_score` |
| Entran demasiados fragmentos irrelevantes | Suba `min_score` (0.2–0.4 con un embedder semántico) y/o baje `top_k` |
| Los fragmentos cortan las ideas por la mitad | Suba `chunk_size` (1000–1500) y `chunk_overlap` (150–200) |
| Fragmentos demasiado genéricos | Baje `chunk_size` (400–600) |
| Prompt demasiado grande / lento | Baje `top_k` y `chunk_size` |

Al cambiar `chunk_size` o `chunk_overlap`, **vuelva a ingerir** los documentos para que valga para lo que ya está en el índice. El `min_score` del embedder `local` suele ser bajo (0.05–0.3); el de los embedders semánticos es más alto.

### Operación

```powershell
my-agent rag ingest .\docs .\manuales\guia.md https://sitio.com/faq      # carpetas, archivos, URLs
my-agent rag search "cómo cancelar un pedido"                            # ver lo que se recuperaría
my-agent rag list                                                        # fuentes y n.º de fragmentos
my-agent rag remove C:\docs\antiguo.md                                   # elimina una fuente
my-agent rag clear                                                       # borra todo
my-agent ask --sources "pregunta"                                        # muestra las fuentes usadas
```

Volver a ingerir la misma fuente **sustituye** sus fragmentos antiguos (no duplica).

---

## 6. Conectores en detalle

Un conector es una **fuente guardada** que sincroniza cuando quiera (`connectors sync`), en vez de repetir la ruta en cada `rag ingest`.

### file

```powershell
my-agent connectors add --name docs --type file --path C:\docs --include "*.md,*.txt" --recursive=true
```

- Apunta a un **archivo** (el filtro `include` se ignora) o a una **carpeta**.
- `include` usa patrones glob aplicados al **nombre del archivo**. Valor por defecto si está vacío: `*.md *.txt *.rst *.go *.py *.js *.ts *.json *.yaml *.yml *.csv *.html`.
- `recursive`: entra en las subcarpetas. Las carpetas ocultas (`.git`…) y `node_modules` siempre se ignoran.
- Se ignoran: archivos **binarios**, archivos **mayores de 5 MB** y archivos ilegibles.
- El HTML se convierte en texto.
- **Los PDF y los archivos de Office (`.pdf`, `.docx`, `.xlsx`) no se leen**: son binarios y por eso se ignoran. Conviértalos antes a texto o Markdown (por ejemplo, exportando como `.txt`) e ingiera el resultado.

### url

```powershell
my-agent connectors add --name faq --type url --url https://ejemplo.com/faq
```

- Descarga **una página** (timeout de 30 s, límite de 5 MB). El HTML se convierte en texto sin scripts ni estilos.
- Para varios sitios, cree un conector por URL.

### Sincronizar

```powershell
my-agent connectors sync          # todos
my-agent connectors sync docs faq # solo estos
my-agent connectors list
my-agent connectors remove docs
```

La sincronización **no borra** los archivos que desaparecieron de la carpeta: use `rag remove` para ellos.

---

## 7. Variables de entorno

| Variable | Efecto |
|---|---|
| `MY_AGENT_CONFIG` | Ruta del archivo de configuración (la flag `--config` tiene prioridad) |
| `MY_AGENT_LANG` | Idioma de esta ejecución: `en`, `pt-BR`, `es` (o `auto`). Sobrescribe el `language` de la configuración; `--lang` la sobrescribe a ella |
| `ANTHROPIC_API_KEY` | Clave de Claude (nombre por defecto en `api_key_env`) |
| `GEMINI_API_KEY` | Clave de Gemini (nombre por defecto en `api_key_env`) |
| `NO_COLOR` | Cualquier valor desactiva los colores y adornos del chat de la CLI (salida en texto plano) |
| `MY_AGENT_DATA` | Carpeta de las sesiones guardadas (predeterminado `%LOCALAPPDATA%\my-agent` o `~/.local/share/my-agent`) |
| `OPENAI_API_KEY`, `OPENROUTER_API_KEY`, `GROQ_API_KEY`… | Nombres predeterminados de `api_key_env` de los proveedores creados con `providers add` / `/connect` |

Los nombres `ANTHROPIC_API_KEY`/`GEMINI_API_KEY` son solo los predeterminados: cambie `api_key_env` para usar otra variable.

---

## 8. Idioma

my-agent habla **inglés**, **portugués (Brasil)** y **español**: mensajes, errores, el árbol de `/help`, las descripciones de las flags, la validación de la configuración, la interfaz web y el prompt predeterminado que escribe `init`.

### Cómo se elige el idioma

Gana el primero de estos que nombre un idioma compatible:

| Prioridad | Origen | Ejemplo |
|---|---|---|
| 1 | Flag `--lang` | `my-agent --lang es chat` |
| 2 | Variable de entorno `MY_AGENT_LANG` | `$env:MY_AGENT_LANG = 'pt-BR'` |
| 3 | `language` en el archivo de configuración | `"language": "es"` |
| 4 | El idioma del sistema (idioma de pantalla de Windows, o `LC_ALL`, `LC_MESSAGES`, `LANGUAGE`, `LANG` en Linux/macOS) | — |
| 5 | Inglés | — |

`auto` (el valor predeterminado) significa "usar el idioma del sistema". Se aceptan alias: `pt`, `pt_BR`, `pt-PT` → portugués (Brasil); `es-MX`, `es_AR`… → español; `en-GB`, `en_US.UTF-8`… → inglés. Un `--lang` no compatible falla con un error; un valor no compatible en la variable de entorno se ignora.

### Cómo cambiarlo

| Dónde | Cómo |
|---|---|
| Para una ejecución | `my-agent --lang pt-BR ...` o `MY_AGENT_LANG=es my-agent ...` |
| De forma permanente (CLI) | `my-agent config set language es` (`auto` vuelve a detectarlo) |
| Dentro del chat | `/config language pt-BR` (o `/lang pt-BR`): se aplica al instante, incluido el árbol de ayuda y la lista de autocompletado |
| Interfaz web | el selector del encabezado (guarda `language`); `Automático` sigue el idioma del navegador |
| Al crear | `my-agent --lang es init` guarda `"language": "es"` y escribe el prompt en español |

### Qué no se traduce

- **Los comandos, las flags, las claves de configuración y los valores del JSON** se quedan como se escriben (`config set rag.top_k`, `--no-rag`, `keep_alive`…), para que los scripts y la documentación funcionen en cualquier idioma.
- **Las respuestas del modelo:** siguen su `system_prompt` y el idioma en que usted escribe. `language` no traduce lo que dice el modelo.
- Los textos de estado que vienen del propio Ollama durante la descarga (`pulling manifest`, …) y la instrucción que se añade al prompt junto con los fragmentos recuperados (está escrita para el modelo, en inglés).
- El `system_prompt` de un agente existente: `init` escribe el prompt en el idioma elegido, pero un archivo que ya existe conserva su texto. Cámbielo con `/config chat prompt "..."`.

---

## 9. Recetas

### A. 100 % local y privado (Ollama)

```powershell
my-agent init
my-agent models select                        # elija/descargue el modelo de chat
my-agent models use --embedding nomic-embed-text
my-agent config set rag.embedder ollama
my-agent config set rag.min_score 0.25
my-agent rag ingest .\docs
my-agent chat
```

### B. Claude para el chat + embeddings locales

```powershell
$env:ANTHROPIC_API_KEY = 'sk-ant-...'
my-agent config set agent.provider claude
my-agent config set rag.embedder ollama       # o gemini; Claude no genera embeddings
my-agent providers test claude
```

### C. Todo en Gemini

```powershell
$env:GEMINI_API_KEY = '...'
my-agent config set agent.provider gemini
my-agent config set rag.embedder gemini
```

### D. Un agente con personalidad y reglas propias

```powershell
my-agent config set agent.name "Asistente Jurídico"
my-agent config set agent.system_prompt "Eres un asistente jurídico. Responde en español, cita las fuentes como [n] y di claramente cuándo el contexto no cubre la pregunta."
my-agent config set agent.temperature 0.1
```

### E. Modelo grande en una GPU pequeña

```powershell
my-agent models ps                             # compruebe: ¿100% GPU?
my-agent config set providers.ollama.num_ctx 2048
my-agent config set providers.ollama.keep_alive 1h
```
Si aún muestra CPU: cierre los programas que usan la GPU o elija un modelo más pequeño (`models select`).

### F. Varios agentes, cada uno con su archivo

```powershell
my-agent --config ventas.json init
my-agent --config ventas.json config set agent.name "Ventas"
my-agent --config ventas.json config set rag.store_path "data/ventas.json"
my-agent --config ventas.json chat
```

Cada archivo tiene su propio `rag.store_path`, así que las bases de conocimiento no se mezclan.

### G. Interfaz web accesible solo desde su máquina (predeterminado) o en otro puerto

```powershell
my-agent serve --addr 127.0.0.1:9000
```

> La interfaz **no tiene autenticación**. Mantenga `127.0.0.1`. No use `0.0.0.0` sin un proxy con autenticación delante: quien acceda puede leer rutas locales (ingesta) y usar sus claves.

### H. OpenRouter y Groq con dos claves cada uno

```powershell
my-agent providers add openrouter --key sk-or-personal
my-agent providers add openrouter --key sk-or-trabajo      # va a api_keys: se usa cuando la primera alcanza el límite
my-agent providers add groq --key gsk_... --model llama-3.3-70b-versatile
my-agent --model openrouter/anthropic/claude-sonnet-4.5
```

### I. Agente de revisión que solo lee y solo ejecuta `git`

```json
"agents": {
  "review": {
    "description": "Revisa los cambios",
    "instructions": "Ejecuta git diff y señala errores y riesgos, del más grave al menos grave. No cambies nada.",
    "tools": { "write": false, "edit": false },
    "permission": { "bash": "deny", "bash:git diff*": "allow", "bash:git log*": "allow", "bash:git status*": "allow" },
    "color": "#f7768e"
  }
}
```

En la interfaz, `tab` hasta `REVIEW` y pida *"revisa mis cambios"*.

### J. GitHub mediante MCP

```json
"mcp": {
  "github": { "type": "local", "command": ["npx", "-y", "@modelcontextprotocol/server-github"],
              "environment": { "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_TOKEN}" } }
},
"permission": { "mcp_github_list_*": "allow", "mcp_github_get_*": "allow" }
```

Las lecturas quedan permitidas; crear issues, PRs etc. sigue pidiendo confirmación (`mcp_*` es `ask`). Compruébelo con `my-agent mcp`.

### K. Asistente de correo y CRM

```json
"connectors": [
  { "name": "mail", "type": "email", "email": { "imap_host": "imap.gmail.com", "smtp_host": "smtp.gmail.com", "username": "yo@ejemplo.com", "password_env": "MAIL_APP_PASSWORD" } },
  { "name": "crm", "type": "database", "database": { "driver": "postgres", "dsn_env": "CRM_DATABASE_URL" } }
],
"agents": {
  "secretaria": {
    "description": "Correo y clientes",
    "instructions": "Ayuda con correos y datos de clientes. Muestra siempre el borrador y espera confirmación antes de enviar.",
    "tools": { "*": false, "email_*": true, "db_query": true, "db_schema": true, "rag_search": true }
  }
}
```

Ejemplo: *"¿qué correos sin leer son de clientes cuyo contrato vence este mes?"*.

### L. Todo local con LM Studio

```powershell
my-agent providers add lmstudio --model qwen2.5-coder-14b-instruct     # base_url http://localhost:1234/v1, sin clave
my-agent config set agents.build.provider lmstudio
```


---

## 10. Validación y errores comunes

`config validate`, `config set` y el botón Guardar de la interfaz señalan **todos** los problemas a la vez, con la ruta del campo. Los mensajes de abajo son los que verá en español.

| Mensaje | Causa | Solución |
|---|---|---|
| `agent.provider: "x" no es un proveedor configurado` | El proveedor predeterminado no existe en `providers` | Cree el proveedor o use uno existente |
| `providers.<n>.type: "x" no es claude, gemini, ollama ni openai` | Tipo inválido | Use uno de los cuatro (`openai` para APIs compatibles) |
| `connectors[i].type: "x" no es file, url, database ni email` | Tipo de conector inválido | Use uno de los cuatro |
| `providers.<n>.model: obligatorio` / `base_url: obligatorio` | Campo obligatorio vacío | Rellénelo |
| `providers.<n>.keep_alive: "..." no es una duración como 30m, segundos o -1` | Formato inválido | Use `30m`, `1h`, segundos (`600`) o `-1` |
| `rag.embedder: debe ser "local" o un proveedor configurado, pero se recibió "x"` | Embedder inexistente | Use `local` o el nombre de un proveedor |
| `rag.chunk_overlap: debe ser >= 0 y < chunk_size` | Solapamiento mayor que el fragmento | Reduzca el solapamiento |
| `agent.temperature: debe estar entre 0 y 2` | Fuera de rango | Ajústelo |
| `connectors[i].path: obligatorio para el tipo file` | Conector `file` sin ruta | Indique `--path` |
| `language: "x" no es compatible (use auto, en, pt-BR o es)` | Idioma no compatible | Use `auto`, `en`, `pt-BR` o `es` |
| `configuración inválida: json: unknown field "x"` | Campo mal escrito en el JSON | Corrija el nombre |
| `proveedor "claude": falta la clave de API (defina providers.claude.api_key o la variable de entorno ANTHROPIC_API_KEY)` | Sin clave | Defina la variable de entorno o `api_key` |
| `aviso: providers.<n>.api_key_env no es un nombre válido de variable de entorno` | Pegó la **clave** en `api_key_env`, que espera el **nombre** de una variable | `my-agent config set providers.<n>.api_key_env GEMINI_API_KEY` y defina la variable; o ponga la clave en `api_key`. Como la clave quedó expuesta en el archivo, considere generar una nueva |
| `el índice fue generado por otro embedder` | Cambió `rag.embedder` con el índice lleno | `rag clear` y vuelva a ingerir |
| `el proveedor no admite embeddings` | El embedder apunta a Claude (o a un proveedor sin `embedding_model`) | Use `local`, `ollama` o `gemini` |
| `no se recibieron datos durante Ns` | El modelo quedó en silencio más allá de `timeout_seconds` | Suba `timeout_seconds`; revise `models ps` |
| `configuración ... no encontrada; ejecute my-agent init primero` | No hay archivo en la ruta resuelta | `my-agent init` o use `--config` |
| `model 'x' not found` (Ollama) | Modelo no descargado | `my-agent models use x` |
| `agente desconocido "x"` | `--agent`, `default_agent` o `/agents` con un nombre que no existe (o está `disabled`) | `my-agent agents` lista los agentes |
| `agents.<n>.provider: el proveedor "x" no está definido en providers` | Un agente apunta a un proveedor inexistente | Créelo (`providers add`) o corrija el nombre |
| `permission.<regla>: permiso no válido "x" (use allow, ask o deny)` | Valor erróneo en una regla | `allow`, `ask` o `deny` |
| `mcp.<n>.type: tipo desconocido` / `mcp.<n>.url debe empezar con http://` / `mcp.<n>.command: obligatorio` | Servidor MCP incompleto | `local` necesita `command`; `remote`, `url` |
| `connectors[i].database.driver: driver desconocido` / `database.dsn: obligatorio` | Conector de base de datos incompleto | `sqlite`, `postgres` o `mysql`, y `dsn` o `dsn_env` |
| `http 429` / `http 503` tras algunos intentos | Límite o sobrecarga en todas las claves, incluso tras los reintentos | Espere, añada otra clave (`/keys`) o cambie de modelo (`ctrl+o`) |
| `el agente alcanzó su límite de pasos (max_steps)` | La tarea necesitó más llamadas de las que el agente permite | Envíe "continúa", o aumente `agents.<n>.max_steps` |
| `%s denegado: necesita permiso (use --yes para permitir)` (en `run`) | Sin interfaz, lo que preguntaría se deniega | `my-agent run --yes …` o una regla `allow` |
| `la interfaz interactiva necesita una terminal` | `my-agent tui` en un pipe/script | Use `my-agent run MENSAJE` |

Consejo: `my-agent config path` muestra qué archivo se está leyendo — muchas "configuraciones que no se aplican" son otro archivo en la carpeta actual.

---

## 11. Agentes, herramientas, MCP y conectores de datos

Estas secciones controlan el agente interactivo (`my-agent` sin comando) y `my-agent run`. Todas son opcionales: un archivo sin ellas funciona con los agentes incluidos y los permisos predeterminados. Ejemplo completo: [examples/my-agent.full.json](examples/my-agent.full.json).

### default_agent y agents

| Campo | Tipo | Predeterminado | Para qué sirve |
|---|---|---|---|
| `default_agent` | texto | `build` | Agente usado al abrir la interfaz (`--agent` lo sobrescribe) |
| `agents.<nombre>` | objeto | — | Crea un agente o cambia uno incluido (`build`, `plan`, `chat`) |

Campos de cada agente (todos opcionales; lo que falte viene del agente incluido del mismo nombre y luego de la sección `agent`):

| Campo | Tipo | Para qué sirve |
|---|---|---|
| `description` | texto | Aparece en el selector de agentes y en `my-agent agents` |
| `system_prompt` | texto | Reemplaza `agent.system_prompt` |
| `instructions` | texto | Se añade al prompt (`plan` lo usa para no cambiar nada) |
| `provider`, `model` | texto | Proveedor (nombre en `providers`) y modelo; sin `model`, el del proveedor |
| `temperature`, `max_tokens` | número | Parámetros de generación |
| `use_rag` | booleano | Búsqueda automática en la base antes de cada mensaje |
| `max_steps` | entero | Llamadas al modelo por mensaje (predeterminado 25) |
| `tools` | mapa nombre/glob → booleano | Activa/desactiva herramientas: `{"*": false, "read": true}` |
| `permission` | mapa nombre/glob → `allow`\|`ask`\|`deny` | Reglas solo de este agente (sobrescriben las globales) |
| `color` | texto `#rrggbb` | Color del agente en la interfaz |
| `disabled` | booleano | Oculta el agente |

`Tab` cambia de agente en la interfaz. En la carpeta del proyecto, un `AGENTS.md` (o `CLAUDE.md`) se añade a las instrucciones.

### permission

Mapa de regla → `allow` (ejecuta), `ask` (pregunta) o `deny` (bloquea; la herramienta ni se ofrece). La clave es el nombre de la herramienta o un glob (`mcp_*`, `email_*`); para `bash`, `bash:<patrón>` se compara con el comando (`bash:git status*`). Gana la regla más específica; `agents.<nombre>.permission` sobrescribe las globales.

Predeterminados: todo `allow`, excepto `write`, `edit`, `bash`, `db_exec`, `email_send` y `mcp_*`, que son `ask`. En `my-agent run`, lo que preguntaría se deniega salvo que pases `--yes`.

### Herramientas

| Herramienta | Qué hace |
|---|---|
| `read` | Lee un archivo de texto con números de línea (`offset`, `limit`) |
| `write` | Crea o sobrescribe un archivo |
| `edit` | Reemplaza un fragmento exacto (acepta archivos CRLF) |
| `bash` | Ejecuta un comando en el shell (`shell`; por defecto PowerShell en Windows y `/bin/sh` en los demás), tiempo límite 120 s (máx. 600) |
| `glob`, `grep`, `ls` | Buscan archivos por patrón (`**/*.go`), por contenido (regex) y listan carpetas; ignoran `.git`, `node_modules` y similares |
| `webfetch` | Descarga una página (el HTML se convierte en texto) |
| `rag_search` | Busca en la base de conocimiento (solo se ofrece cuando tiene contenido) |
| `db_schema`, `db_query`, `db_exec` | Conectores `database` |
| `email_search`, `email_read`, `email_send` | Conectores `email` |
| `mcp_<servidor>_<herramienta>` | Herramientas de los servidores MCP |

### mcp

Mapa de **nombre → servidor**:

| Campo | Tipo | Para qué sirve |
|---|---|---|
| `type` | `local` o `remote` | `local`: subproceso (stdio). `remote`: Streamable HTTP |
| `command` | lista de textos | **local**: programa y argumentos (`["npx", "-y", "paquete"]`) |
| `environment` | mapa | **local**: variables extra; `${VAR}` lee de tu entorno |
| `url` | texto | **remote**: dirección del servidor (`https://…/mcp`) |
| `headers` | mapa | **remote**: cabeceras (p. ej. `Authorization`); `${VAR}` funciona |
| `enabled` | booleano | `false` lo desactiva sin borrarlo (predeterminado `true`) |
| `timeout_seconds` | entero | Plazo de la conexión inicial (predeterminado 30) |

`my-agent mcp` se conecta a todos y lista sus herramientas; en la interfaz, `/mcp` muestra su estado y `/reload` los reinicia después de editar la configuración.

### Conector `database`

```json
{ "name": "crm", "type": "database", "database": { "driver": "postgres", "dsn_env": "CRM_DATABASE_URL" } }
```

| Campo | Tipo | Para qué sirve |
|---|---|---|
| `driver` | `sqlite`, `postgres` o `mysql` | Base de datos (SQLite no necesita nada instalado) |
| `dsn` / `dsn_env` | texto | Conexión, o nombre de la variable que la contiene. SQLite: ruta del archivo. Postgres: `postgres://usuario:clave@host:5432/base`. MySQL: `usuario:clave@tcp(host:3306)/base` |
| `allow_write` | booleano | Ofrece `db_exec` (predeterminado `false`: solo lectura) |
| `query` | texto | Filas que `sync` lleva al RAG (50 filas por documento) |
| `max_rows` | entero | Máximo de filas por consulta (predeterminado 200) |

`db_query` se ejecuta siempre en una transacción de **solo lectura** (Postgres/MySQL) o con `query_only` (SQLite): ni siquiera un `DELETE` enviado por error cambia nada.

### Conector `email`

```json
{ "name": "trabajo", "type": "email",
  "email": { "imap_host": "imap.gmail.com", "smtp_host": "smtp.gmail.com", "username": "yo@ejemplo.com", "password_env": "MAIL_APP_PASSWORD" } }
```

| Campo | Tipo | Predeterminado | Para qué sirve |
|---|---|---|---|
| `imap_host`, `imap_port` | texto, entero | —, 993 | Lectura (993 = TLS; 143 = STARTTLS) |
| `smtp_host`, `smtp_port` | texto, entero | —, 587 | Envío (587 = STARTTLS; 465 = TLS). Sin `smtp_host` no hay `email_send` |
| `username` | texto | — | Usuario (normalmente la dirección) |
| `password` / `password_env` | texto | — | Contraseña, o nombre de la variable que la contiene. Gmail/Outlook: usa una *contraseña de aplicación* |
| `from` | texto | `username` | Remitente |
| `folder` | texto | `INBOX` | Carpeta predeterminada |
| `limit` | entero | 50 | Mensajes que `sync` lleva al RAG |

### shell y theme

| Campo | Predeterminado | Para qué sirve |
|---|---|---|
| `shell` | PowerShell (Windows) / `/bin/sh -c` | Comando usado por `bash`; el comando del agente es el último argumento. P. ej. `"pwsh -NoProfile -Command"`, `"bash -c"` |
| `theme` | `dark` | Colores de la interfaz: `dark`, `light` o `auto` (detecta el fondo de la terminal) |

### Secretos

`config show`, `/config get` y la interfaz web enmascaran `api_key`, `api_keys`, `headers`, `mcp.*.environment`, `mcp.*.headers`, `database.dsn` y `email.password`. Prefiere las variantes `*_env` / `env:NOMBRE` / `${VAR}` para no guardar secretos en el archivo.
