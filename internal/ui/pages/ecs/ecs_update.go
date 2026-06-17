package ecs

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
		p.desiredCountInput.Blur()
		p.stopReasonInput.Blur()
		p.stopLogStreaming()
		return nil
	}
	if p.stage == ecsStageUpdateDesiredCount {
		return p.desiredCountInput.Focus()
	}
	if p.stage == ecsStageStopTaskReason {
		return p.stopReasonInput.Focus()
	}
	return nil
}
func (p *ECSPage) Update(msg tea.Msg, state State) tea.Cmd {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !p.loadingClusters && !p.servicesLoading && !p.tasksLoading && !p.logTargetsLoading && !p.logEventsLoading && !p.taskDefinitionsLoading && !p.updatingService && !p.stoppingTask {
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
		previousARN := p.selectedTask.ARN
		p.tasks = msg.tasks
		p.tasksErr = ""
		p.taskIndex = nearestTaskIndexByARN(p.tasks, previousARN, p.taskIndex)
		if p.stage == ecsStageTaskDetail && previousARN != "" {
			if task, ok := taskByARN(p.tasks, previousARN); ok {
				p.selectedTask = task
			}
		}
		p.syncTaskTable()
		return nil
	case taskDefinitionsLoadedMsg:
		if msg.familyPrefix != p.updateFamilyPrefix {
			return nil
		}
		p.taskDefinitionsLoading = false
		p.updateCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.taskDefinitionsErr = fmt.Sprintf("Unable to load task definitions: %v", msg.err)
			return nil
		}
		p.taskDefinitions = ensureCurrentTaskDefinition(msg.taskDefinitions, p.selectedService)
		p.taskDefinitionsErr = ""
		p.preselectCurrentTaskDefinition()
		p.syncTaskDefinitionSelection()
		return nil
	case serviceUpdatedMsg:
		if msg.clusterARN != p.selectedCluster.ARN {
			return nil
		}
		p.updatingService = false
		p.updateCancel = nil
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		if msg.err != nil {
			p.updateErr = fmt.Sprintf("Unable to update service: %v", msg.err)
			p.stage = ecsStageUpdateReview
			return nil
		}
		p.selectedService = msg.result.Service
		p.stage = ecsStageServiceDetail
		p.updateSuccessSeq++
		p.updateSuccess = "Service update started. Refreshing service and task data..."
		cmds := []tea.Cmd{p.clearUpdateSuccessCmd(p.updateSuccessSeq, 4*time.Second)}
		if state.ActiveSession != nil {
			p.servicesLoading = true
			p.tasksLoading = true
			p.servicesErr = ""
			p.tasksErr = ""
			cmds = append(cmds, p.spinner.Tick, p.loadServicesCmd(state.ActiveSession.Profile, activeRegion(state), p.selectedCluster.ARN), p.loadTasksCmd(state.ActiveSession.Profile, activeRegion(state), p.selectedCluster.ARN))
		}
		return tea.Batch(cmds...)
	case taskStoppedMsg:
		if msg.clusterARN != p.selectedCluster.ARN {
			return nil
		}
		p.stoppingTask = false
		p.updateCancel = nil
		cmds := []tea.Cmd{}
		if state.ActiveSession != nil {
			p.servicesLoading = true
			p.tasksLoading = true
			p.servicesErr = ""
			p.tasksErr = ""
			cmds = append(cmds, p.spinner.Tick, p.loadServicesCmd(state.ActiveSession.Profile, activeRegion(state), p.selectedCluster.ARN), p.loadTasksCmd(state.ActiveSession.Profile, activeRegion(state), p.selectedCluster.ARN))
		}
		if errors.Is(msg.err, context.Canceled) {
			return tea.Batch(cmds...)
		}
		if msg.err != nil {
			p.updateErr = fmt.Sprintf("Unable to stop task: %v", msg.err)
			p.stage = ecsStageStopTaskReview
			return tea.Batch(cmds...)
		}
		p.selectedTask = msg.result.Task
		p.taskDetailTab = taskDetailTabOverview
		p.stage = ecsStageTaskDetail
		p.updateSuccessSeq++
		p.updateSuccess = "Stop task requested. Refreshing service and task data..."
		cmds = append(cmds, p.clearUpdateSuccessCmd(p.updateSuccessSeq, 4*time.Second))
		return tea.Batch(cmds...)
	case updateSuccessClearMsg:
		if msg.seq == p.updateSuccessSeq {
			p.updateSuccess = ""
		}
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
		if key.Matches(k, ecsUpdateServiceKey) {
			return p.startServiceUpdate(state)
		}
	case ecsStageUpdateTaskDefinition:
		return p.updateTaskDefinitionStage(k)
	case ecsStageUpdateDesiredCount:
		return p.updateDesiredCountStage(msg, state)
	case ecsStageUpdateReview:
		return p.updateServiceReviewStage(k, state)
	case ecsStageUpdating:
		if key.Matches(k, ecsBackKey) && p.updateCancel != nil {
			p.updateCancel()
			p.updateCancel = nil
			p.updatingService = false
			p.stage = ecsStageUpdateReview
			return nil
		}
	case ecsStageStopTaskReason:
		return p.updateStopTaskReasonStage(msg, state)
	case ecsStageStopTaskReview:
		return p.updateStopTaskReviewStage(k, state)
	case ecsStageStoppingTask:
		if key.Matches(k, ecsBackKey) && p.updateCancel != nil {
			p.updateCancel()
			p.updateCancel = nil
			p.stoppingTask = false
			p.stage = ecsStageStopTaskReview
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
func (p *ECSPage) startServiceUpdate(state State) tea.Cmd {
	if state.ActiveSession == nil {
		return nil
	}
	p.resetUpdateState()
	p.updateFamilyPrefix = taskDefinitionFamilyFromNameOrARN(p.selectedService.TaskDefinitionARN)
	if p.updateFamilyPrefix == "" {
		p.updateFamilyPrefix = taskDefinitionFamilyFromNameOrARN(p.selectedService.TaskDefinition)
	}
	p.desiredCountInput.SetValue(strconv.Itoa(p.selectedService.DesiredCount))
	p.stage = ecsStageUpdateTaskDefinition
	p.taskDefinitionsLoading = true
	p.taskDefinitionsErr = ""
	return tea.Batch(p.spinner.Tick, p.loadTaskDefinitionsCmd(state.ActiveSession.Profile, activeRegion(state), p.updateFamilyPrefix))
}

func (p *ECSPage) updateTaskDefinitionStage(k tea.KeyMsg) tea.Cmd {
	if key.Matches(k, ecsBackKey) {
		p.resetUpdateState()
		p.stage = ecsStageServiceDetail
		return nil
	}
	if p.taskDefinitionsLoading || len(p.taskDefinitions) == 0 {
		return nil
	}
	switch {
	case key.Matches(k, ecsUpKey):
		if p.taskDefinitionIndex > 0 {
			p.taskDefinitionIndex--
		}
		p.syncTaskDefinitionSelection()
		return nil
	case key.Matches(k, ecsDownKey):
		if p.taskDefinitionIndex < len(p.taskDefinitions)-1 {
			p.taskDefinitionIndex++
		}
		p.syncTaskDefinitionSelection()
		return nil
	case key.Matches(k, ecsEnterKey):
		p.stage = ecsStageUpdateDesiredCount
		return p.desiredCountInput.Focus()
	}
	prev := p.taskDefinitionPaginator.Page
	var cmd tea.Cmd
	p.taskDefinitionPaginator, cmd = p.taskDefinitionPaginator.Update(k)
	if p.taskDefinitionPaginator.Page != prev {
		start, _ := p.taskDefinitionPaginator.GetSliceBounds(len(p.taskDefinitions))
		p.taskDefinitionIndex = start
		p.syncTaskDefinitionSelection()
	}
	return cmd
}

func (p *ECSPage) updateDesiredCountStage(msg tea.Msg, state State) tea.Cmd {
	k, isKey := msg.(tea.KeyMsg)
	if isKey {
		if key.Matches(k, ecsBackKey) {
			p.desiredCountInput.Blur()
			p.stage = ecsStageUpdateTaskDefinition
			return nil
		}
		if key.Matches(k, ecsEnterKey) {
			if _, err := p.parsedDesiredCount(); err != nil {
				p.updateErr = err.Error()
				return nil
			}
			p.updateErr = ""
			p.desiredCountInput.Blur()
			p.stage = ecsStageUpdateReview
			return nil
		}
	}
	if state.PageFocused {
		return p.updateDesiredInput(msg)
	}
	return nil
}

func (p *ECSPage) updateDesiredInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.desiredCountInput, cmd = p.desiredCountInput.Update(msg)
	return cmd
}

func (p *ECSPage) updateServiceReviewStage(k tea.KeyMsg, state State) tea.Cmd {
	if key.Matches(k, ecsBackKey) {
		p.stage = ecsStageUpdateDesiredCount
		return p.desiredCountInput.Focus()
	}
	if key.Matches(k, ecsToggleKey) {
		p.updateForceNewDeployment = !p.updateForceNewDeployment
		return nil
	}
	if key.Matches(k, ecsEnterKey) {
		if state.ActiveSession == nil {
			return nil
		}
		input, err := p.buildUpdateServiceInput(state)
		if err != nil {
			p.updateErr = err.Error()
			return nil
		}
		p.updateErr = ""
		p.updatingService = true
		p.stage = ecsStageUpdating
		return tea.Batch(p.spinner.Tick, p.updateServiceCmd(input))
	}
	return nil
}

func (p *ECSPage) buildUpdateServiceInput(state State) (domainecs.UpdateServiceInput, error) {
	desired, err := p.parsedDesiredCount()
	if err != nil {
		return domainecs.UpdateServiceInput{}, err
	}
	selected := p.selectedTaskDefinition()
	if strings.TrimSpace(selected.ARN) == "" {
		return domainecs.UpdateServiceInput{}, fmt.Errorf("task definition is required")
	}
	input := domainecs.UpdateServiceInput{ProfileName: state.ActiveSession.Profile, Region: activeRegion(state), ClusterARN: p.selectedCluster.ARN, Service: p.selectedService.ARN, ForceNewDeployment: p.updateForceNewDeployment}
	if input.Service == "" {
		input.Service = p.selectedService.Name
	}
	if selected.ARN != p.selectedService.TaskDefinitionARN {
		input.TaskDefinitionARN = selected.ARN
	}
	if desired != p.selectedService.DesiredCount {
		input.DesiredCount = &desired
	}
	if input.TaskDefinitionARN == "" && input.DesiredCount == nil && !input.ForceNewDeployment {
		return domainecs.UpdateServiceInput{}, fmt.Errorf("select a task-definition or desired-count change, or enable force-new-deployment")
	}
	return input, nil
}

func ensureCurrentTaskDefinition(definitions []domainecs.TaskDefinitionSummary, service domainecs.Service) []domainecs.TaskDefinitionSummary {
	currentARN := strings.TrimSpace(service.TaskDefinitionARN)
	if currentARN == "" {
		return definitions
	}
	for _, definition := range definitions {
		if strings.TrimSpace(definition.ARN) == currentARN {
			return definitions
		}
	}
	current := domainecs.TaskDefinitionSummary{ARN: currentARN, DisplayName: service.TaskDefinition, Family: taskDefinitionFamilyFromNameOrARN(currentARN)}
	if strings.TrimSpace(current.DisplayName) == "" {
		current.DisplayName = taskDefinitionNameForUI(currentARN)
	}
	return append([]domainecs.TaskDefinitionSummary{current}, definitions...)
}

func taskDefinitionNameForUI(arn string) string {
	base := strings.TrimSpace(arn)
	if idx := strings.LastIndex(base, "/"); idx >= 0 && idx < len(base)-1 {
		return base[idx+1:]
	}
	return base
}

func (p *ECSPage) preselectCurrentTaskDefinition() {
	p.taskDefinitionIndex = 0
	currentARN := strings.TrimSpace(p.selectedService.TaskDefinitionARN)
	for i, definition := range p.taskDefinitions {
		if strings.TrimSpace(definition.ARN) == currentARN {
			p.taskDefinitionIndex = i
			return
		}
	}
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
	if key.Matches(k, ecsStopTaskKey) {
		return p.startStopTask(state)
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
