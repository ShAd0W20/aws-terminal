package sqs

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	appsqs "aws-terminal/internal/application/sqs"
	domainsession "aws-terminal/internal/domain/session"
	domainsqs "aws-terminal/internal/domain/sqs"
)

type fakeSQSService struct {
	queues       []domainsqs.Queue
	messages     []domainsqs.Message
	calls        int
	receiveCalls int
	purgeCalls   int
}

func (f *fakeSQSService) ListQueues(ctx context.Context, profileName, region string) ([]domainsqs.Queue, error) {
	f.calls++
	return append([]domainsqs.Queue(nil), f.queues...), nil
}
func (f *fakeSQSService) ReceiveMessages(ctx context.Context, input appsqs.QueueActionInput) ([]domainsqs.Message, error) {
	f.receiveCalls++
	return append([]domainsqs.Message(nil), f.messages...), nil
}
func (f *fakeSQSService) PurgeQueue(ctx context.Context, input appsqs.QueueActionInput) error {
	f.purgeCalls++
	return nil
}

func sqsTestState() State {
	return State{ActiveSession: &domainsession.Session{Profile: "dev", Region: "eu-west-1", Account: "123"}, SelectedRegion: "eu-west-1", PageFocused: true}
}

func TestPageLoadsOnStateChangeAndRendersCounts(t *testing.T) {
	service := &fakeSQSService{queues: []domainsqs.Queue{{Name: "orders", URL: "url", ARN: "arn", AvailableMessages: 12, InFlightMessages: 3}}}
	page := NewSQSPage(service)
	state := sqsTestState()
	cmd := page.OnStateChanged(state)
	if cmd == nil {
		t.Fatal("expected load command")
	}
	page.Update(queuesLoadedMsg{sessionKey: page.sessionKey, queues: service.queues}, state)
	view := page.View(state, 120, 30)
	for _, want := range []string{"orders", "12", "3", "Available", "In flight"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "url") || strings.Contains(view, "arn") {
		t.Fatalf("url/arn should not be rendered in initial table:\n%s", view)
	}
}

func TestSearchFiltersQueues(t *testing.T) {
	page := NewSQSPage(&fakeSQSService{})
	state := sqsTestState()
	page.queues = []domainsqs.Queue{{Name: "orders"}, {Name: "payments"}}
	page.Update(tea.KeyMsg{Type: tea.KeyCtrlF}, state)
	page.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pay")}, state)
	view := page.View(state, 120, 30)
	if !strings.Contains(view, "payments") {
		t.Fatalf("filtered queue missing:\n%s", view)
	}
	if strings.Contains(view, "orders") {
		t.Fatalf("unmatched queue rendered:\n%s", view)
	}
}

func TestPullMessagesIsViewOnly(t *testing.T) {
	service := &fakeSQSService{queues: []domainsqs.Queue{{Name: "orders", URL: "url"}}, messages: []domainsqs.Message{{ID: "m1", Body: "hello world", ReceiptHandle: "receipt"}}}
	page := NewSQSPage(service)
	state := sqsTestState()
	page.queues = service.queues
	page.selectedQueue = service.queues[0]
	page.stage = sqsStageQueueActions

	cmd := page.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}, state)
	if cmd == nil || !page.messagesLoading {
		t.Fatal("expected pull command and loading state")
	}
	page.Update(messagesLoadedMsg{queueName: "orders", messages: service.messages}, state)
	if page.stage != sqsStageMessages {
		t.Fatalf("stage=%v", page.stage)
	}
	view := page.View(state, 120, 40)
	if !strings.Contains(view, "view only") || !strings.Contains(view, "hello world") {
		t.Fatalf("unexpected message view:\n%s", view)
	}
	if strings.Contains(view, "receipt") {
		t.Fatalf("receipt handle should not be rendered:\n%s", view)
	}
}

func TestPurgeRequiresQueueNameConfirmation(t *testing.T) {
	service := &fakeSQSService{queues: []domainsqs.Queue{{Name: "orders", URL: "url"}}}
	page := NewSQSPage(service)
	state := sqsTestState()
	page.selectedQueue = service.queues[0]
	page.stage = sqsStageQueueActions
	page.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, state)
	if page.stage != sqsStagePurgeConfirm || !page.purgeInput.Focused() {
		t.Fatalf("expected purge confirm stage, stage=%v focused=%v", page.stage, page.purgeInput.Focused())
	}
	page.purgeInput.SetValue("wrong")
	cmd := page.Update(tea.KeyMsg{Type: tea.KeyEnter}, state)
	if cmd != nil || page.purgeErr == "" {
		t.Fatal("expected confirmation mismatch without purge command")
	}
	page.purgeInput.SetValue("orders")
	cmd = page.Update(tea.KeyMsg{Type: tea.KeyEnter}, state)
	if cmd == nil || !page.purging {
		t.Fatal("expected purge command")
	}
	page.Update(queuePurgedMsg{queueName: "orders"}, state)
	if page.stage != sqsStageQueueActions || page.purgeMessage == "" {
		t.Fatalf("expected action stage with purge message, stage=%v message=%q", page.stage, page.purgeMessage)
	}
}

func TestRefreshLoadsAgain(t *testing.T) {
	service := &fakeSQSService{queues: []domainsqs.Queue{{Name: "orders"}}}
	page := NewSQSPage(service)
	state := sqsTestState()
	cmd := page.OnStateChanged(state)
	if cmd == nil {
		t.Fatal("expected initial load command")
	}
	service.calls++
	page.Update(queuesLoadedMsg{sessionKey: page.sessionKey, queues: service.queues}, state)
	cmd = page.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}, state)
	if cmd == nil {
		t.Fatal("expected refresh command")
	}
	if !page.loading {
		t.Fatal("expected page to enter loading state on refresh")
	}
	page.Update(queuesLoadedMsg{sessionKey: page.sessionKey, queues: service.queues}, state)
	if page.loading {
		t.Fatal("expected loaded message to clear loading state")
	}
}
