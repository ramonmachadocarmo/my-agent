# Guia de configuração

[English](CONFIGURATION.md) · **Português (Brasil)** · [Español](CONFIGURACION.md)

Tudo no my-agent é controlado por **um arquivo JSON**. Este guia mostra o que dá para configurar, o que cada campo faz, os valores aceitos e receitas prontas.

- [1. Onde fica o arquivo](#1-onde-fica-o-arquivo)
- [2. Formas de configurar](#2-formas-de-configurar)
- [3. Referência dos campos](#3-referência-dos-campos)
  - [language](#language) · [agent](#agent) · [providers](#providers) · [rag](#rag) · [connectors](#connectors) · [server](#server)
- [4. Provedores em detalhe](#4-provedores-em-detalhe)
- [5. RAG em detalhe](#5-rag-em-detalhe)
- [6. Conectores em detalhe](#6-conectores-em-detalhe)
- [7. Variáveis de ambiente](#7-variáveis-de-ambiente)
- [8. Idioma](#8-idioma)
- [9. Receitas](#9-receitas)
- [10. Validação e erros comuns](#10-validação-e-erros-comuns)
- [11. Agentes, ferramentas, MCP e conectores de dados](#11-agentes-ferramentas-mcp-e-conectores-de-dados)

---

## 1. Onde fica o arquivo

O programa procura a configuração nesta ordem:

| Prioridade | Origem | Exemplo |
|---|---|---|
| 1 | Flag `--config` (ou `-c`) | `my-agent --config C:\agentes\vendas.json chat` |
| 2 | Variável de ambiente `MY_AGENT_CONFIG` | `$env:MY_AGENT_CONFIG = 'C:\agentes\vendas.json'` |
| 3 | `my-agent.json` na pasta atual | — |
| 4 | Arquivo do usuário: `%APPDATA%\my-agent\my-agent.json` (Windows) ou `~/.config/my-agent/my-agent.json` | criado automaticamente na primeira vez que você roda `my-agent` sem configuração |

Isso permite ter **vários agentes**, cada um com seu arquivo (prompt, modelo e base de conhecimento próprios).

```powershell
my-agent init                         # cria my-agent.json com os valores padrão
my-agent --config vendas.json init    # cria outro agente
my-agent config path                  # mostra qual arquivo está em uso
```

Detalhes:
- O arquivo é salvo com permissão `0600` (pode conter chaves de API).
- Caminhos **relativos** dentro do arquivo (como `rag.store_path`) são resolvidos a partir da **pasta do arquivo de configuração**, não da pasta onde você roda o comando.
- O JSON é **estrito**: campos desconhecidos são rejeitados (evita erro de digitação silencioso). Comentários não são permitidos.
- Arquivos criados por versões antigas continuam válidos; campos novos ausentes assumem o valor padrão descrito abaixo.

---

## 2. Formas de configurar

### 2.0 Na interface interativa (`my-agent`)

| Quero | Comando |
|---|---|
| Adicionar um provedor e a chave | `/connect` (lista de presets; a chave é digitada oculta) |
| Adicionar mais uma chave a um provedor | `/keys <provedor>`; `/keys` lista quantas chaves cada um tem |
| Trocar o modelo desta sessão | `ctrl+o` ou `/models provedor/modelo` |
| Trocar de agente | `tab` / `shift+tab` ou `/agents <nome>` |
| Editar o arquivo inteiro | `/config` (abre no `$EDITOR`, ou no Bloco de Notas, e recarrega ao fechar) |
| Ler ou gravar uma chave | `/config get rag.top_k`, `/config set agents.plan.model gemini-2.5-pro` |
| Aplicar edições feitas fora | `/reload` (também reinicia os servidores MCP) |

`/connect` e `/keys` gravam no arquivo; a escolha de modelo (`ctrl+o`) vale para a sessão. Para torná-la padrão: `/config set agents.build.model <modelo>` (ou `providers.<nome>.model`).


### 2.1 Linha de comando (`config`)

```powershell
my-agent config show                       # config completa (chaves de API mascaradas)
my-agent config get rag.top_k              # um valor
my-agent config set rag.top_k 6            # altera e salva
my-agent config set agent.use_rag false    # booleanos: true/false
my-agent config set agent.temperature 0.7  # números
my-agent config validate                   # confere o arquivo
```

O `set` usa **chave com pontos** (`seção.campo` ou `providers.<nome>.campo`) e converte o texto para o tipo do campo. Regras:

- Só altera **valores simples** (texto, número, booleano). Listas (`connectors`) têm comandos próprios (`connectors add/remove`).
- Cria os campos opcionais `default_agent`, `shell`, `theme` e `agents.<nome>.<campo>` (o agente é criado se não existir). Provedores novos: `providers add`. Listas e mapas (`api_keys`, `mcp`, `permission`, `connectors` de banco/e-mail): edite o JSON.
- O resultado é validado **antes** de salvar: se ficar inválido, nada é gravado e o erro explica o motivo.

Comandos específicos que também alteram a configuração:

| Comando | O que altera |
|---|---|
| `my-agent models select` / `models use NOME` | `providers.<ollama>.model` (baixa o modelo se faltar) |
| `my-agent models use --embedding NOME` | `providers.<ollama>.embedding_model` |
| `my-agent connectors add/remove` | lista `connectors` |
| `my-agent providers add NOME [--preset P] [--key K] [--model M] [--base-url URL]` | cria `providers.<nome>` a partir de um preset, ou acrescenta uma chave (`api_key`, depois `api_keys`) |

### 2.1.1 Dentro do chat (sem sair da conversa)

Tudo o que a CLI configura também dá para fazer **dentro do `my-agent chat`**, com comandos que começam com `/`. Digite `/` para ver a lista (Tab completa, ↑/↓ escolhem) ou `help` para a ajuda numerada. As mudanças são **salvas no arquivo** e valem **na próxima mensagem**, mantendo o histórico.

| Quero mudar | No chat | Chave no JSON |
|---|---|---|
| Provedor padrão | `/config chat provider claude` | `agent.provider` |
| Modelo do provedor atual | `/config chat model qwen2.5:7b` | `providers.<n>.model` |
| Temperatura | `/config chat temperature 0.7` | `agent.temperature` |
| Tamanho da resposta | `/config chat max-tokens 2048` | `agent.max_tokens` |
| Instruções do agente | `/config chat prompt "Você é..."` | `agent.system_prompt` |
| RAG ligado/desligado | `/config chat rag on` / `off` | `agent.use_rag` |
| Embedder | `/config rag embedder ollama` | `rag.embedder` |
| Trechos por pergunta | `/config rag top-k 6` | `rag.top_k` |
| Similaridade mínima | `/config rag min-score 0.2` | `rag.min_score` |
| Tamanho / sobreposição | `/config rag chunk-size 1000`, `chunk-overlap 150` | `rag.chunk_size`, `rag.chunk_overlap` |
| Campo de um provedor | `/config provider gemini keep_alive 1h` | `providers.<n>.<campo>` |
| Chave de API | `/config provider gemini api_key` (entrada oculta) | `providers.<n>.api_key` |
| Idioma | `/config language es` | `language` |
| Qualquer outra chave | `/config set <chave> <valor>` | qualquer escalar |

- Sem valor, o comando **mostra o atual** (`/config chat temperature` → `agent.temperature = 0.2`).
- `/config show`, `get`, `validate` e `path` funcionam como na CLI. Valores inválidos são recusados e **nada é gravado**.
- `/config provider <nome> <campo>`: campos `model`, `base_url`, `api_key`, `api_key_env`, `embedding_model`, `timeout_seconds`, `keep_alive`, `num_ctx`.
- **Chave de API**: use `/config provider <nome> api_key` **sem valor**: a chave é pedida com a digitação oculta (`*`), não aparece na tela nem no histórico do terminal. Digitá-la na própria linha funciona, mas o programa avisa que ela ficaria no histórico. `api_key_env` só aceita **nome** de variável e nunca exibe o valor recusado.
- `/providers use <nome>` troca de provedor **só nesta conversa** (não grava); `/config chat provider <nome>` grava.
- Mudou `rag.embedder`? O chat avisa que o índice antigo foi gerado por outro embedder (`/rag clear` e reingira). Mudou `chunk-size`/`overlap`? Vale para as próximas ingestões.
- O mesmo vale para `/rag ingest|search|list|remove|clear`, `/models list|ps|select|use|pull` e `/connectors list|add|remove|sync`: mesmos nomes e opções da CLI.

**Anexar arquivos:** `@caminho` em qualquer mensagem envia o conteúdo do arquivo junto (texto, até 100 KB, no máximo 5 por mensagem). Tab completa caminhos. Pastas e binários geram um aviso; para uma pasta inteira use `/rag ingest`.

### 2.2 Editando o JSON

Abra o arquivo em qualquer editor (na interface interativa: `/config`). É o caminho para listas e mapas — `agents` completos, `permission`, `mcp`, `api_keys`, `headers`, conectores de banco e e-mail — e para reordenar coisas. Um exemplo com tudo: [examples/my-agent.full.json](examples/my-agent.full.json). Rode `my-agent config validate` depois de editar.

### 2.3 Interface web

```powershell
my-agent serve            # http://127.0.0.1:8080
```

Na aba **Configuração** cada seção vira um formulário. **Salvar configuração** valida e grava o arquivo. Chaves de API já salvas aparecem mascaradas (`********`): deixe assim para **mantê-las**; apague o valor para **removê-las**. Cada provedor tem o botão **Testar**, e os provedores Ollama têm o seletor de modelos (listar, baixar e usar). O **seletor de idioma** no cabeçalho troca o idioma da interface e o grava em `language`.

> Alterações no endereço do servidor (`server.addr`) só valem após reiniciar o `serve`.

---

## 3. Referência dos campos

### Arquivo completo com os valores padrão

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

> Com `my-agent --lang pt-BR init` o `system_prompt` é escrito em português (`Você é um assistente prestativo. Responda de forma concisa e diga quando não souber.`) e `language` fica `"pt-BR"`.

### language

| Campo | Tipo | Padrão | Regra | Para que serve |
|---|---|---|---|---|
| `language` | texto | `auto` | `auto`, `en`, `pt-BR` ou `es` (apelidos como `pt`, `pt_BR`, `es-MX` são gravados como `pt-BR` / `es`) | Idioma das mensagens, da ajuda e da interface web. `auto` segue o idioma do sistema. Veja [Idioma](#8-idioma) |

### agent

Identidade e comportamento do agente.

| Campo | Tipo | Padrão | Regra | Para que serve |
|---|---|---|---|---|
| `name` | texto | `my-agent` | — | Nome exibido no chat e na interface |
| `system_prompt` | texto | (assistente conciso) | — | Instruções fixas enviadas em toda conversa: papel, tom, idioma, regras. O `init` o escreve no idioma escolhido |
| `provider` | texto | `ollama` | deve existir em `providers` | Provedor usado por padrão (`--provider` troca só naquela execução) |
| `temperature` | número | `0.2` | 0 a 2 | Criatividade: perto de 0 = respostas consistentes; perto de 1+ = mais variadas |
| `max_tokens` | inteiro | `1024` | > 0 | Tamanho máximo de cada resposta (em tokens) |
| `use_rag` | booleano | `true` | — | Consulta a base de conhecimento por padrão (`--no-rag` desliga por execução) |

Dicas para o `system_prompt`:
- Escreva o idioma e o tom desejados (`Responda sempre em português do Brasil, de forma objetiva.`). O idioma das respostas vem deste prompt e do que o usuário escreve, não da configuração `language`.
- Com RAG, peça para citar fontes e admitir quando o contexto não cobre a pergunta. O programa já acrescenta ao prompt os trechos recuperados numerados como `[1]`, `[2]` e a instrução de citá-los.

### providers

Mapa de **nome → provedor**. O nome é livre (`claude`, `gemini`, `ollama`, `ollama-servidor`…); o **tipo** é que define como conversar. Você pode ter vários provedores do mesmo tipo.

| Campo | Tipo | Padrão | Regra | Para que serve |
|---|---|---|---|---|
| `type` | texto | — | `claude`, `gemini`, `ollama` ou `openai` | Define a API usada (`openai` = qualquer API compatível: OpenAI, OpenRouter, Groq, DeepSeek, Mistral, xAI, LM Studio, vLLM…) |
| `model` | texto | por tipo | obrigatório | Modelo de chat |
| `base_url` | texto | por tipo | obrigatório | Endereço da API (troque para usar um proxy ou um Ollama remoto) |
| `api_key` | texto | vazio | — | Chave literal. **Tem prioridade** sobre `api_key_env` |
| `api_key_env` | texto | por tipo | nome de variável (letras, números e `_`) | **Nome** da variável de ambiente que contém a chave (recomendado). Não é a chave: se colar a chave aqui o programa avisa e a oculta na exibição |
| `embedding_model` | texto | por tipo | — | Modelo de embeddings (Gemini e Ollama) |
| `timeout_seconds` | inteiro | 120 (Ollama: 300) | ≥ 0 | Veja "Timeouts" abaixo |
| `keep_alive` | texto | `30m` no Ollama | duração (`30m`, `1h`), segundos ou `-1` | **Só Ollama.** Quanto tempo o modelo fica na memória após o uso. Vazio = `30m`; `-1` = para sempre |
| `num_ctx` | inteiro | `0` | ≥ 0 | **Só Ollama.** Janela de contexto. `0` = padrão do Ollama. Menor = menos VRAM |
| `api_keys` | lista de textos | vazio | — | Chaves extras, usadas em **rodízio**: quando uma é recusada (401/403), fica sem crédito (402) ou atinge o limite (429), a próxima é usada. `"env:NOME"` lê a chave de uma variável |
| `headers` | mapa | vazio | — | Cabeçalhos HTTP extras (ex.: `X-Title` no OpenRouter) |
| `models` | lista de textos | vazio | — | Modelos extras oferecidos no seletor da interface (`ctrl+o`) |

`keep_alive` e `num_ctx` aparecem em todos os provedores no arquivo, mas **só o Ollama os usa**.

### rag

Como a base de conhecimento é montada e consultada. Veja a [seção 5](#5-rag-em-detalhe).

| Campo | Tipo | Padrão | Regra | Para que serve |
|---|---|---|---|---|
| `store_path` | texto | `data/rag-store.json` | obrigatório | Arquivo do índice (relativo ao arquivo de config) |
| `embedder` | texto | `local` | `local` ou nome de um provedor | Quem gera os vetores |
| `chunk_size` | inteiro | `800` | > 0 | Tamanho de cada trecho, em **caracteres** |
| `chunk_overlap` | inteiro | `100` | ≥ 0 e < `chunk_size` | Caracteres repetidos entre trechos vizinhos |
| `top_k` | inteiro | `4` | > 0 | Quantos trechos entram no prompt |
| `min_score` | número | `0.05` | -1 a 1 | Similaridade mínima (cosseno) para um trecho contar |

### connectors

Lista de fontes de dados sincronizáveis. Cada item:

| Campo | Tipo | Regra | Para que serve |
|---|---|---|---|
| `name` | texto | obrigatório, único | Identifica o conector |
| `type` | texto | `file`, `url`, `database` ou `email` | Tipo de fonte (`database` e `email`: veja a [seção 11](#11-agentes-ferramentas-mcp-e-conectores-de-dados)) |
| `path` | texto | obrigatório se `file` | Arquivo ou pasta |
| `url` | texto | obrigatório se `url`; começa com `http://` ou `https://` | Página a baixar |
| `include` | lista de textos | — | Padrões de arquivo (`*.md`); vazio = tipos comuns |
| `recursive` | booleano | — | Entra em subpastas |
| `database` / `email` | objeto | obrigatório nesses tipos | Configuração do banco ou da conta de e-mail |

Gerencie pela CLI (`connectors add/remove/list/sync`) ou pela aba **Conectores**.

### server

| Campo | Tipo | Padrão | Para que serve |
|---|---|---|---|
| `addr` | texto | `127.0.0.1:8080` | Endereço da interface web. `--addr` no `serve` sobrescreve |

---

## 4. Provedores em detalhe

### Claude (Anthropic)

- Chave: `ANTHROPIC_API_KEY` (ou `api_key`).
- Modelo: qualquer ID válido da API, ex.: `claude-sonnet-5`.
- **Não tem API de embeddings.** Pode ser o modelo de **chat**, mas o `rag.embedder` deve ser outro (`local`, `ollama` ou `gemini`).

### Gemini (Google)

- Chave: `GEMINI_API_KEY` (ou `api_key`).
- Chat: `model` (ex.: `gemini-2.5-flash`). Embeddings: `embedding_model` (ex.: `gemini-embedding-001`).

### Ollama (local ou remoto)

- Sem chave. `base_url` padrão `http://localhost:11434`; aponte para outra máquina se preferir.
- Gerenciamento de modelos pelo próprio programa:

  ```powershell
  my-agent models list                    # instalados
  my-agent models select                  # menu: escolher ou digitar outro (baixa se faltar) e abre o chat
  my-agent models use qwen2.5:7b          # usa, baixando se preciso
  my-agent models use --embedding bge-m3  # define o modelo de embeddings
  my-agent models ps                      # o que está carregado e quanto está na GPU
  ```
- **Desempenho** (o modelo precisa caber na VRAM livre; senão parte roda na CPU e fica lento):
  - `keep_alive`: mantém o modelo carregado entre mensagens (padrão `30m`; o do Ollama é 5 min).
  - `num_ctx`: reduza (`2048`) para economizar VRAM.
  - Use `models ps` para ver `100% GPU` ou `40% GPU / 60% CPU`.

### Vários provedores e provedores extras

O nome de cada provedor é livre, e pode haver vários do mesmo tipo. O mais rápido é partir de um preset:

```powershell
my-agent providers add openrouter --key sk-or-...
my-agent providers add ollama-servidor --preset ollama --base-url http://192.168.0.10:11434 --model qwen2.5:14b
my-agent providers add meu-vllm --preset openai --base-url http://gpu:8000/v1 --model qwen2.5-coder
```

Presets: `claude`, `gemini`, `ollama`, `openai`, `openrouter`, `groq`, `deepseek`, `mistral`, `xai`, `together`, `lmstudio` (também em `/connect`). Depois: `my-agent config set agent.provider ollama-servidor`, `my-agent --model ollama-servidor/qwen2.5:14b` ou `ctrl+o` na interface.

### Compatíveis com OpenAI (`type: "openai"`)

Serve para qualquer serviço ou servidor que fale a API Chat Completions: OpenAI, OpenRouter, Groq, DeepSeek, Mistral, xAI, Together, LM Studio, vLLM, llama.cpp server…

- `base_url` inclui a versão: `https://api.openai.com/v1`, `https://openrouter.ai/api/v1`, `http://localhost:1234/v1`.
- Servidores em `localhost`/`127.0.0.1` não precisam de chave.
- `embedding_model` usa `/embeddings` (ex.: `text-embedding-3-small`), então o provedor também pode ser o `rag.embedder`.
- `headers` acrescenta cabeçalhos (o OpenRouter usa `HTTP-Referer` e `X-Title` para identificar o app).
- O seletor de modelos lista o que `/models` do servidor devolve; `models` acrescenta nomes fixos.
- O modelo precisa suportar *tool calling* para os agentes `build`/`plan`; sem isso, use o agente `chat`.

### Chaves de API — como fornecer

| Forma | Como | Observação |
|---|---|---|
| Na interface | `/connect` ou `/keys <provedor>` | Digitação oculta; grava no arquivo |
| Variável de ambiente (recomendado) | Defina a variável indicada em `api_key_env` | A chave não fica em arquivo |
| Várias chaves | `api_keys: ["sk-2", "env:OUTRA_VAR"]` ou `providers add NOME --key ...` de novo | Rodízio automático (abaixo) |
| Chave no arquivo | `my-agent config set providers.claude.api_key sk-ant-...` | Arquivo com permissão `0600`; mascarada em `config show` e na interface |

```powershell
# PowerShell — só esta janela
$env:ANTHROPIC_API_KEY = 'sk-ant-...'
# PowerShell — permanente para o seu usuário (abra um terminal novo depois)
[Environment]::SetEnvironmentVariable('ANTHROPIC_API_KEY', 'sk-ant-...', 'User')
```
```bash
# bash / zsh
echo 'export ANTHROPIC_API_KEY=sk-ant-...' >> ~/.bashrc
```

**Rodízio e novas tentativas.** As chaves são usadas nesta ordem: `api_key`, `api_keys`, a variável de `api_key_env` (repetidas são ignoradas). Quando a chave atual é recusada (401/403), fica sem crédito (402) ou atinge o limite (429), a próxima é tentada na hora, e a que funcionar passa a ser a atual até o programa fechar. Se todas falharem com um erro temporário (408, 429, 5xx, 529 — "modelo sobrecarregado"), a chamada é repetida após 1 s, 2 s e 4 s antes de mostrar o erro.

`my-agent providers list` mostra quantas chaves cada provedor tem; `my-agent providers test [nome]` faz uma chamada real de teste.


### Timeouts

`timeout_seconds` tem dois significados:

- **Com streaming (interface interativa, `run`, `chat`, `ask`, web):** tempo máximo **sem receber nenhum dado**. Uma resposta que está fluindo nunca é cortada; o limite cobre o carregamento lento de modelos grandes. Se um modelo grande demorar para começar, aumente: `my-agent config set providers.ollama.timeout_seconds 600`.
- **Demais chamadas (sem streaming, como embeddings e listagens):** tempo total da requisição.

`0` usa 120 s.

---

## 5. RAG em detalhe

**O que é:** o agente consulta seus documentos antes de responder. Fluxo: os documentos são divididos em trechos → cada trecho vira um vetor (embedding) → a cada pergunta, os `top_k` trechos mais parecidos entram no prompt.

### Escolhendo o `embedder`

| Valor | Precisa de | Qualidade | Quando usar |
|---|---|---|---|
| `local` | nada (offline) | **Léxica**: compara palavras em comum | Começar rápido, poucos documentos, testes |
| `ollama` | modelo de embeddings baixado (`nomic-embed-text`) | **Semântica** | Uso real, dados privados, sem custo |
| `gemini` | `GEMINI_API_KEY` | **Semântica** | Se já usa Gemini |
| nome de outro provedor | que ele tenha `embedding_model` | Semântica | Provedores extras |

> Claude **não** serve como embedder. Se configurar `"embedder": "claude"` o arquivo passa na validação, mas a ingestão falha dizendo que o provedor não suporta embeddings.

**Trocar o embedder invalida o índice.** O índice guarda quem o gerou; se você trocar, o programa recusa misturar e pede: `my-agent rag clear` e reingerir.

```powershell
my-agent config set rag.embedder ollama
my-agent rag clear
my-agent rag ingest .\docs
```

### Ajustando a qualidade

| Sintoma | Ajuste |
|---|---|
| Respostas ignoram trechos relevantes | Aumente `top_k` (6–8) e/ou reduza `min_score` |
| Entra muito trecho irrelevante | Aumente `min_score` (0.2–0.4 com embedder semântico) e/ou reduza `top_k` |
| Trechos cortam ideias no meio | Aumente `chunk_size` (1000–1500) e `chunk_overlap` (150–200) |
| Trechos muito genéricos | Reduza `chunk_size` (400–600) |
| Prompt grande demais / lento | Reduza `top_k` e `chunk_size` |

Ao mudar `chunk_size` ou `chunk_overlap`, **reingira** os documentos para valer para o que já está no índice. O `min_score` do embedder `local` costuma ser baixo (0.05–0.3); o de embedders semânticos é mais alto.

### Operação

```powershell
my-agent rag ingest .\docs .\manuais\guia.md https://site.com/faq        # pastas, arquivos, URLs
my-agent rag search "como cancelar um pedido"                            # ver o que seria recuperado
my-agent rag list                                                        # fontes e nº de trechos
my-agent rag remove C:\docs\antigo.md                                    # remove uma fonte
my-agent rag clear                                                       # limpa tudo
my-agent ask --sources "pergunta"                                        # mostra as fontes usadas
```

Reingerir a mesma fonte **substitui** os trechos antigos (não duplica).

---

## 6. Conectores em detalhe

Um conector é uma **fonte salva** que você sincroniza quando quiser (`connectors sync`), em vez de repetir o caminho a cada `rag ingest`.

### file

```powershell
my-agent connectors add --name docs --type file --path C:\docs --include "*.md,*.txt" --recursive=true
```

- Aponta para um **arquivo** (o filtro `include` é ignorado) ou uma **pasta**.
- `include` usa padrões glob aplicados ao **nome do arquivo**. Padrão quando vazio: `*.md *.txt *.rst *.go *.py *.js *.ts *.json *.yaml *.yml *.csv *.html`.
- `recursive`: entra em subpastas. Pastas ocultas (`.git`…) e `node_modules` são sempre ignoradas.
- São ignorados: arquivos **binários**, arquivos **maiores que 5 MB** e arquivos ilegíveis.
- HTML é convertido em texto.
- **PDF e Office (`.pdf`, `.docx`, `.xlsx`) não são lidos**: são arquivos binários e por isso ignorados. Converta antes para texto ou Markdown (por exemplo, exportando como `.txt`) e ingira o resultado.

### url

```powershell
my-agent connectors add --name faq --type url --url https://exemplo.com/faq
```

- Baixa **uma página** (timeout de 30 s, limite de 5 MB). HTML vira texto sem scripts/estilos.
- Para vários sites, crie um conector por URL.

### Sincronizar

```powershell
my-agent connectors sync          # todos
my-agent connectors sync docs faq # só estes
my-agent connectors list
my-agent connectors remove docs
```

A sincronização **não apaga** arquivos que sumiram da pasta: use `rag remove` para eles.

---

## 7. Variáveis de ambiente

| Variável | Efeito |
|---|---|
| `MY_AGENT_CONFIG` | Caminho do arquivo de configuração (a flag `--config` tem prioridade) |
| `MY_AGENT_LANG` | Idioma desta execução: `en`, `pt-BR`, `es` (ou `auto`). Sobrepõe o `language` da configuração; `--lang` sobrepõe a ela |
| `ANTHROPIC_API_KEY` | Chave do Claude (nome padrão em `api_key_env`) |
| `GEMINI_API_KEY` | Chave do Gemini (nome padrão em `api_key_env`) |
| `NO_COLOR` | Qualquer valor desliga cores e enfeites do chat da CLI (saída em texto puro) |
| `MY_AGENT_DATA` | Pasta das sessões salvas (padrão `%LOCALAPPDATA%\my-agent` ou `~/.local/share/my-agent`) |
| `OPENAI_API_KEY`, `OPENROUTER_API_KEY`, `GROQ_API_KEY`… | Nomes padrão de `api_key_env` dos provedores criados por `providers add` / `/connect` |

Os nomes `ANTHROPIC_API_KEY`/`GEMINI_API_KEY` são só os padrões: troque `api_key_env` para usar outra variável.

---

## 8. Idioma

O my-agent fala **inglês**, **português (Brasil)** e **espanhol**: mensagens, erros, a árvore do `/help`, descrições das flags, validação da configuração, a interface web e o prompt padrão que o `init` escreve.

### Como o idioma é escolhido

Vale o primeiro destes que indicar um idioma suportado:

| Prioridade | Origem | Exemplo |
|---|---|---|
| 1 | Flag `--lang` | `my-agent --lang es chat` |
| 2 | Variável de ambiente `MY_AGENT_LANG` | `$env:MY_AGENT_LANG = 'pt-BR'` |
| 3 | `language` no arquivo de configuração | `"language": "es"` |
| 4 | O idioma do sistema (idioma de exibição do Windows, ou `LC_ALL`, `LC_MESSAGES`, `LANGUAGE`, `LANG` no Linux/macOS) | — |
| 5 | Inglês | — |

`auto` (o padrão) significa "usar o idioma do sistema". Apelidos são aceitos: `pt`, `pt_BR`, `pt-PT` → português (Brasil); `es-MX`, `es_AR`… → espanhol; `en-GB`, `en_US.UTF-8`… → inglês. Um `--lang` não suportado falha com erro; um valor não suportado na variável de ambiente é ignorado.

### Como mudar

| Onde | Como |
|---|---|
| Numa execução | `my-agent --lang pt-BR ...` ou `MY_AGENT_LANG=es my-agent ...` |
| Permanente (CLI) | `my-agent config set language es` (`auto` volta a detectar) |
| Dentro do chat | `/config language pt-BR` (ou `/lang pt-BR`): vale na hora, inclusive a árvore de ajuda e a lista de autocompletar |
| Interface web | o seletor no cabeçalho (grava `language`); `Automático` segue o idioma do navegador |
| Na criação | `my-agent --lang es init` grava `"language": "es"` e escreve o prompt em espanhol |

### O que não é traduzido

- **Comandos, flags, chaves de configuração e valores do JSON** ficam como são digitados (`config set rag.top_k`, `--no-rag`, `keep_alive`…), para que scripts e documentação funcionem em qualquer idioma.
- **As respostas do modelo:** seguem o seu `system_prompt` e o idioma em que você escreve. O `language` não traduz o que o modelo diz.
- Os textos de status que vêm do próprio Ollama durante o download (`pulling manifest`, …) e a instrução acrescentada ao prompt junto com os trechos recuperados (escrita para o modelo, em inglês).
- O `system_prompt` de um agente já existente: o `init` escreve o prompt no idioma escolhido, mas um arquivo que já existe mantém o texto. Mude com `/config chat prompt "..."`.

---

## 9. Receitas

### A. 100% local e privado (Ollama)

```powershell
my-agent init
my-agent models select                        # escolha/baixe o modelo de chat
my-agent models use --embedding nomic-embed-text
my-agent config set rag.embedder ollama
my-agent config set rag.min_score 0.25
my-agent rag ingest .\docs
my-agent chat
```

### B. Claude no chat + embeddings locais

```powershell
$env:ANTHROPIC_API_KEY = 'sk-ant-...'
my-agent config set agent.provider claude
my-agent config set rag.embedder ollama       # ou gemini; Claude não gera embeddings
my-agent providers test claude
```

### C. Tudo no Gemini

```powershell
$env:GEMINI_API_KEY = '...'
my-agent config set agent.provider gemini
my-agent config set rag.embedder gemini
```

### D. Agente com personalidade e regras próprias

```powershell
my-agent config set agent.name "Assistente Jurídico"
my-agent config set agent.system_prompt "Você é um assistente jurídico. Responda em português do Brasil, cite as fontes como [n] e diga claramente quando o contexto não cobrir a pergunta."
my-agent config set agent.temperature 0.1
```

### E. Modelo grande em GPU pequena

```powershell
my-agent models ps                             # confira: 100% GPU?
my-agent config set providers.ollama.num_ctx 2048
my-agent config set providers.ollama.keep_alive 1h
```
Se ainda mostrar CPU: feche programas que usam a GPU ou escolha um modelo menor (`models select`).

### F. Vários agentes, cada um com seu arquivo

```powershell
my-agent --config vendas.json init
my-agent --config vendas.json config set agent.name "Vendas"
my-agent --config vendas.json config set rag.store_path "data/vendas.json"
my-agent --config vendas.json chat
```

Cada arquivo tem seu próprio `rag.store_path`, então as bases de conhecimento não se misturam.

### G. Interface web acessível só na sua máquina (padrão) ou em outra porta

```powershell
my-agent serve --addr 127.0.0.1:9000
```

> A interface **não tem autenticação**. Mantenha `127.0.0.1`. Não use `0.0.0.0` sem um proxy com autenticação na frente: quem acessar consegue ler caminhos locais (ingestão) e usar suas chaves.

### H. OpenRouter e Groq com duas chaves cada

```powershell
my-agent providers add openrouter --key sk-or-pessoal
my-agent providers add openrouter --key sk-or-trabalho     # vira api_keys: usada quando a primeira atinge o limite
my-agent providers add groq --key gsk_... --model llama-3.3-70b-versatile
my-agent --model openrouter/anthropic/claude-sonnet-4.5
```

### I. Agente de revisão que só lê e só roda `git`

```json
"agents": {
  "review": {
    "description": "Revisa as mudanças",
    "instructions": "Rode git diff e aponte bugs e riscos, do mais grave ao menos grave. Não altere nada.",
    "tools": { "write": false, "edit": false },
    "permission": { "bash": "deny", "bash:git diff*": "allow", "bash:git log*": "allow", "bash:git status*": "allow" },
    "color": "#f7768e"
  }
}
```

Na interface, `tab` até `REVIEW` e peça *"revise minhas mudanças"*.

### J. GitHub via MCP

```json
"mcp": {
  "github": { "type": "local", "command": ["npx", "-y", "@modelcontextprotocol/server-github"],
              "environment": { "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_TOKEN}" } }
},
"permission": { "mcp_github_list_*": "allow", "mcp_github_get_*": "allow" }
```

Leituras liberadas; criar issues, PRs etc. continua pedindo confirmação (`mcp_*` é `ask`). Confira com `my-agent mcp`.

### K. Assistente de e-mail e CRM

```json
"connectors": [
  { "name": "mail", "type": "email", "email": { "imap_host": "imap.gmail.com", "smtp_host": "smtp.gmail.com", "username": "eu@exemplo.com", "password_env": "MAIL_APP_PASSWORD" } },
  { "name": "crm", "type": "database", "database": { "driver": "postgres", "dsn_env": "CRM_DATABASE_URL" } }
],
"agents": {
  "secretaria": {
    "description": "E-mail e clientes",
    "instructions": "Ajude com e-mails e dados de clientes. Sempre mostre o rascunho e espere confirmação antes de enviar.",
    "tools": { "*": false, "email_*": true, "db_query": true, "db_schema": true, "rag_search": true }
  }
}
```

Exemplo: *"quais e-mails não lidos são de clientes com contrato vencendo este mês?"*.

### L. Tudo local com LM Studio

```powershell
my-agent providers add lmstudio --model qwen2.5-coder-14b-instruct     # base_url http://localhost:1234/v1, sem chave
my-agent config set agents.build.provider lmstudio
```


---

## 10. Validação e erros comuns

`config validate`, `config set` e o botão Salvar da interface apontam **todos** os problemas de uma vez, com o caminho do campo. As mensagens abaixo são as que você vê em português.

| Mensagem | Causa | Correção |
|---|---|---|
| `agent.provider: "x" não é um provedor configurado` | O provedor padrão não existe em `providers` | Crie o provedor ou use um existente |
| `providers.<n>.type: "x" não é claude, gemini, ollama nem openai` | Tipo inválido | Use um dos quatro (`openai` para APIs compatíveis) |
| `connectors[i].type: "x" não é file, url, database nem email` | Tipo de conector inválido | Use um dos quatro |
| `providers.<n>.model: obrigatório` / `base_url: obrigatório` | Campo obrigatório vazio | Preencha |
| `providers.<n>.keep_alive: "..." não é uma duração como 30m, segundos ou -1` | Formato inválido | Use `30m`, `1h`, segundos (`600`) ou `-1` |
| `rag.embedder: deve ser "local" ou um provedor configurado, mas recebi "x"` | Embedder inexistente | Use `local` ou o nome de um provedor |
| `rag.chunk_overlap: deve ser >= 0 e < chunk_size` | Sobreposição maior que o trecho | Reduza o overlap |
| `agent.temperature: deve estar entre 0 e 2` | Fora da faixa | Ajuste |
| `connectors[i].path: obrigatório para o tipo file` | Conector `file` sem caminho | Informe `--path` |
| `language: "x" não é suportado (use auto, en, pt-BR ou es)` | Idioma não suportado | Use `auto`, `en`, `pt-BR` ou `es` |
| `configuração inválida: json: unknown field "x"` | Campo digitado errado no JSON | Corrija o nome |
| `provedor "claude": falta a chave de API (defina providers.claude.api_key ou a variável de ambiente ANTHROPIC_API_KEY)` | Sem chave | Defina a variável de ambiente ou `api_key` |
| `aviso: providers.<n>.api_key_env não é um nome válido de variável de ambiente` | Você colou a **chave** em `api_key_env`, que espera o **nome** de uma variável | `my-agent config set providers.<n>.api_key_env GEMINI_API_KEY` e defina a variável; ou coloque a chave em `api_key`. Como a chave ficou exposta no arquivo, considere gerar uma nova |
| `o índice foi gerado por outro embedder` | Trocou `rag.embedder` com o índice cheio | `rag clear` e reingira |
| `o provedor não suporta embeddings` | Embedder aponta para o Claude (ou provedor sem `embedding_model`) | Use `local`, `ollama` ou `gemini` |
| `nenhum dado recebido por Ns` | O modelo ficou em silêncio além de `timeout_seconds` | Aumente `timeout_seconds`; confira `models ps` |
| `configuração ... não encontrada; rode my-agent init primeiro` | Não existe arquivo no caminho resolvido | `my-agent init` ou use `--config` |
| `model 'x' not found` (Ollama) | Modelo não baixado | `my-agent models use x` |
| `agente desconhecido "x"` | `--agent`, `default_agent` ou `/agents` com um nome que não existe (ou está `disabled`) | `my-agent agents` lista os agentes |
| `agents.<n>.provider: o provedor "x" não está definido em providers` | Agente aponta para um provedor inexistente | Crie o provedor (`providers add`) ou corrija o nome |
| `permission.<regra>: permissão inválida "x" (use allow, ask ou deny)` | Valor errado numa regra | `allow`, `ask` ou `deny` |
| `mcp.<n>.type: tipo desconhecido` / `mcp.<n>.url deve começar com http://` / `mcp.<n>.command: obrigatório` | Servidor MCP incompleto | `local` precisa de `command`; `remote`, de `url` |
| `connectors[i].database.driver: driver desconhecido` / `database.dsn: obrigatório` | Conector de banco incompleto | `sqlite`, `postgres` ou `mysql`, e `dsn` ou `dsn_env` |
| `http 429` / `http 503` depois de algumas tentativas | Limite ou sobrecarga em todas as chaves, mesmo após as novas tentativas | Aguarde, acrescente outra chave (`/keys`) ou troque de modelo (`ctrl+o`) |
| `o agente atingiu o limite de passos (max_steps)` | A tarefa precisou de mais chamadas do que o agente permite | Envie "continue", ou aumente `agents.<n>.max_steps` |
| `%s negado: precisa de permissão (use --yes para permitir)` (no `run`) | Sem interface, o que seria `ask` é negado | `my-agent run --yes …` ou uma regra `allow` |
| `a interface interativa precisa de um terminal` | `my-agent tui` em pipe/script | Use `my-agent run MENSAGEM` |

Dica: `my-agent config path` mostra qual arquivo está sendo lido — muita "configuração que não pega" é outro arquivo na pasta atual.

---

## 11. Agentes, ferramentas, MCP e conectores de dados

Estas seções controlam o agente interativo (`my-agent` sem comando) e o `my-agent run`. Todas são opcionais: um arquivo sem elas funciona com os agentes embutidos e as permissões padrão. Exemplo completo: [examples/my-agent.full.json](examples/my-agent.full.json).

### default_agent e agents

| Campo | Tipo | Padrão | Para que serve |
|---|---|---|---|
| `default_agent` | texto | `build` | Agente usado ao abrir a interface (`--agent` sobrescreve) |
| `agents.<nome>` | objeto | — | Cria um agente ou altera um embutido (`build`, `plan`, `chat`) |

Campos de cada agente (todos opcionais; o que faltar vem do agente embutido de mesmo nome e depois da seção `agent`):

| Campo | Tipo | Para que serve |
|---|---|---|
| `description` | texto | Aparece no seletor de agentes e em `my-agent agents` |
| `system_prompt` | texto | Substitui `agent.system_prompt` |
| `instructions` | texto | Acrescentado ao prompt (o `plan` usa isto para não alterar nada) |
| `provider`, `model` | texto | Provedor (nome em `providers`) e modelo; sem `model`, usa o do provedor |
| `temperature`, `max_tokens` | número | Parâmetros de geração |
| `use_rag` | booleano | Busca automática na base antes de cada mensagem |
| `max_steps` | inteiro | Quantas chamadas ao modelo por mensagem (padrão 25) |
| `tools` | mapa nome/glob → booleano | Liga/desliga ferramentas: `{"*": false, "read": true}` |
| `permission` | mapa nome/glob → `allow`\|`ask`\|`deny` | Regras só deste agente (sobrepõem as globais) |
| `color` | texto `#rrggbb` | Cor do agente na interface |
| `disabled` | booleano | Esconde o agente |

`Tab` alterna entre os agentes na interface. Na pasta do projeto, um `AGENTS.md` (ou `CLAUDE.md`) é acrescentado às instruções.

### permission

Mapa de regra → `allow` (executa), `ask` (pergunta) ou `deny` (bloqueia; a ferramenta nem é oferecida). A chave é o nome da ferramenta ou um glob (`mcp_*`, `email_*`); para o `bash`, `bash:<padrão>` compara com o comando (`bash:git status*`). A regra mais específica vence; as de `agents.<nome>.permission` sobrepõem as globais.

Padrões: tudo `allow`, exceto `write`, `edit`, `bash`, `db_exec`, `email_send` e `mcp_*`, que são `ask`. No `my-agent run`, o que for `ask` é negado, a menos que você passe `--yes`.

### Ferramentas

| Ferramenta | O que faz |
|---|---|
| `read` | Lê um arquivo de texto com números de linha (`offset`, `limit`) |
| `write` | Cria ou sobrescreve um arquivo |
| `edit` | Troca um trecho exato por outro (aceita arquivos CRLF) |
| `bash` | Roda um comando no shell (`shell`; padrão PowerShell no Windows, `/bin/sh` nos demais), timeout 120 s (máx. 600) |
| `glob`, `grep`, `ls` | Procura arquivos por padrão (`**/*.go`), por conteúdo (regex) e lista pastas; ignoram `.git`, `node_modules` e similares |
| `webfetch` | Baixa uma página (HTML vira texto) |
| `rag_search` | Busca na base de conhecimento (só aparece quando a base tem conteúdo) |
| `db_schema`, `db_query`, `db_exec` | Conectores `database` |
| `email_search`, `email_read`, `email_send` | Conectores `email` |
| `mcp_<servidor>_<ferramenta>` | Ferramentas dos servidores MCP |

### mcp

Mapa de **nome → servidor**:

| Campo | Tipo | Para que serve |
|---|---|---|
| `type` | `local` ou `remote` | `local`: subprocesso (stdio). `remote`: Streamable HTTP |
| `command` | lista de textos | **local**: programa e argumentos (`["npx", "-y", "pacote"]`) |
| `environment` | mapa | **local**: variáveis extras; `${VAR}` lê do seu ambiente |
| `url` | texto | **remote**: endereço do servidor (`https://…/mcp`) |
| `headers` | mapa | **remote**: cabeçalhos (ex.: `Authorization`); `${VAR}` funciona |
| `enabled` | booleano | `false` desliga sem apagar (padrão `true`) |
| `timeout_seconds` | inteiro | Prazo da conexão inicial (padrão 30) |

`my-agent mcp` conecta a todos e lista as ferramentas; na interface, `/mcp` mostra o estado e `/reload` reinicia os servidores após editar a configuração.

### Conector `database`

```json
{ "name": "crm", "type": "database", "database": { "driver": "postgres", "dsn_env": "CRM_DATABASE_URL" } }
```

| Campo | Tipo | Para que serve |
|---|---|---|
| `driver` | `sqlite`, `postgres` ou `mysql` | Banco (SQLite sem dependências externas) |
| `dsn` / `dsn_env` | texto | Conexão, ou nome da variável que a contém. SQLite: caminho do arquivo. Postgres: `postgres://usuario:senha@host:5432/banco`. MySQL: `usuario:senha@tcp(host:3306)/banco` |
| `allow_write` | booleano | Oferece `db_exec` (padrão `false`: somente leitura) |
| `query` | texto | Linhas levadas ao RAG por `sync` (50 linhas por documento) |
| `max_rows` | inteiro | Máximo de linhas por consulta (padrão 200) |

`db_query` roda sempre numa transação **somente leitura** (Postgres/MySQL) ou com `query_only` (SQLite): mesmo um `DELETE` enviado por engano não altera nada.

### Conector `email`

```json
{ "name": "trabalho", "type": "email",
  "email": { "imap_host": "imap.gmail.com", "smtp_host": "smtp.gmail.com", "username": "eu@exemplo.com", "password_env": "MAIL_APP_PASSWORD" } }
```

| Campo | Tipo | Padrão | Para que serve |
|---|---|---|---|
| `imap_host`, `imap_port` | texto, inteiro | —, 993 | Leitura (993 = TLS; 143 = STARTTLS) |
| `smtp_host`, `smtp_port` | texto, inteiro | —, 587 | Envio (587 = STARTTLS; 465 = TLS). Sem `smtp_host`, não há `email_send` |
| `username` | texto | — | Usuário (normalmente o e-mail) |
| `password` / `password_env` | texto | — | Senha, ou nome da variável que a contém. Gmail/Outlook: use uma *senha de app* |
| `from` | texto | `username` | Remetente |
| `folder` | texto | `INBOX` | Pasta padrão |
| `limit` | inteiro | 50 | Mensagens levadas ao RAG por `sync` |

### shell e theme

| Campo | Padrão | Para que serve |
|---|---|---|
| `shell` | PowerShell (Windows) / `/bin/sh -c` | Comando usado pelo `bash`; o comando do agente é o último argumento. Ex.: `"pwsh -NoProfile -Command"`, `"bash -c"` |
| `theme` | `dark` | Cores da interface: `dark`, `light` ou `auto` (detecta o fundo do terminal) |

### Segredos

`config show`, `/config get` e a interface web mascaram `api_key`, `api_keys`, `headers`, `mcp.*.environment`, `mcp.*.headers`, `database.dsn` e `email.password`. Prefira sempre as variantes `*_env` / `env:NOME` / `${VAR}` para não gravar segredos no arquivo.
