package sqs

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (p *SQSPage) OnStateChanged(state State) tea.Cmd {
	sessionKey := sqsSessionKey(state)
	if sessionKey != p.sessionKey {
		p.sessionKey = sessionKey
		p.resetForSession()
	}
	if state.ActiveSession == nil || p.loading || p.loadedFor == sessionKey {
		return nil
	}
	p.loading = true
	p.loadErr = ""
	return tea.Batch(p.spinner.Tick, p.loadQueuesCmd(state.ActiveSession.Profile, activeRegion(state), sessionKey))
}

func (p *SQSPage) SetFocused(focused bool) tea.Cmd {
	if !focused {
		p.search.Blur()
		p.purgeInput.Blur()
	}
	return nil
}

func (p *SQSPage) HasFocusedInput() bool { return p.search.Focused() || p.purgeInput.Focused() }

func (p *SQSPage) Update(msg tea.Msg, state State) tea.Cmd {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !p.loading && !p.messagesLoading && !p.purging {
			return nil
		}
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		return cmd
	case queuesLoadedMsg:
		return p.handleQueuesLoaded(msg)
	case messagesLoadedMsg:
		return p.handleMessagesLoaded(msg)
	case queuePurgedMsg:
		return p.handleQueuePurged(msg, state)
	}

	keyMsg, isKey := msg.(tea.KeyMsg)
	if !isKey || !state.PageFocused {
		return p.updateFocusedInput(msg)
	}

	if p.search.Focused() {
		if key.Matches(keyMsg, sqsCancelKey) {
			p.search.Blur()
			return nil
		}
		cmd := p.updateFocusedInput(msg)
		p.queueIndex = 0
		p.syncTable()
		return cmd
	}
	if p.purgeInput.Focused() {
		return p.updatePurgeConfirmStage(msg, state)
	}

	switch p.stage {
	case sqsStageQueues:
		return p.updateQueuesStage(keyMsg, state)
	case sqsStageQueueActions:
		return p.updateQueueActionsStage(keyMsg, state)
	case sqsStageMessages:
		return p.updateMessagesStage(keyMsg, state)
	case sqsStagePurgeConfirm:
		return p.updatePurgeConfirmStage(msg, state)
	default:
		return nil
	}
}

func (p *SQSPage) handleQueuesLoaded(msg queuesLoadedMsg) tea.Cmd {
	if msg.sessionKey != p.sessionKey {
		return nil
	}
	p.loading = false
	p.loadedFor = msg.sessionKey
	p.cancel = nil
	if errors.Is(msg.err, context.Canceled) {
		return nil
	}
	if msg.err != nil {
		p.loadErr = fmt.Sprintf("Unable to load SQS queues: %v", msg.err)
		p.queues = nil
		p.syncTable()
		return nil
	}
	p.loadErr = ""
	p.queues = msg.queues
	if p.queueIndex >= len(p.filteredQueues()) {
		p.queueIndex = max(0, len(p.filteredQueues())-1)
	}
	p.syncTable()
	return nil
}

func (p *SQSPage) handleMessagesLoaded(msg messagesLoadedMsg) tea.Cmd {
	p.messagesLoading = false
	p.messagesCancel = nil
	if errors.Is(msg.err, context.Canceled) {
		return nil
	}
	if msg.err != nil {
		p.messagesErr = fmt.Sprintf("Unable to pull messages: %v", msg.err)
		p.messages = nil
		return nil
	}
	p.messagesErr = ""
	p.messages = msg.messages
	p.messageIndex = 0
	p.stage = sqsStageMessages
	return nil
}

func (p *SQSPage) handleQueuePurged(msg queuePurgedMsg, state State) tea.Cmd {
	p.purging = false
	p.purgeCancel = nil
	if errors.Is(msg.err, context.Canceled) {
		return nil
	}
	if msg.err != nil {
		p.purgeErr = fmt.Sprintf("Unable to purge queue: %v", msg.err)
		return nil
	}
	p.purgeErr = ""
	p.purgeMessage = fmt.Sprintf("Purge requested for %s.", msg.queueName)
	p.purgeInput.SetValue("")
	p.purgeInput.Blur()
	p.stage = sqsStageQueueActions
	if state.ActiveSession != nil {
		p.loading = true
		return tea.Batch(p.spinner.Tick, p.loadQueuesCmd(state.ActiveSession.Profile, activeRegion(state), p.sessionKey))
	}
	return nil
}

func (p *SQSPage) updateQueuesStage(msg tea.KeyMsg, state State) tea.Cmd {
	switch {
	case key.Matches(msg, sqsSearchKey):
		return p.search.Focus()
	case key.Matches(msg, sqsRefreshKey):
		return p.startQueueRefresh(state)
	case key.Matches(msg, sqsCancelKey):
		if p.loading {
			p.cancelLoad()
			p.loading = false
			p.loadErr = "Queue loading cancelled."
		}
	case key.Matches(msg, sqsUpKey):
		if p.queueIndex > 0 {
			p.queueIndex--
			p.syncTable()
		}
	case key.Matches(msg, sqsDownKey):
		if p.queueIndex < len(p.filteredQueues())-1 {
			p.queueIndex++
			p.syncTable()
		}
	case key.Matches(msg, sqsEnterKey):
		queue := p.currentQueue()
		if queue.Name == "" {
			return nil
		}
		p.selectedQueue = queue
		p.stage = sqsStageQueueActions
		p.messagesErr = ""
		p.purgeErr = ""
		p.purgeMessage = ""
	}
	return nil
}

func (p *SQSPage) updateQueueActionsStage(msg tea.KeyMsg, state State) tea.Cmd {
	switch {
	case key.Matches(msg, sqsBackKey, sqsCancelKey):
		p.stage = sqsStageQueues
		p.messagesErr = ""
		p.purgeErr = ""
	case key.Matches(msg, sqsRefreshKey):
		return p.startQueueRefresh(state)
	case key.Matches(msg, sqsPullKey):
		if state.ActiveSession == nil || p.messagesLoading || p.selectedQueue.URL == "" {
			return nil
		}
		p.messagesLoading = true
		p.messagesErr = ""
		p.messages = nil
		return tea.Batch(p.spinner.Tick, p.receiveMessagesCmd(state.ActiveSession.Profile, activeRegion(state), p.selectedQueue))
	case key.Matches(msg, sqsPurgeKey):
		if p.selectedQueue.Name == "" || p.purging {
			return nil
		}
		p.stage = sqsStagePurgeConfirm
		p.purgeInput.SetValue("")
		return p.purgeInput.Focus()
	}
	return nil
}

func (p *SQSPage) updateMessagesStage(msg tea.KeyMsg, state State) tea.Cmd {
	switch {
	case key.Matches(msg, sqsBackKey, sqsCancelKey):
		p.stage = sqsStageQueueActions
	case key.Matches(msg, sqsPullKey, sqsRefreshKey):
		if state.ActiveSession == nil || p.messagesLoading || p.selectedQueue.URL == "" {
			return nil
		}
		p.messagesLoading = true
		p.messagesErr = ""
		return tea.Batch(p.spinner.Tick, p.receiveMessagesCmd(state.ActiveSession.Profile, activeRegion(state), p.selectedQueue))
	case key.Matches(msg, sqsUpKey):
		if p.messageIndex > 0 {
			p.messageIndex--
		}
	case key.Matches(msg, sqsDownKey):
		if p.messageIndex < len(p.messages)-1 {
			p.messageIndex++
		}
	}
	return nil
}

func (p *SQSPage) updatePurgeConfirmStage(msg tea.Msg, state State) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch {
		case key.Matches(keyMsg, sqsBackKey, sqsCancelKey):
			p.stage = sqsStageQueueActions
			p.purgeInput.SetValue("")
			p.purgeInput.Blur()
			p.purgeErr = ""
			return nil
		case key.Matches(keyMsg, sqsEnterKey):
			if state.ActiveSession == nil || p.purging {
				return nil
			}
			if p.purgeInput.Value() != p.selectedQueue.Name {
				p.purgeErr = "Queue name confirmation does not match."
				return nil
			}
			p.purging = true
			p.purgeErr = ""
			return tea.Batch(p.spinner.Tick, p.purgeQueueCmd(state.ActiveSession.Profile, activeRegion(state), p.selectedQueue))
		}
	}
	var cmd tea.Cmd
	p.purgeInput, cmd = p.purgeInput.Update(msg)
	return cmd
}

func (p *SQSPage) startQueueRefresh(state State) tea.Cmd {
	if state.ActiveSession == nil || p.loading {
		return nil
	}
	p.loading = true
	p.loadErr = ""
	return tea.Batch(p.spinner.Tick, p.loadQueuesCmd(state.ActiveSession.Profile, activeRegion(state), p.sessionKey))
}

func (p *SQSPage) updateFocusedInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	if p.search.Focused() {
		p.search, cmd = p.search.Update(msg)
		return cmd
	}
	if p.purgeInput.Focused() {
		p.purgeInput, cmd = p.purgeInput.Update(msg)
	}
	return cmd
}
