package shell

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"aws-terminal/internal/ui/styles"
)

type quickAction struct {
	Title       string
	Description string
	Run         func(*Model) tea.Cmd
}

func (m Model) quickActions() []quickAction {
	actions := []quickAction{
		{
			Title:       "Refresh AWS profiles",
			Description: "Reload shared AWS config and credentials profiles.",
			Run: func(m *Model) tea.Cmd {
				m.statusMessage = "Refreshing AWS profiles..."
				m.errorMessage = ""
				return m.loadProfilesCmd()
			},
		},
	}

	if m.updateService != nil {
		actions = append(actions,
			quickAction{
				Title:       "Check for updates",
				Description: "Look for the latest GitHub release.",
				Run: func(m *Model) tea.Cmd {
					m.statusMessage = "Checking for updates..."
					m.errorMessage = ""
					return m.checkUpdatesCmd(false)
				},
			},
			quickAction{
				Title:       "Disable startup update checks",
				Description: "Stop checking GitHub releases automatically on launch.",
				Run:         func(m *Model) tea.Cmd { return m.disableStartupUpdateChecks() },
			},
		)
	}

	actions = append(actions,
		quickAction{
			Title:       "Focus profiles",
			Description: "Move keyboard focus to the profile list.",
			Run:         func(m *Model) tea.Cmd { return m.setFocus(focusProfiles) },
		},
		quickAction{
			Title:       "Focus regions",
			Description: "Move keyboard focus to the region list.",
			Run:         func(m *Model) tea.Cmd { return m.setFocus(focusRegions) },
		},
		quickAction{
			Title:       "Focus pages",
			Description: "Move keyboard focus to the page navigation list.",
			Run:         func(m *Model) tea.Cmd { return m.setFocus(focusNavigation) },
		},
		quickAction{
			Title:       "Focus current page workflow",
			Description: "Move keyboard focus into the selected page workflow.",
			Run:         func(m *Model) tea.Cmd { return m.setFocus(focusPage) },
		},
	)

	if m.updateAvailable != nil {
		latest := m.updateAvailable.LatestVersion
		actions = append(actions, quickAction{
			Title:       "Update aws-terminal to " + latest,
			Description: "Download, verify, and install the latest release.",
			Run: func(m *Model) tea.Cmd {
				m.statusMessage = "Updating aws-terminal to " + latest + "..."
				m.errorMessage = ""
				return m.installUpdateCmd()
			},
		})
	}

	for _, page := range m.pageRegistry {
		pageID := page.ID()
		title := page.Title()
		description := page.Description()
		actions = append(actions, quickAction{
			Title:       "Open " + title,
			Description: description,
			Run: func(m *Model) tea.Cmd {
				previousPageID := m.currentPageID()
				m.selectPageListItem(pageID)
				m.rememberPage(m.currentPageID())

				cmds := []tea.Cmd{m.currentPageStateCmd()}
				if previousPageID != m.currentPageID() && m.focus == focusPage {
					if previousPage := m.pageByID(previousPageID); previousPage != nil {
						cmds = append(cmds, previousPage.SetFocused(false))
					}
				}
				cmds = append(cmds, m.setFocus(focusPage))
				return tea.Batch(cmds...)
			},
		})
	}

	return actions
}

func (m *Model) openPalette() {
	m.paletteOpen = true
	m.paletteIndex = 0
}

func (m *Model) closePalette() {
	m.paletteOpen = false
	m.paletteIndex = 0
}

func (m *Model) updatePalette(msg tea.KeyMsg) tea.Cmd {
	actions := m.quickActions()
	if len(actions) == 0 {
		m.closePalette()
		return nil
	}

	switch msg.String() {
	case "esc", "ctrl+c":
		m.closePalette()
		return nil
	case "up", "k":
		if m.paletteIndex > 0 {
			m.paletteIndex--
		}
		return nil
	case "down", "j":
		if m.paletteIndex < len(actions)-1 {
			m.paletteIndex++
		}
		return nil
	case "enter":
		index := clamp(m.paletteIndex, 0, len(actions)-1)
		action := actions[index]
		m.closePalette()
		return action.Run(m)
	default:
		return nil
	}
}

func (m Model) paletteView(width, height int) string {
	if !m.paletteOpen || width <= 0 || height <= 0 {
		return ""
	}

	actions := m.quickActions()
	visible := min(len(actions), max(1, height-6))
	start := max(0, m.paletteIndex-visible/2)
	end := min(len(actions), start+visible)
	if end-start < visible {
		start = max(0, end-visible)
	}

	lines := []string{
		styles.SectionTitleStyle.Render("Command palette"),
		styles.MutedStyle.Render("↑/↓ choose • enter run • esc close"),
		"",
	}
	for index := start; index < end; index++ {
		action := actions[index]
		prefix := "  "
		style := styles.SidebarItemStyle
		if index == m.paletteIndex {
			prefix = "▸ "
			style = styles.FocusedSelectedSidebarItemStyle
		}
		line := fmt.Sprintf("%s%s", prefix, action.Title)
		if strings.TrimSpace(action.Description) != "" {
			line += styles.MutedStyle.Render(" — " + action.Description)
		}
		lines = append(lines, style.Render(line))
	}

	boxWidth := min(width, max(40, width-4))
	boxHeight := min(height, lipgloss.Height(strings.Join(lines, "\n"))+2)
	return styles.RenderBox(styles.PanelStyle, boxWidth, boxHeight, strings.Join(lines, "\n"))
}
