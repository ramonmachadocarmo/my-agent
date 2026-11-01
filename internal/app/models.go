package app

import (
	"strings"

	"my-agent/internal/config"
	"my-agent/internal/i18n"
	"my-agent/internal/provider"
)

// OllamaManager resolves a provider name (empty = agent default) to its model
// manager. Only providers of type ollama host local models.
func (a *App) OllamaManager(name string) (string, provider.ModelManager, error) {
	if name == "" {
		name = a.Cfg.Agent.Provider
	}
	pc, ok := a.Cfg.Providers[name]
	if !ok {
		return "", nil, i18n.Errorf("app.err.unknown_provider", name)
	}
	if pc.Type != config.TypeOllama {
		return "", nil, i18n.Errorf("app.err.not_ollama", name, pc.Type)
	}
	p, err := a.Provider(name)
	if err != nil {
		return "", nil, err
	}
	mm, ok := p.(provider.ModelManager)
	if !ok {
		return "", nil, i18n.Errorf("app.err.no_model_mgmt", name)
	}
	return name, mm, nil
}

// SetModel stores model as the chat model (or the embedding model) of the
// named provider and saves the config. The in-memory config is restored if
// saving fails.
func (a *App) SetModel(name, model string, embedding bool) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return i18n.Err("common.err.model_empty")
	}
	pc, ok := a.Cfg.Providers[name]
	if !ok {
		return i18n.Errorf("app.err.unknown_provider", name)
	}
	prev := pc
	if embedding {
		pc.EmbeddingModel = model
	} else {
		pc.Model = model
	}
	a.Cfg.Providers[name] = pc
	if err := a.Save(); err != nil {
		a.Cfg.Providers[name] = prev
		return err
	}
	return nil
}
