package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"my-agent/internal/config"
	"my-agent/internal/i18n"
	"my-agent/internal/rag"
)

// param is a configuration value with a friendly name, editable in place
// with /config chat <name> <value> or /config rag <name> <value>.
type param struct {
	name, key string
	usageKey  string // catalog key of the argument hint, or...
	usageLit  string // ...a literal hint that needs no translation
	descKey   string
	kind      string // "", "bool", "text", "provider", "model", "embedder"
}

func (p param) usage() string {
	if p.usageKey != "" {
		return i18n.T(p.usageKey)
	}
	return p.usageLit
}

var chatParams = []param{
	{name: "provider", key: "agent.provider", usageKey: "arg.name", descKey: "tree.param.provider", kind: "provider"},
	{name: "model", usageKey: "arg.model", descKey: "tree.param.model", kind: "model"},
	{name: "temperature", key: "agent.temperature", usageLit: "<0-2>", descKey: "tree.param.temperature"},
	{name: "max-tokens", key: "agent.max_tokens", usageLit: "<n>", descKey: "tree.param.max_tokens"},
	{name: "prompt", key: "agent.system_prompt", usageKey: "arg.text", descKey: "tree.param.prompt", kind: "text"},
	{name: "rag", key: "agent.use_rag", usageLit: "on|off", descKey: "tree.param.rag", kind: "bool"},
}

var ragParams = []param{
	{name: "embedder", key: "rag.embedder", usageKey: "arg.embedder", descKey: "tree.param.embedder", kind: "embedder"},
	{name: "top-k", key: "rag.top_k", usageLit: "<n>", descKey: "tree.param.top_k"},
	{name: "min-score", key: "rag.min_score", usageLit: "<-1..1>", descKey: "tree.param.min_score"},
	{name: "chunk-size", key: "rag.chunk_size", usageLit: "<n>", descKey: "tree.param.chunk_size"},
	{name: "chunk-overlap", key: "rag.chunk_overlap", usageLit: "<n>", descKey: "tree.param.chunk_overlap"},
	{name: "store", key: "rag.store_path", usageKey: "arg.file", descKey: "tree.param.store"},
}

var providerFields = []string{"model", "base_url", "api_key", "api_key_env", "embedding_model", "timeout_seconds", "keep_alive", "num_ctx"}

// buildTree defines every in-chat command. Adding one here makes it appear
// in /help, in autocomplete and in execution. It is called again when the
// language changes, so descriptions and hints follow it.
func (c *chat) buildTree() []*node {
	T := i18n.T
	pass := func(fn func(c *chat, args []string) error, sub string) func(*chat, []string) error {
		return func(c *chat, args []string) error { return fn(c, append([]string{sub}, args...)) }
	}
	viaConfig := func(c *chat, args []string) error { return c.e.configCommand(c.a, args) }
	viaRAG := func(c *chat, args []string) error { return c.e.ragCommand(c.ctx, c.a, args) }
	viaConnectors := func(c *chat, args []string) error { return c.e.connectorsCommand(c.ctx, c.a, args) }
	viaProviders := func(c *chat, args []string) error { return c.e.providersCommand(c.ctx, c.a, args) }
	viaModels := func(c *chat, args []string) error {
		return c.e.modelsCommand(c.ctx, c.a, args, c.rd, modelsOpts{defProvider: c.ollamaProvider()})
	}

	leaf := func(name, usage, desc string, run func(c *chat, args []string) error, complete func(c *chat, args []string, prefix string) []suggestion) *node {
		return &node{name: name, usage: usage, desc: desc, run: run, complete: complete}
	}
	group := func(name, desc string, children ...*node) *node {
		return &node{name: name, desc: desc, children: children}
	}
	paramNodes := func(ps []param) []*node {
		var out []*node
		for _, p := range ps {
			p := p
			out = append(out, leaf(p.name, p.usage(), T(p.descKey),
				func(c *chat, args []string) error { return c.setParam(p, args) },
				func(c *chat, args []string, prefix string) []suggestion {
					if len(args) > 0 && p.kind != "text" {
						return nil
					}
					return c.paramValues(p, prefix)
				}))
		}
		return out
	}

	// conversation commands, also offered as top-level shortcuts
	convo := []*node{
		leaf("reset", "", T("tree.chat.reset"), func(c *chat, _ []string) error {
			c.sess.Reset()
			fmt.Fprintln(c.e.out, c.st.dim(i18n.T("chat.history_cleared")))
			return nil
		}, nil),
		leaf("sources", "", T("tree.chat.sources"), func(c *chat, _ []string) error {
			c.showSources = !c.showSources
			fmt.Fprintln(c.e.out, c.st.dim(i18n.T("chat.show_sources", c.showSources)))
			return nil
		}, nil),
		leaf("model", "", T("tree.chat.model"), func(c *chat, _ []string) error {
			fmt.Fprintf(c.e.out, "%s · %s %s\n", c.provName, c.model(), c.st.dim(i18n.T("chat.model_hint")))
			return nil
		}, nil),
		leaf("clear", "", T("tree.chat.clear"), func(c *chat, _ []string) error {
			if c.st.on {
				fmt.Fprint(c.e.out, "\x1b[2J\x1b[H")
			}
			return nil
		}, nil),
		leaf("exit", "", T("tree.chat.exit"), func(*chat, []string) error { return errQuit }, nil),
	}
	alias := func(n *node, name string) *node {
		cp := *n
		cp.alias = true
		if name != "" {
			cp.name = name
		}
		return &cp
	}

	configKeyArg := func(c *chat, args []string, prefix string) []suggestion {
		if len(args) > 0 {
			return nil
		}
		return filterVals(configKeys(c.a.Cfg), prefix)
	}
	languageNode := leaf("language", T("arg.lang"), T("tree.config.language"),
		func(c *chat, args []string) error { return c.setLanguage(args) },
		func(c *chat, args []string, prefix string) []suggestion {
			if len(args) > 0 {
				return nil
			}
			return filterVals([]string{i18n.Auto, string(i18n.EN), string(i18n.PtBR), string(i18n.ES)}, prefix)
		})
	configNode := group("config", T("tree.config"),
		group("chat", T("tree.config.chat"), paramNodes(chatParams)...),
		group("rag", T("tree.config.rag"), paramNodes(ragParams)...),
		leaf("provider", T("arg.provider_field"), T("tree.config.provider"),
			func(c *chat, args []string) error { return c.configProvider(args) },
			func(c *chat, args []string, prefix string) []suggestion {
				switch len(args) {
				case 0:
					return filterVals(providerNames(c.a.Cfg), prefix)
				case 1:
					return filterVals(providerFields, prefix)
				}
				return nil
			}),
		languageNode,
		leaf("show", "", T("tree.config.show"), pass(viaConfig, "show"), nil),
		leaf("get", T("arg.key"), T("tree.config.get"), pass(viaConfig, "get"), configKeyArg),
		leaf("set", T("arg.key_value"), T("tree.config.set"), pass(viaConfig, "set"), configKeyArg),
		leaf("validate", "", T("tree.config.validate"), pass(viaConfig, "validate"), nil),
		leaf("path", "", T("tree.config.path"), pass(viaConfig, "path"), nil),
	)

	ragNode := group("rag", T("tree.rag"),
		leaf("ingest", T("arg.ingest"), T("tree.rag.ingest"), pass(viaRAG, "ingest"), func(c *chat, args []string, prefix string) []suggestion {
			return pathSuggestions(prefix, "")
		}),
		leaf("search", T("arg.query"), T("tree.rag.search"), pass(viaRAG, "search"), nil),
		leaf("list", "", T("tree.rag.list"), pass(viaRAG, "list"), nil),
		leaf("remove", T("arg.source"), T("tree.rag.remove"), pass(viaRAG, "remove"), func(c *chat, args []string, prefix string) []suggestion {
			if len(args) > 0 {
				return nil
			}
			return filterVals(c.ragSources(), prefix)
		}),
		leaf("clear", "", T("tree.rag.clear"), pass(viaRAG, "clear"), nil),
	)

	modelNames := func(c *chat, args []string, prefix string) []suggestion {
		if len(args) > 0 {
			return nil
		}
		return filterVals(c.installedModels(), prefix)
	}
	modelsNode := group("models", T("tree.models"),
		leaf("list", "", T("tree.models.list"), pass(viaModels, "list"), nil),
		leaf("ps", "", T("tree.models.ps"), pass(viaModels, "ps"), nil),
		leaf("select", "", T("tree.models.select"), pass(viaModels, "select"), nil),
		leaf("use", T("arg.model"), T("tree.models.use"), pass(viaModels, "use"), modelNames),
		leaf("pull", T("arg.model"), T("tree.models.pull"), pass(viaModels, "pull"), modelNames),
	)

	provNames := func(c *chat, args []string, prefix string) []suggestion {
		if len(args) > 0 {
			return nil
		}
		return filterVals(providerNames(c.a.Cfg), prefix)
	}
	providersNode := group("providers", T("tree.providers"),
		leaf("list", "", T("tree.providers.list"), pass(viaProviders, "list"), nil),
		leaf("test", T("arg.name_opt"), T("tree.providers.test"), pass(viaProviders, "test"), provNames),
		leaf("use", T("arg.name"), T("tree.providers.use"), func(c *chat, args []string) error {
			if len(args) != 1 {
				return usageSyntax("/providers use " + i18n.T("arg.name"))
			}
			return c.useProvider(args[0])
		}, provNames),
	)

	connNames := func(c *chat, args []string, prefix string) []suggestion {
		if len(args) > 0 {
			return nil
		}
		var names []string
		for _, cn := range c.a.Cfg.Connectors {
			names = append(names, cn.Name)
		}
		return filterVals(names, prefix)
	}
	connectorsNode := group("connectors", T("tree.connectors"),
		leaf("list", "", T("tree.connectors.list"), pass(viaConnectors, "list"), nil),
		leaf("add", T("arg.connector_add"), T("tree.connectors.add"), pass(viaConnectors, "add"), nil),
		leaf("remove", T("arg.name"), T("tree.connectors.remove"), pass(viaConnectors, "remove"), connNames),
		leaf("sync", T("arg.names_opt"), T("tree.connectors.sync"), pass(viaConnectors, "sync"), connNames),
	)

	chatNode := group("chat", T("tree.chat"), convo...)

	helpNode := leaf("help", T("arg.command_opt"), T("tree.help"), func(c *chat, args []string) error {
		c.printHelp(args)
		return nil
	}, func(c *chat, args []string, prefix string) []suggestion {
		level := c.tree
		for _, w := range args {
			m, _ := matchNode(level, w)
			if m == nil {
				return nil
			}
			level = m.children
		}
		var names []string
		for _, n := range level {
			if !n.alias {
				names = append(names, n.name)
			}
		}
		return filterVals(names, prefix)
	})

	tree := []*node{configNode, ragNode, modelsNode, providersNode, connectorsNode, chatNode, helpNode}
	for _, n := range convo {
		tree = append(tree, alias(n, ""))
	}
	tree = append(tree, alias(convo[len(convo)-1], "quit"), alias(languageNode, "lang"))
	return tree
}

// ---- parameters -------------------------------------------------------

func (c *chat) showKey(p param) {
	if p.kind == "model" {
		fmt.Fprintf(c.e.out, "providers.%s.model = %s\n", c.provName, c.model())
		return
	}
	v, err := config.Get(c.a.Cfg.Masked(), p.key)
	if err != nil {
		fmt.Fprintln(c.e.out, p.key)
		return
	}
	fmt.Fprintf(c.e.out, "%s = %v\n", p.key, v)
}

func (c *chat) setParam(p param, args []string) error {
	if len(args) == 0 {
		c.showKey(p)
		return nil
	}
	value := args[0]
	if p.kind == "text" {
		value = strings.Join(args, " ")
	} else if len(args) > 1 {
		return usageSyntax(p.name + " " + p.usage())
	}
	switch p.kind {
	case "bool":
		v, ok := parseOnOff(value)
		if !ok {
			return usageSyntax(p.name + " on|off")
		}
		value = v
	case "provider":
		if _, ok := c.a.Cfg.Providers[value]; !ok {
			return i18n.Errorf("chat.err.unknown_provider", value, strings.Join(providerNames(c.a.Cfg), ", "))
		}
	case "model":
		return c.setModel(value)
	}
	if err := setConfig(c.a, p.key, value); err != nil {
		return err
	}
	switch p.key {
	case "agent.provider":
		c.provName = value
	case "agent.use_rag":
		c.useRAG = value == "true"
	}
	c.showKey(p)
	c.paramNote(p)
	return nil
}

// paramNote explains side effects of a change that are easy to miss.
func (c *chat) paramNote(p param) {
	note := ""
	switch p.key {
	case "rag.chunk_size", "rag.chunk_overlap":
		note = i18n.T("chat.note.chunks")
	case "rag.embedder":
		if ix, err := c.a.Index(); err == nil && ix.Store.Len() > 0 && ix.Store.EmbedderID() != ix.Embedder.ID() {
			note = i18n.T("chat.note.embedder")
		}
	case "rag.store_path":
		note = i18n.T("chat.note.store")
	}
	if note != "" {
		fmt.Fprintln(c.e.out, c.st.yellow(i18n.T("cli.note", note)))
	}
}

func (c *chat) setModel(value string) error {
	if c.a.Cfg.Providers[c.provName].Type == config.TypeOllama {
		name, mm, err := c.a.OllamaManager(c.provName)
		if err != nil {
			return err
		}
		return c.e.use(c.ctx, c.a, name, mm, value, false)
	}
	if err := c.a.SetModel(c.provName, value, false); err != nil {
		return err
	}
	fmt.Fprintf(c.e.out, "providers.%s.model = %s\n", c.provName, strings.TrimSpace(value))
	return nil
}

func (c *chat) useProvider(name string) error {
	if _, ok := c.a.Cfg.Providers[name]; !ok {
		return i18n.Errorf("chat.err.unknown_provider", name, strings.Join(providerNames(c.a.Cfg), ", "))
	}
	prev := c.provName
	c.provName = name
	if err := c.rebuild(); err != nil {
		c.provName = prev
		return err
	}
	fmt.Fprintln(c.e.out, i18n.T("chat.using", name, c.model()))
	return nil
}

// setLanguage implements /config language [auto|en|pt-BR|es]: it saves the
// choice, switches the messages right away and redraws the command tree in the
// new language.
func (c *chat) setLanguage(args []string) error {
	if len(args) == 0 {
		fmt.Fprintf(c.e.out, "language = %s (%s)\n", c.a.Cfg.Language, i18n.Current())
		return nil
	}
	if len(args) != 1 {
		return usageSyntax("/config language " + i18n.T("arg.lang"))
	}
	if err := setConfig(c.a, "language", args[0]); err != nil {
		return err
	}
	c.tree = c.buildTree()
	fmt.Fprintln(c.e.out, i18n.T("chat.language.set", c.a.Cfg.Language))
	return nil
}

// configProvider implements /config provider <name> <field> [value].
func (c *chat) configProvider(args []string) error {
	fields := strings.Join(providerFields, ", ")
	if len(args) < 1 || len(args) > 3 {
		return usageSyntax(i18n.T("chat.provider.syntax", fields))
	}
	name := args[0]
	if _, ok := c.a.Cfg.Providers[name]; !ok {
		return i18n.Errorf("chat.err.unknown_provider", name, strings.Join(providerNames(c.a.Cfg), ", "))
	}
	if len(args) == 1 {
		b, _ := json.MarshalIndent(c.a.Cfg.Masked().Providers[name], "", "  ")
		fmt.Fprintln(c.e.out, string(b))
		return nil
	}
	field := args[1]
	if !contains(providerFields, field) {
		return usageKey("chat.provider.unknown_field", field, fields)
	}
	key := "providers." + name + "." + field

	var value string
	switch {
	case len(args) == 3:
		value = args[2]
		if field == "api_key" {
			fmt.Fprintln(c.e.out, c.st.yellow(i18n.T("chat.provider.key_tip", name)))
		}
	case field == "api_key":
		v, err := c.readSecret(i18n.T("chat.provider.secret_prompt", name))
		if err != nil {
			return err
		}
		if strings.TrimSpace(v) == "" {
			return usageKey("chat.provider.empty_secret")
		}
		value = strings.TrimSpace(v)
	default:
		v, _ := config.Get(c.a.Cfg.Masked(), key)
		fmt.Fprintf(c.e.out, "%s = %v\n", key, v)
		return nil
	}

	if field == "api_key_env" && !config.ValidEnvName(value) { // never echo: it may be a key
		return usageKey("chat.provider.env_not_name", name)
	}
	if err := setConfig(c.a, key, value); err != nil {
		return err
	}
	if field == "api_key" {
		fmt.Fprintln(c.e.out, i18n.T("chat.provider.saved_hidden", key))
	} else {
		fmt.Fprintf(c.e.out, "%s = %s\n", key, value)
	}
	return nil
}

// parseOnOff accepts on/off and the yes/no words of the supported languages.
func parseOnOff(s string) (string, bool) {
	switch strings.ToLower(s) {
	case "on", "true", "yes", "y", "1", "sim", "s", "sí", "si":
		return "true", true
	case "off", "false", "no", "n", "0", "nao", "não":
		return "false", true
	}
	return "", false
}

// ---- values for autocomplete ---------------------------------------------

func (c *chat) paramValues(p param, prefix string) []suggestion {
	switch p.kind {
	case "provider":
		return filterVals(providerNames(c.a.Cfg), prefix)
	case "embedder":
		return filterVals(append([]string{config.LocalEmbedder}, providerNames(c.a.Cfg)...), prefix)
	case "bool":
		return filterVals([]string{"on", "off"}, prefix)
	case "model":
		return filterVals(c.installedModels(), prefix)
	}
	return nil
}

func providerNames(cfg config.Config) []string {
	names := make([]string, 0, len(cfg.Providers))
	for n := range cfg.Providers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// configKeys lists every scalar setting as a dotted key.
func configKeys(cfg config.Config) []string {
	b, err := json.Marshal(cfg)
	if err != nil {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	var out []string
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, x := range t {
				if prefix != "" {
					k = prefix + "." + k
				}
				walk(k, x)
			}
		case string, float64, bool:
			out = append(out, prefix)
		}
	}
	walk("", m)
	sort.Strings(out)
	return out
}

func (c *chat) ragSources() []string {
	st, err := rag.OpenStore(c.a.StorePath())
	if err != nil {
		return nil
	}
	var out []string
	for _, s := range st.Sources() {
		out = append(out, s.Source)
	}
	return out
}

// ollamaProvider is the Ollama provider that /models acts on: the current
// one if it is Ollama, otherwise the first Ollama provider configured.
func (c *chat) ollamaProvider() string {
	if c.a.Cfg.Providers[c.provName].Type == config.TypeOllama {
		return c.provName
	}
	for _, n := range providerNames(c.a.Cfg) {
		if c.a.Cfg.Providers[n].Type == config.TypeOllama {
			return n
		}
	}
	return ""
}

type modelCache struct {
	at    time.Time
	names []string
}

// installedModels lists the models installed on the Ollama server, cached
// for a few seconds so autocomplete does not hit the server on every key.
func (c *chat) installedModels() []string {
	if time.Since(c.models.at) < 15*time.Second {
		return c.models.names
	}
	c.models.at = time.Now()
	c.models.names = nil
	_, mm, err := c.a.OllamaManager(c.ollamaProvider())
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(c.base, 1500*time.Millisecond)
	defer cancel()
	list, err := mm.ListModels(ctx)
	if err != nil {
		return nil
	}
	for _, m := range list {
		c.models.names = append(c.models.names, m.Name)
	}
	return c.models.names
}
