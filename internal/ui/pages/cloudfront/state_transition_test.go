package cloudfront

import (
	"aws-terminal/internal/ui/pageapi"
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	appcloudfront "aws-terminal/internal/application/cloudfront"
	domaincloudfront "aws-terminal/internal/domain/cloudfront"
	domainsession "aws-terminal/internal/domain/session"
)

type staticCloudFrontService struct{}

func (staticCloudFrontService) ListDistributions(context.Context, string, string) ([]domaincloudfront.Distribution, error) {
	return []domaincloudfront.Distribution{{ID: "DIST1", DomainName: "example.cloudfront.net", Enabled: true}}, nil
}

func (staticCloudFrontService) CreateInvalidation(context.Context, appcloudfront.CreateInvalidationInput) (domaincloudfront.Invalidation, error) {
	return domaincloudfront.Invalidation{ID: "INV1", DistributionID: "DIST1", Status: "InProgress"}, nil
}

func (staticCloudFrontService) GetInvalidation(context.Context, string, string, string, string) (domaincloudfront.Invalidation, error) {
	return domaincloudfront.Invalidation{ID: "INV1", DistributionID: "DIST1", Status: "Completed"}, nil
}

func TestSessionChangeResetsAndStartsDistributionLoad(t *testing.T) {
	page := NewCloudFrontPage(staticCloudFrontService{})
	page.stage = cloudFrontStageResult
	page.invalidation = &domaincloudfront.Invalidation{ID: "old"}
	page.selectedDistribution = domaincloudfront.Distribution{ID: "old-dist"}

	cmd := page.OnStateChanged(pageapi.State{ActiveSession: &domainsession.Session{Profile: "profile-a", Region: "us-east-1"}})
	if cmd == nil {
		t.Fatal("expected distribution load command")
	}
	if page.stage != cloudFrontStageDistribution {
		t.Fatalf("expected distribution stage, got %v", page.stage)
	}
	if page.invalidation != nil {
		t.Fatal("expected invalidation reset")
	}
	if page.selectedDistribution.ID != "" {
		t.Fatalf("expected selected distribution reset, got %q", page.selectedDistribution.ID)
	}
	if !page.loading {
		t.Fatal("expected loading state")
	}
}

func TestDistributionEnterMovesToPathsStage(t *testing.T) {
	page := NewCloudFrontPage(staticCloudFrontService{})
	page.distributions = []domaincloudfront.Distribution{{ID: "DIST1"}, {ID: "DIST2"}}
	page.distributionIndex = 1

	cmd := page.updateDistributionStage(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected paths input focus command")
	}
	if page.stage != cloudFrontStagePaths {
		t.Fatalf("expected paths stage, got %v", page.stage)
	}
	if page.selectedDistribution.ID != "DIST2" {
		t.Fatalf("expected DIST2 selected, got %q", page.selectedDistribution.ID)
	}
}

func TestInvalidationLifecycleTransitionsToCompleted(t *testing.T) {
	page := NewCloudFrontPage(staticCloudFrontService{})
	state := pageapi.State{ActiveSession: &domainsession.Session{Profile: "profile-a", Region: "us-east-1"}}

	cmd := page.Update(cloudFrontInvalidationCreatedMsg{invalidation: domaincloudfront.Invalidation{ID: "INV1", DistributionID: "DIST1", Status: "InProgress", CreatedAt: time.Now()}}, state)
	if cmd == nil {
		t.Fatal("expected spinner/poll command after in-progress invalidation")
	}
	if page.stage != cloudFrontStageResult {
		t.Fatalf("expected result stage, got %v", page.stage)
	}
	if !page.creating {
		t.Fatal("expected creating while invalidation is in progress")
	}

	cmd = page.Update(cloudFrontInvalidationPolledMsg{invalidation: domaincloudfront.Invalidation{ID: "INV1", DistributionID: "DIST1", Status: "Completed"}}, state)
	if cmd != nil {
		t.Fatalf("expected no command after completed invalidation, got %T", cmd)
	}
	if page.creating {
		t.Fatal("expected creating false after completion")
	}
	if page.invalidation == nil || page.invalidation.Status != "Completed" {
		t.Fatalf("expected completed invalidation stored, got %#v", page.invalidation)
	}
}
