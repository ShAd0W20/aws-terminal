package ecs

import (
	"aws-terminal/internal/ui/pageapi"
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

func (p *ECSPage) View(state pageapi.State, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := []string{styles.SectionTitleStyle.Render("ECS"), styles.SubtitleStyle.Render("Browse ECS clusters, services, and tasks."), ""}
	if state.ActiveSession == nil {
		lines = append(lines, styles.MutedStyle.Render("No active AWS profile. Authenticate a profile from the sidebar first."))
		return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
	}
	lines = append(lines, fmt.Sprintf("Active profile: %s", state.ActiveSession.Profile), fmt.Sprintf("Account: %s", workflow.ValueOrFallback(state.ActiveSession.Account, "unknown")), fmt.Sprintf("Region: %s", workflow.ValueOrFallback(workflow.ActiveRegion(state), "unknown")))
	if state.PageFocused {
		lines = append(lines, styles.StatusStyle.Render("Page focus active · tab returns to navigation."))
	} else {
		lines = append(lines, styles.MutedStyle.Render("Focus Page to interact with ECS."))
	}
	if p.updateSuccess != "" {
		lines = append(lines, styles.StatusStyle.Render(p.updateSuccess))
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
	case ecsStageUpdateTaskDefinition:
		lines = append(lines, p.updateTaskDefinitionLines(width, height, len(lines))...)
	case ecsStageUpdateDesiredCount:
		lines = append(lines, p.updateDesiredCountLines(width)...)
	case ecsStageUpdateReview:
		lines = append(lines, p.updateReviewLines()...)
	case ecsStageUpdating:
		lines = append(lines, p.updatingServiceLines()...)
	case ecsStageStopTaskReason:
		lines = append(lines, p.stopTaskReasonLines()...)
	case ecsStageStopTaskReview:
		lines = append(lines, p.stopTaskReviewLines()...)
	case ecsStageStoppingTask:
		lines = append(lines, p.stoppingTaskLines()...)
	}
	return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
}
func (p *ECSPage) ShortHelp() []key.Binding {
	switch p.stage {
	case ecsStageClusters:
		return []key.Binding{ecsUpKey, ecsDownKey, ecsPagePrevKey, ecsPageNextKey, ecsEnterKey, ecsSearchKey, ecsRefreshKey, ecsTabHelpKey}
	case ecsStageResources:
		return []key.Binding{ecsUpKey, ecsDownKey, ecsPagePrevKey, ecsPageNextKey, ecsPrevTabKey, ecsNextTabKey, ecsEnterKey, ecsSearchKey, ecsRefreshKey, ecsBackKey, ecsTabHelpKey}
	case ecsStageUpdateTaskDefinition:
		return []key.Binding{ecsUpKey, ecsDownKey, ecsPagePrevKey, ecsPageNextKey, ecsEnterKey, ecsBackKey, ecsTabHelpKey}
	case ecsStageUpdateDesiredCount:
		return []key.Binding{ecsEnterKey, ecsBackKey, ecsTabHelpKey}
	case ecsStageUpdateReview:
		return []key.Binding{ecsToggleKey, ecsEnterKey, ecsBackKey, ecsTabHelpKey}
	case ecsStageUpdating, ecsStageStoppingTask:
		return []key.Binding{ecsBackKey, ecsTabHelpKey}
	case ecsStageStopTaskReason:
		return []key.Binding{ecsEnterKey, ecsBackKey, ecsTabHelpKey}
	case ecsStageStopTaskReview:
		return []key.Binding{ecsEnterKey, ecsBackKey, ecsTabHelpKey}
	case ecsStageTaskDetail:
		stopKeys := []key.Binding{}
		if isTaskStoppable(p.selectedTask, p.selectedCluster.ARN) {
			stopKeys = append(stopKeys, ecsStopTaskKey)
		}
		if p.taskDetailTab == taskDetailTabLogs {
			return append([]key.Binding{ecsPrevTabKey, ecsNextTabKey, ecsPrevContainerKey, ecsNextContainerKey, ecsUpKey, ecsDownKey}, append(stopKeys, ecsBackKey, ecsTabHelpKey)...)
		}
		return append([]key.Binding{ecsPrevTabKey, ecsNextTabKey}, append(stopKeys, ecsBackKey, ecsTabHelpKey)...)
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
	return styles.MutedStyle.Render("Ctrl+F search " + scope + " · keys in footer")
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
		styles.MutedStyle.Render("u updates service · b/Esc returns · keys in footer"),
	)
	return lines
}

func (p *ECSPage) updateTaskDefinitionLines(width, height, usedLines int) []string {
	lines := []string{styles.MutedStyle.Render("Step 1 of 3 · Select task definition"), fmt.Sprintf("Cluster: %s", p.selectedCluster.Name), fmt.Sprintf("Service: %s", p.selectedService.Name)}
	if p.taskDefinitionsLoading {
		return append(lines, "", styles.StatusStyle.Render(p.spinner.View()+" Loading task definitions..."))
	}
	if p.taskDefinitionsErr != "" {
		return append(lines, "", styles.ErrorStyle.Render(p.taskDefinitionsErr), styles.MutedStyle.Render("b/Esc returns"))
	}
	if len(p.taskDefinitions) == 0 {
		return append(lines, "", styles.MutedStyle.Render("No task definitions found for family "+value(p.updateFamilyPrefix)+"."), styles.MutedStyle.Render("b/Esc returns"))
	}
	p.taskDefinitionPaginator.PerPage = max(5, height-usedLines-len(lines)-7)
	p.syncTaskDefinitionSelection()
	start, end := p.taskDefinitionPaginator.GetSliceBounds(len(p.taskDefinitions))
	lines = append(lines, "")
	for i := start; i < end; i++ {
		definition := p.taskDefinitions[i]
		marker := "  "
		if i == p.taskDefinitionIndex {
			marker = "> "
		}
		current := ""
		if definition.ARN == p.selectedService.TaskDefinitionARN {
			current = " (current)"
		}
		line := fmt.Sprintf("%s%s%s", marker, value(definition.DisplayName), current)
		if definition.Status != "" {
			line += " · " + definition.Status
		}
		lines = append(lines, ansi.Truncate(line, max(20, width-8), "…"))
	}
	start, end = p.taskDefinitionPaginator.GetSliceBounds(len(p.taskDefinitions))
	lines = append(lines, "", styles.MutedStyle.Render(fmt.Sprintf("Page %s · showing %d-%d of %d · Enter continues · b/Esc returns", p.taskDefinitionPaginator.View(), start+1, end, len(p.taskDefinitions))))
	return lines
}

func (p *ECSPage) updateDesiredCountLines(width int) []string {
	p.desiredCountInput.Width = max(8, width-24)
	lines := []string{styles.MutedStyle.Render("Step 2 of 3 · Desired task count"), fmt.Sprintf("Cluster: %s", p.selectedCluster.Name), fmt.Sprintf("Service: %s", p.selectedService.Name), fmt.Sprintf("Task definition: %s", value(p.selectedTaskDefinition().DisplayName)), "", p.desiredCountInput.View(), styles.MutedStyle.Render(fmt.Sprintf("Current desired count: %d · Enter continues · b/Esc returns", p.selectedService.DesiredCount))}
	if p.updateErr != "" {
		lines = append(lines, "", styles.ErrorStyle.Render(p.updateErr))
	}
	return lines
}

func (p *ECSPage) updateReviewLines() []string {
	desired, err := p.parsedDesiredCount()
	selected := p.selectedTaskDefinition()
	lines := []string{styles.MutedStyle.Render("Step 3 of 3 · Review service update"), fmt.Sprintf("Cluster: %s", p.selectedCluster.Name), fmt.Sprintf("Service: %s", p.selectedService.Name), "", styles.MutedStyle.Render("Task definition"), detailKV("Current", p.selectedService.TaskDefinition), detailKV("New", selected.DisplayName), "", styles.MutedStyle.Render("Desired count"), detailKV("Current", fmt.Sprint(p.selectedService.DesiredCount))}
	if err == nil {
		lines = append(lines, detailKV("New", fmt.Sprint(desired)))
	} else {
		lines = append(lines, detailKV("New", "invalid"))
	}
	force := "No"
	if p.updateForceNewDeployment {
		force = "Yes"
	}
	lines = append(lines, "", detailKV("Force deploy", force), "", styles.MutedStyle.Render("Space toggles force-new-deployment · Enter confirms update · b/Esc returns"))
	if p.updateErr != "" {
		lines = append(lines, "", styles.ErrorStyle.Render(p.updateErr))
	}
	return lines
}

func (p *ECSPage) updatingServiceLines() []string {
	return []string{styles.MutedStyle.Render("Updating ECS service"), fmt.Sprintf("Cluster: %s", p.selectedCluster.Name), fmt.Sprintf("Service: %s", p.selectedService.Name), "", styles.StatusStyle.Render(p.spinner.View() + " Calling ECS UpdateService..."), styles.MutedStyle.Render("b/Esc cancels")}
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
	viewportWidth := max(30, width-styles.PanelStyle.GetHorizontalFrameSize()-6)
	viewportHeight := max(7, height-usedLines-8)
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
	lines = append(lines, "", styles.MutedStyle.Render("Task logs"), fmt.Sprintf("Task definition: %s", value(p.selectedTask.TaskDefinition)), fmt.Sprintf("Container: %s  •  %s", containerLabel, state))
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
	lines = append(lines, p.logViewport.View(), styles.MutedStyle.Render("Keys in footer · tab returns to navigation"))
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
	plainPrefix := timestamp + " "
	if width < 96 && len(timestamp) >= 5 {
		plainPrefix = timestamp[:5] + " "
	}
	prefix := styles.MutedStyle.Render(plainPrefix)
	message = strings.TrimRight(message, "\n")
	prefixWidth := lipgloss.Width(plainPrefix)
	wrapped := ansi.Wrap(colorizeLogSeverityMarker(message), max(16, width-prefixWidth), "")
	parts := strings.Split(wrapped, "\n")
	if len(parts) == 0 {
		return prefix
	}
	parts[0] = prefix + parts[0]
	indent := strings.Repeat(" ", min(prefixWidth, 4))
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

func (p *ECSPage) stopTaskReasonLines() []string {
	lines := []string{styles.MutedStyle.Render("Step 1 of 2 · Stop task reason"), fmt.Sprintf("Cluster: %s", value(p.selectedCluster.Name)), fmt.Sprintf("Task: %s", value(p.selectedTask.ID)), "", p.stopReasonInput.View()}
	if p.updateErr != "" {
		lines = append(lines, "", styles.ErrorStyle.Render(p.updateErr))
	}
	lines = append(lines, "", styles.MutedStyle.Render("Enter continues · b/Esc cancels"))
	return lines
}

func (p *ECSPage) stopTaskReviewLines() []string {
	t := p.selectedTask
	lines := []string{
		styles.MutedStyle.Render("Step 2 of 2 · Review stop task"),
		fmt.Sprintf("Cluster: %s", value(p.selectedCluster.Name)),
		detailKV("Task ID", t.ID),
		detailKV("Last status", t.LastStatus),
		detailKV("Launch type", t.LaunchType),
		detailKV("Task definition", t.TaskDefinition),
		detailKV("Group", t.Group),
		detailKV("Task ARN", t.ARN),
		"",
		detailKV("Stop reason", strings.TrimSpace(p.stopReasonInput.Value())),
	}
	if isServiceManagedTask(t) {
		lines = append(lines, "", styles.StatusStyle.Render("ECS may launch a replacement task to maintain the service desired count."))
	}
	lines = append(lines, "", styles.MutedStyle.Render("Containers"))
	if len(t.Containers) == 0 {
		lines = append(lines, styles.MutedStyle.Render("No containers reported."))
	}
	for _, c := range t.Containers {
		lines = append(lines, fmt.Sprintf("%s  %s  •  %s", statusLabel(c.LastStatus), value(c.Name), shortImage(c.Image)))
	}
	if p.updateErr != "" {
		lines = append(lines, "", styles.ErrorStyle.Render(p.updateErr))
	}
	lines = append(lines, "", styles.MutedStyle.Render("Enter stops task · b/Esc returns"))
	return lines
}

func (p *ECSPage) stoppingTaskLines() []string {
	return []string{styles.MutedStyle.Render("Stopping task"), fmt.Sprintf("Cluster: %s", value(p.selectedCluster.Name)), fmt.Sprintf("Task: %s", value(p.selectedTask.ID)), "", styles.StatusStyle.Render(p.spinner.View() + " Sending StopTask request..."), styles.MutedStyle.Render("b/Esc cancels waiting; AWS may still complete the request.")}
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
		styles.MutedStyle.Render("b/Esc returns · keys in footer"),
	)
	return lines
}
