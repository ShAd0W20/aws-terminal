package ecs

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"aws-terminal/internal/ui/styles"
	"aws-terminal/internal/ui/tableutil"
	"aws-terminal/internal/ui/workflow"
)

func (p *ECSPage) View(state State, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := []string{styles.SectionTitleStyle.Render("ECS"), styles.SubtitleStyle.Render("Browse ECS clusters, services, and tasks."), ""}
	if state.ActiveSession == nil {
		lines = append(lines, styles.MutedStyle.Render("No active AWS profile. Authenticate a profile from the sidebar first."))
		return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
	}
	lines = append(lines, fmt.Sprintf("Active profile: %s", state.ActiveSession.Profile), fmt.Sprintf("Account: %s", workflow.ValueOrFallback(state.ActiveSession.Account, "unknown")), fmt.Sprintf("Region: %s", workflow.ValueOrFallback(activeRegion(state), "unknown")))
	if state.PageFocused {
		lines = append(lines, styles.StatusStyle.Render("Page focus is active. Use the page-specific keys below."))
	} else {
		lines = append(lines, styles.MutedStyle.Render("Move focus to the Page area to interact with ECS."))
	}
	lines = append(lines, "")
	switch p.stage {
	case ecsStageClusters:
		lines = append(lines, p.clusterLines(width, height, len(lines))...)
	case ecsStageResources:
		lines = append(lines, p.resourceLines(width, height, len(lines))...)
	case ecsStageServiceDetail:
		lines = append(lines, p.serviceDetailLines()...)
	case ecsStageTaskDetail:
		p.configureLogViewport(width, height, len(lines))
		lines = append(lines, p.taskDetailLines()...)
	}
	return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
}
func (p *ECSPage) ShortHelp() []key.Binding {
	switch p.stage {
	case ecsStageClusters:
		return []key.Binding{ecsUpKey, ecsDownKey, ecsPagePrevKey, ecsPageNextKey, ecsEnterKey, ecsSearchKey, ecsRefreshKey, ecsTabHelpKey}
	case ecsStageResources:
		return []key.Binding{ecsUpKey, ecsDownKey, ecsPagePrevKey, ecsPageNextKey, ecsPrevTabKey, ecsNextTabKey, ecsEnterKey, ecsSearchKey, ecsRefreshKey, ecsBackKey, ecsTabHelpKey}
	case ecsStageTaskDetail:
		if p.taskDetailTab == taskDetailTabLogs {
			return []key.Binding{ecsPrevTabKey, ecsNextTabKey, ecsPrevContainerKey, ecsNextContainerKey, ecsUpKey, ecsDownKey, ecsBackKey, ecsTabHelpKey}
		}
		return []key.Binding{ecsPrevTabKey, ecsNextTabKey, ecsBackKey, ecsTabHelpKey}
	default:
		return []key.Binding{ecsBackKey, ecsTabHelpKey}
	}
}
func (p *ECSPage) FullHelp() [][]key.Binding { return [][]key.Binding{p.ShortHelp()} }

func (p *ECSPage) clusterLines(width, height, usedLines int) []string {
	lines := []string{styles.MutedStyle.Render("ECS clusters"), p.searchHint("clusters"), p.searchInput.View()}
	if p.loadingClusters {
		return append(lines, styles.StatusStyle.Render(p.spinner.View()+" Loading clusters..."))
	}
	if p.clustersErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.clustersErr))
	}
	items := p.filteredClusters()
	if len(items) == 0 {
		if strings.TrimSpace(p.searchInput.Value()) != "" {
			return append(lines, styles.MutedStyle.Render("No ECS clusters match the current search."))
		}
		return append(lines, styles.MutedStyle.Render("No ECS clusters found in this region."))
	}
	tableWidth := max(40, width-styles.PanelStyle.GetHorizontalFrameSize()-8)
	p.configureClusterTable(tableWidth, height-usedLines-len(lines)-6)
	p.syncClusterTable()
	lines = append(lines, "", tableutil.RenderBox(p.clusterTable.View(), tableWidth+4))
	start, end := p.clusterPaginator.GetSliceBounds(len(items))
	lines = append(lines, styles.MutedStyle.Render(fmt.Sprintf("Page %s · showing %d-%d of %d", p.clusterPaginator.View(), start+1, end, len(items))))
	return lines
}
func (p *ECSPage) resourceLines(width, height, usedLines int) []string {
	tab := "Services"
	if p.tab == ecsTabTasks {
		tab = "Tasks"
	}
	lines := []string{fmt.Sprintf("Cluster: %s", p.selectedCluster.Name), styles.MutedStyle.Render("Tabs: [ Services ] Tasks"), p.searchHint(strings.ToLower(tab)), p.searchInput.View()}
	if p.tab == ecsTabTasks {
		lines[1] = styles.MutedStyle.Render("Tabs: Services [ Tasks ]")
	}
	if p.tab == ecsTabServices {
		lines = append(lines, p.servicesLines(width, height, usedLines+len(lines))...)
	} else {
		lines = append(lines, p.tasksLines(width, height, usedLines+len(lines))...)
	}
	return lines
}
func (p *ECSPage) servicesLines(width, height, usedLines int) []string {
	lines := []string{}
	if p.servicesLoading {
		return append(lines, styles.StatusStyle.Render(p.spinner.View()+" Loading services..."))
	}
	if p.servicesErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.servicesErr))
	}
	items := p.filteredServices()
	if len(items) == 0 {
		if strings.TrimSpace(p.searchInput.Value()) != "" {
			return append(lines, styles.MutedStyle.Render("No ECS services match the current search."))
		}
		return append(lines, styles.MutedStyle.Render("No ECS services found in this cluster."))
	}
	tableWidth := max(48, width-styles.PanelStyle.GetHorizontalFrameSize()-8)
	p.configureServiceTable(tableWidth, height-usedLines-len(lines)-6)
	p.syncServiceTable()
	lines = append(lines, "", tableutil.RenderBox(p.serviceTable.View(), tableWidth+4))
	start, end := p.servicePaginator.GetSliceBounds(len(items))
	return append(lines, styles.MutedStyle.Render(fmt.Sprintf("Page %s · showing %d-%d of %d", p.servicePaginator.View(), start+1, end, len(items))))
}
func (p *ECSPage) tasksLines(width, height, usedLines int) []string {
	lines := []string{}
	if p.tasksLoading {
		return append(lines, styles.StatusStyle.Render(p.spinner.View()+" Loading tasks..."))
	}
	if p.tasksErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.tasksErr))
	}
	items := p.filteredTasks()
	if len(items) == 0 {
		if strings.TrimSpace(p.searchInput.Value()) != "" {
			return append(lines, styles.MutedStyle.Render("No ECS tasks match the current search."))
		}
		return append(lines, styles.MutedStyle.Render("No non-stopped ECS tasks found in this cluster."))
	}
	tableWidth := max(56, width-styles.PanelStyle.GetHorizontalFrameSize()-8)
	p.configureTaskTable(tableWidth, height-usedLines-len(lines)-6)
	p.syncTaskTable()
	lines = append(lines, "", tableutil.RenderBox(p.taskTable.View(), tableWidth+4))
	start, end := p.taskPaginator.GetSliceBounds(len(items))
	return append(lines, styles.MutedStyle.Render(fmt.Sprintf("Page %s · showing %d-%d of %d", p.taskPaginator.View(), start+1, end, len(items))))
}
func (p *ECSPage) searchHint(scope string) string {
	if p.searchInput.Focused() {
		return styles.StatusStyle.Render("Search active. Type to filter; Esc leaves search.")
	}
	return styles.MutedStyle.Render("Press Ctrl+F to search " + scope + ".")
}

func (p *ECSPage) serviceDetailLines() []string {
	s := p.selectedService
	lines := []string{
		styles.MutedStyle.Render("Service detail"),
		"",
		fmt.Sprintf("%s  •  %d running / %d desired  •  %d pending", statusLabel(s.Status), s.RunningCount, s.DesiredCount, s.PendingCount),
	}

	if reason := serviceAttentionReason(s); reason != "" {
		lines = append(lines, "", styles.StatusStyle.Render("Needs attention"), reason)
	}

	lines = append(lines,
		"",
		styles.MutedStyle.Render("Network"),
		detailKV("Subnets", fmt.Sprint(s.SubnetCount)),
		detailKV("Security groups", fmt.Sprint(s.SecurityGroupCount)),
		detailKV("Public IP", s.AssignPublicIP),
		"",
		styles.MutedStyle.Render("Runtime"),
		detailKV("Launch type", s.LaunchType),
		detailKV("Capacity", strings.Join(s.CapacityProviders, ", ")),
		detailKV("Platform", s.PlatformVersion),
		detailKV("Task definition", s.TaskDefinition),
		detailKV("Created", timeText(s.CreatedAt)),
	)
	if strings.TrimSpace(s.DeploymentController) != "" {
		lines = append(lines, detailKV("Controller", s.DeploymentController))
	}

	if len(s.Deployments) > 0 {
		lines = append(lines, "", styles.MutedStyle.Render("Deployments"))
		for _, d := range s.Deployments {
			state := d.RolloutState
			if strings.TrimSpace(state) == "" {
				state = d.Status
			}
			lines = append(lines, fmt.Sprintf("%s  •  %s  •  %d running / %d desired  •  %d pending", statusLabel(state), value(d.TaskDefinition), d.RunningCount, d.DesiredCount, d.PendingCount))
		}
	}

	lines = append(lines,
		"",
		styles.MutedStyle.Render("Identifiers"),
		detailKV("Name", s.Name),
		detailKV("Service ARN", s.ARN),
		detailKV("Task def ARN", s.TaskDefinitionARN),
		"",
		styles.MutedStyle.Render("Press b or Esc to return."),
	)
	return lines
}

func (p *ECSPage) taskDetailLines() []string {
	if p.taskDetailTab == taskDetailTabLogs {
		return p.taskLogLines()
	}
	return append(p.taskTabHeaderLines(), p.taskOverviewLines()...)
}

func (p *ECSPage) taskTabHeaderLines() []string {
	if p.taskDetailTab == taskDetailTabLogs {
		return []string{styles.MutedStyle.Render("Tabs: Overview [ Logs ]")}
	}
	return []string{styles.MutedStyle.Render("Tabs: [ Overview ] Logs")}
}

func (p *ECSPage) configureLogViewport(width, height, usedLines int) {
	viewportWidth := max(30, width-styles.PanelStyle.GetHorizontalFrameSize()-8)
	viewportHeight := max(5, height-usedLines-10)
	if p.logViewport.Width != viewportWidth || p.logViewport.Height != viewportHeight {
		p.logViewport.Width = viewportWidth
		p.logViewport.Height = viewportHeight
		p.renderLogViewportContent()
	}
}

func (p *ECSPage) taskLogLines() []string {
	lines := p.taskTabHeaderLines()
	target := p.selectedLogTarget()
	containerLabel := value(target.ContainerName)
	if len(p.logTargets) > 0 {
		containerLabel = fmt.Sprintf("%s (%d/%d)", value(target.ContainerName), p.logContainerIndex+1, len(p.logTargets))
	}
	state := "stopped"
	if p.logStreaming {
		state = "streaming"
	}
	lines = append(lines, "", styles.MutedStyle.Render("Task logs"), fmt.Sprintf("Container: %s  •  %s", containerLabel, state))
	if p.logTargetsLoading {
		return append(lines, styles.StatusStyle.Render(p.spinner.View()+" Resolving log configuration..."))
	}
	if p.logTargetsErr != "" {
		return append(lines, styles.ErrorStyle.Render(p.logTargetsErr))
	}
	if len(p.logTargets) == 0 {
		return append(lines, styles.MutedStyle.Render("No containers reported for this task."))
	}
	if !target.Supported {
		return append(lines, styles.MutedStyle.Render(value(target.Message)))
	}
	if p.logEventsErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.logEventsErr))
	}
	if p.logEventsLoading && len(p.logEvents) == 0 {
		lines = append(lines, styles.StatusStyle.Render(p.spinner.View()+" Loading logs..."))
	}
	if len(p.logEvents) == 0 && !p.logEventsLoading && p.logEventsErr == "" {
		lines = append(lines, styles.MutedStyle.Render("No log events in the last 15 minutes yet."))
	}
	lines = append(lines, p.logViewport.View(), styles.MutedStyle.Render("Scroll ↑/↓ or k/j · switch tabs [/]: Overview/Logs · containers ctrl+h/ctrl+l · b/Esc back"))
	return lines
}

func (p *ECSPage) renderLogViewportContent() {
	atBottom := p.logViewport.AtBottom()
	width := p.logViewport.Width
	if width <= 0 {
		width = 80
	}
	rows := make([]string, 0, len(p.logEvents))
	for _, event := range p.logEvents {
		rows = append(rows, renderLogEvent(event.Timestamp.Local().Format("15:04:05"), event.Message, width))
	}
	p.logViewport.SetContent(strings.Join(rows, "\n"))
	if atBottom || p.logViewport.YOffset >= max(0, p.logViewport.TotalLineCount()-p.logViewport.Height-2) {
		p.logViewport.GotoBottom()
	}
}

func renderLogEvent(timestamp, message string, width int) string {
	prefix := styles.MutedStyle.Render(timestamp + " ")
	message = strings.TrimRight(message, "\n")
	wrapped := ansi.Wrap(colorizeLogSeverityMarker(message), max(10, width-9), "")
	parts := strings.Split(wrapped, "\n")
	if len(parts) == 0 {
		return prefix
	}
	parts[0] = prefix + parts[0]
	indent := strings.Repeat(" ", 9)
	for i := 1; i < len(parts); i++ {
		parts[i] = indent + parts[i]
	}
	return strings.Join(parts, "\n")
}

var logLevelPatterns = map[string]*regexp.Regexp{
	"error": regexp.MustCompile(`(?i)^\s*(?:\[?error\]?\b|level[=:]\s*"?error"?|"level"\s*:\s*"error")`),
	"warn":  regexp.MustCompile(`(?i)^\s*(?:\[?warn(?:ing)?\]?\b|level[=:]\s*"?warn(?:ing)?"?|"level"\s*:\s*"warn(?:ing)?")`),
	"info":  regexp.MustCompile(`(?i)^\s*(?:\[?info\]?\b|level[=:]\s*"?info"?|"level"\s*:\s*"info")`),
	"debug": regexp.MustCompile(`(?i)^\s*(?:\[?debug\]?\b|level[=:]\s*"?debug"?|"level"\s*:\s*"debug")`),
}

func detectLogSeverity(message string) string {
	level, _, _ := findLogSeverityMarker(message)
	return level
}

func colorizeLogSeverityMarker(message string) string {
	level, start, end := findLogSeverityMarker(message)
	if level == "" {
		return message
	}
	return message[:start] + severityStyle(level).Render(message[start:end]) + message[end:]
}

func findLogSeverityMarker(message string) (string, int, int) {
	prefix := stripLeadingLogTimestamp(message)
	if len(prefix) > 80 {
		prefix = prefix[:80]
	}
	baseOffset := strings.Index(message, prefix)
	if baseOffset < 0 {
		baseOffset = len(message) - len(strings.TrimLeft(message, " \t"))
	}
	for level, pattern := range logLevelPatterns {
		if loc := pattern.FindStringIndex(prefix); loc != nil {
			return level, baseOffset + loc[0], baseOffset + loc[1]
		}
	}
	return "", 0, 0
}

func stripLeadingLogTimestamp(message string) string {
	message = strings.TrimSpace(message)
	fields := strings.Fields(message)
	if len(fields) == 0 {
		return message
	}
	first := fields[0]
	if looksLikeLogTimestamp(first) && len(fields) > 1 {
		return strings.TrimSpace(strings.TrimPrefix(message, first))
	}
	return message
}

func looksLikeLogTimestamp(value string) bool {
	if len(value) < len("2006-01-02T15:04:05") {
		return false
	}
	return len(value) >= 19 && value[4] == '-' && value[7] == '-' && (value[10] == 'T' || value[10] == ' ')
}

func severityStyle(level string) lipgloss.Style {
	switch level {
	case "error":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	case "warn":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	case "info":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	case "debug":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("111"))
	default:
		return lipgloss.NewStyle()
	}
}

func (p *ECSPage) taskOverviewLines() []string {
	t := p.selectedTask
	health := strings.ToLower(value(t.HealthStatus))
	lines := []string{
		styles.MutedStyle.Render("Task detail"),
		"",
		fmt.Sprintf("%s  •  health %s", statusLabel(t.LastStatus), health),
	}

	if reason := taskAttentionReason(t); reason != "" {
		lines = append(lines, "", styles.StatusStyle.Render("Needs attention"), reason)
	}

	timeLabel, timeValue := "Started", timeText(t.StartedAt)
	if !t.StoppedAt.IsZero() || strings.EqualFold(t.LastStatus, "STOPPED") {
		timeLabel, timeValue = "Stopped", timeText(t.StoppedAt)
	} else if timeValue == "—" {
		timeLabel, timeValue = "Created", timeText(t.CreatedAt)
	}
	lines = append(lines,
		"",
		detailKV("IP", t.PrivateIP),
		detailKV("Location", t.AvailabilityZone),
		detailKV("Runtime", t.LaunchType),
		detailKV("Connectivity", t.Connectivity),
		detailKV(timeLabel, timeValue),
	)

	lines = append(lines, "", styles.MutedStyle.Render("Containers"))
	if len(t.Containers) == 0 {
		lines = append(lines, styles.MutedStyle.Render("No containers reported."))
	}
	for _, c := range t.Containers {
		exit := ""
		if c.ExitCode != nil {
			exit = fmt.Sprintf("  •  exit %d", *c.ExitCode)
		}
		lines = append(lines, fmt.Sprintf("%s  %s  •  %s%s", statusLabel(c.LastStatus), value(c.Name), shortImage(c.Image), exit))
		if strings.TrimSpace(c.Reason) != "" {
			lines = append(lines, "  reason: "+c.Reason)
		}
	}

	if len(t.Attachments) > 0 {
		lines = append(lines, "", styles.MutedStyle.Render("Network attachments"))
		for _, a := range t.Attachments {
			lines = append(lines, fmt.Sprintf("ENI %s  •  subnet %s  •  private IP %s", value(a.ENI), value(a.Subnet), value(a.PrivateIP)))
		}
	}

	lines = append(lines,
		"",
		styles.MutedStyle.Render("Identifiers"),
		detailKV("Task ID", t.ID),
		detailKV("Task definition", t.TaskDefinition),
		detailKV("Group", t.Group),
		detailKV("Task ARN", t.ARN),
		detailKV("Task def ARN", t.TaskDefinitionARN),
		"",
		styles.MutedStyle.Render("Press b or Esc to return."),
	)
	return lines
}
