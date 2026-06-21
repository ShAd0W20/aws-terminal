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

type cloudFrontNoopService struct{}

func (cloudFrontNoopService) ListDistributions(context.Context, string, string) ([]domaincloudfront.Distribution, error) {
	return nil, nil
}

func (cloudFrontNoopService) CreateInvalidation(context.Context, appcloudfront.CreateInvalidationInput) (domaincloudfront.Invalidation, error) {
	return domaincloudfront.Invalidation{}, nil
}

func (cloudFrontNoopService) GetInvalidation(context.Context, string, string, string, string) (domaincloudfront.Invalidation, error) {
	return domaincloudfront.Invalidation{}, nil
}

func TestResetCancelsPendingPollDelay(t *testing.T) {
	page := NewCloudFrontPage(cloudFrontNoopService{})
	state := pageapi.State{ActiveSession: &domainsession.Session{Profile: "test", Region: "us-east-1"}}

	cmd := page.pollInvalidationCmd(state, "DIST", "INV", time.Hour)
	if cmd == nil {
		t.Fatal("expected poll command")
	}

	msgCh := make(chan interface{}, 1)
	go func() { msgCh <- cmd() }()

	page.resetForSession()

	select {
	case msg := <-msgCh:
		polled, ok := msg.(cloudFrontInvalidationPolledMsg)
		if !ok {
			t.Fatalf("expected cloudFrontInvalidationPolledMsg, got %T", msg)
		}
		if polled.err == nil {
			t.Fatal("expected cancellation error")
		}
	case <-time.After(time.Second):
		t.Fatal("poll command did not unblock after cancellation")
	}
}

func TestEscCancelsPendingPollDelay(t *testing.T) {
	page := NewCloudFrontPage(cloudFrontNoopService{})
	state := pageapi.State{ActiveSession: &domainsession.Session{Profile: "test", Region: "us-east-1"}}
	cmd := page.pollInvalidationCmd(state, "DIST", "INV", time.Hour)

	msgCh := make(chan interface{}, 1)
	go func() { msgCh <- cmd() }()

	page.stage = cloudFrontStageResult
	page.creating = true
	page.updateResultStage(tea.KeyMsg{Type: tea.KeyEsc}, state)

	select {
	case msg := <-msgCh:
		polled, ok := msg.(cloudFrontInvalidationPolledMsg)
		if !ok {
			t.Fatalf("expected cloudFrontInvalidationPolledMsg, got %T", msg)
		}
		if polled.err == nil {
			t.Fatal("expected cancellation error")
		}
	case <-time.After(time.Second):
		t.Fatal("esc did not unblock polling command")
	}
	if page.creating {
		t.Fatal("expected creating to be false after esc")
	}
}
