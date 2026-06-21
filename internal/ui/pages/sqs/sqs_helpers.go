package sqs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	domainsqs "aws-terminal/internal/domain/sqs"
	"aws-terminal/internal/ui/styles"
	"aws-terminal/internal/ui/tableutil"
)

func (p *SQSPage) resetForSession() {
	p.cancelAll()
	p.stage = sqsStageQueues
	p.loadedFor = ""
	p.loading = false
	p.loadErr = ""
	p.queues = nil
	p.queueIndex = 0
	p.selectedQueue = domainsqs.Queue{}
	p.search.SetValue("")
	p.search.Blur()
	p.messagesLoading = false
	p.messagesErr = ""
	p.messages = nil
	p.messageIndex = 0
	p.purging = false
	p.purgeErr = ""
	p.purgeMessage = ""
	p.purgeInput.SetValue("")
	p.purgeInput.Blur()
	p.syncTable()
}

func (p *SQSPage) cancelAll() {
	p.cancelLoad()
	if p.messagesCancel != nil {
		p.messagesCancel()
		p.messagesCancel = nil
	}
	if p.purgeCancel != nil {
		p.purgeCancel()
		p.purgeCancel = nil
	}
}

func (p *SQSPage) cancelLoad() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

func (p *SQSPage) filteredQueues() []domainsqs.Queue {
	query := strings.ToLower(strings.TrimSpace(p.search.Value()))
	if query == "" {
		return append([]domainsqs.Queue(nil), p.queues...)
	}
	filtered := make([]domainsqs.Queue, 0, len(p.queues))
	for _, queue := range p.queues {
		if strings.Contains(strings.ToLower(queue.Name), query) {
			filtered = append(filtered, queue)
		}
	}
	return filtered
}

func (p *SQSPage) currentQueue() domainsqs.Queue {
	queues := p.filteredQueues()
	if len(queues) == 0 {
		return domainsqs.Queue{}
	}
	if p.queueIndex >= len(queues) {
		p.queueIndex = len(queues) - 1
	}
	if p.queueIndex < 0 {
		p.queueIndex = 0
	}
	return queues[p.queueIndex]
}

func queueTableColumns() []table.Column {
	return []table.Column{
		{Title: "Queue", Width: 36},
		{Title: "Available", Width: 12},
		{Title: "In flight", Width: 12},
	}
}

func queueTableStyles() table.Styles {
	style := table.DefaultStyles()
	style.Header = style.Header.Bold(true).Foreground(lipgloss.Color("81")).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true)
	style.Selected = styles.FocusedSelectedSidebarItemStyle
	return style
}

func (p *SQSPage) configureTable(width, height int) {
	if width < 40 {
		width = 40
	}
	queueWidth := max(16, width-36)
	p.table.SetColumns([]table.Column{
		{Title: "Queue", Width: queueWidth},
		{Title: "Available", Width: 12},
		{Title: "In flight", Width: 12},
	})
	p.table.SetHeight(max(4, height))
}

func (p *SQSPage) syncTable() {
	queues := p.filteredQueues()
	if p.queueIndex >= len(queues) {
		p.queueIndex = max(0, len(queues)-1)
	}
	rows := make([]table.Row, 0, len(queues))
	for _, queue := range queues {
		rows = append(rows, table.Row{
			tableutil.Truncate(queue.Name, max(8, p.table.Columns()[0].Width)),
			fmt.Sprintf("%d", queue.AvailableMessages),
			fmt.Sprintf("%d", queue.InFlightMessages),
		})
	}
	p.table.SetRows(rows)
	if len(rows) > 0 {
		p.table.SetCursor(p.queueIndex)
	}
}

func messageBodyPreview(body string, width int) string {
	body = strings.ReplaceAll(body, "\n", " ")
	body = strings.ReplaceAll(body, "\r", " ")
	body = strings.Join(strings.Fields(body), " ")
	return tableutil.Truncate(body, max(16, width))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
