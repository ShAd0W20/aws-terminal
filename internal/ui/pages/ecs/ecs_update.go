package ecs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	domainecs "aws-terminal/internal/domain/ecs"
)

func (p *ECSPage) OnStateChanged(state State) tea.Cmd {
	if !state.PageFocused && p.logStreaming {
		p.stopLogStreaming()
	}
	key := sessionKey(state)
	if key != p.sessionKey {
		p.sessionKey = key
		p.resetForSession()
	}
	if state.ActiveSession == nil || p.loadingClusters || p.loadedFor == key {
		return nil
	}
	p.loadingClusters = true
	p.clustersErr = ""
	return tea.Batch(p.spinner.Tick, p.loadClustersCmd(state.ActiveSession.Profile, activeRegion(state), key))
}
func (p *ECSPage) SetFocused(focused bool) tea.Cmd {
	if !focused {
		p.searchInput.Blur()
		p.stopLogStreaming()
	}
	return nil
}
func (p *ECSPage) Update(msg tea.Msg, state State) tea.Cmd {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !p.loadingClusters && !p.servicesLoading && !p.tasksLoading && !p.logTargetsLoading && !p.logEventsLoading {
			return nil
		}
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		return cmd
	case clustersLoadedMsg:
		if msg.sessionKey != p.sessionKey {
			return nil
		}
		p.loadingClusters = false
		p.loadedFor = msg.sessionKey
		p.clustersCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.clustersErr = fmt.Sprintf("Unable to load ECS clusters: %v", msg.err)
			return nil
		}
		p.clusters = msg.clusters
		p.clustersErr = ""
		p.clusterIndex = 0
		p.syncClusterTable()
		return nil
	case servicesLoadedMsg:
		if msg.clusterARN != p.selectedCluster.ARN {
			return nil
		}
		p.servicesLoading = false
		p.servicesCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.servicesErr = fmt.Sprintf("Unable to load ECS services: %v", msg.err)
			return nil
		}
		p.services = msg.services
		p.servicesErr = ""
		p.serviceIndex = 0
		p.syncServiceTable()
		return nil
	case tasksLoadedMsg:
		if msg.clusterARN != p.selectedCluster.ARN {
			return nil
		}
		p.tasksLoading = false
		p.tasksCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.tasksErr = fmt.Sprintf("Unable to load ECS tasks: %v", msg.err)
			return nil
		}
		p.tasks = msg.tasks
		p.tasksErr = ""
		p.taskIndex = 0
		p.syncTaskTable()
		return nil
	case taskLogTargetsLoadedMsg:
		if msg.taskDefinitionARN != p.selectedTask.TaskDefinitionARN || msg.taskID != p.selectedTask.ID {
			return nil
		}
		p.logTargetsLoading = false
		p.logsCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.logTargetsErr = fmt.Sprintf("Unable to load task log targets: %v", msg.err)
			p.logStreaming = false
			return nil
		}
		p.logTargetsByTaskDefinition[msg.taskDefinitionARN] = msg.targets
		p.logTargets = msg.targets
		p.logTargetsErr = ""
		p.selectFirstSupportedLogTarget()
		return p.startSelectedLogStream(state)
	case taskLogEventsLoadedMsg:
		if msg.taskARN != p.selectedTask.ARN || msg.containerName != p.selectedLogTarget().ContainerName {
			return nil
		}
		p.logEventsLoading = false
		p.logsCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.logEventsErr = fmt.Sprintf("Unable to load CloudWatch logs: %v", msg.err)
			p.logStreaming = false
			return nil
		}
		if strings.TrimSpace(msg.page.LogStream) != "" && p.logContainerIndex >= 0 && p.logContainerIndex < len(p.logTargets) {
			p.logTargets[p.logContainerIndex].LogStream = msg.page.LogStream
			p.logTargetsByTaskDefinition[p.selectedTask.TaskDefinitionARN] = p.logTargets
		}
		p.appendLogEvents(msg.page.Events)
		p.logNextToken = msg.page.NextForwardToken
		p.renderLogViewportContent()
		if p.isActivelyViewingLogs(state) && p.logStreaming {
			return p.logPollTickCmd(p.selectedTask.ARN, msg.containerName, 3*time.Second)
		}
		return nil
	case taskLogPollTickMsg:
		if !p.isActivelyViewingLogs(state) || msg.taskARN != p.selectedTask.ARN || msg.containerName != p.selectedLogTarget().ContainerName || !p.logStreaming {
			p.stopLogStreaming()
			return nil
		}
		return p.fetchSelectedLogEvents(state, p.logNextToken, 0)
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok || !state.PageFocused {
		return p.updateInput(msg)
	}
	if p.searchInput.Focused() {
		if k.Type == tea.KeyEsc {
			p.searchInput.Blur()
			return nil
		}
		if textInputKey(k) {
			cmd := p.updateInput(msg)
			p.resetFilteredCursor()
			return cmd
		}
	}
	switch p.stage {
	case ecsStageClusters:
		return p.updateClusters(k, state)
	case ecsStageResources:
		return p.updateResources(k, state)
	case ecsStageServiceDetail:
		if key.Matches(k, ecsBackKey) {
			p.stage = ecsStageResources
			return nil
		}
	case ecsStageTaskDetail:
		return p.updateTaskDetail(k, state)
	}
	return nil
}
func (p *ECSPage) updateInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.searchInput, cmd = p.searchInput.Update(msg)
	return cmd
}
func (p *ECSPage) resetFilteredCursor() {
	switch p.stage {
	case ecsStageClusters:
		p.clusterIndex = 0
		p.clusterPaginator.Page = 0
		p.syncClusterTable()
	case ecsStageResources:
		if p.tab == ecsTabServices {
			p.serviceIndex = 0
			p.servicePaginator.Page = 0
			p.syncServiceTable()
		} else {
			p.taskIndex = 0
			p.taskPaginator.Page = 0
			p.syncTaskTable()
		}
	}
}
func (p *ECSPage) updateClusters(k tea.KeyMsg, state State) tea.Cmd {
	if key.Matches(k, ecsSearchKey) {
		return p.searchInput.Focus()
	}
	if key.Matches(k, ecsRefreshKey) && state.ActiveSession != nil {
		p.loadingClusters = true
		p.clustersErr = ""
		return tea.Batch(p.spinner.Tick, p.loadClustersCmd(state.ActiveSession.Profile, activeRegion(state), p.sessionKey))
	}
	items := p.filteredClusters()
	if p.clusterIndex >= len(items) {
		p.clusterIndex = max(0, len(items)-1)
	}
	switch {
	case key.Matches(k, ecsUpKey):
		if p.clusterIndex > 0 {
			p.clusterIndex--
		}
		p.syncClusterTable()
		return nil
	case key.Matches(k, ecsDownKey):
		if p.clusterIndex < len(items)-1 {
			p.clusterIndex++
		}
		p.syncClusterTable()
		return nil
	case key.Matches(k, ecsEnterKey):
		if len(items) > 0 && state.ActiveSession != nil {
			p.selectedCluster = items[p.clusterIndex]
			p.logTargetsByTaskDefinition = map[string][]domainecs.LogTarget{}
			p.resetLogState()
			p.stage = ecsStageResources
			p.tab = ecsTabServices
			p.searchInput.SetValue("")
			p.searchInput.Blur()
			p.servicesLoading = true
			p.tasksLoading = true
			p.servicesErr = ""
			p.tasksErr = ""
			return tea.Batch(p.spinner.Tick, p.loadServicesCmd(state.ActiveSession.Profile, activeRegion(state), p.selectedCluster.ARN), p.loadTasksCmd(state.ActiveSession.Profile, activeRegion(state), p.selectedCluster.ARN))
		}
	}
	return p.updatePaged(k, true)
}
func (p *ECSPage) updateResources(k tea.KeyMsg, state State) tea.Cmd {
	if key.Matches(k, ecsBackKey) {
		p.stopLogStreaming()
		p.stage = ecsStageClusters
		p.searchInput.SetValue("")
		p.searchInput.Blur()
		p.syncClusterTable()
		return nil
	}
	if key.Matches(k, ecsSearchKey) {
		return p.searchInput.Focus()
	}
	if key.Matches(k, ecsPrevTabKey) || key.Matches(k, ecsNextTabKey) {
		if p.tab == ecsTabServices {
			p.tab = ecsTabTasks
		} else {
			p.tab = ecsTabServices
		}
		p.searchInput.SetValue("")
		p.resetFilteredCursor()
		return nil
	}
	if key.Matches(k, ecsRefreshKey) && state.ActiveSession != nil {
		p.stopLogStreaming()
		p.servicesLoading = true
		p.tasksLoading = true
		p.servicesErr = ""
		p.tasksErr = ""
		return tea.Batch(p.spinner.Tick, p.loadServicesCmd(state.ActiveSession.Profile, activeRegion(state), p.selectedCluster.ARN), p.loadTasksCmd(state.ActiveSession.Profile, activeRegion(state), p.selectedCluster.ARN))
	}
	if p.tab == ecsTabServices {
		return p.updateServicesTable(k)
	}
	return p.updateTasksTable(k)
}
func (p *ECSPage) updateServicesTable(k tea.KeyMsg) tea.Cmd {
	items := p.filteredServices()
	switch {
	case key.Matches(k, ecsUpKey):
		if p.serviceIndex > 0 {
			p.serviceIndex--
		}
		p.syncServiceTable()
		return nil
	case key.Matches(k, ecsDownKey):
		if p.serviceIndex < len(items)-1 {
			p.serviceIndex++
		}
		p.syncServiceTable()
		return nil
	case key.Matches(k, ecsEnterKey):
		if len(items) > 0 {
			p.selectedService = items[p.serviceIndex]
			p.stage = ecsStageServiceDetail
		}
		return nil
	}
	return p.updatePaged(k, false)
}
func (p *ECSPage) updateTasksTable(k tea.KeyMsg) tea.Cmd {
	items := p.filteredTasks()
	switch {
	case key.Matches(k, ecsUpKey):
		if p.taskIndex > 0 {
			p.taskIndex--
		}
		p.syncTaskTable()
		return nil
	case key.Matches(k, ecsDownKey):
		if p.taskIndex < len(items)-1 {
			p.taskIndex++
		}
		p.syncTaskTable()
		return nil
	case key.Matches(k, ecsEnterKey):
		if len(items) > 0 {
			p.selectedTask = items[p.taskIndex]
			p.stage = ecsStageTaskDetail
			p.taskDetailTab = taskDetailTabOverview
			p.logTargetsByTaskDefinition = map[string][]domainecs.LogTarget{}
			p.resetLogState()
		}
		return nil
	}
	return p.updatePaged(k, false)
}
func (p *ECSPage) updateTaskDetail(k tea.KeyMsg, state State) tea.Cmd {
	if key.Matches(k, ecsBackKey) {
		p.stopLogStreaming()
		p.stage = ecsStageResources
		p.taskDetailTab = taskDetailTabOverview
		return nil
	}
	if key.Matches(k, ecsPrevTabKey) || key.Matches(k, ecsNextTabKey) {
		if p.taskDetailTab == taskDetailTabOverview {
			p.taskDetailTab = taskDetailTabLogs
			return p.ensureLogTargetsLoaded(state)
		}
		p.taskDetailTab = taskDetailTabOverview
		p.stopLogStreaming()
		return nil
	}
	if p.taskDetailTab == taskDetailTabLogs {
		if key.Matches(k, ecsPrevContainerKey) {
			return p.switchLogContainer(state, -1)
		}
		if key.Matches(k, ecsNextContainerKey) {
			return p.switchLogContainer(state, 1)
		}
		var cmd tea.Cmd
		p.logViewport, cmd = p.logViewport.Update(k)
		return cmd
	}
	return nil
}

func (p *ECSPage) ensureLogTargetsLoaded(state State) tea.Cmd {
	if !p.isActivelyViewingLogs(state) || state.ActiveSession == nil {
		return nil
	}
	if targets, ok := p.logTargetsByTaskDefinition[p.selectedTask.TaskDefinitionARN]; ok {
		p.logTargets = targets
		p.selectFirstSupportedLogTarget()
		return p.startSelectedLogStream(state)
	}
	p.resetLogState()
	p.logTargetsLoading = true
	p.logStreaming = true
	return tea.Batch(p.spinner.Tick, p.loadLogTargetsCmd(state.ActiveSession.Profile, activeRegion(state), p.selectedTask.TaskDefinitionARN, p.selectedTask.ID))
}

func (p *ECSPage) switchLogContainer(state State, delta int) tea.Cmd {
	if len(p.logTargets) == 0 {
		return nil
	}
	p.logContainerIndex = (p.logContainerIndex + delta + len(p.logTargets)) % len(p.logTargets)
	p.logEvents = nil
	p.logEventsErr = ""
	p.logSeenEventIDs = map[string]struct{}{}
	p.logNextToken = ""
	p.logViewport.SetContent("")
	p.logViewport.GotoBottom()
	return p.startSelectedLogStream(state)
}

func (p *ECSPage) startSelectedLogStream(state State) tea.Cmd {
	if !p.isActivelyViewingLogs(state) || state.ActiveSession == nil {
		p.stopLogStreaming()
		return nil
	}
	target := p.selectedLogTarget()
	if !target.Supported {
		p.logStreaming = false
		p.logEventsLoading = false
		p.renderLogViewportContent()
		return nil
	}
	p.logEvents = nil
	p.logSeenEventIDs = map[string]struct{}{}
	p.logNextToken = ""
	p.logEventsErr = ""
	p.logStreaming = true
	p.logEventsLoading = true
	p.logViewport.SetContent("")
	p.logViewport.GotoBottom()
	return tea.Batch(p.spinner.Tick, p.fetchSelectedLogEvents(state, "", 15*time.Minute))
}

func (p *ECSPage) fetchSelectedLogEvents(state State, nextToken string, lookback time.Duration) tea.Cmd {
	if state.ActiveSession == nil {
		return nil
	}
	target := p.selectedLogTarget()
	if !target.Supported {
		return nil
	}
	p.logEventsLoading = true
	return p.loadLogEventsCmd(state.ActiveSession.Profile, activeRegion(state), target, nextToken, lookback, 500)
}

func (p *ECSPage) isActivelyViewingLogs(state State) bool {
	return state.PageFocused && p.stage == ecsStageTaskDetail && p.taskDetailTab == taskDetailTabLogs
}

func (p *ECSPage) selectedLogTarget() domainecs.LogTarget {
	if len(p.logTargets) == 0 || p.logContainerIndex < 0 || p.logContainerIndex >= len(p.logTargets) {
		return domainecs.LogTarget{}
	}
	return p.logTargets[p.logContainerIndex]
}

func (p *ECSPage) selectFirstSupportedLogTarget() {
	p.logContainerIndex = 0
	for i, target := range p.logTargets {
		if target.Supported {
			p.logContainerIndex = i
			return
		}
	}
}

func (p *ECSPage) appendLogEvents(events []domainecs.LogEvent) {
	if p.logSeenEventIDs == nil {
		p.logSeenEventIDs = map[string]struct{}{}
	}
	for _, event := range events {
		key := event.ID
		if key == "" {
			key = event.Timestamp.String() + event.Message
		}
		if _, ok := p.logSeenEventIDs[key]; ok {
			continue
		}
		p.logSeenEventIDs[key] = struct{}{}
		p.logEvents = append(p.logEvents, event)
	}
}

func (p *ECSPage) updatePaged(k tea.KeyMsg, clusters bool) tea.Cmd {
	if clusters {
		prev := p.clusterPaginator.Page
		var cmd tea.Cmd
		p.clusterPaginator, cmd = p.clusterPaginator.Update(k)
		if p.clusterPaginator.Page != prev {
			start, _ := p.clusterPaginator.GetSliceBounds(len(p.filteredClusters()))
			p.clusterIndex = start
			p.syncClusterTable()
		}
		return cmd
	}
	if p.tab == ecsTabServices {
		prev := p.servicePaginator.Page
		var cmd tea.Cmd
		p.servicePaginator, cmd = p.servicePaginator.Update(k)
		if p.servicePaginator.Page != prev {
			start, _ := p.servicePaginator.GetSliceBounds(len(p.filteredServices()))
			p.serviceIndex = start
			p.syncServiceTable()
		}
		return cmd
	}
	prev := p.taskPaginator.Page
	var cmd tea.Cmd
	p.taskPaginator, cmd = p.taskPaginator.Update(k)
	if p.taskPaginator.Page != prev {
		start, _ := p.taskPaginator.GetSliceBounds(len(p.filteredTasks()))
		p.taskIndex = start
		p.syncTaskTable()
	}
	return cmd
}
