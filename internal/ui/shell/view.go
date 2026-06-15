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

	helpView := m.help.View(m.currentHelpMap())
	if width < 64 {
		helpView = styles.MutedStyle.Render(m.condensedHelpText())
	}

	statusParts := []string{"Focus: " + m.focusLabel()}
	if activeProfile := m.activeProfileName(); activeProfile != "" {
		statusParts = append(statusParts, "Active: "+activeProfile)
	}
	if activeRegion := m.activeRegion(); activeRegion != "" {
		statusParts = append(statusParts, "Region: "+activeRegion)
	}
	statusParts = append(statusParts, "Page: "+m.currentPage().Title())
	if pageStatus := m.currentPageStatus(); pageStatus.Error != "" {
		statusParts = append(statusParts, "Page error: "+compactStatusText(pageStatus.Error))
	} else if pageStatus.Message != "" {
		statusParts = append(statusParts, "Page status: "+compactStatusText(pageStatus.Message))
	}
	if m.profileBusy {
		statusParts = append(statusParts, "SSO login running")
	}
	if m.updateCheckBusy {
		statusParts = append(statusParts, "Checking updates")
	}
	if m.updateInstallBusy {
		statusParts = append(statusParts, "Updating app")
	}
	if m.updateAvailable != nil {
		statusParts = append(statusParts, "Update: "+m.updateAvailable.LatestVersion+" available")
	}
	if width >= 90 {
		statusParts = append(statusParts, fmt.Sprintf("%dx%d", m.width, m.height))
	}

	return components.RenderFooter(components.FooterProps{
		Width:       width,
		Help:        helpView,
		StatusParts: statusParts,
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

func (m Model) condensedHelpText() string {
	if m.focus == focusPage {
		return "tab/shift+tab focus • use page keys • q quit"
	}

	return "↑/↓ move • tab/shift+tab focus • enter apply/open • : commands • q quit"
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
