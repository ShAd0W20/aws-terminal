package ec2

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	domainec2 "aws-terminal/internal/domain/ec2"
	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/workflow"
)

func (p *EC2Page) OnStateChanged(state pageapi.State) tea.Cmd {
	sessionKey := workflow.SessionKey(state)
	if sessionKey != p.sessionKey {
		p.sessionKey = sessionKey
		p.resetForSession()
	}
	if state.ActiveSession == nil || p.loading || p.loadedFor == sessionKey {
		return nil
	}
	p.loading = true
	p.loadErr = ""
	return tea.Batch(p.spinner.Tick, p.loadInstancesCmd(state.ActiveSession.Profile, workflow.ActiveRegion(state), sessionKey))
}

func (p *EC2Page) SetFocused(focused bool) tea.Cmd {
	if !focused {
		p.search.Blur()
		p.terminateInput.Blur()
		return nil
	}
	if p.stage == ec2StageTerminateConfirm {
		return p.terminateInput.Focus()
	}
	return nil
}

func (p *EC2Page) Update(msg tea.Msg, state pageapi.State) tea.Cmd {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !p.loading && !p.stopping && !p.terminating {
			return nil
		}
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		return cmd
	case instancesLoadedMsg:
		return p.handleInstancesLoaded(msg)
	case instanceStoppedMsg:
		return p.handleInstanceStopped(msg, state)
	case instanceTerminatedMsg:
		return p.handleInstanceTerminated(msg, state)
	}

	keyMsg, isKey := msg.(tea.KeyMsg)
	if !isKey || !state.PageFocused {
		return p.updateFocusedInput(msg)
	}

	if p.search.Focused() {
		if key.Matches(keyMsg, ec2CancelKey) {
			p.search.Blur()
			return nil
		}
		cmd := p.updateFocusedInput(msg)
		p.instanceIndex = 0
		p.syncTable()
		return cmd
	}
	if p.terminateInput.Focused() {
		return p.updateTerminateConfirmStage(msg, state)
	}

	switch p.stage {
	case ec2StageInstances:
		return p.updateInstancesStage(keyMsg, state)
	case ec2StageInstanceDetail:
		return p.updateInstanceDetailStage(keyMsg, state)
	case ec2StageStopReview:
		return p.updateStopReviewStage(keyMsg, state)
	case ec2StageTerminateConfirm:
		return p.updateTerminateConfirmStage(msg, state)
	case ec2StageStopping, ec2StageTerminating:
		if key.Matches(keyMsg, ec2CancelKey, ec2BackKey) {
			p.cancelAction()
			p.stopping = false
			p.terminating = false
			p.stage = ec2StageInstanceDetail
			p.actionErr = "Action cancelled."
		}
	}
	return nil
}

func (p *EC2Page) handleInstancesLoaded(msg instancesLoadedMsg) tea.Cmd {
	if msg.sessionKey != p.sessionKey {
		return nil
	}
	p.loading = false
	p.loadedFor = msg.sessionKey
	p.loadCancel = nil
	if errors.Is(msg.err, context.Canceled) {
		return nil
	}
	if msg.err != nil {
		p.loadErr = fmt.Sprintf("Unable to load EC2 instances: %v", msg.err)
		p.instances = nil
		p.syncTable()
		return nil
	}
	previousID := p.selected.ID
	p.instances = msg.instances
	p.loadErr = ""
	if p.stage == ec2StageInstanceDetail && previousID != "" {
		if instance, ok := findInstanceByID(p.instances, previousID); ok {
			p.selected = instance
		} else {
			p.stage = ec2StageInstances
			p.selected = domainec2.Instance{}
		}
	}
	p.syncTable()
	return nil
}

func (p *EC2Page) handleInstanceStopped(msg instanceStoppedMsg, state pageapi.State) tea.Cmd {
	if msg.sessionKey != p.sessionKey || msg.instanceID != p.selected.ID {
		return nil
	}
	p.stopping = false
	p.actionCancel = nil
	if errors.Is(msg.err, context.Canceled) {
		return nil
	}
	if msg.err != nil {
		p.actionErr = fmt.Sprintf("Unable to stop instance: %v", msg.err)
		p.stage = ec2StageStopReview
		return nil
	}
	if msg.result.Instance.ID != "" {
		if msg.result.Instance.State != "" {
			p.selected.State = msg.result.Instance.State
		}
		p.instances = updateInstanceByID(p.instances, p.selected)
	}
	p.stage = ec2StageInstanceDetail
	p.actionErr = ""
	p.actionMessage = "Stop requested. Refreshing EC2 instances..."
	return p.startRefresh(state)
}

func (p *EC2Page) handleInstanceTerminated(msg instanceTerminatedMsg, state pageapi.State) tea.Cmd {
	if msg.sessionKey != p.sessionKey || msg.instanceID != p.selected.ID {
		return nil
	}
	p.terminating = false
	p.actionCancel = nil
	p.terminateInput.SetValue("")
	p.terminateInput.Blur()
	if errors.Is(msg.err, context.Canceled) {
		return nil
	}
	if msg.err != nil {
		p.actionErr = fmt.Sprintf("Unable to terminate instance: %v", msg.err)
		p.stage = ec2StageTerminateConfirm
		return p.terminateInput.Focus()
	}
	if msg.result.Instance.ID != "" && msg.result.Instance.State != "" {
		p.selected.State = msg.result.Instance.State
		p.instances = updateInstanceByID(p.instances, p.selected)
	}
	p.stage = ec2StageInstanceDetail
	p.actionErr = ""
	p.actionMessage = "Terminate requested. Refreshing EC2 instances..."
	return p.startRefresh(state)
}

func (p *EC2Page) updateInstancesStage(msg tea.KeyMsg, state pageapi.State) tea.Cmd {
	switch {
	case key.Matches(msg, ec2SearchKey):
		return p.search.Focus()
	case key.Matches(msg, ec2RefreshKey):
		return p.startRefresh(state)
	case key.Matches(msg, ec2CancelKey):
		if p.loading {
			p.cancelLoad()
			p.loading = false
			p.loadErr = "Instance loading cancelled."
		}
	case key.Matches(msg, ec2UpKey):
		if p.instanceIndex > 0 {
			p.instanceIndex--
			p.syncTable()
		}
	case key.Matches(msg, ec2DownKey):
		if p.instanceIndex < len(p.filteredInstances())-1 {
			p.instanceIndex++
			p.syncTable()
		}
	case key.Matches(msg, ec2EnterKey):
		instance := p.currentInstance()
		if instance.ID == "" {
			return nil
		}
		p.selected = instance
		p.stage = ec2StageInstanceDetail
		p.actionErr = ""
	}
	return nil
}

func (p *EC2Page) updateInstanceDetailStage(msg tea.KeyMsg, state pageapi.State) tea.Cmd {
	switch {
	case key.Matches(msg, ec2BackKey, ec2CancelKey):
		p.stage = ec2StageInstances
		p.actionErr = ""
	case key.Matches(msg, ec2RefreshKey):
		return p.startRefresh(state)
	case key.Matches(msg, ec2StopKey):
		if !isStoppable(p.selected) {
			p.actionErr = "Instance cannot be stopped from its current state."
			return nil
		}
		p.actionErr = ""
		p.actionMessage = ""
		p.stage = ec2StageStopReview
	case key.Matches(msg, ec2TerminateKey):
		if !isTerminable(p.selected) {
			p.actionErr = "Instance cannot be terminated from its current state."
			return nil
		}
		p.actionErr = ""
		p.actionMessage = ""
		p.terminateInput.SetValue("")
		p.stage = ec2StageTerminateConfirm
		return p.terminateInput.Focus()
	}
	return nil
}

func (p *EC2Page) updateStopReviewStage(msg tea.KeyMsg, state pageapi.State) tea.Cmd {
	if key.Matches(msg, ec2BackKey, ec2CancelKey) {
		p.stage = ec2StageInstanceDetail
		p.actionErr = ""
		return nil
	}
	if key.Matches(msg, ec2EnterKey) {
		if state.ActiveSession == nil {
			return nil
		}
		if !isStoppable(p.selected) {
			p.actionErr = "Instance cannot be stopped from its current state."
			return nil
		}
		p.stopping = true
		p.actionErr = ""
		p.stage = ec2StageStopping
		input := domainec2.StopInstanceInput{ProfileName: state.ActiveSession.Profile, Region: workflow.ActiveRegion(state), InstanceID: p.selected.ID}
		return tea.Batch(p.spinner.Tick, p.stopInstanceCmd(input, p.sessionKey))
	}
	return nil
}

func (p *EC2Page) updateTerminateConfirmStage(msg tea.Msg, state pageapi.State) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch {
		case key.Matches(keyMsg, ec2BackKey, ec2CancelKey):
			p.stage = ec2StageInstanceDetail
			p.terminateInput.SetValue("")
			p.terminateInput.Blur()
			p.actionErr = ""
			return nil
		case key.Matches(keyMsg, ec2EnterKey):
			if state.ActiveSession == nil {
				return nil
			}
			if strings.TrimSpace(p.terminateInput.Value()) != p.selected.ID {
				p.actionErr = "Type the instance ID exactly to confirm termination."
				return nil
			}
			if !isTerminable(p.selected) {
				p.actionErr = "Instance cannot be terminated from its current state."
				return nil
			}
			p.terminating = true
			p.actionErr = ""
			p.stage = ec2StageTerminating
			p.terminateInput.Blur()
			input := domainec2.TerminateInstanceInput{ProfileName: state.ActiveSession.Profile, Region: workflow.ActiveRegion(state), InstanceID: p.selected.ID}
			return tea.Batch(p.spinner.Tick, p.terminateInstanceCmd(input, p.sessionKey))
		}
	}
	var cmd tea.Cmd
	p.terminateInput, cmd = p.terminateInput.Update(msg)
	return cmd
}

func (p *EC2Page) startRefresh(state pageapi.State) tea.Cmd {
	if state.ActiveSession == nil || p.loading {
		return nil
	}
	p.loading = true
	p.loadErr = ""
	return tea.Batch(p.spinner.Tick, p.loadInstancesCmd(state.ActiveSession.Profile, workflow.ActiveRegion(state), p.sessionKey))
}

func (p *EC2Page) updateFocusedInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	if p.search.Focused() {
		p.search, cmd = p.search.Update(msg)
		return cmd
	}
	if p.terminateInput.Focused() {
		p.terminateInput, cmd = p.terminateInput.Update(msg)
		return cmd
	}
	return nil
}
