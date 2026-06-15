package ecs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"

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

func (p *ECSPage) taskDetailLines() []string { return p.taskOverviewLines() }

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
