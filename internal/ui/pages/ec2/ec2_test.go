package ec2

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	domainec2 "aws-terminal/internal/domain/ec2"
	domainsession "aws-terminal/internal/domain/session"
	"aws-terminal/internal/ui/pageapi"
)

type fakeEC2Service struct {
	instances      []domainec2.Instance
	stopCalls      int
	terminateCalls int
}

func (f *fakeEC2Service) ListInstances(ctx context.Context, profileName, region string) ([]domainec2.Instance, error) {
	return append([]domainec2.Instance(nil), f.instances...), nil
}
func (f *fakeEC2Service) StopInstance(ctx context.Context, input domainec2.StopInstanceInput) (domainec2.StopInstanceResult, error) {
	f.stopCalls++
	return domainec2.StopInstanceResult{Instance: domainec2.Instance{ID: input.InstanceID, State: "stopping"}}, nil
}
func (f *fakeEC2Service) TerminateInstance(ctx context.Context, input domainec2.TerminateInstanceInput) (domainec2.TerminateInstanceResult, error) {
	f.terminateCalls++
	return domainec2.TerminateInstanceResult{Instance: domainec2.Instance{ID: input.InstanceID, State: "shutting-down"}}, nil
}

func ec2TestState() pageapi.State {
	return pageapi.State{ActiveSession: &domainsession.Session{Profile: "dev", Region: "eu-west-1", Account: "123"}, SelectedRegion: "eu-west-1", PageFocused: true}
}

func TestPageLoadsAndRendersInstances(t *testing.T) {
	service := &fakeEC2Service{instances: []domainec2.Instance{{ID: "i-123", Name: "api", State: "running", Type: "t3.micro", AvailabilityZone: "eu-west-1a", PrivateIP: "10.0.0.1"}}}
	page := NewEC2Page(service)
	state := ec2TestState()
	cmd := page.OnStateChanged(state)
	if cmd == nil {
		t.Fatal("expected load command")
	}
	page.Update(instancesLoadedMsg{sessionKey: page.sessionKey, instances: service.instances}, state)
	view := page.View(state, 140, 35)
	for _, want := range []string{"api", "i-123", "running", "t3.micro", "terminated instances are hidden"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestSearchFiltersInstances(t *testing.T) {
	page := NewEC2Page(&fakeEC2Service{})
	state := ec2TestState()
	page.instances = []domainec2.Instance{{ID: "i-api", Name: "api"}, {ID: "i-worker", Name: "worker"}}
	page.Update(tea.KeyMsg{Type: tea.KeyCtrlF}, state)
	page.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("worker")}, state)
	view := page.View(state, 120, 30)
	if !strings.Contains(view, "worker") || strings.Contains(view, "i-api") {
		t.Fatalf("unexpected filtered view:\n%s", view)
	}
}

func TestOpenDetailStopReviewAndSuccess(t *testing.T) {
	service := &fakeEC2Service{}
	page := NewEC2Page(service)
	state := ec2TestState()
	page.instances = []domainec2.Instance{{ID: "i-123", Name: "api", State: "running"}}
	page.syncTable()
	page.Update(tea.KeyMsg{Type: tea.KeyEnter}, state)
	if page.stage != ec2StageInstanceDetail {
		t.Fatalf("stage=%v", page.stage)
	}
	page.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}, state)
	if page.stage != ec2StageStopReview {
		t.Fatalf("expected stop review, stage=%v", page.stage)
	}
	cmd := page.Update(tea.KeyMsg{Type: tea.KeyEnter}, state)
	if cmd == nil || !page.stopping || page.stage != ec2StageStopping {
		t.Fatalf("expected stop command, stage=%v stopping=%v", page.stage, page.stopping)
	}
	page.Update(instanceStoppedMsg{sessionKey: page.sessionKey, instanceID: "i-123", result: domainec2.StopInstanceResult{Instance: domainec2.Instance{ID: "i-123", State: "stopping"}}}, state)
	if page.stage != ec2StageInstanceDetail || page.selected.State != "stopping" || page.actionMessage == "" {
		t.Fatalf("unexpected stop success: stage=%v selected=%#v message=%q", page.stage, page.selected, page.actionMessage)
	}
}

func TestTerminateRequiresExactInstanceID(t *testing.T) {
	page := NewEC2Page(&fakeEC2Service{})
	state := ec2TestState()
	page.stage = ec2StageInstanceDetail
	page.selected = domainec2.Instance{ID: "i-123", Name: "api", State: "stopped"}
	page.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}, state)
	if page.stage != ec2StageTerminateConfirm || !page.terminateInput.Focused() {
		t.Fatalf("expected terminate confirm, stage=%v focused=%v", page.stage, page.terminateInput.Focused())
	}
	view := page.View(state, 120, 35)
	if !strings.Contains(view, "permanently terminates") || !strings.Contains(view, "DeleteOnTermination") {
		t.Fatalf("warning missing:\n%s", view)
	}
	page.terminateInput.SetValue("wrong")
	cmd := page.Update(tea.KeyMsg{Type: tea.KeyEnter}, state)
	if cmd != nil || page.actionErr == "" {
		t.Fatal("expected mismatch error without command")
	}
	page.terminateInput.SetValue("i-123")
	cmd = page.Update(tea.KeyMsg{Type: tea.KeyEnter}, state)
	if cmd == nil || !page.terminating || page.stage != ec2StageTerminating {
		t.Fatalf("expected terminate command, stage=%v terminating=%v", page.stage, page.terminating)
	}
}

func TestStaleActionResultsAreIgnored(t *testing.T) {
	page := NewEC2Page(&fakeEC2Service{})
	page.sessionKey = "current"
	page.selected = domainec2.Instance{ID: "i-current", State: "running"}
	page.stopping = true
	page.Update(instanceStoppedMsg{sessionKey: "old", instanceID: "i-current", result: domainec2.StopInstanceResult{Instance: domainec2.Instance{ID: "i-current", State: "stopping"}}}, ec2TestState())
	page.Update(instanceStoppedMsg{sessionKey: "current", instanceID: "i-other", result: domainec2.StopInstanceResult{Instance: domainec2.Instance{ID: "i-other", State: "stopping"}}}, ec2TestState())
	if page.selected.State != "running" || page.actionMessage != "" {
		t.Fatalf("stale action applied: selected=%#v message=%q", page.selected, page.actionMessage)
	}
}

func TestPageStatusReportsActivityAndErrors(t *testing.T) {
	page := NewEC2Page(&fakeEC2Service{})
	page.terminating = true
	if got := page.PageStatus(pageapi.State{}).Message; got != "Terminating EC2 instance..." {
		t.Fatalf("status message=%q", got)
	}
	page.actionErr = "boom"
	if got := page.PageStatus(pageapi.State{}).Error; got != "boom" {
		t.Fatalf("status error=%q", got)
	}
}
