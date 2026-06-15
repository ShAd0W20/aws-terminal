package shell

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"

	"aws-terminal/internal/ui/components"
	"aws-terminal/internal/ui/pages"
	"aws-terminal/internal/ui/styles"
)

func (m Model) View() string {
	if !m.ready {
		return styles.DocStyle.Render("Loading AWS Terminal...")
	}

	innerWidth := max(0, m.innerWidth())
	innerHeight := max(0, m.innerHeight())
	if innerWidth == 0 || innerHeight == 0 {
		return ""
	}

	header := m.headerView(innerWidth)
	footer := m.footerView(innerWidth)
	contentHeight := max(0, innerHeight-lipgloss.Height(header)-lipgloss.Height(footer))
	content := m.contentView(innerWidth, contentHeight)

	body := lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
	body = lipgloss.NewStyle().
		Width(innerWidth).
		Height(innerHeight).
		MaxWidth(innerWidth).
		MaxHeight(innerHeight).
		Render(body)

	return styles.DocStyle.Render(body)
}

func (m Model) headerView(width int) string {
	return components.RenderHeader(components.HeaderProps{
		Width:  width,
		Title:  "AWS Terminal",
		Status: m.headerStatusText(),
	})
}

func (m Model) headerStatusText() string {
	profileText := "none"
	if activeProfile := m.activeProfileName(); activeProfile != "" {
		profileText = activeProfile
	}
	if m.profileBusy {
		if profile, ok := m.selectedProfile(); ok {
			profileText = profile.Name + " " + m.spinner.View()
		}
	}

	regionText := valueOrFallback(m.activeRegion(), "none")
	return fmt.Sprintf("Profile: %s • Region: %s", profileText, regionText)
}

func (m Model) contentView(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	if shouldCollapseSidebar(width, height, m.focus == focusPage) {
		return m.detailView(width, height)
	}

	sidebarWidth, sidebarHeight := sidebarDimensions(width, height)
	if sidebarWidth == width {
		return m.stackedContentView(width, height, sidebarHeight)
	}

	detailWidth := max(0, width-sidebarWidth-1)
	sidebar := m.sidebarView(sidebarWidth, sidebarHeight)
	detail := m.detailView(detailWidth, height)

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", detail)
}

func (m Model) stackedContentView(width, height, sidebarHeight int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	detailHeight := max(0, height-sidebarHeight)
	sidebar := m.sidebarView(width, sidebarHeight)
	detail := m.detailView(width, detailHeight)
	if detail == "" {
		return sidebar
	}

	return lipgloss.JoinVertical(lipgloss.Left, sidebar, detail)
}

func (m Model) sidebarView(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	profileHeight, regionHeight, pageHeight, showHint := sidebarPaneHeights(height, len(m.profiles), len(m.regions), len(m.pageRegistry))
	hint := ""
	if showHint {
		hint = "tab/shift+tab focus • enter apply/open • r refresh"
	}

	contentWidth := sidebarContentWidth(width)
	return components.RenderSidebar(components.SidebarProps{
		Width:  width,
		Height: height,
		Sections: []components.SidebarSection{
			{
				Title:    "Profiles",
				Focused:  m.focus == focusProfiles,
				Content:  m.listContent(m.profileList, contentWidth, profileHeight),
				MaxLines: profileHeight,
			},
			{
				Title:    "Regions",
				Focused:  m.focus == focusRegions,
				Content:  m.listContent(m.regionList, contentWidth, regionHeight),
				MaxLines: regionHeight,
			},
			{
				Title:    "Pages",
				Focused:  m.focus == focusNavigation,
				Content:  m.listContent(m.pageList, contentWidth, pageHeight),
				MaxLines: pageHeight,
			},
		},
		Hint: hint,
	})
}

func (m Model) listContent(current list.Model, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	current.SetSize(width, height)
	return current.View()
}

func (m Model) detailView(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if m.paletteOpen {
		return m.paletteView(width, height)
	}

	return m.currentPage().View(m.pageState(), width, height)
}

func (m Model) footerView(width int) string {
	if width <= 0 {
		return ""
	}

	compact := width < 150 || shouldCollapseSidebar(width, max(0, m.innerHeight()), m.focus == focusPage)
	helpView := m.help.View(m.currentHelpMap())
	if compact {
		helpView = styles.MutedStyle.Render(m.compactHelpText(width))
	}

	return components.RenderFooter(components.FooterProps{
		Width:       width,
		Help:        helpView,
		StatusParts: m.footerStatusParts(width, compact),
		Compact:     compact,
	})
}

func (m *Model) syncSidebarListsLayout() {
	innerWidth := max(0, m.innerWidth())
	innerHeight := max(0, m.innerHeight())
	if innerWidth <= 0 || innerHeight <= 0 {
		return
	}

	headerHeight := lipgloss.Height(m.headerView(innerWidth))
	footerHeight := lipgloss.Height(m.footerView(innerWidth))
	contentHeight := max(0, innerHeight-headerHeight-footerHeight)
	if shouldCollapseSidebar(innerWidth, contentHeight, m.focus == focusPage) {
		return
	}

	sidebarWidth, sidebarHeight := sidebarDimensions(innerWidth, contentHeight)
	if sidebarWidth <= 0 || sidebarHeight <= 0 {
		return
	}

	contentWidth := sidebarContentWidth(sidebarWidth)
	profileHeight, regionHeight, pageHeight, _ := sidebarPaneHeights(sidebarHeight, len(m.profiles), len(m.regions), len(m.pageRegistry))
	m.profileList.SetSize(contentWidth, profileHeight)
	m.regionList.SetSize(contentWidth, regionHeight)
	m.pageList.SetSize(contentWidth, pageHeight)
}

func (m *Model) applySidebarListFocus() {
	m.profileList.SetDelegate(newSidebarListDelegate(m.focus == focusProfiles))
	m.regionList.SetDelegate(newSidebarListDelegate(m.focus == focusRegions))
	m.pageList.SetDelegate(newSidebarListDelegate(m.focus == focusNavigation))
}

func (m Model) currentHelpMap() interface {
	ShortHelp() []key.Binding
	FullHelp() [][]key.Binding
} {
	if m.focus == focusPage {
		return m.currentPage()
	}

	return m.keys
}

func (m Model) compactHelpText(width int) string {
	bindings := m.currentHelpMap().ShortHelp()
	parts := make([]string, 0, len(bindings)+2)
	keyOnlyParts := make([]string, 0, len(bindings)+2)
	seen := map[string]struct{}{}
	for _, binding := range bindings {
		help := binding.Help()
		keyOnly := strings.TrimSpace(help.Key)
		desc := strings.TrimSpace(help.Desc)
		if keyOnly == "" {
			continue
		}
		keyText := keyOnly
		if desc != "" {
			desc = compactHelpDesc(desc)
			keyText += " " + desc
		}
		if _, ok := seen[keyText]; ok {
			continue
		}
		seen[keyText] = struct{}{}
		parts = append(parts, keyText)
		keyOnlyParts = append(keyOnlyParts, keyOnly)
	}
	if m.focus == focusPage {
		parts = append(parts, "tab nav")
		keyOnlyParts = append(keyOnlyParts, "tab nav")
	} else {
		parts = append(parts, ": commands")
		keyOnlyParts = append(keyOnlyParts, ":")
	}
	parts = append(parts, "q quit")
	keyOnlyParts = append(keyOnlyParts, "q")

	if len(parts) == 0 {
		return "tab nav • q quit"
	}

	line := strings.Join(parts, " • ")
	if width <= 0 || lipgloss.Width(line) <= width {
		return line
	}

	keyOnlyLine := strings.Join(keyOnlyParts, " • ")
	if lipgloss.Width(keyOnlyLine) <= width {
		return keyOnlyLine
	}

	essential := []string{}
	for _, part := range parts {
		if strings.HasPrefix(part, "tab") || strings.HasPrefix(part, "shift+tab") || strings.HasPrefix(part, "q ") || strings.HasPrefix(part, "b/") || strings.HasPrefix(part, "esc") || strings.HasPrefix(part, "enter") {
			essential = append(essential, part)
		}
	}
	if len(essential) == 0 {
		essential = parts[:min(len(parts), 4)]
	}
	return strings.Join(essential, " • ")
}

func compactHelpDesc(desc string) string {
	switch desc {
	case "select/detail":
		return "select"
	case "select/continue":
		return "select"
	case "continue/create":
		return "continue"
	case "switch focus":
		return "nav"
	case "next focus":
		return "focus"
	case "prev focus":
		return "prev focus"
	case "apply / open":
		return "open"
	case "refresh profiles":
		return "refresh"
	default:
		return desc
	}
}

func (m Model) footerStatusParts(width int, compact bool) []string {
	statusParts := []string{"Focus: " + m.focusLabel()}
	if compact && m.focus == focusPage {
		statusParts = append(statusParts, "tab nav")
	}
	if activeProfile := m.activeProfileName(); activeProfile != "" && (!compact || width >= 90) {
		statusParts = append(statusParts, "Active: "+activeProfile)
	}
	if activeRegion := m.activeRegion(); activeRegion != "" && (!compact || width >= 72) {
		statusParts = append(statusParts, "Region: "+activeRegion)
	}
	statusParts = append(statusParts, "Page: "+m.currentPage().Title())
	if pageStatus := m.currentPageStatus(); pageStatus.Error != "" {
		statusParts = append(statusParts, "Page error: "+compactStatusText(pageStatus.Error))
	} else if pageStatus.Message != "" && (!compact || width >= 120) {
		statusParts = append(statusParts, "Page status: "+compactStatusText(pageStatus.Message))
	}
	if m.profileBusy {
		statusParts = append(statusParts, "SSO login running")
	}
	if m.updateCheckBusy && !compact {
		statusParts = append(statusParts, "Checking updates")
	}
	if m.updateInstallBusy {
		statusParts = append(statusParts, "Updating app")
	}
	if m.updateAvailable != nil && (!compact || width >= 110) {
		statusParts = append(statusParts, "Update: "+m.updateAvailable.LatestVersion+" available")
	}
	if width >= 90 {
		statusParts = append(statusParts, fmt.Sprintf("%dx%d", m.width, m.height))
	}
	return statusParts
}

func (m Model) currentPageStatus() pages.Status {
	provider, ok := m.currentPage().(pages.StatusProvider)
	if !ok {
		return pages.Status{}
	}

	return provider.PageStatus(m.pageState())
}

func compactStatusText(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= 80 {
		return value
	}

	return value[:77] + "..."
}

func valueOrFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
