package sqs

import (
	"context"
	"reflect"
	"testing"

	domainsqs "aws-terminal/internal/domain/sqs"
)

type fakeQueueAPI struct {
	profile   string
	region    string
	queues    []domainsqs.Queue
	messages  []domainsqs.Message
	purged    bool
	lastInput QueueActionInput
}

func (f *fakeQueueAPI) ListQueues(ctx context.Context, profileName, region string) ([]domainsqs.Queue, error) {
	f.profile = profileName
	f.region = region
	return append([]domainsqs.Queue(nil), f.queues...), nil
}
func (f *fakeQueueAPI) ReceiveMessages(ctx context.Context, input QueueActionInput) ([]domainsqs.Message, error) {
	f.profile = input.Profile
	f.region = input.Region
	f.lastInput = input
	return append([]domainsqs.Message(nil), f.messages...), nil
}
func (f *fakeQueueAPI) PurgeQueue(ctx context.Context, input QueueActionInput) error {
	f.profile = input.Profile
	f.region = input.Region
	f.lastInput = input
	f.purged = true
	return nil
}

func TestListQueuesValidatesProfileAndRegion(t *testing.T) {
	svc := NewService(&fakeQueueAPI{})
	if _, err := svc.ListQueues(context.Background(), " ", "us-east-1"); err == nil {
		t.Fatal("expected missing profile error")
	}
	if _, err := svc.ListQueues(context.Background(), "dev", " "); err == nil {
		t.Fatal("expected missing region error")
	}
}

func TestQueueActionsValidateQueue(t *testing.T) {
	svc := NewService(&fakeQueueAPI{})
	if _, err := svc.ReceiveMessages(context.Background(), QueueActionInput{Profile: "dev", Region: "us-east-1", Queue: domainsqs.Queue{Name: "q"}}); err == nil {
		t.Fatal("expected missing queue URL error")
	}
	if err := svc.PurgeQueue(context.Background(), QueueActionInput{Profile: "dev", Region: "us-east-1", Queue: domainsqs.Queue{Name: "q", URL: "url"}}); err != nil {
		t.Fatal(err)
	}
}

func TestQueueActionsTrimBeforeDelegating(t *testing.T) {
	api := &fakeQueueAPI{}
	svc := NewService(api)
	input := QueueActionInput{Profile: " dev ", Region: " eu-west-1 ", Queue: domainsqs.Queue{Name: " orders ", URL: " url "}, MaxCount: 3}
	if _, err := svc.ReceiveMessages(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if api.lastInput.Profile != "dev" || api.lastInput.Region != "eu-west-1" || api.lastInput.Queue.Name != "orders" || api.lastInput.Queue.URL != "url" {
		t.Fatalf("input not trimmed: %#v", api.lastInput)
	}
	if api.lastInput.MaxCount != 3 {
		t.Fatalf("max count changed: %d", api.lastInput.MaxCount)
	}
}

func TestReceiveMessagesDefaultsInvalidMaxCount(t *testing.T) {
	for _, maxCount := range []int32{0, -1, 11} {
		api := &fakeQueueAPI{}
		svc := NewService(api)
		_, err := svc.ReceiveMessages(context.Background(), QueueActionInput{Profile: "dev", Region: "eu-west-1", Queue: domainsqs.Queue{Name: "orders", URL: "url"}, MaxCount: maxCount})
		if err != nil {
			t.Fatal(err)
		}
		if api.lastInput.MaxCount != 10 {
			t.Fatalf("MaxCount %d defaulted to %d, want 10", maxCount, api.lastInput.MaxCount)
		}
	}
}

func TestListQueuesTrimsAndSortsByName(t *testing.T) {
	api := &fakeQueueAPI{queues: []domainsqs.Queue{{Name: "zeta"}, {Name: "Alpha"}, {Name: "beta"}}}
	svc := NewService(api)
	queues, err := svc.ListQueues(context.Background(), " dev ", " eu-west-1 ")
	if err != nil {
		t.Fatal(err)
	}
	if api.profile != "dev" || api.region != "eu-west-1" {
		t.Fatalf("profile/region not trimmed: %q/%q", api.profile, api.region)
	}
	got := []string{queues[0].Name, queues[1].Name, queues[2].Name}
	if !reflect.DeepEqual(got, []string{"Alpha", "beta", "zeta"}) {
		t.Fatalf("unexpected order: %v", got)
	}
}
