package tableutil

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"aws-terminal/internal/ui/styles"
)

type ColumnSpec struct {
	Title  string
	Min    int
	Weight int
	Max    int
}

func FitColumns(width int, specs []ColumnSpec) []table.Column {
	if len(specs) == 0 {
		return nil
	}
	contentWidth := width - len(specs)*2
	if contentWidth < len(specs) {
		contentWidth = len(specs)
	}
	cols := make([]table.Column, len(specs))
	total := 0
	weight := 0
	for i, spec := range specs {
		minWidth := max(1, spec.Min)
		cols[i] = table.Column{Title: spec.Title, Width: minWidth}
		total += minWidth
		weight += max(0, spec.Weight)
	}
	if total > contentWidth {
		for total > contentWidth {
			changed := false
			for i := range cols {
				if total <= contentWidth {
					break
				}
				if cols[i].Width > 1 {
					cols[i].Width--
					total--
					changed = true
				}
			}
			if !changed {
				break
			}
		}
		return cols
	}
	remaining := contentWidth - total
	for remaining > 0 && weight > 0 {
		changed := false
		for i, spec := range specs {
			if remaining <= 0 {
				break
			}
			if spec.Weight <= 0 {
				continue
			}
			if spec.Max > 0 && cols[i].Width >= spec.Max {
				continue
			}
			cols[i].Width++
			remaining--
			changed = true
		}
		if !changed {
			break
		}
	}
	return cols
}

func AdaptiveRows(pageHeight, usedLines int) int {
	return max(5, pageHeight-usedLines-styles.PanelStyle.GetVerticalFrameSize())
}

func RenderBox(content string, width int) string {
	boxWidth := max(12, width)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).
		Width(boxWidth).
		Render(content)
}

func Truncate(value string, width int) string {
	value = strings.TrimSpace(value)
	if width <= 0 || len([]rune(value)) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(value)
	return string(runes[:width-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
