package ecr

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"

	"aws-terminal/internal/ui/styles"
	"aws-terminal/internal/ui/tableutil"
	"aws-terminal/internal/ui/workflow"
)

func (p *ECRPage) View(state State, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := []string{styles.SectionTitleStyle.Render("ECR"), styles.SubtitleStyle.Render("Search private ECR repositories, view images, then push local Docker images."), ""}
	if state.ActiveSession == nil {
		lines = append(lines, styles.MutedStyle.Render("No active AWS profile. Authenticate a profile from the sidebar first."))
		return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
	}
	lines = append(lines, fmt.Sprintf("Active profile: %s", state.ActiveSession.Profile), fmt.Sprintf("Account: %s", workflow.ValueOrFallback(state.ActiveSession.Account, "unknown")), fmt.Sprintf("Region: %s", workflow.ValueOrFallback(activeRegion(state), "unknown")))
	if state.PageFocused {
		lines = append(lines, styles.StatusStyle.Render("Page focus is active. Use the page-specific keys below."))
	} else {
		lines = append(lines, styles.MutedStyle.Render("Move focus to the Page area to interact with ECR."))
	}
	lines = append(lines, "")
	lines = append(lines, p.summaryLines()...)
	lines = append(lines, "")
	switch p.stage {
	case ecrStageRepository:
		lines = append(lines, p.repositoryLines(width, height)...)
	case ecrStageCreateRepository:
		lines = append(lines, p.createLines(width)...)
	case ecrStageRepositoryImages:
		lines = append(lines, p.imageLines(width, height, len(lines))...)
	case ecrStageLocalImage:
		lines = append(lines, p.localLines(width, height, len(lines))...)
	case ecrStageTag:
		lines = append(lines, p.tagLines(width)...)
	case ecrStageReview:
		lines = append(lines, p.reviewLines()...)
	case ecrStagePush:
		lines = append(lines, p.pushLines(width)...)
	}
	return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
}

func (p *ECRPage) ShortHelp() []key.Binding {
	switch p.stage {
	case ecrStageRepository:
		return []key.Binding{ecrUpKey, ecrDownKey, ecrEnterKey, ecrSearchKey, ecrCreateKey, ecrRefreshKey, ecrTabHelpKey}
	case ecrStageCreateRepository:
		return []key.Binding{ecrEnterKey, ecrBackKey, ecrTabHelpKey}
	case ecrStageRepositoryImages:
		return []key.Binding{ecrUpKey, ecrDownKey, ecrPagePrevKey, ecrPageNextKey, ecrEnterKey, ecrBackKey, ecrRefreshKey, ecrTabHelpKey}
	case ecrStageLocalImage:
		return []key.Binding{ecrUpKey, ecrDownKey, ecrPagePrevKey, ecrPageNextKey, ecrSearchKey, ecrEnterKey, ecrRefreshKey, ecrTabHelpKey}
	case ecrStageTag, ecrStageReview, ecrStagePush:
		return []key.Binding{ecrEnterKey, ecrBackKey, ecrTabHelpKey}
	default:
		return []key.Binding{ecrTabHelpKey}
	}
}
func (p *ECRPage) FullHelp() [][]key.Binding { return [][]key.Binding{p.ShortHelp()} }

func (p *ECRPage) summaryLines() []string {
	return []string{styles.MutedStyle.Render("Workflow summary"), fmt.Sprintf("Repository: %s", workflow.ValueOrFallback(p.selectedRepository.Name, "not selected")), fmt.Sprintf("Repository URI: %s", workflow.ValueOrFallback(p.selectedRepository.URI, "not selected")), fmt.Sprintf("Source image: %s", workflow.ValueOrFallback(p.selectedSourceImage(), "not selected"))}
}

func (p *ECRPage) repositoryLines(width, height int) []string {
	searchHint := "Press Ctrl+F to search."
	if p.searchInput.Focused() {
		searchHint = "Search active. Type to filter; Esc leaves search."
	}
	lines := []string{styles.MutedStyle.Render("Private ECR repositories"), styles.MutedStyle.Render(searchHint), p.searchInput.View()}
	if p.loadingRepositories {
		lines = append(lines, styles.StatusStyle.Render(p.spinner.View()+" Loading repositories..."))
		return lines
	}
	if p.repositoryErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.repositoryErr))
	}
	filtered := p.filteredRepositories()
	if len(filtered) == 0 {
		lines = append(lines, styles.MutedStyle.Render("No repositories match. Press Enter or c to create one."))
		return lines
	}
	visible := max(5, height-18)
	start := max(0, p.repositoryIndex-visible/2)
	end := min(len(filtered), start+visible)
	if end-start < visible {
		start = max(0, end-visible)
	}
	listWidth := max(40, width-styles.PanelStyle.GetHorizontalFrameSize()-8)
	listLines := []string{}
	for i := start; i < end; i++ {
		repo := filtered[i]
		prefix := "  "
		style := styles.SidebarItemStyle
		if i == p.repositoryIndex {
			prefix = "▸ "
			style = styles.FocusedSelectedSidebarItemStyle
		}
		name := tableutil.Truncate(repo.Name, max(12, listWidth/3))
		uri := tableutil.Truncate(repo.URI, max(12, listWidth-len([]rune(name))-6))
		listLines = append(listLines, style.Render(fmt.Sprintf("%s%s  %s", prefix, name, styles.MutedStyle.Render(uri))))
	}
	lines = append(lines, tableutil.RenderBox(strings.Join(listLines, "\n"), listWidth))
	return lines
}
func (p *ECRPage) createLines(width int) []string {
	lines := []string{styles.MutedStyle.Render("Create private ECR repository"), p.createInput.View()}
	if p.repositoryErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.repositoryErr))
	}
	if p.loadingRepositories {
		lines = append(lines, styles.StatusStyle.Render(p.spinner.View()+" Creating repository..."))
	}
	lines = append(lines, styles.MutedStyle.Render("Repository names must be lowercase and may include /, _, ., or -."))
	return lines
}
func (p *ECRPage) imageLines(width, height, usedLines int) []string {
	lines := []string{styles.MutedStyle.Render("Repository images"), fmt.Sprintf("Repository: %s", p.selectedRepository.Name)}
	if p.imagesLoading {
		return append(lines, styles.StatusStyle.Render(p.spinner.View()+" Loading images..."))
	}
	if p.imagesErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.imagesErr))
	}
	if p.pushMessage != "" {
		lines = append(lines, styles.StatusStyle.Render(p.pushMessage))
	}
	if len(p.repositoryImages) == 0 {
		lines = append(lines, styles.MutedStyle.Render("No images found in this repository yet."))
	}

	if len(p.repositoryImages) > 0 {
		tableWidth := max(60, width-styles.PanelStyle.GetHorizontalFrameSize()-8)
		p.configureImageTable(tableWidth, height-usedLines-len(lines)-7)
		p.syncImageTable()
		lines = append(lines, "", tableutil.RenderBox(p.imageTable.View(), tableWidth+4))
		if p.imagePaginator.TotalPages > 1 {
			start, end := p.imagePaginator.GetSliceBounds(len(p.repositoryImages))
			lines = append(lines, styles.MutedStyle.Render(fmt.Sprintf("Page %s · showing %d-%d of %d · use ←/h and →/l to page", p.imagePaginator.View(), start+1, end, len(p.repositoryImages))))
		}
	}
	lines = append(lines, "", styles.MutedStyle.Render("Press Enter to choose a local Docker image to push."))
	return lines
}
func (p *ECRPage) localLines(width, height, usedLines int) []string {
	searchHint := "Press Ctrl+F to search local images; if no image matches, the search text is used as a manual image reference."
	if p.manualInput.Focused() {
		searchHint = "Local image search active. Type to filter; Esc leaves search."
	}
	lines := []string{styles.MutedStyle.Render("Step 1 of 3 · Select a local Docker image"), styles.MutedStyle.Render(searchHint), p.manualInput.View()}
	if p.localLoading {
		lines = append(lines, styles.StatusStyle.Render(p.spinner.View()+" Loading local Docker images..."))
		return lines
	}
	if p.localErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.localErr), styles.MutedStyle.Render("You can still type a manual image reference above."))
	}
	filtered := p.filteredLocalImages()
	if len(filtered) == 0 {
		lines = append(lines, styles.MutedStyle.Render("No local Docker images match. Press Enter to use the typed image reference manually."))
		return lines
	}

	tableWidth := max(60, width-styles.PanelStyle.GetHorizontalFrameSize()-8)
	p.configureLocalTable(tableWidth, height-usedLines-len(lines)-6)
	p.syncLocalTable()
	lines = append(lines, "", tableutil.RenderBox(p.localTable.View(), tableWidth+4))
	start, end := p.localPaginator.GetSliceBounds(len(filtered))
	if p.localPaginator.TotalPages > 1 {
		lines = append(lines, styles.MutedStyle.Render(fmt.Sprintf("Page %s · showing %d-%d of %d local images · use ←/h and →/l to page", p.localPaginator.View(), start+1, end, len(filtered))))
	} else {
		lines = append(lines, styles.MutedStyle.Render(fmt.Sprintf("Showing %d-%d of %d local images", start+1, end, len(filtered))))
	}
	return lines
}
func (p *ECRPage) tagLines(width int) []string {
	lines := []string{styles.MutedStyle.Render("Step 2 of 3 · Edit destination tag"), fmt.Sprintf("Source: %s", p.selectedSourceImage()), p.tagInput.View()}
	if p.planErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.planErr))
	}
	if p.planning {
		lines = append(lines, styles.StatusStyle.Render("Building push plan..."))
	}
	return lines
}
func (p *ECRPage) reviewLines() []string {
	lines := []string{styles.MutedStyle.Render("Step 3 of 3 · Review push")}
	if p.planErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.planErr))
	}
	if p.plan == nil {
		return append(lines, styles.MutedStyle.Render("No push plan available."))
	}
	lines = append(lines, fmt.Sprintf("Source: %s", p.plan.SourceImage), fmt.Sprintf("Destination: %s", p.plan.DestinationImage), "", styles.MutedStyle.Render("Press Enter to login, tag, and push using the Docker Engine API."))
	return lines
}
func (p *ECRPage) pushLines(width int) []string {
	lines := []string{styles.MutedStyle.Render("Pushing image")}
	if p.pushing {
		lines = append(lines, styles.StatusStyle.Render(p.spinner.View()+" Pushing..."), fmt.Sprintf("%s %s %s", p.pushProgress.ID, p.pushProgress.Status, p.pushProgress.Detail))
	}
	if p.pushErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.pushErr))
	}
	if p.pushResult != nil {
		lines = append(lines, styles.StatusStyle.Render("Push complete."), fmt.Sprintf("Destination: %s", p.pushResult.DestinationImage))
		if p.pushResult.Digest != "" {
			lines = append(lines, fmt.Sprintf("Digest: %s", p.pushResult.Digest))
		}
	}
	return lines
}
func shortDigest(v string) string {
	if len(v) > 19 {
		return v[:19] + "…"
	}
	return v
}

var _ = time.Second
