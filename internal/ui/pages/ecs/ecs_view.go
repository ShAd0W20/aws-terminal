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
	lines := []string{styles.MutedStyle.Render("Service detail"), "Name: " + value(s.Name), "ARN: " + value(s.ARN), "Status: " + value(s.Status), "Task definition: " + value(s.TaskDefinition), "Task definition ARN: " + value(s.TaskDefinitionARN), fmt.Sprintf("Desired/Running/Pending: %d/%d/%d", s.DesiredCount, s.RunningCount, s.PendingCount), "Launch type: " + value(s.LaunchType), "Capacity providers: " + value(strings.Join(s.CapacityProviders, ", ")), "Platform version: " + value(s.PlatformVersion), "Created: " + timeText(s.CreatedAt), "Deployment controller: " + value(s.DeploymentController), fmt.Sprintf("Network: %d subnets, %d security groups, public IP %s", s.SubnetCount, s.SecurityGroupCount, value(s.AssignPublicIP)), "", styles.MutedStyle.Render("Deployments")}
	for _, d := range s.Deployments {
		lines = append(lines, fmt.Sprintf("- %s %s %s %d/%d (+%d)", value(d.Status), value(d.RolloutState), value(d.TaskDefinition), d.RunningCount, d.DesiredCount, d.PendingCount))
	}
	lines = append(lines, "", styles.MutedStyle.Render("Press b or Esc to return."))
	return lines
}
func (p *ECSPage) taskDetailLines() []string {
	t := p.selectedTask
	lines := []string{styles.MutedStyle.Render("Task detail"), "Task ID: " + value(t.ID), "ARN: " + value(t.ARN), "Last status: " + value(t.LastStatus), "Desired status: " + value(t.DesiredStatus), "Health: " + value(t.HealthStatus), "Task definition: " + value(t.TaskDefinition), "Task definition ARN: " + value(t.TaskDefinitionARN), "Group: " + value(t.Group), "Launch type: " + value(t.LaunchType), "Platform version: " + value(t.PlatformVersion), "Availability zone: " + value(t.AvailabilityZone), "Connectivity: " + value(t.Connectivity), "Private IP: " + value(t.PrivateIP), "Created: " + timeText(t.CreatedAt), "Pull started/stopped: " + timeText(t.PullStartedAt) + " / " + timeText(t.PullStoppedAt), "Started: " + timeText(t.StartedAt), "Stopping/stopped: " + timeText(t.StoppingAt) + " / " + timeText(t.StoppedAt), "Stopped reason: " + value(t.StoppedReason), "", styles.MutedStyle.Render("Containers")}
	for _, c := range t.Containers {
		exit := "—"
		if c.ExitCode != nil {
			exit = fmt.Sprint(*c.ExitCode)
		}
		lines = append(lines, fmt.Sprintf("- %s %s %s exit=%s %s", value(c.Name), value(c.Image), value(c.LastStatus), exit, value(c.Reason)))
	}
	lines = append(lines, "", styles.MutedStyle.Render("Attachments"))
	for _, a := range t.Attachments {
		lines = append(lines, fmt.Sprintf("- ENI %s subnet %s MAC %s private IP %s", value(a.ENI), value(a.Subnet), value(a.MAC), value(a.PrivateIP)))
	}
	lines = append(lines, "", styles.MutedStyle.Render("Press b or Esc to return."))
	return lines
}
