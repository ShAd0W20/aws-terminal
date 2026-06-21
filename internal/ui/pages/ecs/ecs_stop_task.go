package ecs

import (
	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/workflow"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	domainecs "aws-terminal/internal/domain/ecs"
)

const defaultStopTaskReason = "Stopped from aws-terminal"

func (p *ECSPage) startStopTask(state pageapi.State) tea.Cmd {
	if state.ActiveSession == nil {
		return nil
	}
	if !isTaskStoppable(p.selectedTask, p.selectedCluster.ARN) {
		p.updateErr = "Task cannot be stopped from its current state."
		return nil
	}
	p.updateSuccess = ""
	p.updateErr = ""
	p.stopTaskOriginTab = p.taskDetailTab
	p.stopLogStreaming()
	p.stopReasonInput.SetValue(defaultStopTaskReason)
	p.stage = ecsStageStopTaskReason
	return p.stopReasonInput.Focus()
}

func (p *ECSPage) updateStopTaskReasonStage(msg tea.Msg, state pageapi.State) tea.Cmd {
	k, isKey := msg.(tea.KeyMsg)
	if isKey {
		if key.Matches(k, ecsBackKey) {
			p.stopReasonInput.Blur()
			p.updateErr = ""
			p.taskDetailTab = p.stopTaskOriginTab
			p.stage = ecsStageTaskDetail
			return nil
		}
		if key.Matches(k, ecsEnterKey) {
			if strings.TrimSpace(p.stopReasonInput.Value()) == "" {
				p.updateErr = "stop reason is required"
				return nil
			}
			p.updateErr = ""
			p.stopReasonInput.Blur()
			p.stage = ecsStageStopTaskReview
			return nil
		}
	}
	if state.PageFocused {
		var cmd tea.Cmd
		p.stopReasonInput, cmd = p.stopReasonInput.Update(msg)
		return cmd
	}
	return nil
}

func (p *ECSPage) updateStopTaskReviewStage(k tea.KeyMsg, state pageapi.State) tea.Cmd {
	if key.Matches(k, ecsBackKey) {
		p.stage = ecsStageStopTaskReason
		return p.stopReasonInput.Focus()
	}
	if key.Matches(k, ecsEnterKey) {
		if state.ActiveSession == nil {
			return nil
		}
		input, err := p.buildStopTaskInput(state)
		if err != nil {
			p.updateErr = err.Error()
			return nil
		}
		p.updateErr = ""
		p.stoppingTask = true
		p.stage = ecsStageStoppingTask
		return tea.Batch(p.spinner.Tick, p.stopTaskCmd(input))
	}
	return nil
}

func (p *ECSPage) buildStopTaskInput(state pageapi.State) (domainecs.StopTaskInput, error) {
	if state.ActiveSession == nil {
		return domainecs.StopTaskInput{}, fmt.Errorf("active AWS profile is required")
	}
	if !isTaskStoppable(p.selectedTask, p.selectedCluster.ARN) {
		return domainecs.StopTaskInput{}, fmt.Errorf("task cannot be stopped from its current state")
	}
	reason := strings.TrimSpace(p.stopReasonInput.Value())
	if reason == "" {
		return domainecs.StopTaskInput{}, fmt.Errorf("stop reason is required")
	}
	return domainecs.StopTaskInput{ProfileName: state.ActiveSession.Profile, Region: workflow.ActiveRegion(state), ClusterARN: p.selectedCluster.ARN, Task: p.selectedTask.ARN, Reason: reason}, nil
}
