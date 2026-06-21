package cloudfront

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/workflow"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	domaincloudfront "aws-terminal/internal/domain/cloudfront"
)

func (p *CloudFrontPage) OnStateChanged(state pageapi.State) tea.Cmd {
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
	return tea.Batch(p.spinner.Tick, p.loadDistributionsCmd(state.ActiveSession.Profile, workflow.ActiveRegion(state), sessionKey))
}

func (p *CloudFrontPage) SetFocused(focused bool) tea.Cmd {
	if !focused {
		p.pathsInput.Blur()
		return nil
	}
	if p.stage == cloudFrontStagePaths {
		return p.pathsInput.Focus()
	}
	return nil
}

func (p *CloudFrontPage) Update(msg tea.Msg, state pageapi.State) tea.Cmd {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !p.loading && !p.creating {
			return nil
		}
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		return cmd
	case cloudFrontDistributionsLoadedMsg:
		if msg.sessionKey != p.sessionKey {
			return nil
		}
		p.loading = false
		p.loadCancel = nil
		p.loadedFor = msg.sessionKey
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.loadErr = fmt.Sprintf("Unable to load distributions: %v", msg.err)
			p.distributions = nil
			return nil
		}

		p.loadErr = ""
		p.distributions = msg.distributions
		if p.distributionIndex >= len(p.distributions) {
			p.distributionIndex = max(0, len(p.distributions)-1)
		}
		return nil
	case cloudFrontInvalidationCreatedMsg:
		p.createCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			p.creating = false
			return nil
		}
		if msg.err != nil {
			p.creating = false
			p.createErr = fmt.Sprintf("Unable to create invalidation: %v", msg.err)
			return nil
		}
		result := msg.invalidation
		p.invalidation = &result
		p.createErr = ""
		p.copiedMessage = ""
		p.stage = cloudFrontStageResult
		if strings.EqualFold(result.Status, "Completed") {
			p.creating = false
			return nil
		}
		p.creating = true
		return tea.Batch(p.spinner.Tick, p.pollInvalidationCmd(state, result.DistributionID, result.ID, 2*time.Second))
	case cloudFrontInvalidationPolledMsg:
		p.pollCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			p.creating = false
			return nil
		}
		if msg.err != nil {
			p.creating = false
			p.createErr = fmt.Sprintf("Unable to refresh invalidation status: %v", msg.err)
			return nil
		}
		result := msg.invalidation
		p.invalidation = &result
		if strings.EqualFold(result.Status, "Completed") {
			p.creating = false
			return nil
		}
		p.creating = true
		return tea.Batch(p.spinner.Tick, p.pollInvalidationCmd(state, result.DistributionID, result.ID, 2*time.Second))
	case cloudFrontCopiedMsg:
		if msg.err != nil {
			p.createErr = fmt.Sprintf("Unable to copy invalidation command: %v", msg.err)
			p.copiedMessage = ""
			return nil
		}
		p.createErr = ""
		p.copiedMessage = "Invalidation command copied to clipboard."
		return nil
	}

	keyMsg, isKey := msg.(tea.KeyMsg)
	if !isKey || !state.PageFocused {
		if p.stage == cloudFrontStagePaths {
			var cmd tea.Cmd
			p.pathsInput, cmd = p.pathsInput.Update(msg)
			return cmd
		}
		return nil
	}

	switch p.stage {
	case cloudFrontStageDistribution:
		return p.updateDistributionStage(keyMsg)
	case cloudFrontStagePaths:
		return p.updatePathsStage(msg, state)
	case cloudFrontStageResult:
		return p.updateResultStage(keyMsg, state)
	default:
		return nil
	}
}

func (p *CloudFrontPage) updateDistributionStage(msg tea.KeyMsg) tea.Cmd {
	p.copiedMessage = ""
	if key.Matches(msg, cloudFrontCancelKey) {
		if p.loading {
			p.cancelCommands()
			p.loading = false
			p.loadErr = "Distribution loading cancelled."
		}
		return nil
	}
	if len(p.distributions) == 0 {
		return nil
	}

	switch {
	case key.Matches(msg, cloudFrontUpKey):
		if p.distributionIndex > 0 {
			p.distributionIndex--
		}
	case key.Matches(msg, cloudFrontDownKey):
		if p.distributionIndex < len(p.distributions)-1 {
			p.distributionIndex++
		}
	case key.Matches(msg, cloudFrontEnterKey):
		p.selectedDistribution = p.distributions[p.distributionIndex]
		p.stage = cloudFrontStagePaths
		p.createErr = ""
		p.invalidation = nil
		return p.pathsInput.Focus()
	}
	return nil
}

func (p *CloudFrontPage) updatePathsStage(msg tea.Msg, state pageapi.State) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, cloudFrontBackKey, cloudFrontCancelKey):
			p.stage = cloudFrontStageDistribution
			p.pathsInput.Blur()
			p.copiedMessage = ""
			return nil
		case key.Matches(keyMsg, cloudFrontEnterKey):
			if state.ActiveSession == nil || p.selectedDistribution.ID == "" || p.creating {
				return nil
			}
			paths := parseInvalidationPaths(p.pathsInput.Value())
			p.stage = cloudFrontStageResult
			p.creating = true
			p.createErr = ""
			p.copiedMessage = ""
			p.invalidation = nil
			return tea.Batch(
				p.spinner.Tick,
				p.createInvalidationCmd(state.ActiveSession.Profile, workflow.ActiveRegion(state), p.selectedDistribution.ID, paths),
			)
		case key.Matches(keyMsg, cloudFrontCopyKey):
			if state.ActiveSession == nil || p.selectedDistribution.ID == "" {
				return nil
			}
			p.copiedMessage = ""
			return p.copyCommandCmd(state.ActiveSession.Profile, p.selectedDistribution.ID, parseInvalidationPaths(p.pathsInput.Value()))
		}
	}

	var cmd tea.Cmd
	p.pathsInput, cmd = p.pathsInput.Update(msg)
	return cmd
}

func (p *CloudFrontPage) updateResultStage(msg tea.KeyMsg, state pageapi.State) tea.Cmd {
	if p.creating {
		if key.Matches(msg, cloudFrontCancelKey) {
			p.cancelCommands()
			p.creating = false
			p.createErr = "Stopped waiting for invalidation status. The invalidation may still continue in CloudFront."
		}
		return nil
	}

	switch {
	case key.Matches(msg, cloudFrontBackKey, cloudFrontCancelKey):
		p.stage = cloudFrontStageDistribution
		p.copiedMessage = ""
		p.invalidation = nil
		return nil
	case key.Matches(msg, cloudFrontCopyKey):
		if p.selectedDistribution.ID == "" {
			return nil
		}
		profileName := ""
		if state.ActiveSession != nil {
			profileName = state.ActiveSession.Profile
		}
		return p.copyCommandCmd(profileName, p.selectedDistribution.ID, parseInvalidationPaths(p.pathsInput.Value()))
	}
	return nil
}

func (p *CloudFrontPage) resetForSession() {
	p.cancelCommands()
	p.loadedFor = ""
	p.loading = false
	p.loadErr = ""
	p.distributions = nil
	p.distributionIndex = 0
	p.selectedDistribution = domaincloudfront.Distribution{}
	p.pathsInput.SetValue("/*")
	p.pathsInput.Blur()
	p.stage = cloudFrontStageDistribution
	p.creating = false
	p.createErr = ""
	p.copiedMessage = ""
	p.invalidation = nil
}
