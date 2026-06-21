package cloudfront

import (
	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/workflow"
	"context"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	appcloudfront "aws-terminal/internal/application/cloudfront"
)

func (p *CloudFrontPage) cancelCommands() {
	if p.loadCancel != nil {
		p.loadCancel()
		p.loadCancel = nil
	}
	if p.createCancel != nil {
		p.createCancel()
		p.createCancel = nil
	}
	if p.pollCancel != nil {
		p.pollCancel()
		p.pollCancel = nil
	}
}

func (p *CloudFrontPage) loadDistributionsCmd(profileName, region, sessionKey string) tea.Cmd {
	if p.loadCancel != nil {
		p.loadCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.loadCancel = cancel
	return func() tea.Msg {
		distributions, err := p.service.ListDistributions(ctx, profileName, region)
		return cloudFrontDistributionsLoadedMsg{sessionKey: sessionKey, distributions: distributions, err: err}
	}
}

func (p *CloudFrontPage) createInvalidationCmd(profileName, region, distributionID string, paths []string) tea.Cmd {
	if p.createCancel != nil {
		p.createCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.createCancel = cancel
	return func() tea.Msg {
		invalidation, err := p.service.CreateInvalidation(ctx, appcloudfront.CreateInvalidationInput{
			Profile:        profileName,
			Region:         region,
			DistributionID: distributionID,
			Paths:          paths,
		})
		return cloudFrontInvalidationCreatedMsg{invalidation: invalidation, err: err}
	}
}

func (p *CloudFrontPage) pollInvalidationCmd(state pageapi.State, distributionID, invalidationID string, delay time.Duration) tea.Cmd {
	if state.ActiveSession == nil {
		return nil
	}
	profileName := state.ActiveSession.Profile
	region := workflow.ActiveRegion(state)
	if p.pollCancel != nil {
		p.pollCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.pollCancel = cancel

	return func() tea.Msg {
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return cloudFrontInvalidationPolledMsg{err: ctx.Err()}
			case <-timer.C:
			}
		}
		invalidation, err := p.service.GetInvalidation(ctx, profileName, region, distributionID, invalidationID)
		return cloudFrontInvalidationPolledMsg{invalidation: invalidation, err: err}
	}
}

func (p *CloudFrontPage) copyCommandCmd(profileName, distributionID string, paths []string) tea.Cmd {
	command := buildInvalidationCommand(profileName, distributionID, paths)
	return func() tea.Msg {
		return cloudFrontCopiedMsg{err: clipboard.WriteAll(command)}
	}
}
