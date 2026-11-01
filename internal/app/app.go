// Package app wires configuration, providers, RAG and connectors together.
// Both the CLI and the web UI are thin layers over App.
package app

import (
	"context"
	"fmt"
	"path/filepath"

	"my-agent/internal/agent"
	"my-agent/internal/config"
	"my-agent/internal/connectors"
	"my-agent/internal/i18n"
	"my-agent/internal/provider"
	"my-agent/internal/rag"
)

type App struct {
	Path string // config file path
	Cfg  config.Config

	// NewProvider builds providers; tests replace it with fakes.
	NewProvider func(name string, pc config.ProviderConfig) (provider.Provider, error)
}

// Load reads the config at path.
func Load(path string) (*App, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	return &App{Path: path, Cfg: cfg, NewProvider: provider.New}, nil
}

// Save persists the current config.
func (a *App) Save() error { return a.Cfg.Save(a.Path) }

// Provider returns the named provider (empty name = the agent's default).
func (a *App) Provider(name string) (provider.Provider, error) {
	if name == "" {
		name = a.Cfg.Agent.Provider
	}
	pc, ok := a.Cfg.Providers[name]
	if !ok {
		return nil, i18n.Errorf("app.err.unknown_provider", name)
	}
	return a.NewProvider(name, pc)
}

// StorePath resolves rag.store_path relative to the config file.
func (a *App) StorePath() string {
	p := a.Cfg.RAG.StorePath
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(filepath.Dir(a.Path), p)
}

// Embedder returns the configured embedder.
func (a *App) Embedder() (rag.Embedder, error) {
	name := a.Cfg.RAG.Embedder
	if name == config.LocalEmbedder {
		return rag.LocalEmbedder{}, nil
	}
	p, err := a.Provider(name)
	if err != nil {
		return nil, err
	}
	model := a.Cfg.Providers[name].EmbeddingModel
	return rag.FuncEmbedder{Name: fmt.Sprintf("%s:%s", name, model), Fn: p.Embed}, nil
}

// Index opens the RAG index. Opening the store is cheap; callers may open
// one per operation.
func (a *App) Index() (*rag.Index, error) {
	emb, err := a.Embedder()
	if err != nil {
		return nil, err
	}
	st, err := rag.OpenStore(a.StorePath())
	if err != nil {
		return nil, err
	}
	r := a.Cfg.RAG
	return &rag.Index{Store: st, Embedder: emb, ChunkSize: r.ChunkSize, Overlap: r.ChunkOverlap, TopK: r.TopK, MinScore: r.MinScore}, nil
}

// Agent builds the agent. providerName overrides the default provider;
// useRAG=false disables retrieval regardless of config.
func (a *App) Agent(providerName string, useRAG bool) (*agent.Agent, error) {
	p, err := a.Provider(providerName)
	if err != nil {
		return nil, err
	}
	ag := &agent.Agent{Config: a.Cfg.Agent, Provider: p}
	ag.Config.UseRAG = ag.Config.UseRAG && useRAG
	if ag.Config.UseRAG {
		if ag.Index, err = a.Index(); err != nil {
			return nil, err
		}
	}
	return ag, nil
}

// SyncConnector ingests a configured connector.
func (a *App) SyncConnector(ctx context.Context, name string) (connectors.Stats, error) {
	cn, ok := a.Cfg.FindConnector(name)
	if !ok {
		return connectors.Stats{}, i18n.Errorf("app.err.unknown_connector", name)
	}
	return a.IngestConnector(ctx, cn)
}

// IngestConnector ingests an ad-hoc connector definition.
func (a *App) IngestConnector(ctx context.Context, cn config.ConnectorConfig) (connectors.Stats, error) {
	l, err := connectors.New(cn)
	if err != nil {
		return connectors.Stats{}, err
	}
	ix, err := a.Index()
	if err != nil {
		return connectors.Stats{}, err
	}
	return connectors.Ingest(ctx, ix, l)
}
