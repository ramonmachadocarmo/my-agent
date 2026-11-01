package tui

import (
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
)

// theme holds the colors of the interface (Tokyo Night inspired).
type theme struct {
	dark bool

	primary, secondary, accent lipgloss.Color
	success, warning, danger   lipgloss.Color
	text, muted, subtle        lipgloss.Color
	panel, border, onAccent    lipgloss.Color
}

func newTheme(dark bool) theme {
	if dark {
		return theme{
			dark:    true,
			primary: "#7aa2f7", secondary: "#bb9af7", accent: "#7dcfff",
			success: "#9ece6a", warning: "#e0af68", danger: "#f7768e",
			text: "#c0caf5", muted: "#737aa2", subtle: "#3b4261",
			panel: "#1f2335", border: "#414868", onAccent: "#1a1b26",
		}
	}
	return theme{
		primary: "#2e7de9", secondary: "#9854f1", accent: "#007197",
		success: "#587539", warning: "#8c6c3e", danger: "#f52a65",
		text: "#3760bf", muted: "#6172b0", subtle: "#c4c8da",
		panel: "#e9e9ed", border: "#a8aecb", onAccent: "#ffffff",
	}
}

func (t theme) glamourStyle() string {
	if t.dark {
		return styles.DarkStyle
	}
	return styles.LightStyle
}

func (t theme) fg(c lipgloss.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

func (t theme) mutedS() lipgloss.Style  { return t.fg(t.muted) }
func (t theme) dangerS() lipgloss.Style { return t.fg(t.danger) }

// badge is a colored label such as the agent name in the status bar.
func (t theme) badge(label string, bg lipgloss.Color) string {
	return lipgloss.NewStyle().Background(bg).Foreground(t.onAccent).Bold(true).Padding(0, 1).Render(label)
}

// agentColor is the configured color of an agent, or one derived from its
// name.
func (t theme) agentColor(name, configured string) lipgloss.Color {
	if configured != "" {
		return lipgloss.Color(configured)
	}
	palette := []lipgloss.Color{t.primary, t.secondary, t.accent, t.success, t.warning}
	h := 0
	for _, r := range name {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return palette[h%len(palette)]
}
