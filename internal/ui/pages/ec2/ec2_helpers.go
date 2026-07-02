package ec2

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	domainec2 "aws-terminal/internal/domain/ec2"
	"aws-terminal/internal/ui/styles"
	"aws-terminal/internal/ui/tableutil"
)

func (p *EC2Page) resetForSession() {
	p.cancelAll()
	p.stage = ec2StageInstances
	p.loadedFor = ""
	p.loading = false
	p.loadErr = ""
	p.instances = nil
	p.instanceIndex = 0
	p.selected = domainec2.Instance{}
	p.search.SetValue("")
	p.search.Blur()
	p.terminateInput.SetValue("")
	p.terminateInput.Blur()
	p.stopping = false
	p.connecting = false
	p.terminating = false
	p.actionErr = ""
	p.actionMessage = ""
	p.syncTable()
}

func (p *EC2Page) filteredInstances() []domainec2.Instance {
	query := strings.ToLower(strings.TrimSpace(p.search.Value()))
	if query == "" {
		return append([]domainec2.Instance(nil), p.instances...)
	}
	filtered := make([]domainec2.Instance, 0, len(p.instances))
	for _, instance := range p.instances {
		haystack := strings.ToLower(strings.Join([]string{instance.Name, instance.ID, instance.State, instance.Type, instance.AvailabilityZone, instance.PrivateIP, instance.PublicIP}, " "))
		if strings.Contains(haystack, query) {
			filtered = append(filtered, instance)
		}
	}
	return filtered
}

func (p *EC2Page) currentInstance() domainec2.Instance {
	instances := p.filteredInstances()
	if len(instances) == 0 {
		return domainec2.Instance{}
	}
	if p.instanceIndex >= len(instances) {
		p.instanceIndex = len(instances) - 1
	}
	if p.instanceIndex < 0 {
		p.instanceIndex = 0
	}
	return instances[p.instanceIndex]
}

func (p *EC2Page) syncTable() {
	instances := p.filteredInstances()
	if p.instanceIndex >= len(instances) {
		p.instanceIndex = max(0, len(instances)-1)
	}
	rows := make([]table.Row, 0, len(instances))
	columns := p.table.Columns()
	if len(columns) == 0 {
		columns = instanceColumns()
	}
	for _, instance := range instances {
		rows = append(rows, table.Row{tableutil.Truncate(instanceName(instance), columns[0].Width), tableutil.Truncate(instance.ID, columns[1].Width), tableutil.Truncate(instance.State, columns[2].Width), tableutil.Truncate(instance.Type, columns[3].Width), tableutil.Truncate(instance.AvailabilityZone, columns[4].Width), tableutil.Truncate(firstValue(instance.PrivateIP, "-"), columns[5].Width), tableutil.Truncate(firstValue(instance.PublicIP, "-"), columns[6].Width)})
	}
	p.table.SetRows(rows)
	if len(rows) > 0 {
		p.table.SetCursor(p.instanceIndex)
	}
}

func (p *EC2Page) configureTable(width, height int) {
	if width < 64 {
		width = 64
	}
	nameWidth := max(12, width-78)
	p.table.SetColumns([]table.Column{{Title: "Name", Width: nameWidth}, {Title: "Instance", Width: 20}, {Title: "State", Width: 12}, {Title: "Type", Width: 12}, {Title: "AZ", Width: 14}, {Title: "Private IP", Width: 15}, {Title: "Public IP", Width: 15}})
	p.table.SetHeight(max(4, height))
}

func instanceColumns() []table.Column {
	return []table.Column{{Title: "Name", Width: 24}, {Title: "Instance", Width: 20}, {Title: "State", Width: 12}, {Title: "Type", Width: 12}, {Title: "AZ", Width: 14}, {Title: "Private IP", Width: 15}, {Title: "Public IP", Width: 15}}
}

func instanceTableStyles() table.Styles {
	style := table.DefaultStyles()
	style.Header = style.Header.Bold(true).Foreground(lipgloss.Color("81")).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true)
	style.Selected = styles.FocusedSelectedSidebarItemStyle
	return style
}

func updateInstanceByID(instances []domainec2.Instance, updated domainec2.Instance) []domainec2.Instance {
	if strings.TrimSpace(updated.ID) == "" {
		return instances
	}
	for i := range instances {
		if instances[i].ID == updated.ID {
			if strings.TrimSpace(updated.State) != "" {
				instances[i].State = updated.State
			}
			return instances
		}
	}
	return instances
}

func findInstanceByID(instances []domainec2.Instance, id string) (domainec2.Instance, bool) {
	for _, instance := range instances {
		if instance.ID == id {
			return instance, true
		}
	}
	return domainec2.Instance{}, false
}

func isStoppable(instance domainec2.Instance) bool {
	return strings.EqualFold(strings.TrimSpace(instance.State), "running")
}

func isConnectable(instance domainec2.Instance) bool {
	return strings.EqualFold(strings.TrimSpace(instance.State), "running")
}

func isTerminable(instance domainec2.Instance) bool {
	state := strings.TrimSpace(instance.State)
	return state != "" && !strings.EqualFold(state, "terminated") && !strings.EqualFold(state, "shutting-down")
}

func instanceName(instance domainec2.Instance) string {
	return firstValue(instance.Name, "-")
}

func firstValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func statusLabel(state string) string {
	state = firstValue(state, "unknown")
	switch strings.ToLower(state) {
	case "running":
		return styles.StatusStyle.Render(state)
	case "stopped", "stopping", "pending":
		return styles.MutedStyle.Render(state)
	case "shutting-down", "terminated":
		return styles.ErrorStyle.Render(state)
	default:
		return state
	}
}

func detailKV(key, value string) string {
	return fmt.Sprintf("%-18s %s", key+":", firstValue(value, "-"))
}

func securityGroupText(groups []domainec2.SecurityGroup) string {
	if len(groups) == 0 {
		return "-"
	}
	values := make([]string, 0, len(groups))
	for _, group := range groups {
		label := group.ID
		if group.Name != "" {
			label = group.Name + " (" + group.ID + ")"
		}
		values = append(values, label)
	}
	return strings.Join(values, ", ")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
