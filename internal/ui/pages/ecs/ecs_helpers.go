package ecs

import (
	"fmt"
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
	return tableutil.FitColumns(width, []tableutil.ColumnSpec{{Title: "Task", Min: 12, Weight: 1, Max: 20}, {Title: "Last", Min: 8, Weight: 1, Max: 14}, {Title: "Desired", Min: 8, Weight: 1, Max: 14}, {Title: "Task definition", Min: 16, Weight: 3, Max: 42}, {Title: "IP", Min: 12, Weight: 1, Max: 15}, {Title: "Created", Min: 12, Weight: 1, Max: 16}, {Title: "Started", Min: 12, Weight: 1, Max: 16}})
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

func value(v string) string {
	if strings.TrimSpace(v) == "" {
		return "—"
	}
	return v
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
		rows = append(rows, table.Row{tableutil.Truncate(t.ID, cols[0].Width), tableutil.Truncate(t.LastStatus, cols[1].Width), tableutil.Truncate(t.DesiredStatus, cols[2].Width), tableutil.Truncate(value(t.TaskDefinition), cols[3].Width), tableutil.Truncate(value(t.PrivateIP), cols[4].Width), tableTimeText(t.CreatedAt), tableTimeText(t.StartedAt)})
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
