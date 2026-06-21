package ecr

import (
	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/workflow"
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (p *ECRPage) OnStateChanged(state pageapi.State) tea.Cmd {
	sessionKey := workflow.SessionKey(state)
	if sessionKey != p.sessionKey {
		p.sessionKey = sessionKey
		p.resetForSession()
	}
	if state.ActiveSession == nil || p.loadingRepositories || p.loadedFor == sessionKey {
		return nil
	}
	p.loadingRepositories = true
	p.repositoryErr = ""
	return p.loadRepositoriesCmd(state.ActiveSession.Profile, workflow.ActiveRegion(state), sessionKey)
}

func (p *ECRPage) SetFocused(focused bool) tea.Cmd {
	if focused {
		p.focusForStage()
	} else {
		p.searchInput.Blur()
		p.createInput.Blur()
		p.manualInput.Blur()
		p.tagInput.Blur()
	}
	return nil
}

func (p *ECRPage) Update(msg tea.Msg, state pageapi.State) tea.Cmd {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !p.pushing && !p.loadingRepositories && !p.imagesLoading && !p.localLoading {
			return nil
		}
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		return cmd
	case repositoriesLoadedMsg:
		if msg.sessionKey != p.sessionKey {
			return nil
		}
		p.loadingRepositories = false
		p.loadedFor = msg.sessionKey
		p.loadCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.repositoryErr = fmt.Sprintf("Unable to load ECR repositories: %v", msg.err)
			return nil
		}
		p.repositories = msg.repositories
		p.repositoryErr = ""
		if p.repositoryIndex >= len(p.repositories) {
			p.repositoryIndex = max(0, len(p.repositories)-1)
		}
		return nil
	case repositoryCreatedMsg:
		p.createCancel = nil
		p.loadingRepositories = false
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.repositoryErr = fmt.Sprintf("Unable to create repository: %v", msg.err)
			return nil
		}
		p.selectedRepository = msg.repository
		p.repositories = append(p.repositories, msg.repository)
		p.stage = ecrStageRepositoryImages
		p.focusForStage()
		p.imagesLoading = true
		if state.ActiveSession == nil {
			return nil
		}
		return tea.Batch(p.spinner.Tick, p.loadImagesCmd(state.ActiveSession.Profile, workflow.ActiveRegion(state), msg.repository.Name))
	case repositoryImagesLoadedMsg:
		p.imagesLoading = false
		p.imagesCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.imagesErr = fmt.Sprintf("Unable to load repository images: %v", msg.err)
			return nil
		}
		p.repositoryImages = msg.images
		p.imagePaginator.Page = 0
		p.syncImageTable()
		p.imagesErr = ""
		return nil
	case localImagesLoadedMsg:
		p.localLoading = false
		p.localCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			p.localErr = ""
			return nil
		}
		if msg.err != nil {
			p.localErr = fmt.Sprintf("Unable to list local Docker images: %v", msg.err)
			return nil
		}
		p.localImages = msg.images
		filtered := p.filteredLocalImages()
		if p.localIndex >= len(filtered) {
			p.localIndex = max(0, len(filtered)-1)
		}
		p.localPaginator.Page = 0
		p.syncLocalTable()
		p.localErr = ""
		return nil
	case pushPlanBuiltMsg:
		p.planning = false
		if msg.err != nil {
			p.planErr = fmt.Sprintf("Unable to build push plan: %v", msg.err)
			return nil
		}
		p.plan = &msg.plan
		p.planErr = ""
		p.stage = ecrStageReview
		p.focusForStage()
		return nil
	case pushStartedMsg:
		p.pushEvents = msg.events
		return p.waitForPushEventCmd()
	case pushEventMsg:
		if msg.event.progress != nil {
			p.pushProgress = *msg.event.progress
			return p.waitForPushEventCmd()
		}
		if !msg.event.done {
			return p.waitForPushEventCmd()
		}
		p.pushing = false
		p.pushCancel = nil
		if msg.event.err != nil {
			p.pushErr = fmt.Sprintf("Unable to push image: %v", msg.event.err)
			return nil
		}
		if msg.event.result != nil {
			result := *msg.event.result
			p.resetPushState()
			p.pushMessage = fmt.Sprintf("Pushed %s successfully.", result.DestinationImage)
			p.stage = ecrStageRepositoryImages
			p.focusForStage()
			if state.ActiveSession != nil && p.selectedRepository.Name != "" {
				p.imagesLoading = true
				return tea.Batch(p.spinner.Tick, p.loadImagesCmd(state.ActiveSession.Profile, workflow.ActiveRegion(state), p.selectedRepository.Name))
			}
		}
		return nil
	}

	keyMsg, isKey := msg.(tea.KeyMsg)
	if !isKey || !state.PageFocused {
		return p.updateInputs(msg)
	}
	switch p.stage {
	case ecrStageRepository:
		return p.updateRepositoryStage(keyMsg, state)
	case ecrStageCreateRepository:
		return p.updateCreateStage(msg, state)
	case ecrStageRepositoryImages:
		return p.updateImagesStage(keyMsg, state)
	case ecrStageLocalImage:
		return p.updateLocalStage(msg, state)
	case ecrStageTag:
		return p.updateTagStage(msg, state)
	case ecrStageReview:
		return p.updateReviewStage(keyMsg)
	case ecrStagePush:
		return p.updatePushStage(keyMsg)
	default:
		return nil
	}
}

func (p *ECRPage) updateInputs(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch p.stage {
	case ecrStageRepository:
		p.searchInput, cmd = p.searchInput.Update(msg)
	case ecrStageCreateRepository:
		p.createInput, cmd = p.createInput.Update(msg)
	case ecrStageLocalImage:
		p.manualInput, cmd = p.manualInput.Update(msg)
	case ecrStageTag:
		p.tagInput, cmd = p.tagInput.Update(msg)
	}
	return cmd
}

func (p *ECRPage) updateRepositoryStage(msg tea.KeyMsg, state pageapi.State) tea.Cmd {
	if p.searchInput.Focused() {
		if msg.Type == tea.KeyEsc {
			p.searchInput.Blur()
			return nil
		}
		if ecrTextInputKey(msg) {
			return p.updateInputs(msg)
		}
	}
	if key.Matches(msg, ecrSearchKey) {
		return p.searchInput.Focus()
	}
	if key.Matches(msg, ecrRefreshKey) && state.ActiveSession != nil {
		p.loadingRepositories = true
		return tea.Batch(p.spinner.Tick, p.loadRepositoriesCmd(state.ActiveSession.Profile, workflow.ActiveRegion(state), p.sessionKey))
	}
	if key.Matches(msg, ecrCreateKey) {
		p.searchInput.Blur()
		p.stage = ecrStageCreateRepository
		p.focusForStage()
		return nil
	}
	filtered := p.filteredRepositories()
	if p.repositoryIndex >= len(filtered) {
		p.repositoryIndex = max(0, len(filtered)-1)
	}
	switch {
	case key.Matches(msg, ecrUpKey):
		if p.repositoryIndex > 0 {
			p.repositoryIndex--
		}
	case key.Matches(msg, ecrDownKey):
		if p.repositoryIndex < len(filtered)-1 {
			p.repositoryIndex++
		}
	case key.Matches(msg, ecrEnterKey):
		if len(filtered) == 0 {
			p.stage = ecrStageCreateRepository
			p.createInput.SetValue(p.searchInput.Value())
			p.focusForStage()
			return nil
		}
		if p.repositoryIndex >= len(filtered) {
			p.repositoryIndex = len(filtered) - 1
		}
		p.searchInput.Blur()
		p.selectedRepository = filtered[p.repositoryIndex]
		p.pushMessage = ""
		p.stage = ecrStageRepositoryImages
		p.focusForStage()
		if state.ActiveSession != nil {
			p.imagesLoading = true
			return tea.Batch(p.spinner.Tick, p.loadImagesCmd(state.ActiveSession.Profile, workflow.ActiveRegion(state), p.selectedRepository.Name))
		}
	}
	return p.updateInputs(msg)
}

func (p *ECRPage) updateCreateStage(msg tea.Msg, state pageapi.State) tea.Cmd {
	if k, ok := msg.(tea.KeyMsg); ok {
		if ecrTextInputKey(k) {
			return p.updateInputs(msg)
		}
		if key.Matches(k, ecrBackKey) {
			p.stage = ecrStageRepository
			p.focusForStage()
			return nil
		}
		if key.Matches(k, ecrEnterKey) && state.ActiveSession != nil {
			p.loadingRepositories = true
			return tea.Batch(p.spinner.Tick, p.createRepositoryCmd(state.ActiveSession.Profile, workflow.ActiveRegion(state), p.createInput.Value()))
		}
	}
	return p.updateInputs(msg)
}
func (p *ECRPage) updateImagesStage(msg tea.KeyMsg, state pageapi.State) tea.Cmd {
	if key.Matches(msg, ecrBackKey) {
		p.stage = ecrStageRepository
		p.focusForStage()
		return nil
	}
	if key.Matches(msg, ecrRefreshKey) && state.ActiveSession != nil {
		p.imagesLoading = true
		return tea.Batch(p.spinner.Tick, p.loadImagesCmd(state.ActiveSession.Profile, workflow.ActiveRegion(state), p.selectedRepository.Name))
	}
	if key.Matches(msg, ecrEnterKey) {
		p.pushMessage = ""
		p.stage = ecrStageLocalImage
		p.focusForStage()
		p.localLoading = true
		return tea.Batch(p.spinner.Tick, p.loadLocalImagesCmd())
	}
	previousPage := p.imagePaginator.Page
	var cmd tea.Cmd
	p.imagePaginator, cmd = p.imagePaginator.Update(msg)
	if p.imagePaginator.Page != previousPage {
		p.syncImageTable()
		return cmd
	}
	p.imageTable, cmd = p.imageTable.Update(msg)
	return cmd
}
func (p *ECRPage) updateLocalStage(msg tea.Msg, state pageapi.State) tea.Cmd {
	if k, ok := msg.(tea.KeyMsg); ok {
		if p.manualInput.Focused() {
			if k.Type == tea.KeyEsc {
				p.manualInput.Blur()
				return nil
			}
			if ecrTextInputKey(k) {
				cmd := p.updateInputs(msg)
				p.localIndex = 0
				p.localPaginator.Page = 0
				p.syncLocalTable()
				return cmd
			}
		}
		if key.Matches(k, ecrSearchKey) {
			return p.manualInput.Focus()
		}
		switch {
		case key.Matches(k, ecrBackKey):
			p.stage = ecrStageRepositoryImages
			p.focusForStage()
			return nil
		case key.Matches(k, ecrUpKey):
			if p.localIndex > 0 {
				p.localIndex--
			}
			page := p.localIndex / max(1, p.localPaginator.PerPage)
			if page != p.localPaginator.Page {
				p.localPaginator.Page = page
			}
			p.syncLocalTable()
			return nil
		case key.Matches(k, ecrDownKey):
			if p.localIndex < len(p.filteredLocalImages())-1 {
				p.localIndex++
			}
			page := p.localIndex / max(1, p.localPaginator.PerPage)
			if page != p.localPaginator.Page {
				p.localPaginator.Page = page
			}
			p.syncLocalTable()
			return nil
		case key.Matches(k, ecrEnterKey):
			source := p.selectedSourceImage()
			p.tagInput.SetValue(defaultTag(source))
			p.stage = ecrStageTag
			p.focusForStage()
			return nil
		case key.Matches(k, ecrRefreshKey):
			p.localLoading = true
			p.localIndex = 0
			return tea.Batch(p.spinner.Tick, p.loadLocalImagesCmd())
		}
		previousPage := p.localPaginator.Page
		var cmd tea.Cmd
		p.localPaginator, cmd = p.localPaginator.Update(msg)
		if p.localPaginator.Page != previousPage {
			start, _ := p.localPaginator.GetSliceBounds(len(p.filteredLocalImages()))
			p.localIndex = start
			p.syncLocalTable()
			return cmd
		}
		p.localTable, cmd = p.localTable.Update(msg)
		start, _ := p.localPaginator.GetSliceBounds(len(p.filteredLocalImages()))
		p.localIndex = start + p.localTable.Cursor()
		return cmd
	}
	return p.updateInputs(msg)
}
func (p *ECRPage) updateTagStage(msg tea.Msg, state pageapi.State) tea.Cmd {
	if k, ok := msg.(tea.KeyMsg); ok {
		if ecrTextInputKey(k) {
			return p.updateInputs(msg)
		}
		if key.Matches(k, ecrBackKey) {
			p.stage = ecrStageLocalImage
			p.focusForStage()
			return nil
		}
		if key.Matches(k, ecrEnterKey) {
			p.planning = true
			return p.buildPlanCmd(state)
		}
	}
	return p.updateInputs(msg)
}
func (p *ECRPage) updateReviewStage(msg tea.KeyMsg) tea.Cmd {
	if key.Matches(msg, ecrBackKey) {
		p.stage = ecrStageTag
		p.focusForStage()
		return nil
	}
	if key.Matches(msg, ecrEnterKey) && p.plan != nil {
		p.stage = ecrStagePush
		p.pushing = true
		p.pushErr = ""
		p.pushMessage = ""
		p.pushResult = nil
		return tea.Batch(p.spinner.Tick, p.startPushCmd(*p.plan))
	}
	return nil
}
func (p *ECRPage) updatePushStage(msg tea.KeyMsg) tea.Cmd {
	if p.pushing {
		if key.Matches(msg, ecrBackKey) {
			if p.pushCancel != nil {
				p.pushCancel()
			}
			p.pushing = false
			p.pushErr = "Push cancelled. The remote registry may still finish any already-started upload."
		}
		return nil
	}
	if key.Matches(msg, ecrBackKey) {
		p.stage = ecrStageRepositoryImages
		p.focusForStage()
	}
	return nil
}
