package ecs

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	domainecs "aws-terminal/internal/domain/ecs"
	domainsession "aws-terminal/internal/domain/session"
)

type fakeECSService struct{}

func (fakeECSService) ListClusters(context.Context, string, string) ([]domainecs.Cluster, error) {
	return nil, nil
}
func (fakeECSService) ListServices(context.Context, string, string, string) ([]domainecs.Service, error) {
	return nil, nil
}
func (fakeECSService) ListTaskDefinitions(context.Context, string, string, string) ([]domainecs.TaskDefinitionSummary, error) {
	return nil, nil
}
func (fakeECSService) ListTasks(context.Context, string, string, string) ([]domainecs.Task, error) {
	return nil, nil
}
func (fakeECSService) UpdateService(context.Context, domainecs.UpdateServiceInput) (domainecs.UpdateServiceResult, error) {
	return domainecs.UpdateServiceResult{}, nil
}
func (fakeECSService) DescribeTaskLogTargets(context.Context, string, string, string, string) ([]domainecs.LogTarget, error) {
	return []domainecs.LogTarget{{ContainerName: "app", LogGroup: "group", LogStream: "prefix/app/task", Supported: true}, {ContainerName: "sidecar", Supported: false, Message: "No awslogs CloudWatch Logs configuration found for container sidecar."}}, nil
}
func (fakeECSService) FetchTaskLogEvents(context.Context, string, string, domainecs.LogTarget, string, time.Duration, int32) (domainecs.LogEventsPage, error) {
	return domainecs.LogEventsPage{Events: []domainecs.LogEvent{{ID: "1", Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), Message: "INFO hello"}}, NextForwardToken: "next"}, nil
}

func testState() State {
	return State{ActiveSession: &domainsession.Session{Profile: "dev", Region: "eu-west-1"}, SelectedRegion: "eu-west-1", PageFocused: true}
}

func TestSearchInputAcceptsKeybindLetters(t *testing.T) {
	p := NewECSPage(fakeECSService{})
	p.stage = ecsStageClusters
	if cmd := p.Update(tea.KeyMsg{Type: tea.KeyCtrlF}, testState()); cmd == nil {
		t.Fatal("expected focus command")
	}
	p.searchInput.Focus()
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}, testState())
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}, testState())
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")}, testState())
	if got := p.searchInput.Value(); got != "rqb" {
		t.Fatalf("search value = %q", got)
	}
}

func TestSelectingClusterStartsServicesAndTasksLoads(t *testing.T) {
	p := NewECSPage(fakeECSService{})
	p.clusters = []domainecs.Cluster{{Name: "prod", ARN: "arn:cluster"}}
	p.syncClusterTable()
	cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter}, testState())
	if p.stage != ecsStageResources {
		t.Fatalf("stage = %v", p.stage)
	}
	if !p.servicesLoading || !p.tasksLoading {
		t.Fatalf("services/tasks should be loading")
	}
	if cmd == nil {
		t.Fatal("expected load command")
	}
}

func TestTaskDetailTabsStartAndStopLogStreaming(t *testing.T) {
	p := NewECSPage(fakeECSService{})
	p.stage = ecsStageTaskDetail
	p.selectedTask = domainecs.Task{ID: "task", ARN: "task-arn", TaskDefinitionARN: "td", Containers: []domainecs.Container{{Name: "app"}}}
	cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")}, testState())
	if p.taskDetailTab != taskDetailTabLogs {
		t.Fatalf("task detail tab = %v", p.taskDetailTab)
	}
	if !p.logTargetsLoading || !p.logStreaming || cmd == nil {
		t.Fatalf("expected log target load and streaming state")
	}
	p.Update(taskLogTargetsLoadedMsg{taskDefinitionARN: "td", taskID: "task", targets: []domainecs.LogTarget{{ContainerName: "app", Supported: true, LogGroup: "group", LogStream: "stream"}}}, testState())
	p.Update(taskLogEventsLoadedMsg{taskARN: "task-arn", containerName: "app", page: domainecs.LogEventsPage{Events: []domainecs.LogEvent{{ID: "1", Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), Message: "INFO hello"}}, NextForwardToken: "next", LogStream: "resolved-stream"}}, testState())
	if len(p.logEvents) != 1 || p.logNextToken != "next" {
		t.Fatalf("log events not appended: events=%d token=%q", len(p.logEvents), p.logNextToken)
	}
	if p.logTargets[0].LogStream != "resolved-stream" {
		t.Fatalf("log stream = %q, want resolved-stream", p.logTargets[0].LogStream)
	}
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")}, testState())
	if p.taskDetailTab != taskDetailTabOverview || p.logStreaming {
		t.Fatalf("leaving logs should stop streaming, tab=%v streaming=%v", p.taskDetailTab, p.logStreaming)
	}
}

func TestLogContainerSwitchResetsEvents(t *testing.T) {
	p := NewECSPage(fakeECSService{})
	p.stage = ecsStageTaskDetail
	p.taskDetailTab = taskDetailTabLogs
	p.selectedTask = domainecs.Task{ID: "task", ARN: "task-arn", TaskDefinitionARN: "td"}
	p.logTargets = []domainecs.LogTarget{{ContainerName: "app", Supported: true, LogGroup: "group", LogStream: "app"}, {ContainerName: "sidecar", Supported: false, Message: "no logs"}}
	p.logEvents = []domainecs.LogEvent{{ID: "1", Message: "INFO old"}}
	p.logNextToken = "next"
	p.Update(tea.KeyMsg{Type: tea.KeyCtrlL}, testState())
	if p.logContainerIndex != 1 {
		t.Fatalf("container index = %d", p.logContainerIndex)
	}
	if len(p.logEvents) != 0 || p.logNextToken != "" {
		t.Fatalf("switch should reset log events/token")
	}
}

func TestLogPollingStopsWhenPageNotFocused(t *testing.T) {
	p := NewECSPage(fakeECSService{})
	p.stage = ecsStageTaskDetail
	p.taskDetailTab = taskDetailTabLogs
	p.selectedTask = domainecs.Task{ARN: "task-arn"}
	p.logTargets = []domainecs.LogTarget{{ContainerName: "app", Supported: true, LogGroup: "group", LogStream: "stream"}}
	p.logStreaming = true
	state := testState()
	state.PageFocused = false
	cmd := p.Update(taskLogPollTickMsg{taskARN: "task-arn", containerName: "app"}, state)
	if cmd != nil || p.logStreaming {
		t.Fatalf("polling should stop without focused logs view")
	}
}

func TestStartServiceUpdateLoadsTaskDefinitionsForFamily(t *testing.T) {
	p := NewECSPage(fakeECSService{})
	p.stage = ecsStageServiceDetail
	p.selectedCluster = domainecs.Cluster{Name: "prod", ARN: "cluster"}
	p.selectedService = domainecs.Service{Name: "api", ARN: "svc", TaskDefinitionARN: "arn:aws:ecs:eu-west-1:123:task-definition/api:2", TaskDefinition: "api:2", DesiredCount: 2}
	cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")}, testState())
	if cmd == nil || p.stage != ecsStageUpdateTaskDefinition || !p.taskDefinitionsLoading {
		t.Fatalf("expected task definition loading update stage, stage=%v loading=%v cmd nil=%v", p.stage, p.taskDefinitionsLoading, cmd == nil)
	}
	if p.updateFamilyPrefix != "api" || p.desiredCountInput.Value() != "2" {
		t.Fatalf("family/input not prefilled: family=%q desired=%q", p.updateFamilyPrefix, p.desiredCountInput.Value())
	}
}

func TestTaskDefinitionSelectionPreselectsCurrentAndBuildsUpdate(t *testing.T) {
	p := NewECSPage(fakeECSService{})
	p.stage = ecsStageUpdateTaskDefinition
	p.updateFamilyPrefix = "api"
	p.selectedCluster = domainecs.Cluster{Name: "prod", ARN: "cluster"}
	p.selectedService = domainecs.Service{Name: "api", ARN: "svc", TaskDefinitionARN: "td:2", TaskDefinition: "api:2", DesiredCount: 2}
	p.Update(taskDefinitionsLoadedMsg{familyPrefix: "api", taskDefinitions: []domainecs.TaskDefinitionSummary{{ARN: "td:3", DisplayName: "api:3", Family: "api", Revision: 3}, {ARN: "td:2", DisplayName: "api:2", Family: "api", Revision: 2}}}, testState())
	if p.taskDefinitionIndex != 1 {
		t.Fatalf("expected current task definition preselected, got index %d", p.taskDefinitionIndex)
	}
	p.Update(tea.KeyMsg{Type: tea.KeyUp}, testState())
	p.Update(tea.KeyMsg{Type: tea.KeyEnter}, testState())
	if p.stage != ecsStageUpdateDesiredCount || !p.desiredCountInput.Focused() {
		t.Fatalf("expected desired count stage with focused input, stage=%v focused=%v", p.stage, p.desiredCountInput.Focused())
	}
	p.desiredCountInput.SetValue("4")
	p.Update(tea.KeyMsg{Type: tea.KeyEnter}, testState())
	if p.stage != ecsStageUpdateReview {
		t.Fatalf("expected review stage, got %v", p.stage)
	}
	p.Update(tea.KeyMsg{Type: tea.KeySpace}, testState())
	input, err := p.buildUpdateServiceInput(testState())
	if err != nil {
		t.Fatal(err)
	}
	if input.TaskDefinitionARN != "td:3" || input.DesiredCount == nil || *input.DesiredCount != 4 || !input.ForceNewDeployment {
		t.Fatalf("unexpected update input: %#v", input)
	}
}

func TestSwitchTabsAndOpenDetails(t *testing.T) {
	p := NewECSPage(fakeECSService{})
	p.stage = ecsStageResources
	p.tab = ecsTabServices
	p.services = []domainecs.Service{{Name: "api", ARN: "svc"}}
	p.tasks = []domainecs.Task{{ID: "task", ARN: "task"}}
	p.syncServiceTable()
	p.syncTaskTable()
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")}, testState())
	if p.tab != ecsTabTasks {
		t.Fatalf("tab = %v", p.tab)
	}
	p.Update(tea.KeyMsg{Type: tea.KeyEnter}, testState())
	if p.stage != ecsStageTaskDetail {
		t.Fatalf("stage = %v", p.stage)
	}
	p.Update(tea.KeyMsg{Type: tea.KeyEsc}, testState())
	if p.stage != ecsStageResources {
		t.Fatalf("stage after back = %v", p.stage)
	}
}
