package ecs

import (
	"strings"
	"testing"

	domainecs "aws-terminal/internal/domain/ecs"
)

func TestShortImage(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  string
	}{
		{name: "ecr image", image: "180294211964.dkr.ecr.eu-west-1.amazonaws.com/pricing-strategies-task:4.1.2", want: "pricing-strategies-task:4.1.2"},
		{name: "plain image", image: "redis:7", want: "redis:7"},
		{name: "empty image", image: "", want: "—"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortImage(tt.image); got != tt.want {
				t.Fatalf("shortImage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestServiceAttentionReason(t *testing.T) {
	tests := []struct {
		name string
		svc  domainecs.Service
		want string
	}{
		{name: "healthy", svc: domainecs.Service{Status: "ACTIVE", DesiredCount: 1, RunningCount: 1}, want: ""},
		{name: "below desired", svc: domainecs.Service{Status: "ACTIVE", DesiredCount: 2, RunningCount: 1}, want: "Fewer running tasks than desired"},
		{name: "pending", svc: domainecs.Service{Status: "ACTIVE", DesiredCount: 1, RunningCount: 1, PendingCount: 1}, want: "Tasks are pending"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serviceAttentionReason(tt.svc); got != tt.want {
				t.Fatalf("serviceAttentionReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTaskDetailSurfacesStoppedReasonAndShortImage(t *testing.T) {
	p := NewECSPage(fakeECSService{})
	p.selectedTask = domainecs.Task{
		ID:            "task-1",
		LastStatus:    "STOPPED",
		HealthStatus:  "UNHEALTHY",
		PrivateIP:     "10.0.19.114",
		StoppedReason: "Essential container exited",
		Containers: []domainecs.Container{{
			Name:       "api",
			Image:      "123.dkr.ecr.eu-west-1.amazonaws.com/api:1.2.3",
			LastStatus: "STOPPED",
		}},
	}

	view := strings.Join(p.taskDetailLines(), "\n")
	if !strings.Contains(view, "Essential container exited") {
		t.Fatalf("task detail should include stopped reason near the top:\n%s", view)
	}
	if !strings.Contains(view, "api:1.2.3") {
		t.Fatalf("task detail should include shortened image:\n%s", view)
	}
	if strings.Contains(view, "123.dkr.ecr") {
		t.Fatalf("task detail should not show full image URL in container overview:\n%s", view)
	}
}
