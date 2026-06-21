package s3

import (
	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/workflow"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	domains3 "aws-terminal/internal/domain/s3"
)

func (p *S3Page) OnStateChanged(state pageapi.State) tea.Cmd {
	sessionKey := workflow.SessionKey(state)
	if sessionKey != p.sessionKey {
		p.sessionKey = sessionKey
		p.resetWorkflowForSession()
	}

	if state.ActiveSession == nil {
		return nil
	}
	if p.loadingBuckets || p.bucketsLoadedFor == sessionKey {
		return nil
	}

	p.loadingBuckets = true
	p.bucketErr = ""
	return p.loadBucketsCmd(state.ActiveSession.Profile, workflow.ActiveRegion(state), sessionKey)
}

func (p *S3Page) SetFocused(focused bool) tea.Cmd {
	if !focused {
		p.prefixInput.Blur()
		p.confirmInput.Blur()
		return nil
	}
	if p.stage == s3StagePrefix {
		p.confirmInput.Blur()
		return p.prefixInput.Focus()
	}
	if p.stage == s3StageConfirmDelete {
		p.prefixInput.Blur()
		return p.confirmInput.Focus()
	}

	return nil
}

func (p *S3Page) Update(msg tea.Msg, state pageapi.State) tea.Cmd {
	switch msg := msg.(type) {
	case s3BucketsLoadedMsg:
		if msg.sessionKey != p.sessionKey {
			return nil
		}
		p.loadingBuckets = false
		p.loadBucketsCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.bucketErr = fmt.Sprintf("Unable to load buckets: %v", msg.err)
			p.buckets = nil
			p.selectedBucket = ""
			p.bucketsLoadedFor = msg.sessionKey
			return nil
		}

		p.bucketErr = ""
		p.buckets = msg.buckets
		p.bucketsLoadedFor = msg.sessionKey
		if p.bucketIndex >= len(p.buckets) {
			p.bucketIndex = max(0, len(p.buckets)-1)
		}
		if p.selectedBucket == "" && len(p.buckets) > 0 {
			p.selectedBucket = p.buckets[p.bucketIndex].Name
		}
		return nil
	case s3SourceInspectedMsg:
		p.inspectingSource = false
		p.inspectCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.sourceInfo = nil
			p.sourceErr = fmt.Sprintf("Unable to inspect source: %v", msg.err)
			return nil
		}

		selectedSource := msg.source
		p.sourceInfo = &selectedSource
		p.rememberSource(selectedSource)
		p.sourceErr = ""
		p.plan = nil
		p.planErr = ""
		p.syncErr = ""
		p.syncMessage = ""
		p.offerInvalidation = false
		p.syncResult = nil
		p.stage = s3StagePrefix
		if state.PageFocused {
			return p.prefixInput.Focus()
		}
		return nil
	case s3SyncPlanBuiltMsg:
		p.planning = false
		p.planCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.plan = nil
			p.planErr = fmt.Sprintf("Unable to build sync plan: %v", msg.err)
			return nil
		}

		plan := msg.plan
		p.plan = &plan
		p.planErr = ""
		if p.sourceInfo != nil && p.sourceInfo.Kind == domains3.SourceKindFile {
			p.deleteEnabled = false
		}
		return nil
	case s3SyncStartedMsg:
		p.syncEvents = msg.events
		p.syncing = true
		p.syncErr = ""
		p.syncMessage = ""
		p.offerInvalidation = false
		p.syncResult = nil
		p.lastSyncStartedAt = time.Now()
		return p.waitForSyncEventCmd()
	case s3SyncEventMsg:
		if msg.event.progress != nil {
			p.syncProgress = *msg.event.progress
		}
		if msg.event.done {
			p.syncing = false
			p.lastSyncFinishedAt = time.Now()
			if msg.event.result != nil {
				result := *msg.event.result
				p.syncResult = &result
			}
			p.syncCancel = nil
			if errors.Is(msg.event.err, context.Canceled) {
				p.syncErr = "Sync cancelled."
				return nil
			}
			if msg.event.err != nil {
				p.syncErr = msg.event.err.Error()
				return nil
			}
			if p.syncResult != nil {
				duration := p.lastSyncFinishedAt.Sub(p.lastSyncStartedAt)
				offerInvalidation := p.syncResult.Uploaded > 0 || p.syncResult.Deleted > 0
				message := fmt.Sprintf(
					"Sync complete • %d uploaded • %d deleted • %d skipped • %s",
					p.syncResult.Uploaded,
					p.syncResult.Deleted,
					p.syncResult.Skipped,
					duration.Round(time.Second),
				)
				if offerInvalidation {
					message += " • press i for CloudFront invalidation"
				}
				p.resetWorkflow()
				p.offerInvalidation = offerInvalidation
				p.syncMessageSeq++
				p.syncMessage = message
				return p.clearSyncMessageCmd(p.syncMessageSeq, 4*time.Second)
			}
			return nil
		}
		return p.waitForSyncEventCmd()
	case s3ClearSyncMessageMsg:
		if msg.seq == p.syncMessageSeq {
			p.syncMessage = ""
			p.offerInvalidation = false
		}
		return nil
	}

	keyMsg, isKeyMsg := msg.(tea.KeyMsg)

	switch p.stage {
	case s3StageBucket:
		if !state.PageFocused || !isKeyMsg {
			return nil
		}
		return p.updateBucketStage(keyMsg)
	case s3StageSource:
		if !state.PageFocused && isKeyMsg {
			return nil
		}
		return p.updateSourceStage(msg)
	case s3StagePrefix:
		if !state.PageFocused && isKeyMsg {
			return nil
		}
		return p.updatePrefixStage(msg, state)
	case s3StageReview:
		if !state.PageFocused || !isKeyMsg {
			return nil
		}
		return p.updateReviewStage(keyMsg, state)
	case s3StageConfirmDelete:
		if !state.PageFocused && isKeyMsg {
			return nil
		}
		return p.updateConfirmDeleteStage(msg)
	case s3StageSync:
		if !state.PageFocused || !isKeyMsg {
			return nil
		}
		return p.updateSyncStage(keyMsg, state)
	default:
		return nil
	}
}

func (p *S3Page) updateBucketStage(msg tea.KeyMsg) tea.Cmd {
	if key.Matches(msg, s3CancelKey) {
		if p.loadingBuckets {
			p.cancelLoadBuckets()
			p.loadingBuckets = false
			p.bucketErr = "Bucket loading cancelled."
		}
		p.syncMessage = ""
		p.offerInvalidation = false
		return nil
	}

	if key.Matches(msg, s3InvalidateKey) && p.offerInvalidation {
		p.syncMessage = ""
		p.offerInvalidation = false
		return func() tea.Msg {
			return pageapi.OpenPageMsg{PageID: "cloudfront", Focus: true}
		}
	}

	if p.syncMessage != "" {
		p.syncMessage = ""
		p.offerInvalidation = false
	}
	if len(p.buckets) == 0 {
		return nil
	}

	switch {
	case key.Matches(msg, s3BucketUpKey):
		if p.bucketIndex > 0 {
			p.bucketIndex--
		}
	case key.Matches(msg, s3BucketDownKey):
		if p.bucketIndex < len(p.buckets)-1 {
			p.bucketIndex++
		}
	case key.Matches(msg, s3EnterKey):
		p.selectedBucket = p.buckets[p.bucketIndex].Name
		p.stage = s3StageSource
		p.plan = nil
		p.planErr = ""
		p.syncErr = ""
		p.syncMessage = ""
		p.syncResult = nil
		p.sourceErr = ""
		return p.picker.Init()
	}

	return nil
}

func (p *S3Page) updateSourceStage(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, s3BackKey, s3CancelKey):
			if p.inspectingSource {
				p.cancelWorkflowCommands()
				p.inspectingSource = false
			}
			p.stage = s3StageBucket
			return nil
		case key.Matches(keyMsg, s3RefreshKey):
			if p.sessionKey != "" && !p.loadingBuckets {
				parts := strings.SplitN(p.sessionKey, "|", 2)
				profileName := parts[0]
				region := ""
				if len(parts) > 1 {
					region = parts[1]
				}
				p.loadingBuckets = true
				p.bucketErr = ""
				return p.loadBucketsCmd(profileName, region, p.sessionKey)
			}
		}
	}

	var cmd tea.Cmd
	p.picker, cmd = p.picker.Update(msg)
	if didSelect, path := p.picker.DidSelectFile(msg); didSelect {
		p.inspectingSource = true
		p.sourceErr = ""
		return tea.Batch(cmd, p.inspectSourceCmd(path))
	}
	if didSelectDisabled, path := p.picker.DidSelectDisabledFile(msg); didSelectDisabled {
		p.sourceErr = fmt.Sprintf("%q cannot be selected.", path)
	}
	return cmd
}

func (p *S3Page) updatePrefixStage(msg tea.Msg, state pageapi.State) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, s3EnterKey):
			p.rememberPrefix(p.prefixInput.Value())
			p.stage = s3StageReview
			p.reviewViewport.GotoTop()
			p.planning = true
			p.planErr = ""
			p.syncErr = ""
			p.syncResult = nil
			p.prefixInput.Blur()
			return p.buildPlanCmd(state)
		case key.Matches(keyMsg, s3BackKey, s3CancelKey):
			p.stage = s3StageSource
			p.prefixInput.Blur()
			return p.picker.Init()
		}
	}

	var cmd tea.Cmd
	p.prefixInput, cmd = p.prefixInput.Update(msg)
	return cmd
}

func (p *S3Page) updateReviewStage(msg tea.KeyMsg, state pageapi.State) tea.Cmd {
	switch {
	case key.Matches(msg, s3BackKey, s3CancelKey):
		if p.planning {
			p.cancelWorkflowCommands()
			p.planning = false
		}
		p.stage = s3StagePrefix
		if state.PageFocused {
			return p.prefixInput.Focus()
		}
		return nil
	case key.Matches(msg, s3ToggleKey):
		if p.sourceInfo == nil || p.sourceInfo.Kind != domains3.SourceKindDirectory {
			return nil
		}
		p.deleteEnabled = !p.deleteEnabled
		p.reviewViewport.GotoTop()
		p.planning = true
		p.planErr = ""
		return p.buildPlanCmd(state)
	case key.Matches(msg, s3OptimizeKey):
		p.optimizedPlanning = !p.optimizedPlanning
		p.reviewViewport.GotoTop()
		p.planning = true
		p.planErr = ""
		return p.buildPlanCmd(state)
	case key.Matches(msg, s3MetadataKey):
		p.staticWebsitePreset = !p.staticWebsitePreset
		p.reviewViewport.GotoTop()
		p.planning = true
		p.planErr = ""
		return p.buildPlanCmd(state)
	case key.Matches(msg, s3EnterKey):
		if p.plan == nil || p.planning || p.syncing {
			return nil
		}
		if requiresDeleteConfirmation(p.plan) {
			p.stage = s3StageConfirmDelete
			p.confirmInput.SetValue("")
			if state.PageFocused {
				return p.confirmInput.Focus()
			}
			return nil
		}
		p.stage = s3StageSync
		return p.startSyncCmd(*p.plan)
	}

	var cmd tea.Cmd
	p.reviewViewport, cmd = p.reviewViewport.Update(msg)
	return cmd
}

func (p *S3Page) updateConfirmDeleteStage(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, s3BackKey, s3CancelKey):
			p.stage = s3StageReview
			p.confirmInput.SetValue("")
			p.confirmInput.Blur()
			return nil
		case key.Matches(keyMsg, s3EnterKey):
			if p.plan == nil || !deleteConfirmationMatches(p.confirmInput.Value()) {
				return nil
			}
			p.stage = s3StageSync
			p.confirmInput.Blur()
			return p.startSyncCmd(*p.plan)
		}
	}

	var cmd tea.Cmd
	p.confirmInput, cmd = p.confirmInput.Update(msg)
	return cmd
}

func (p *S3Page) updateSyncStage(msg tea.KeyMsg, state pageapi.State) tea.Cmd {
	if p.syncing {
		if key.Matches(msg, s3CancelKey) {
			if p.syncCancel != nil {
				p.syncCancel()
			}
			p.syncErr = "Cancelling sync..."
		}
		return nil
	}
	if key.Matches(msg, s3BackKey, s3CancelKey) {
		p.stage = s3StageReview
		return nil
	}
	if key.Matches(msg, s3EnterKey) && p.plan != nil {
		p.stage = s3StageSync
		return p.startSyncCmd(*p.plan)
	}
	if key.Matches(msg, s3ContinueKey) {
		p.stage = s3StageBucket
		return p.OnStateChanged(state)
	}
	return nil
}

func (p *S3Page) resetWorkflow() {
	p.cancelWorkflowCommands()
	p.selectedBucket = ""
	p.stage = s3StageBucket
	p.sourceInfo = nil
	p.inspectingSource = false
	p.sourceErr = ""
	p.plan = nil
	p.planErr = ""
	p.syncing = false
	p.syncErr = ""
	p.syncResult = nil
	p.offerInvalidation = false
	p.syncEvents = nil
	p.syncProgress = domains3.SyncProgress{}
	p.deleteEnabled = false
	p.optimizedPlanning = false
	p.staticWebsitePreset = false
	p.lastSyncStartedAt = time.Time{}
	p.lastSyncFinishedAt = time.Time{}
	p.prefixInput.SetValue("")
	p.prefixInput.Blur()
	p.confirmInput.SetValue("")
	p.confirmInput.Blur()
	p.reviewViewport.GotoTop()
}

func (p *S3Page) resetWorkflowForSession() {
	p.cancelWorkflowCommands()
	p.cancelLoadBuckets()
	p.bucketsLoadedFor = ""
	p.loadingBuckets = false
	p.bucketErr = ""
	p.buckets = nil
	p.bucketIndex = 0
	p.syncMessage = ""
	p.offerInvalidation = false
	p.resetWorkflow()
}
