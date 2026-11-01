package provider

import (
	"context"
	"strings"
	"time"

	"my-agent/internal/i18n"
)

// ModelInfo describes a locally installed model.
type ModelInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// PullProgress is one progress update of a model download.
type PullProgress struct {
	Status    string `json:"status"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

// Percent returns 0-100, or -1 when the total is unknown.
func (p PullProgress) Percent() int {
	if p.Total <= 0 {
		return -1
	}
	return int(p.Completed * 100 / p.Total)
}

// ModelManager is implemented by providers that host local models (Ollama).
type ModelManager interface {
	ListModels(ctx context.Context) ([]ModelInfo, error)
	PullModel(ctx context.Context, name string, progress func(PullProgress)) error
}

// ModelInstalled reports whether name is in models. A name without a tag
// matches its ":latest" tag, as Ollama does.
func ModelInstalled(models []ModelInfo, name string) bool {
	want := withTag(name)
	for _, m := range models {
		if withTag(m.Name) == want {
			return true
		}
	}
	return false
}

func withTag(name string) string {
	name = strings.TrimSpace(name)
	if !strings.Contains(name, ":") {
		name += ":latest"
	}
	return name
}

// EnsureModel downloads name unless it is already installed. pulled reports
// whether a download happened.
func EnsureModel(ctx context.Context, mm ModelManager, name string, progress func(PullProgress)) (pulled bool, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, i18n.Err("common.err.model_empty")
	}
	models, err := mm.ListModels(ctx)
	if err != nil {
		return false, err
	}
	if ModelInstalled(models, name) {
		return false, nil
	}
	return true, mm.PullModel(ctx, name, progress)
}

// Throttle forwards an update only when the status text changes or the
// percentage advances, so UIs are not flooded by thousands of tiny updates.
func Throttle(next func(PullProgress)) func(PullProgress) {
	lastStatus, lastPct := "", -2
	return func(p PullProgress) {
		if pct := p.Percent(); p.Status != lastStatus || pct != lastPct {
			lastStatus, lastPct = p.Status, pct
			next(p)
		}
	}
}

// RunningModel is a model currently loaded in memory.
type RunningModel struct {
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	SizeVRAM      int64     `json:"size_vram"`
	ContextLength int       `json:"context_length"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// GPUPercent is the share of the model held in GPU memory (0-100).
func (m RunningModel) GPUPercent() int {
	if m.Size <= 0 {
		return 0
	}
	return int(m.SizeVRAM * 100 / m.Size)
}

// OnCPU reports whether part of the model runs on the CPU, which makes
// generation much slower.
func (m RunningModel) OnCPU() bool { return m.Size > 0 && m.SizeVRAM < m.Size }

// Runner is implemented by providers that can list loaded models.
type Runner interface {
	RunningModels(ctx context.Context) ([]RunningModel, error)
}

// Warmer is implemented by providers that can preload their model so the
// first reply does not pay the loading time.
type Warmer interface {
	Warmup(ctx context.Context) error
}
