package ecs

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	domainecs "aws-terminal/internal/domain/ecs"
	"aws-terminal/internal/ui/styles"
	"aws-terminal/internal/ui/tableutil"
	"aws-terminal/internal/ui/workflow"
)

func sessionKey(state State) string   { return workflow.SessionKey(state) }
func activeRegion(state State) string { return workflow.ActiveRegion(state) }

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).Foreground(lipgloss.Color("39"))
	s.Selected = styles.FocusedSelectedSidebarItemStyle
	return s
}
func clusterColumnsForWidth(width int) []table.Column {
	return tableutil.FitColumns(width, []tableutil.ColumnSpec{{Title: "Cluster", Min: 18, Weight: 4, Max: 56}, {Title: "Status", Min: 8, Weight: 1, Max: 12}, {Title: "Services", Min: 8}, {Title: "Running", Min: 8}, {Title: "Pending", Min: 8}, {Title: "Instances", Min: 9}})
}
func serviceColumnsForWidth(width int) []table.Column {
	return tableutil.FitColumns(width, []tableutil.ColumnSpec{{Title: "Service", Min: 18, Weight: 4, Max: 48}, {Title: "Status", Min: 8, Weight: 1, Max: 12}, {Title: "Task definition", Min: 18, Weight: 3, Max: 44}, {Title: "Tasks", Min: 10, Weight: 1, Max: 18}, {Title: "Created", Min: 16}})
}
func taskColumnsForWidth(width int) []table.Column {
	if width < 86 {
		return tableutil.FitColumns(width, []tableutil.ColumnSpec{{Title: "Task definition", Min: 30, Weight: 4, Max: 38}, {Title: "Last", Min: 8, Weight: 1, Max: 12}, {Title: "Desired", Min: 8, Weight: 1, Max: 12}})
	}
	if width < 116 {
		return tableutil.FitColumns(width, []tableutil.ColumnSpec{{Title: "Task definition", Min: 34, Weight: 4, Max: 42}, {Title: "Last", Min: 8, Weight: 1, Max: 12}, {Title: "Desired", Min: 8, Weight: 1, Max: 12}, {Title: "IP", Min: 10, Weight: 2, Max: 15}})
	}
	return tableutil.FitColumns(width, []tableutil.ColumnSpec{{Title: "Task definition", Min: 34, Weight: 4, Max: 44}, {Title: "Last", Min: 8, Weight: 1, Max: 14}, {Title: "Desired", Min: 8, Weight: 1, Max: 14}, {Title: "Task", Min: 8, Weight: 2, Max: 20}, {Title: "IP", Min: 10, Weight: 2, Max: 15}, {Title: "Created", Min: 11, Weight: 1, Max: 14}, {Title: "Started", Min: 11, Weight: 1, Max: 14}})
}
func clusterColumns() []table.Column { return clusterColumnsForWidth(96) }
func serviceColumns() []table.Column { return serviceColumnsForWidth(96) }
func taskColumns() []table.Column    { return taskColumnsForWidth(112) }

func textInputKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace, tea.KeyBackspace, tea.KeyDelete:
		return true
	default:
		return false
	}
}
func lowerContains(v, q string) bool { return strings.Contains(strings.ToLower(v), strings.ToLower(q)) }
func timeText(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func tableTimeText(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("01-02 15:04")
}

func taskDefinitionFamilyFromNameOrARN(value string) string {
	name := path.Base(strings.TrimSpace(value))
	if idx := strings.LastIndex(name, ":"); idx >= 0 {
		return name[:idx]
	}
	return name
}

func (p *ECSPage) selectedTaskDefinition() domainecs.TaskDefinitionSummary {
	if len(p.taskDefinitions) == 0 || p.taskDefinitionIndex < 0 || p.taskDefinitionIndex >= len(p.taskDefinitions) {
		return domainecs.TaskDefinitionSummary{}
	}
	return p.taskDefinitions[p.taskDefinitionIndex]
}

func (p *ECSPage) parsedDesiredCount() (int, error) {
	value := strings.TrimSpace(p.desiredCountInput.Value())
	if value == "" {
		return 0, fmt.Errorf("desired count is required")
	}
	count, err := strconv.Atoi(value)
	if err != nil || count < 0 {
		return 0, fmt.Errorf("desired count must be a non-negative integer")
	}
	return count, nil
}

func value(v string) string {
	if strings.TrimSpace(v) == "" {
		return "—"
	}
	return v
}

func detailKV(label, v string) string {
	return fmt.Sprintf("%-16s %s", label, value(v))
}

func statusLabel(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "? UNKNOWN"
	}
	upper := strings.ToUpper(status)
	symbol := "?"
	switch upper {
	case "ACTIVE", "COMPLETED", "RUNNING", "HEALTHY", "CONNECTED", "PRIMARY":
		symbol = "✓"
	case "PENDING", "PROVISIONING", "ACTIVATING", "DRAINING", "DEACTIVATING", "IN_PROGRESS":
		symbol = "…"
	case "STOPPED", "INACTIVE", "FAILED", "UNHEALTHY", "DISCONNECTED":
		symbol = "!"
	}
	return symbol + " " + upper
}

func shortImage(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return "—"
	}
	if idx := strings.LastIndex(image, "/"); idx >= 0 && idx < len(image)-1 {
		return image[idx+1:]
	}
	return image
}

func serviceAttentionReason(s domainecs.Service) string {
	if s.RunningCount < s.DesiredCount {
		return "Fewer running tasks than desired"
	}
	if s.PendingCount > 0 {
		return "Tasks are pending"
	}
	if strings.TrimSpace(s.Status) != "" && !strings.EqualFold(s.Status, "ACTIVE") {
		return "Service status is " + s.Status
	}
	for _, d := range s.Deployments {
		if strings.EqualFold(d.RolloutState, "FAILED") {
			return "Deployment failed"
		}
	}
	return ""
}

func taskAttentionReason(t domainecs.Task) string {
	if strings.TrimSpace(t.StoppedReason) != "" {
		return t.StoppedReason
	}
	if strings.EqualFold(t.HealthStatus, "UNHEALTHY") {
		return "Task health is unhealthy"
	}
	for _, c := range t.Containers {
		if strings.TrimSpace(c.Reason) != "" {
			return c.Reason
		}
		if c.ExitCode != nil && *c.ExitCode != 0 {
			return fmt.Sprintf("Container %s exited with code %d", value(c.Name), *c.ExitCode)
		}
	}
	if strings.TrimSpace(t.LastStatus) != "" && !strings.EqualFold(t.LastStatus, "RUNNING") && !strings.EqualFold(t.LastStatus, "PENDING") {
		return "Task status is " + t.LastStatus
	}
	return ""
}

func taskCount(s domainecs.Service) string {
	base := fmt.Sprintf("%d/%d", s.RunningCount, s.DesiredCount)
	if s.PendingCount > 0 {
		base += fmt.Sprintf(" (+%d pending)", s.PendingCount)
	}
	return base
}

func (p *ECSPage) configureClusterTable(width, rows int) {
	rows = max(5, rows)
	p.clusterPaginator.PerPage = rows
	p.clusterTable.SetHeight(rows + 1)
	p.clusterTable.SetWidth(width)
	p.clusterTable.SetColumns(clusterColumnsForWidth(width))
}
func (p *ECSPage) configureServiceTable(width, rows int) {
	rows = max(5, rows)
	p.servicePaginator.PerPage = rows
	p.serviceTable.SetHeight(rows + 1)
	p.serviceTable.SetWidth(width)
	p.serviceTable.SetColumns(serviceColumnsForWidth(width))
}
func (p *ECSPage) configureTaskTable(width, rows int) {
	rows = max(5, rows)
	p.taskPaginator.PerPage = rows
	p.taskTable.SetHeight(rows + 1)
	p.taskTable.SetWidth(width)
	p.taskTable.SetColumns(taskColumnsForWidth(width))
}

func (p *ECSPage) filteredClusters() []domainecs.Cluster {
	q := strings.TrimSpace(p.searchInput.Value())
	if q == "" {
		return p.clusters
	}
	out := []domainecs.Cluster{}
	for _, c := range p.clusters {
		if lowerContains(c.Name, q) || lowerContains(c.Status, q) {
			out = append(out, c)
		}
	}
	return out
}
func (p *ECSPage) filteredServices() []domainecs.Service {
	q := strings.TrimSpace(p.searchInput.Value())
	if q == "" {
		return p.services
	}
	out := []domainecs.Service{}
	for _, s := range p.services {
		if lowerContains(s.Name, q) || lowerContains(s.Status, q) || lowerContains(s.TaskDefinition, q) {
			out = append(out, s)
		}
	}
	return out
}
func (p *ECSPage) filteredTasks() []domainecs.Task {
	q := strings.TrimSpace(p.searchInput.Value())
	if q == "" {
		return p.tasks
	}
	out := []domainecs.Task{}
	for _, t := range p.tasks {
		if lowerContains(t.ID, q) || lowerContains(t.LastStatus, q) || lowerContains(t.DesiredStatus, q) || lowerContains(t.TaskDefinition, q) || lowerContains(t.PrivateIP, q) {
			out = append(out, t)
		}
	}
	return out
}

func (p *ECSPage) syncClusterTable() {
	items := p.filteredClusters()
	p.clusterPaginator.SetTotalPages(len(items))
	if p.clusterPaginator.Page >= p.clusterPaginator.TotalPages {
		p.clusterPaginator.Page = max(0, p.clusterPaginator.TotalPages-1)
	}
	if p.clusterIndex >= len(items) {
		p.clusterIndex = max(0, len(items)-1)
	}
	if len(items) > 0 {
		p.clusterPaginator.Page = p.clusterIndex / max(1, p.clusterPaginator.PerPage)
	}
	start, end := p.clusterPaginator.GetSliceBounds(len(items))
	if p.clusterIndex < start || p.clusterIndex >= end {
		p.clusterIndex = start
	}
	cols := p.clusterTable.Columns()
	rows := []table.Row{}
	for _, c := range items[start:end] {
		rows = append(rows, table.Row{tableutil.Truncate(c.Name, cols[0].Width), tableutil.Truncate(c.Status, cols[1].Width), fmt.Sprint(c.ActiveServicesCount), fmt.Sprint(c.RunningTasksCount), fmt.Sprint(c.PendingTasksCount), fmt.Sprint(c.RegisteredInstanceCount)})
	}
	p.clusterTable.SetRows(rows)
	p.clusterTable.SetCursor(max(0, p.clusterIndex-start))
}
func (p *ECSPage) syncServiceTable() {
	items := p.filteredServices()
	p.servicePaginator.SetTotalPages(len(items))
	if p.servicePaginator.Page >= p.servicePaginator.TotalPages {
		p.servicePaginator.Page = max(0, p.servicePaginator.TotalPages-1)
	}
	if p.serviceIndex >= len(items) {
		p.serviceIndex = max(0, len(items)-1)
	}
	if len(items) > 0 {
		p.servicePaginator.Page = p.serviceIndex / max(1, p.servicePaginator.PerPage)
	}
	start, end := p.servicePaginator.GetSliceBounds(len(items))
	if p.serviceIndex < start || p.serviceIndex >= end {
		p.serviceIndex = start
	}
	cols := p.serviceTable.Columns()
	rows := []table.Row{}
	for _, s := range items[start:end] {
		rows = append(rows, table.Row{tableutil.Truncate(s.Name, cols[0].Width), tableutil.Truncate(s.Status, cols[1].Width), tableutil.Truncate(value(s.TaskDefinition), cols[2].Width), tableutil.Truncate(taskCount(s), cols[3].Width), timeText(s.CreatedAt)})
	}
	p.serviceTable.SetRows(rows)
	p.serviceTable.SetCursor(max(0, p.serviceIndex-start))
}
func (p *ECSPage) syncTaskDefinitionSelection() {
	p.taskDefinitionPaginator.SetTotalPages(len(p.taskDefinitions))
	if p.taskDefinitionPaginator.Page >= p.taskDefinitionPaginator.TotalPages {
		p.taskDefinitionPaginator.Page = max(0, p.taskDefinitionPaginator.TotalPages-1)
	}
	if p.taskDefinitionIndex >= len(p.taskDefinitions) {
		p.taskDefinitionIndex = max(0, len(p.taskDefinitions)-1)
	}
	if len(p.taskDefinitions) > 0 {
		p.taskDefinitionPaginator.Page = p.taskDefinitionIndex / max(1, p.taskDefinitionPaginator.PerPage)
	}
}

func (p *ECSPage) syncTaskTable() {
	items := p.filteredTasks()
	p.taskPaginator.SetTotalPages(len(items))
	if p.taskPaginator.Page >= p.taskPaginator.TotalPages {
		p.taskPaginator.Page = max(0, p.taskPaginator.TotalPages-1)
	}
	if p.taskIndex >= len(items) {
		p.taskIndex = max(0, len(items)-1)
	}
	if len(items) > 0 {
		p.taskPaginator.Page = p.taskIndex / max(1, p.taskPaginator.PerPage)
	}
	start, end := p.taskPaginator.GetSliceBounds(len(items))
	if p.taskIndex < start || p.taskIndex >= end {
		p.taskIndex = start
	}
	cols := p.taskTable.Columns()
	rows := []table.Row{}
	for _, t := range items[start:end] {
		row := make(table.Row, 0, len(cols))
		for _, col := range cols {
			switch col.Title {
			case "Task definition":
				row = append(row, tableutil.Truncate(value(t.TaskDefinition), col.Width))
			case "Task":
				row = append(row, tableutil.Truncate(t.ID, col.Width))
			case "Last":
				row = append(row, tableutil.Truncate(t.LastStatus, col.Width))
			case "Desired":
				row = append(row, tableutil.Truncate(t.DesiredStatus, col.Width))
			case "IP":
				row = append(row, tableutil.Truncate(value(t.PrivateIP), col.Width))
			case "Created":
				row = append(row, tableutil.Truncate(tableTimeText(t.CreatedAt), col.Width))
			case "Started":
				row = append(row, tableutil.Truncate(tableTimeText(t.StartedAt), col.Width))
			}
		}
		rows = append(rows, row)
	}
	p.taskTable.SetRows(rows)
	p.taskTable.SetCursor(max(0, p.taskIndex-start))
}

func shortText(v string, width int) string {
	if width <= 1 || len([]rune(v)) <= width {
		return v
	}
	runes := []rune(v)
	return string(runes[:width-1]) + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
