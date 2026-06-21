package sqs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"

	"aws-terminal/internal/ui/styles"
	"aws-terminal/internal/ui/tableutil"
	"aws-terminal/internal/ui/workflow"
)

func (p *SQSPage) View(state State, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := []string{
		styles.SectionTitleStyle.Render("SQS"),
		styles.SubtitleStyle.Render("Visualize SQS queues and approximate message counts."),
		"",
	}
	if state.ActiveSession == nil {
		lines = append(lines, styles.MutedStyle.Render("No active AWS session. Authenticate a profile first."))
		return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
	}

	lines = append(lines,
		fmt.Sprintf("Active profile: %s", state.ActiveSession.Profile),
		fmt.Sprintf("Account: %s", workflow.ValueOrFallback(state.ActiveSession.Account, "unknown")),
		fmt.Sprintf("Region: %s", valueOrFallback(activeRegion(state), "unknown")),
	)
	if state.PageFocused {
		lines = append(lines, styles.StatusStyle.Render("Page focus active · tab returns to navigation."))
	} else {
		lines = append(lines, styles.MutedStyle.Render("Focus Page to interact with SQS."))
	}

	lines = append(lines, "")
	switch p.stage {
	case sqsStageQueues:
		lines = append(lines, p.queueLines(width, height, len(lines))...)
	case sqsStageQueueActions:
		lines = append(lines, p.queueActionLines()...)
	case sqsStageMessages:
		lines = append(lines, p.messageLines(width, height, len(lines))...)
	case sqsStagePurgeConfirm:
		lines = append(lines, p.purgeConfirmLines()...)
	}
	return styles.RenderBox(styles.PanelStyle, width, height, strings.Join(lines, "\n"))
}

func (p *SQSPage) ShortHelp() []key.Binding {
	switch p.stage {
	case sqsStageQueues:
		return []key.Binding{sqsUpKey, sqsDownKey, sqsEnterKey, sqsSearchKey, sqsRefreshKey, sqsCancelKey, sqsTabKey}
	case sqsStageQueueActions:
		return []key.Binding{sqsPullKey, sqsPurgeKey, sqsRefreshKey, sqsBackKey, sqsCancelKey, sqsTabKey}
	case sqsStageMessages:
		return []key.Binding{sqsUpKey, sqsDownKey, sqsPullKey, sqsRefreshKey, sqsBackKey, sqsTabKey}
	case sqsStagePurgeConfirm:
		return []key.Binding{sqsEnterKey, sqsBackKey, sqsCancelKey, sqsTabKey}
	default:
		return []key.Binding{sqsTabKey}
	}
}

func (p *SQSPage) FullHelp() [][]key.Binding { return [][]key.Binding{p.ShortHelp()} }

func (p *SQSPage) queueLines(width, height, usedLines int) []string {
	searchHint := "Ctrl+F search · r refresh · Enter queue actions"
	if p.search.Focused() {
		searchHint = "Search active. Type to filter; Esc leaves search."
	}
	lines := []string{styles.MutedStyle.Render("Queues"), styles.MutedStyle.Render(searchHint), p.search.View()}
	if p.loading {
		lines = append(lines, styles.StatusStyle.Render(p.spinner.View()+" Loading queues..."))
	}
	if p.loadErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.loadErr))
	}
	if p.purgeMessage != "" {
		lines = append(lines, styles.StatusStyle.Render(p.purgeMessage))
	}

	filtered := p.filteredQueues()
	if len(filtered) == 0 {
		if p.loading {
			return lines
		}
		if strings.TrimSpace(p.search.Value()) != "" {
			return append(lines, styles.MutedStyle.Render("No queues match your search."))
		}
		return append(lines, styles.MutedStyle.Render("No SQS queues found for this profile and region."))
	}

	tableWidth := max(48, width-styles.PanelStyle.GetHorizontalFrameSize()-8)
	p.configureTable(tableWidth, height-usedLines-len(lines)-5)
	p.syncTable()
	lines = append(lines, "", tableutil.RenderBox(p.table.View(), tableWidth+4))
	selected := p.currentQueue()
	if selected.Name != "" {
		lines = append(lines, styles.MutedStyle.Render(fmt.Sprintf("Selected %s · available %d · in flight %d", selected.Name, selected.AvailableMessages, selected.InFlightMessages)))
	}
	return lines
}

func (p *SQSPage) queueActionLines() []string {
	queue := p.selectedQueue
	lines := []string{
		styles.MutedStyle.Render("Queue actions"),
		fmt.Sprintf("Queue: %s", queue.Name),
		fmt.Sprintf("Available: %d", queue.AvailableMessages),
		fmt.Sprintf("In flight: %d", queue.InFlightMessages),
		"",
		styles.MutedStyle.Render("p pulls up to 10 messages for view-only inspection."),
		styles.MutedStyle.Render("x purges the queue after typing the queue name."),
	}
	if p.messagesLoading {
		lines = append(lines, styles.StatusStyle.Render(p.spinner.View()+" Pulling messages..."))
	}
	if p.purging {
		lines = append(lines, styles.StatusStyle.Render(p.spinner.View()+" Purging queue..."))
	}
	if p.messagesErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.messagesErr))
	}
	if p.purgeErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.purgeErr))
	}
	if p.purgeMessage != "" {
		lines = append(lines, styles.StatusStyle.Render(p.purgeMessage))
	}
	lines = append(lines, "", styles.MutedStyle.Render("b/Esc returns to queues."))
	return lines
}

func (p *SQSPage) messageLines(width, height, usedLines int) []string {
	lines := []string{
		styles.MutedStyle.Render("Pulled messages · view only"),
		fmt.Sprintf("Queue: %s", p.selectedQueue.Name),
		styles.MutedStyle.Render("Messages are not deleted. Received messages may remain temporarily in-flight until visibility timeout expires."),
	}
	if p.messagesLoading {
		lines = append(lines, styles.StatusStyle.Render(p.spinner.View()+" Pulling messages..."))
	}
	if p.messagesErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.messagesErr))
	}
	if len(p.messages) == 0 {
		if !p.messagesLoading {
			lines = append(lines, styles.MutedStyle.Render("No messages returned by SQS."))
		}
		return append(lines, "", styles.MutedStyle.Render("p/r pulls again · b/Esc returns."))
	}

	visible := max(3, height-usedLines-len(lines)-8)
	start := max(0, p.messageIndex-visible/2)
	end := start + visible
	if end > len(p.messages) {
		end = len(p.messages)
		start = max(0, end-visible)
	}
	listWidth := max(48, width-styles.PanelStyle.GetHorizontalFrameSize()-8)
	bodyWidth := max(16, listWidth-28)
	messageRows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		message := p.messages[i]
		prefix := "  "
		style := styles.SidebarItemStyle
		if i == p.messageIndex {
			prefix = "▸ "
			style = styles.FocusedSelectedSidebarItemStyle
		}
		label := fmt.Sprintf("%s%s  %s", prefix, tableutil.Truncate(message.ID, 20), messageBodyPreview(message.Body, bodyWidth))
		messageRows = append(messageRows, style.Render(label))
	}
	lines = append(lines, "", tableutil.RenderBox(strings.Join(messageRows, "\n"), listWidth+4))
	selected := p.messages[p.messageIndex]
	if !selected.SentAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Sent: %s", selected.SentAt.Local().Format("2006-01-02 15:04:05")))
	}
	lines = append(lines, "", styles.MutedStyle.Render("Selected body:"), tableutil.RenderBox(selected.Body, listWidth+4), styles.MutedStyle.Render("p/r pulls again · b/Esc returns."))
	return lines
}

func (p *SQSPage) purgeConfirmLines() []string {
	lines := []string{
		styles.ErrorStyle.Render("Purge queue"),
		fmt.Sprintf("Queue: %s", p.selectedQueue.Name),
		styles.MutedStyle.Render("This deletes all messages in the queue. AWS may reject repeated purges within the purge cooldown window."),
		"",
		styles.MutedStyle.Render("Type the queue name exactly, then press Enter:"),
		p.purgeInput.View(),
	}
	if p.purging {
		lines = append(lines, styles.StatusStyle.Render(p.spinner.View()+" Purging queue..."))
	}
	if p.purgeErr != "" {
		lines = append(lines, styles.ErrorStyle.Render(p.purgeErr))
	}
	lines = append(lines, "", styles.MutedStyle.Render("b/Esc cancels."))
	return lines
}
