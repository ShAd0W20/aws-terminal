package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"aws-terminal/internal/ui/styles"
)

type FooterProps struct {
	Width       int
	Help        string
	StatusParts []string
	Compact     bool
}

func RenderFooter(props FooterProps) string {
	if props.Width <= 0 {
		return ""
	}

	lines := []string{truncateLine(props.Help, props.Width)}
	if len(props.StatusParts) > 0 {
		lines = append(lines, styles.MutedStyle.Render(truncateLine(strings.Join(props.StatusParts, " • "), props.Width)))
	}

	style := lipgloss.NewStyle().Width(props.Width)
	if !props.Compact {
		style = style.MarginTop(1)
	}

	return style.Render(strings.Join(lines, "\n"))
}

func truncateLine(value string, width int) string {
	value = strings.Join(strings.Fields(value), " ")
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	return string(runes[:width-1]) + "…"
}
