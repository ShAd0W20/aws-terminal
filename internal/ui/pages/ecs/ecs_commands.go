package ecs

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	domainecs "aws-terminal/internal/domain/ecs"
)

func (p *ECSPage) loadClustersCmd(profile, region, key string) tea.Cmd {
	if p.clustersCancel != nil {
		p.clustersCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.clustersCancel = cancel
	return func() tea.Msg {
		clusters, err := p.service.ListClusters(ctx, profile, region)
		return clustersLoadedMsg{sessionKey: key, clusters: clusters, err: err}
	}
}
func (p *ECSPage) loadServicesCmd(profile, region, clusterARN string) tea.Cmd {
	if p.servicesCancel != nil {
		p.servicesCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.servicesCancel = cancel
	return func() tea.Msg {
		services, err := p.service.ListServices(ctx, profile, region, clusterARN)
		return servicesLoadedMsg{clusterARN: clusterARN, services: services, err: err}
	}
}
func (p *ECSPage) loadTasksCmd(profile, region, clusterARN string) tea.Cmd {
	if p.tasksCancel != nil {
		p.tasksCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.tasksCancel = cancel
	return func() tea.Msg {
		tasks, err := p.service.ListTasks(ctx, profile, region, clusterARN)
		return tasksLoadedMsg{clusterARN: clusterARN, tasks: tasks, err: err}
	}
}
func (p *ECSPage) loadLogTargetsCmd(profile, region, taskDefinitionARN, taskID string) tea.Cmd {
	if p.logsCancel != nil {
		p.logsCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.logsCancel = cancel
	return func() tea.Msg {
		targets, err := p.service.DescribeTaskLogTargets(ctx, profile, region, taskDefinitionARN, taskID)
		return taskLogTargetsLoadedMsg{taskDefinitionARN: taskDefinitionARN, taskID: taskID, targets: targets, err: err}
	}
}

func (p *ECSPage) loadLogEventsCmd(profile, region string, target domainecs.LogTarget, nextToken string, lookback time.Duration, limit int32) tea.Cmd {
	if p.logsCancel != nil {
		p.logsCancel()
	}
	taskARN := p.selectedTask.ARN
	ctx, cancel := context.WithCancel(context.Background())
	p.logsCancel = cancel
	return func() tea.Msg {
		page, err := p.service.FetchTaskLogEvents(ctx, profile, region, target, nextToken, lookback, limit)
		return taskLogEventsLoadedMsg{taskARN: taskARN, containerName: target.ContainerName, page: page, err: err}
	}
}

func (p *ECSPage) logPollTickCmd(taskARN, containerName string, delay time.Duration) tea.Cmd {
	return func() tea.Msg {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
		return taskLogPollTickMsg{taskARN: taskARN, containerName: containerName}
	}
}

func (p *ECSPage) cancelResourceLoads() {
	if p.servicesCancel != nil {
		p.servicesCancel()
		p.servicesCancel = nil
	}
	if p.tasksCancel != nil {
		p.tasksCancel()
		p.tasksCancel = nil
	}
	p.stopLogStreaming()
}

func (p *ECSPage) stopLogStreaming() {
	p.logStreaming = false
	p.logEventsLoading = false
	p.logTargetsLoading = false
	if p.logsCancel != nil {
		p.logsCancel()
		p.logsCancel = nil
	}
}

func (p *ECSPage) resetLogState() {
	p.stopLogStreaming()
	p.logTargetsErr = ""
	p.logTargets = nil
	p.logContainerIndex = 0
	p.logEventsErr = ""
	p.logEvents = nil
	p.logSeenEventIDs = map[string]struct{}{}
	p.logNextToken = ""
	p.logViewport.SetContent("")
	p.logViewport.GotoBottom()
}
func (p *ECSPage) resetForSession() {
	if p.clustersCancel != nil {
		p.clustersCancel()
		p.clustersCancel = nil
	}
	p.cancelResourceLoads()
	p.loadedFor = ""
	p.loadingClusters = false
	p.clustersErr = ""
	p.clusters = nil
	p.clusterIndex = 0
	p.stage = ecsStageClusters
	p.selectedCluster = domainecs.Cluster{}
	p.services = nil
	p.tasks = nil
	p.selectedTask = domainecs.Task{}
	p.taskDetailTab = taskDetailTabOverview
	p.logTargetsByTaskDefinition = map[string][]domainecs.LogTarget{}
	p.resetLogState()
	p.searchInput.SetValue("")
}
