package ecr

import (
	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/workflow"
	"context"

	tea "github.com/charmbracelet/bubbletea"

	appsecr "aws-terminal/internal/application/ecr"
	domainecr "aws-terminal/internal/domain/ecr"
)

func (p *ECRPage) cancelAll() {
	for _, cancel := range []context.CancelFunc{p.loadCancel, p.createCancel, p.imagesCancel, p.localCancel, p.pushCancel} {
		if cancel != nil {
			cancel()
		}
	}
	p.loadCancel, p.createCancel, p.imagesCancel, p.localCancel, p.pushCancel = nil, nil, nil, nil, nil
}

func (p *ECRPage) loadRepositoriesCmd(profile, region, sessionKey string) tea.Cmd {
	if p.loadCancel != nil {
		p.loadCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.loadCancel = cancel
	return func() tea.Msg {
		repos, err := p.service.ListRepositories(ctx, profile, region)
		return repositoriesLoadedMsg{sessionKey: sessionKey, repositories: repos, err: err}
	}
}

func (p *ECRPage) createRepositoryCmd(profile, region, name string) tea.Cmd {
	if p.createCancel != nil {
		p.createCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.createCancel = cancel
	return func() tea.Msg {
		repo, err := p.service.CreateRepository(ctx, domainecr.CreateRepositoryInput{Profile: profile, Region: region, Name: name})
		return repositoryCreatedMsg{repository: repo, err: err}
	}
}

func (p *ECRPage) loadImagesCmd(profile, region, repositoryName string) tea.Cmd {
	if p.imagesCancel != nil {
		p.imagesCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.imagesCancel = cancel
	return func() tea.Msg {
		imgs, err := p.service.ListImages(ctx, profile, region, repositoryName)
		return repositoryImagesLoadedMsg{repositoryName: repositoryName, images: imgs, err: err}
	}
}

func (p *ECRPage) loadLocalImagesCmd() tea.Cmd {
	if p.localCancel != nil {
		p.localCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.localCancel = cancel
	return func() tea.Msg {
		imgs, err := p.service.ListLocalImages(ctx)
		return localImagesLoadedMsg{images: imgs, err: err}
	}
}

func (p *ECRPage) buildPlanCmd(state pageapi.State) tea.Cmd {
	if state.ActiveSession == nil {
		return nil
	}
	source := p.selectedSourceImage()
	input := appsecr.BuildPushPlanInput{Profile: state.ActiveSession.Profile, Region: workflow.ActiveRegion(state), RepositoryName: p.selectedRepository.Name, RepositoryURI: p.selectedRepository.URI, SourceImage: source, DestinationTag: p.tagInput.Value()}
	return func() tea.Msg {
		plan, err := p.service.BuildPushPlan(input)
		return pushPlanBuiltMsg{plan: plan, err: err}
	}
}

func (p *ECRPage) startPushCmd(plan domainecr.PushPlan) tea.Cmd {
	if p.pushCancel != nil {
		p.pushCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.pushCancel = cancel
	return func() tea.Msg {
		events := make(chan pushEvent)
		go func() {
			defer close(events)
			progressCh := make(chan domainecr.PushProgress, 128)
			resultCh := make(chan struct {
				result domainecr.PushResult
				err    error
			}, 1)
			go func() {
				result, err := p.service.ExecutePush(ctx, plan, progressCh)
				close(progressCh)
				resultCh <- struct {
					result domainecr.PushResult
					err    error
				}{result, err}
			}()
			for pr := range progressCh {
				cur := pr
				events <- pushEvent{progress: &cur}
			}
			out := <-resultCh
			events <- pushEvent{result: &out.result, err: out.err, done: true}
		}()
		return pushStartedMsg{events: events}
	}
}

func (p *ECRPage) waitForPushEventCmd() tea.Cmd {
	if p.pushEvents == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-p.pushEvents
		if !ok {
			return pushEventMsg{event: pushEvent{done: true}}
		}
		return pushEventMsg{event: ev}
	}
}
