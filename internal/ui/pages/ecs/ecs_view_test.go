package ecs

import (
	"strings"
	"testing"
	"time"

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

func TestRenderLogEventWrapsAndDetectsSeverity(t *testing.T) {
	line := renderLogEvent("12:00:00", "level=error something very long that should wrap in a tiny viewport", 30)
	if !strings.Contains(line, "12:00") || !strings.Contains(line, "something") {
		t.Fatalf("rendered line missing timestamp/message: %q", line)
	}
	if !strings.Contains(line, "\n") {
		t.Fatalf("expected wrapped log line: %q", line)
	}
	if got := detectLogSeverity("this mentions error later"); got != "" {
		t.Fatalf("severity should not match arbitrary later words, got %q", got)
	}
	if got := detectLogSeverity("[WARN] heads up"); got != "warn" {
		t.Fatalf("severity = %q", got)
	}
	if got := detectLogSeverity("2026-06-15T18:03:58.332Z WARN 1 --- pool is empty"); got != "warn" {
		t.Fatalf("timestamp-prefixed severity = %q", got)
	}
	level, start, end := findLogSeverityMarker("2026-06-15T18:03:58.332Z WARN 1 --- pool is empty")
	if level != "warn" || "2026-06-15T18:03:58.332Z WARN 1 --- pool is empty"[start:end] != "WARN" {
		t.Fatalf("marker = %q %d:%d", level, start, end)
	}
}

func TestTaskLogLinesShowsUnsupportedEmptyState(t *testing.T) {
	p := NewECSPage(fakeECSService{})
	p.taskDetailTab = taskDetailTabLogs
	p.logTargets = []domainecs.LogTarget{{ContainerName: "sidecar", Supported: false, Message: "No awslogs CloudWatch Logs configuration found for container sidecar."}}
	view := strings.Join(p.taskLogLines(), "\n")
	if !strings.Contains(view, "sidecar") || !strings.Contains(view, "No awslogs") {
		t.Fatalf("expected friendly unsupported state:\n%s", view)
	}
}

func TestRenderLogViewportContentAutoFollowsAtBottom(t *testing.T) {
	p := NewECSPage(fakeECSService{})
	p.logViewport.Width = 40
	p.logViewport.Height = 3
	p.logEvents = []domainecs.LogEvent{{ID: "1", Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), Message: "INFO one"}}
	p.renderLogViewportContent()
	if !strings.Contains(p.logViewport.View(), "INFO one") || !strings.Contains(p.logViewport.View(), ":00") {
		t.Fatalf("viewport missing compact timestamp/message: %q", p.logViewport.View())
	}
}

func TestUpdateReviewShowsOldAndNewValues(t *testing.T) {
	p := NewECSPage(fakeECSService{})
	p.selectedCluster = domainecs.Cluster{Name: "prod"}
	p.selectedService = domainecs.Service{Name: "api", TaskDefinition: "api:2", TaskDefinitionARN: "td:2", DesiredCount: 2}
	p.taskDefinitions = []domainecs.TaskDefinitionSummary{{ARN: "td:3", DisplayName: "api:3"}}
	p.taskDefinitionIndex = 0
	p.desiredCountInput.SetValue("4")
	p.updateForceNewDeployment = true
	view := strings.Join(p.updateReviewLines(), "\n")
	for _, want := range []string{"prod", "api", "api:2", "api:3", "Force deploy", "Yes", "4"} {
		if !strings.Contains(view, want) {
			t.Fatalf("review missing %q:\n%s", want, view)
		}
	}
}

func TestTaskDefinitionSelectShowsCurrentOption(t *testing.T) {
	p := NewECSPage(fakeECSService{})
	p.selectedCluster = domainecs.Cluster{Name: "prod"}
	p.selectedService = domainecs.Service{Name: "api", TaskDefinitionARN: "td:2"}
	p.taskDefinitions = []domainecs.TaskDefinitionSummary{{ARN: "td:3", DisplayName: "api:3", Status: "ACTIVE"}, {ARN: "td:2", DisplayName: "api:2", Status: "ACTIVE"}}
	p.taskDefinitionIndex = 1
	view := strings.Join(p.updateTaskDefinitionLines(100, 30, 0), "\n")
	if !strings.Contains(view, "api:2 (current)") || !strings.Contains(view, "> api:2") {
		t.Fatalf("task definition select did not show current selection:\n%s", view)
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
