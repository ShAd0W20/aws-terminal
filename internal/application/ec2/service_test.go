package ec2

import (
	"context"
	"reflect"
	"testing"
	"time"

	domainec2 "aws-terminal/internal/domain/ec2"
)

type fakeEC2API struct {
	profile   string
	region    string
	instances []domainec2.Instance
	stopInput domainec2.StopInstanceInput
	termInput domainec2.TerminateInstanceInput
}

func (f *fakeEC2API) ListInstances(ctx context.Context, profileName, region string) ([]domainec2.Instance, error) {
	f.profile = profileName
	f.region = region
	return append([]domainec2.Instance(nil), f.instances...), nil
}
func (f *fakeEC2API) StopInstance(ctx context.Context, input domainec2.StopInstanceInput) (domainec2.StopInstanceResult, error) {
	f.stopInput = input
	return domainec2.StopInstanceResult{Instance: domainec2.Instance{ID: input.InstanceID, State: "stopping"}}, nil
}
func (f *fakeEC2API) TerminateInstance(ctx context.Context, input domainec2.TerminateInstanceInput) (domainec2.TerminateInstanceResult, error) {
	f.termInput = input
	return domainec2.TerminateInstanceResult{Instance: domainec2.Instance{ID: input.InstanceID, State: "shutting-down"}}, nil
}

func TestListInstancesValidatesProfileAndRegion(t *testing.T) {
	svc := NewService(&fakeEC2API{})
	if _, err := svc.ListInstances(context.Background(), " ", "eu-west-1"); err == nil {
		t.Fatal("expected missing profile error")
	}
	if _, err := svc.ListInstances(context.Background(), "dev", " "); err == nil {
		t.Fatal("expected missing region error")
	}
}

func TestListInstancesFiltersTerminatedAndSorts(t *testing.T) {
	now := time.Now()
	api := &fakeEC2API{instances: []domainec2.Instance{
		{ID: "i-stopped", Name: "zeta", State: "stopped", LaunchTime: now.Add(-time.Hour)},
		{ID: "i-terminated", Name: "gone", State: "terminated", LaunchTime: now},
		{ID: "i-running", Name: "api", State: "running", LaunchTime: now.Add(-2 * time.Hour)},
	}}
	svc := NewService(api)
	instances, err := svc.ListInstances(context.Background(), " dev ", " eu-west-1 ")
	if err != nil {
		t.Fatal(err)
	}
	if api.profile != "dev" || api.region != "eu-west-1" {
		t.Fatalf("profile/region not trimmed: %q/%q", api.profile, api.region)
	}
	got := []string{instances[0].ID, instances[1].ID}
	if !reflect.DeepEqual(got, []string{"i-running", "i-stopped"}) {
		t.Fatalf("unexpected instances/order: %#v", instances)
	}
}

func TestActionsTrimAndValidateInstanceID(t *testing.T) {
	api := &fakeEC2API{}
	svc := NewService(api)
	if _, err := svc.StopInstance(context.Background(), domainec2.StopInstanceInput{ProfileName: "dev", Region: "eu-west-1"}); err == nil {
		t.Fatal("expected missing instance ID error")
	}
	if _, err := svc.StopInstance(context.Background(), domainec2.StopInstanceInput{ProfileName: " dev ", Region: " eu-west-1 ", InstanceID: " i-123 "}); err != nil {
		t.Fatal(err)
	}
	if api.stopInput.ProfileName != "dev" || api.stopInput.Region != "eu-west-1" || api.stopInput.InstanceID != "i-123" {
		t.Fatalf("stop input not trimmed: %#v", api.stopInput)
	}
	if _, err := svc.TerminateInstance(context.Background(), domainec2.TerminateInstanceInput{ProfileName: " dev ", Region: " eu-west-1 ", InstanceID: " i-123 "}); err != nil {
		t.Fatal(err)
	}
	if api.termInput.InstanceID != "i-123" {
		t.Fatalf("terminate input not trimmed: %#v", api.termInput)
	}
}
