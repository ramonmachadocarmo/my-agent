package cli

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"my-agent/internal/app"
	"my-agent/internal/config"
	"my-agent/internal/provider"
)

// perfFake adds the optional Runner and Warmer capabilities to the base fake.
type perfFake struct {
	*fakeProvider
	running []provider.RunningModel
	warmed  atomic.Int32
}

func (p *perfFake) RunningModels(context.Context) ([]provider.RunningModel, error) {
	return p.running, nil
}
func (p *perfFake) Warmup(context.Context) error { p.warmed.Add(1); return nil }

func setupPerf(t *testing.T, running ...provider.RunningModel) (string, *perfFake) {
	t.Helper()
	cfg, fp := setup(t)
	pf := &perfFake{fakeProvider: fp, running: running}
	orig := loadApp
	loadApp = func(path string) (*app.App, error) {
		a, err := orig(path)
		if err == nil {
			a.NewProvider = func(string, config.ProviderConfig) (provider.Provider, error) { return pf, nil }
		}
		return a, err
	}
	t.Cleanup(func() { loadApp = orig })
	return cfg, pf
}

var (
	onGPU = provider.RunningModel{Name: "llama3.2:latest", Size: 2000, SizeVRAM: 2000, ContextLength: 4096, ExpiresAt: time.Now().Add(29 * time.Minute)}
	onCPU = provider.RunningModel{Name: "llama3.2:latest", Size: 2000, SizeVRAM: 800, ContextLength: 4096, ExpiresAt: time.Now().Add(29 * time.Minute)}
)

func TestModelsPS(t *testing.T) {
	cfg, _ := setupPerf(t, onGPU)
	mustRun(t, "", "-config", cfg, "init")
	out := mustRun(t, "", "-config", cfg, "models", "ps")
	if !strings.Contains(out, "llama3.2:latest") || !strings.Contains(out, "100% GPU") || !strings.Contains(out, "29m") || strings.Contains(out, "warning") {
		t.Fatalf("%s", out)
	}

	cfg, _ = setupPerf(t, onCPU)
	mustRun(t, "", "-config", cfg, "init")
	out = mustRun(t, "", "-config", cfg, "models", "ps")
	if !strings.Contains(out, "40% GPU / 60% CPU") || !strings.Contains(out, "warning: llama3.2:latest runs 60% on the CPU") || !strings.Contains(out, "num_ctx") {
		t.Fatalf("offload not reported:\n%s", out)
	}

	cfg, _ = setupPerf(t)
	mustRun(t, "", "-config", cfg, "init")
	if out := mustRun(t, "", "-config", cfg, "models", "ps"); !strings.Contains(out, "no model is loaded") {
		t.Fatalf("%s", out)
	}
}

func TestChatWarmsUpAndWarnsOnceWhenOnCPU(t *testing.T) {
	cfg, pf := setupPerf(t, onCPU)
	mustRun(t, "", "-config", cfg, "init")
	out := mustRun(t, "one\ntwo\n", "-config", cfg, "chat", "--no-rag")
	deadline := time.Now().Add(2 * time.Second) // warm-up runs in the background
	for pf.warmed.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if pf.warmed.Load() != 1 {
		t.Fatalf("expected one background warm-up, got %d", pf.warmed.Load())
	}
	if n := strings.Count(out, "warning: llama3.2:latest runs 60% on the CPU"); n != 1 {
		t.Fatalf("want the CPU warning exactly once (after the first reply), got %d:\n%s", n, out)
	}
	if strings.Index(out, "agent> echo: one") > strings.Index(out, "warning:") {
		t.Fatalf("warning must come after the first reply:\n%s", out)
	}
}

func TestChatDoesNotWarnWhenOnGPU(t *testing.T) {
	cfg, _ := setupPerf(t, onGPU)
	mustRun(t, "", "-config", cfg, "init")
	if out := mustRun(t, "hi\n", "-config", cfg, "chat", "--no-rag"); strings.Contains(out, "warning") {
		t.Fatalf("unexpected warning:\n%s", out)
	}
}

func TestUnloadsIn(t *testing.T) {
	for want, at := range map[string]time.Time{
		"never": {},
		"now":   time.Now().Add(-time.Minute),
		"29m":   time.Now().Add(29 * time.Minute),
	} {
		if got := unloadsIn(at); got != want {
			t.Errorf("unloadsIn = %q, want %q", got, want)
		}
	}
	if got := unloadsIn(time.Now().Add(time.Hour * 24 * 400)); got != "never" {
		t.Errorf("far future should read as never, got %q", got)
	}
}
