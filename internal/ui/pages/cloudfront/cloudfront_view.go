package cloudfront

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"

	"aws-terminal/internal/ui/styles"
	"aws-terminal/internal/ui/tableutil"
)

func (p *CloudFrontPage) View(state State, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	lines := []string{
		styles.SectionTitleStyle.Render("CloudFront"),
		styles.SubtitleStyle.Render("Select a distribution and create or copy an invalidation."),
		"",
	}

	if state.ActiveSession == nil {
		lines = append(lines, styles.MutedStyle.Render("No active AWS session. Authenticate a profile first."))
		return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
	}

	lines = append(lines,
		fmt.Sprintf("Active profile: %s", state.ActiveSession.Profile),
		fmt.Sprintf("Region: %s", valueOrFallback(activeRegionFromState(state), "us-east-1")),
	)
	if state.PageFocused {
		lines = append(lines, styles.StatusStyle.Render("Page focus is active. Tab or Shift+Tab returns to Pages."))
	}
	if p.copiedMessage != "" {
		lines = append(lines, "", styles.StatusStyle.Render(p.copiedMessage))
	}
	if p.createErr != "" {
		lines = append(lines, "", styles.ErrorStyle.Render(p.createErr))
	}

	lines = append(lines, "")
	switch p.stage {
	case cloudFrontStageDistribution:
		lines = append(lines, p.distributionStageLines(width, height)...)
	case cloudFrontStagePaths:
		lines = append(lines, p.pathsStageLines(width)...)
	case cloudFrontStageResult:
		lines = append(lines, p.resultStageLines()...)
	}

	return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
}

func (p *CloudFrontPage) ShortHelp() []key.Binding {
	switch p.stage {
	case cloudFrontStageDistribution:
		return []key.Binding{cloudFrontUpKey, cloudFrontDownKey, cloudFrontEnterKey, cloudFrontCancelKey, cloudFrontTabKey}
	case cloudFrontStagePaths:
		return []key.Binding{cloudFrontEnterKey, cloudFrontCopyKey, cloudFrontBackKey, cloudFrontCancelKey, cloudFrontTabKey}
	case cloudFrontStageResult:
		if p.creating {
			return []key.Binding{cloudFrontCancelKey, cloudFrontTabKey}
		}
		return []key.Binding{cloudFrontCopyKey, cloudFrontBackKey, cloudFrontCancelKey, cloudFrontTabKey}
	default:
		return []key.Binding{cloudFrontTabKey}
	}
}

func (p *CloudFrontPage) FullHelp() [][]key.Binding {
	return [][]key.Binding{p.ShortHelp()}
}

func (p *CloudFrontPage) distributionStageLines(width, height int) []string {
	lines := []string{styles.MutedStyle.Render("Step 1 · Select a distribution")}
	if p.loading {
		return append(lines, styles.StatusStyle.Render("Loading distributions..."))
	}
	if p.loadErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.loadErr))
	}
	if len(p.distributions) == 0 {
		return append(lines, styles.MutedStyle.Render("No distributions found for this profile."))
	}

	visible := max(5, height-14)
	start := max(0, p.distributionIndex-visible/2)
	end := min(len(p.distributions), start+visible)
	if end-start < visible {
		start = max(0, end-visible)
	}

	listLines := []string{}
	listWidth := max(40, width-styles.PanelStyle.GetHorizontalFrameSize()-8)
	for index := start; index < end; index++ {
		distribution := p.distributions[index]
		prefix := "  "
		style := styles.SidebarItemStyle
		if index == p.distributionIndex {
			prefix = "▸ "
			style = styles.FocusedSelectedSidebarItemStyle
		}

		label := distribution.ID
		if alias := firstAlias(distribution.Aliases); alias != "" {
			label += "  " + alias
		} else if distribution.DomainName != "" {
			label += "  " + distribution.DomainName
		}
		label = tableutil.Truncate(label, listWidth-4)
		if !distribution.Enabled {
			label += styles.MutedStyle.Render("  disabled")
		}
		listLines = append(listLines, style.Render(prefix+label))
	}
	lines = append(lines, tableutil.RenderBox(strings.Join(listLines, "\n"), listWidth+4))

	lines = append(lines, "", styles.MutedStyle.Render("Press Enter to continue with the selected distribution."))
	return lines
}

func (p *CloudFrontPage) pathsStageLines(width int) []string {
	lines := []string{
		styles.MutedStyle.Render("Step 2 · Invalidation"),
		fmt.Sprintf("Distribution: %s", p.selectedDistribution.ID),
	}
	if alias := firstAlias(p.selectedDistribution.Aliases); alias != "" {
		lines = append(lines, fmt.Sprintf("Alias: %s", alias))
	}
	if p.selectedDistribution.DomainName != "" {
		lines = append(lines, fmt.Sprintf("Domain: %s", p.selectedDistribution.DomainName))
	}

	p.pathsInput.Width = max(12, width-12)
	lines = append(lines, "", p.pathsInput.View())
	lines = append(lines,
		"",
		styles.MutedStyle.Render("Use comma or space separated paths, for example: /* or /assets/* /index.html"),
		styles.MutedStyle.Render("Enter creates the invalidation. Press c to copy the AWS CLI command instead."),
	)
	return lines
}

func (p *CloudFrontPage) resultStageLines() []string {
	lines := []string{styles.MutedStyle.Render("Invalidation")}
	if p.creating && p.invalidation == nil {
		lines = append(lines,
			styles.StatusStyle.Render(p.spinner.View()+" Creating invalidation..."),
			styles.MutedStyle.Render("Waiting for CloudFront to accept the request."),
		)
		return lines
	}
	if p.invalidation == nil {
		return lines
	}

	statusLine := fmt.Sprintf("Status: %s", valueOrFallback(p.invalidation.Status, "unknown"))
	if p.creating {
		statusLine = p.spinner.View() + " " + statusLine
	}

	lines = append(lines,
		styles.StatusStyle.Render(fmt.Sprintf("ID: %s", p.invalidation.ID)),
		styles.StatusStyle.Render(statusLine),
	)
	if p.creating {
		lines = append(lines, styles.MutedStyle.Render("Refreshing invalidation status..."))
	}
	if !p.invalidation.CreatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Created: %s", p.invalidation.CreatedAt.Local().Format("2006-01-02 15:04:05")))
	}
	if len(p.invalidation.Paths) > 0 {
		lines = append(lines, fmt.Sprintf("Paths: %s", strings.Join(p.invalidation.Paths, ", ")))
	}
	if p.creating {
		lines = append(lines, "", styles.MutedStyle.Render("Press Esc to stop waiting for status. The invalidation may still continue in CloudFront."))
	} else {
		lines = append(lines, "", styles.MutedStyle.Render("Press b or Esc to select another distribution, or c to copy the CLI command."))
	}
	return lines
}
