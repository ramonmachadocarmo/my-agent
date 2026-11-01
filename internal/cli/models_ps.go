package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"my-agent/internal/app"
	"my-agent/internal/i18n"
	"my-agent/internal/provider"
)

// modelsPS shows which models are loaded and whether they fit in GPU memory.
func (e *env) modelsPS(ctx context.Context, a *app.App, provName string, mm provider.ModelManager) error {
	rn, ok := mm.(provider.Runner)
	if !ok {
		return i18n.Errorf("cli.ps.cannot_report", provName)
	}
	running, err := rn.RunningModels(ctx)
	if err != nil {
		return err
	}
	if len(running) == 0 {
		fmt.Fprintln(e.out, i18n.T("cli.ps.none"))
		return nil
	}
	h := strings.Split(i18n.T("cli.ps.header"), "|") // NAME|SIZE|PROCESSOR|CONTEXT|UNLOADS IN
	for len(h) < 5 {
		h = append(h, "")
	}
	fmt.Fprintf(e.out, "%-32s %9s  %-18s %7s  %s\n", h[0], h[1], h[2], h[3], h[4])
	for _, m := range running {
		fmt.Fprintf(e.out, "%-32s %9s  %-18s %7d  %s\n", m.Name, formatBytes(m.Size), processor(m), m.ContextLength, unloadsIn(m.ExpiresAt))
	}
	for _, m := range running {
		if m.OnCPU() {
			fmt.Fprintln(e.out, "\n"+offloadWarning(m, provName))
		}
	}
	return nil
}

func processor(m provider.RunningModel) string {
	if !m.OnCPU() {
		return "100% GPU"
	}
	g := m.GPUPercent()
	return fmt.Sprintf("%d%% GPU / %d%% CPU", g, 100-g)
}

func unloadsIn(t time.Time) string {
	d := time.Until(t)
	switch {
	case t.IsZero() || d > 365*24*time.Hour:
		return i18n.T("cli.ps.never")
	case d <= 0:
		return i18n.T("cli.ps.now")
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm", int(d.Round(time.Minute).Minutes()))
}

func offloadWarning(m provider.RunningModel, provName string) string {
	return i18n.T("cli.ps.warning", m.Name, 100-m.GPUPercent(), provName)
}

// checkOffload warns once, after the first reply, if the model in use is
// partly on the CPU. Failures are ignored: this is a hint, not a feature.
func (e *env) checkOffload(ctx context.Context, st style, p provider.Provider, provName, model string) {
	rn, ok := p.(provider.Runner)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	running, err := rn.RunningModels(ctx)
	if err != nil {
		return
	}
	for _, m := range running {
		if provider.ModelInstalled([]provider.ModelInfo{{Name: m.Name}}, model) && m.OnCPU() {
			fmt.Fprintf(e.out, "\n%s\n", st.yellow(offloadWarning(m, provName)))
			return
		}
	}
}

// warmUp loads the model in the background while the user types the first
// message, so that message does not pay the loading time.
func warmUp(ctx context.Context, p provider.Provider) {
	w, ok := p.(provider.Warmer)
	if !ok {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		_ = w.Warmup(ctx) // errors surface on the first real request
	}()
}
