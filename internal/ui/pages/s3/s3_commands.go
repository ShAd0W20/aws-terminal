package s3

import (
	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/workflow"
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	apps3 "aws-terminal/internal/application/s3"
	domains3 "aws-terminal/internal/domain/s3"
)

func (p *S3Page) cancelLoadBuckets() {
	if p.loadBucketsCancel != nil {
		p.loadBucketsCancel()
		p.loadBucketsCancel = nil
	}
}

func (p *S3Page) cancelWorkflowCommands() {
	if p.inspectCancel != nil {
		p.inspectCancel()
		p.inspectCancel = nil
	}
	if p.planCancel != nil {
		p.planCancel()
		p.planCancel = nil
	}
	if p.syncCancel != nil {
		p.syncCancel()
		p.syncCancel = nil
	}
}

func (p *S3Page) loadBucketsCmd(profileName, region, sessionKey string) tea.Cmd {
	p.cancelLoadBuckets()
	ctx, cancel := context.WithCancel(context.Background())
	p.loadBucketsCancel = cancel
	return func() tea.Msg {
		buckets, err := p.service.ListBuckets(ctx, profileName, region)
		return s3BucketsLoadedMsg{sessionKey: sessionKey, buckets: buckets, err: err}
	}
}

func (p *S3Page) inspectSourceCmd(sourcePath string) tea.Cmd {
	if p.inspectCancel != nil {
		p.inspectCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.inspectCancel = cancel
	return func() tea.Msg {
		select {
		case <-ctx.Done():
			return s3SourceInspectedMsg{err: ctx.Err()}
		default:
		}
		selection, err := p.service.InspectSource(sourcePath)
		if err == nil {
			select {
			case <-ctx.Done():
				err = ctx.Err()
			default:
			}
		}
		return s3SourceInspectedMsg{source: selection, err: err}
	}
}

func (p *S3Page) buildPlanCmd(state pageapi.State) tea.Cmd {
	if state.ActiveSession == nil || p.sourceInfo == nil || strings.TrimSpace(p.selectedBucket) == "" {
		return nil
	}

	uploadPlanningMode := domains3.UploadPlanningModeFullRefresh
	if p.optimizedPlanning {
		uploadPlanningMode = domains3.UploadPlanningModeSizeOnly
	}

	input := apps3.BuildSyncPlanInput{
		Profile:             state.ActiveSession.Profile,
		Region:              workflow.ActiveRegion(state),
		Bucket:              p.selectedBucket,
		Prefix:              p.prefixInput.Value(),
		SourcePath:          p.sourceInfo.Path,
		DeleteEnabled:       p.deleteEnabled,
		UploadPlanningMode:  uploadPlanningMode,
		StaticWebsitePreset: p.staticWebsitePreset,
	}

	if p.planCancel != nil {
		p.planCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.planCancel = cancel
	return func() tea.Msg {
		plan, err := p.service.BuildSyncPlan(ctx, input)
		return s3SyncPlanBuiltMsg{plan: plan, err: err}
	}
}

func (p *S3Page) startSyncCmd(plan domains3.SyncPlan) tea.Cmd {
	if p.syncCancel != nil {
		p.syncCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.syncCancel = cancel
	return func() tea.Msg {
		events := make(chan s3SyncEvent)
		go func() {
			defer close(events)

			progressCh := make(chan domains3.SyncProgress, 128)
			resultCh := make(chan struct {
				result domains3.SyncResult
				err    error
			}, 1)

			go func() {
				result, err := p.service.ExecuteSync(ctx, plan, progressCh)
				close(progressCh)
				resultCh <- struct {
					result domains3.SyncResult
					err    error
				}{result: result, err: err}
			}()

			for progress := range progressCh {
				current := progress
				events <- s3SyncEvent{progress: &current}
			}

			outcome := <-resultCh
			events <- s3SyncEvent{result: &outcome.result, err: outcome.err, done: true}
		}()
		return s3SyncStartedMsg{events: events}
	}
}

func (p *S3Page) waitForSyncEventCmd() tea.Cmd {
	if p.syncEvents == nil {
		return nil
	}

	return func() tea.Msg {
		event, ok := <-p.syncEvents
		if !ok {
			return s3SyncEventMsg{event: s3SyncEvent{done: true}}
		}
		return s3SyncEventMsg{event: event}
	}
}

func (p *S3Page) clearSyncMessageCmd(seq int, delay time.Duration) tea.Cmd {
	return func() tea.Msg {
		if delay > 0 {
			<-time.After(delay)
		}
		return s3ClearSyncMessageMsg{seq: seq}
	}
}
