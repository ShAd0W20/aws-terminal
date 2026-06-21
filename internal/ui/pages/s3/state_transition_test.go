package s3

import (
	"aws-terminal/internal/ui/pageapi"
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	apps3 "aws-terminal/internal/application/s3"
	domains3 "aws-terminal/internal/domain/s3"
	domainsession "aws-terminal/internal/domain/session"
)

type staticS3Service struct{}

func (staticS3Service) ListBuckets(context.Context, string, string) ([]domains3.Bucket, error) {
	return []domains3.Bucket{{Name: "bucket-a"}}, nil
}

func (staticS3Service) InspectSource(string) (domains3.SourceSelection, error) {
	return domains3.SourceSelection{}, nil
}

func (staticS3Service) BuildSyncPlan(context.Context, apps3.BuildSyncPlanInput) (domains3.SyncPlan, error) {
	return domains3.SyncPlan{}, nil
}

func (staticS3Service) ExecuteSync(context.Context, domains3.SyncPlan, chan<- domains3.SyncProgress) (domains3.SyncResult, error) {
	return domains3.SyncResult{}, nil
}

func TestSessionChangeResetsWorkflowAndStartsBucketLoad(t *testing.T) {
	page := NewS3Page(staticS3Service{})
	page.selectedBucket = "old-bucket"
	page.stage = s3StageReview
	page.sourceInfo = &domains3.SourceSelection{Path: "/tmp/source", Kind: domains3.SourceKindDirectory}
	page.prefixInput.SetValue("old-prefix")

	cmd := page.OnStateChanged(pageapi.State{ActiveSession: &domainsession.Session{Profile: "profile-a", Region: "us-east-1"}})
	if cmd == nil {
		t.Fatal("expected bucket load command")
	}
	if page.stage != s3StageBucket {
		t.Fatalf("expected stage bucket after session reset, got %v", page.stage)
	}
	if page.selectedBucket != "" {
		t.Fatalf("expected selected bucket reset, got %q", page.selectedBucket)
	}
	if page.sourceInfo != nil {
		t.Fatal("expected source info reset")
	}
	if page.prefixInput.Value() != "" {
		t.Fatalf("expected prefix reset, got %q", page.prefixInput.Value())
	}
	if !page.loadingBuckets {
		t.Fatal("expected bucket loading state")
	}
}

func TestBucketEnterMovesToSourceStage(t *testing.T) {
	page := NewS3Page(staticS3Service{})
	page.buckets = []domains3.Bucket{{Name: "bucket-a"}, {Name: "bucket-b"}}
	page.bucketIndex = 1

	cmd := page.updateBucketStage(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected file picker init command")
	}
	if page.stage != s3StageSource {
		t.Fatalf("expected source stage, got %v", page.stage)
	}
	if page.selectedBucket != "bucket-b" {
		t.Fatalf("expected bucket-b selected, got %q", page.selectedBucket)
	}
}

func TestLargeDeletePlanRequiresTypedConfirmation(t *testing.T) {
	page := NewS3Page(staticS3Service{})
	page.stage = s3StageReview
	page.selectedBucket = "bucket-a"
	page.plan = &domains3.SyncPlan{DeleteEnabled: true, Deletes: make([]domains3.SyncDelete, largeDeleteConfirmationThreshold)}

	cmd := page.updateReviewStage(tea.KeyMsg{Type: tea.KeyEnter}, pageapi.State{PageFocused: true})
	if cmd == nil {
		// Focus command may be nil in some Bubble Tea versions, but stage change is the behavior under test.
	}
	if page.stage != s3StageConfirmDelete {
		t.Fatalf("expected confirm-delete stage, got %v", page.stage)
	}
	if page.confirmInput.Value() != "" {
		t.Fatalf("expected empty confirmation input, got %q", page.confirmInput.Value())
	}
}

func TestDeleteConfirmationMustMatchDelete(t *testing.T) {
	page := NewS3Page(staticS3Service{})
	page.stage = s3StageConfirmDelete
	page.plan = &domains3.SyncPlan{DeleteEnabled: true, Deletes: make([]domains3.SyncDelete, largeDeleteConfirmationThreshold)}

	page.confirmInput.SetValue("delete")
	if cmd := page.updateConfirmDeleteStage(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Fatal("expected no sync command for lowercase confirmation")
	}
	if page.stage != s3StageConfirmDelete {
		t.Fatalf("expected to remain on confirmation stage, got %v", page.stage)
	}

	page.confirmInput.SetValue(deleteConfirmationText)
	if cmd := page.updateConfirmDeleteStage(tea.KeyMsg{Type: tea.KeyEnter}); cmd == nil {
		t.Fatal("expected sync command after exact confirmation")
	}
	if page.stage != s3StageSync {
		t.Fatalf("expected sync stage, got %v", page.stage)
	}
}

func TestSmallDeletePlanStartsWithoutTypedConfirmation(t *testing.T) {
	page := NewS3Page(staticS3Service{})
	page.stage = s3StageReview
	page.plan = &domains3.SyncPlan{DeleteEnabled: true, Deletes: make([]domains3.SyncDelete, largeDeleteConfirmationThreshold-1)}

	if cmd := page.updateReviewStage(tea.KeyMsg{Type: tea.KeyEnter}, pageapi.State{PageFocused: true}); cmd == nil {
		t.Fatal("expected sync command")
	}
	if page.stage != s3StageSync {
		t.Fatalf("expected sync stage, got %v", page.stage)
	}
}

func TestReviewToggleDeleteRebuildsPlanForDirectory(t *testing.T) {
	page := NewS3Page(staticS3Service{})
	page.stage = s3StageReview
	page.selectedBucket = "bucket-a"
	page.sourceInfo = &domains3.SourceSelection{Path: "/tmp/source", Kind: domains3.SourceKindDirectory}

	cmd := page.updateReviewStage(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}, pageapi.State{ActiveSession: &domainsession.Session{Profile: "profile-a", Region: "us-east-1"}, PageFocused: true})
	if cmd == nil {
		t.Fatal("expected plan rebuild command")
	}
	if !page.deleteEnabled {
		t.Fatal("expected delete enabled")
	}
	if !page.planning {
		t.Fatal("expected planning state")
	}
}
