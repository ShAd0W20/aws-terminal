package ecs

import (
	"context"
	"reflect"
	"testing"
	"time"

	domainecs "aws-terminal/internal/domain/ecs"
)

type fakeAPI struct {
	clusters      []domainecs.Cluster
	services      []domainecs.Service
	tasks         []domainecs.Task
	fetchLookback time.Duration
	fetchLimit    int32
}

func (f fakeAPI) ListClusters(context.Context, string, string) ([]domainecs.Cluster, error) {
	return append([]domainecs.Cluster(nil), f.clusters...), nil
}
func (f fakeAPI) ListServices(context.Context, string, string, string) ([]domainecs.Service, error) {
	return append([]domainecs.Service(nil), f.services...), nil
}
func (f fakeAPI) ListTasks(context.Context, string, string, string) ([]domainecs.Task, error) {
	return append([]domainecs.Task(nil), f.tasks...), nil
}
func (f fakeAPI) DescribeTaskLogTargets(context.Context, string, string, string, string) ([]domainecs.LogTarget, error) {
	return []domainecs.LogTarget{{ContainerName: "app", Supported: true, LogGroup: "group", LogStream: "prefix/app/task"}}, nil
}
func (f *fakeAPI) FetchTaskLogEvents(_ context.Context, _ string, _ string, _ domainecs.LogTarget, _ string, lookback time.Duration, limit int32) (domainecs.LogEventsPage, error) {
	f.fetchLookback = lookback
	f.fetchLimit = limit
	return domainecs.LogEventsPage{}, nil
}

func TestListClustersValidatesProfileAndSortsActiveFirst(t *testing.T) {
	svc := NewService(&fakeAPI{clusters: []domainecs.Cluster{{Name: "z", Status: "INACTIVE"}, {Name: "b", Status: "ACTIVE"}, {Name: "a", Status: "ACTIVE"}}})
	if _, err := svc.ListClusters(context.Background(), " ", "eu-west-1"); err == nil {
		t.Fatal("expected profile validation error")
	}
	got, err := svc.ListClusters(context.Background(), "dev", "eu-west-1")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{got[0].Name, got[1].Name, got[2].Name}
	if !reflect.DeepEqual(names, []string{"a", "b", "z"}) {
		t.Fatalf("unexpected order: %v", names)
	}
}

func TestListServicesValidatesInputsAndSortsActiveFirst(t *testing.T) {
	svc := NewService(&fakeAPI{services: []domainecs.Service{{Name: "z", Status: "DRAINING"}, {Name: "b", Status: "ACTIVE"}, {Name: "a", Status: "ACTIVE"}}})
	if _, err := svc.ListServices(context.Background(), "dev", "eu-west-1", " "); err == nil {
		t.Fatal("expected cluster ARN validation error")
	}
	got, err := svc.ListServices(context.Background(), "dev", "eu-west-1", "cluster")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{got[0].Name, got[1].Name, got[2].Name}
	if !reflect.DeepEqual(names, []string{"a", "b", "z"}) {
		t.Fatalf("unexpected order: %v", names)
	}
}

func TestDescribeTaskLogTargetsValidatesInputs(t *testing.T) {
	svc := NewService(&fakeAPI{})
	if _, err := svc.DescribeTaskLogTargets(context.Background(), " ", "eu-west-1", "td", "task"); err == nil {
		t.Fatal("expected profile validation error")
	}
	if _, err := svc.DescribeTaskLogTargets(context.Background(), "dev", "eu-west-1", " ", "task"); err == nil {
		t.Fatal("expected task definition validation error")
	}
	if _, err := svc.DescribeTaskLogTargets(context.Background(), "dev", "eu-west-1", "td", " "); err == nil {
		t.Fatal("expected task ID validation error")
	}
}

func TestFetchTaskLogEventsValidatesTargetAndCapsLimit(t *testing.T) {
	api := &fakeAPI{}
	svc := NewService(api)
	if _, err := svc.FetchTaskLogEvents(context.Background(), "dev", "eu-west-1", domainecs.LogTarget{Supported: false}, "", 0, 0); err == nil {
		t.Fatal("expected unsupported target validation error")
	}
	if _, err := svc.FetchTaskLogEvents(context.Background(), "dev", "eu-west-1", domainecs.LogTarget{Supported: true, LogGroup: "group", LogStream: "stream"}, "", 0, 999); err != nil {
		t.Fatalf("unexpected valid fetch error: %v", err)
	}
	if api.fetchLookback != 15*time.Minute {
		t.Fatalf("lookback = %s, want 15m", api.fetchLookback)
	}
	if api.fetchLimit != 500 {
		t.Fatalf("limit = %d, want 500", api.fetchLimit)
	}
}

func TestListTasksFiltersStoppedAndSortsNonRunningNewestFirst(t *testing.T) {
	older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	svc := NewService(&fakeAPI{tasks: []domainecs.Task{{ID: "run", LastStatus: "RUNNING", CreatedAt: newer}, {ID: "stop", LastStatus: "STOPPED", CreatedAt: newer}, {ID: "pend-old", LastStatus: "PENDING", CreatedAt: older}, {ID: "pend-new", LastStatus: "PENDING", CreatedAt: newer}}})
	got, err := svc.ListTasks(context.Background(), "dev", "eu-west-1", "cluster")
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(got))
	for i := range got {
		ids[i] = got[i].ID
	}
	want := []string{"pend-new", "pend-old", "run"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("got %v want %v", ids, want)
	}
}
