package s3

import (
	"aws-terminal/internal/ui/pageapi"
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	apps3 "aws-terminal/internal/application/s3"
	domains3 "aws-terminal/internal/domain/s3"
)

type blockingS3Service struct {
	started  chan struct{}
	canceled chan struct{}
}

func (s *blockingS3Service) ListBuckets(context.Context, string, string) ([]domains3.Bucket, error) {
	return nil, nil
}

func (s *blockingS3Service) InspectSource(string) (domains3.SourceSelection, error) {
	return domains3.SourceSelection{}, nil
}

func (s *blockingS3Service) BuildSyncPlan(context.Context, apps3.BuildSyncPlanInput) (domains3.SyncPlan, error) {
	return domains3.SyncPlan{}, nil
}

func (s *blockingS3Service) ExecuteSync(ctx context.Context, _ domains3.SyncPlan, _ chan<- domains3.SyncProgress) (domains3.SyncResult, error) {
	close(s.started)
	<-ctx.Done()
	close(s.canceled)
	return domains3.SyncResult{}, ctx.Err()
}

func TestResetWorkflowCancelsRunningSync(t *testing.T) {
	service := &blockingS3Service{started: make(chan struct{}), canceled: make(chan struct{})}
	page := NewS3Page(service)

	cmd := page.startSyncCmd(domains3.SyncPlan{Uploads: []domains3.SyncUpload{{Key: "index.html"}}})
	if cmd == nil {
		t.Fatal("expected sync command")
	}
	_ = cmd()

	select {
	case <-service.started:
	case <-time.After(time.Second):
		t.Fatal("sync did not start")
	}

	page.resetWorkflow()

	select {
	case <-service.canceled:
	case <-time.After(time.Second):
		t.Fatal("sync context was not cancelled")
	}
}

func TestEscCancelsRunningSync(t *testing.T) {
	service := &blockingS3Service{started: make(chan struct{}), canceled: make(chan struct{})}
	page := NewS3Page(service)
	cmd := page.startSyncCmd(domains3.SyncPlan{Uploads: []domains3.SyncUpload{{Key: "index.html"}}})
	_ = cmd()

	select {
	case <-service.started:
	case <-time.After(time.Second):
		t.Fatal("sync did not start")
	}

	page.stage = s3StageSync
	page.syncing = true
	page.updateSyncStage(tea.KeyMsg{Type: tea.KeyEsc}, pageapi.State{})

	select {
	case <-service.canceled:
	case <-time.After(time.Second):
		t.Fatal("esc did not cancel running sync")
	}
}
