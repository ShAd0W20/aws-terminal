package ecs

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"

	domainecs "aws-terminal/internal/domain/ecs"
	"aws-terminal/internal/ui/styles"
)

type ECSService interface {
	ListClusters(ctx context.Context, profileName, region string) ([]domainecs.Cluster, error)
	ListServices(ctx context.Context, profileName, region, clusterARN string) ([]domainecs.Service, error)
	ListTaskDefinitions(ctx context.Context, profileName, region, familyPrefix string) ([]domainecs.TaskDefinitionSummary, error)
	ListTasks(ctx context.Context, profileName, region, clusterARN string) ([]domainecs.Task, error)
	UpdateService(ctx context.Context, input domainecs.UpdateServiceInput) (domainecs.UpdateServiceResult, error)
	DescribeTaskLogTargets(ctx context.Context, profileName, region, taskDefinitionARN, taskID string) ([]domainecs.LogTarget, error)
	FetchTaskLogEvents(ctx context.Context, profileName, region string, target domainecs.LogTarget, nextToken string, lookback time.Duration, limit int32) (domainecs.LogEventsPage, error)
}

type ecsStage int

const (
	ecsStageClusters ecsStage = iota
	ecsStageResources
	ecsStageServiceDetail
	ecsStageTaskDetail
	ecsStageUpdateTaskDefinition
	ecsStageUpdateDesiredCount
	ecsStageUpdateReview
	ecsStageUpdating
)

type ecsTab int

type taskDetailTab int

const (
	ecsTabServices ecsTab = iota
	ecsTabTasks
)

const (
	taskDetailTabOverview taskDetailTab = iota
	taskDetailTabLogs
)

type clustersLoadedMsg struct {
	sessionKey string
	clusters   []domainecs.Cluster
	err        error
}
type servicesLoadedMsg struct {
	clusterARN string
	services   []domainecs.Service
	err        error
}
type tasksLoadedMsg struct {
	clusterARN string
	tasks      []domainecs.Task
	err        error
}
type taskLogTargetsLoadedMsg struct {
	taskDefinitionARN string
	taskID            string
	targets           []domainecs.LogTarget
	err               error
}
type taskLogEventsLoadedMsg struct {
	taskARN       string
	containerName string
	page          domainecs.LogEventsPage
	err           error
}
type taskLogPollTickMsg struct {
	taskARN       string
	containerName string
}
type taskDefinitionsLoadedMsg struct {
	familyPrefix    string
	taskDefinitions []domainecs.TaskDefinitionSummary
	err             error
}
type serviceUpdatedMsg struct {
	clusterARN string
	result     domainecs.UpdateServiceResult
	err        error
}
type updateSuccessClearMsg struct{ seq int }

func (clustersLoadedMsg) OwnerPageID() string        { return "ecs" }
func (servicesLoadedMsg) OwnerPageID() string        { return "ecs" }
func (tasksLoadedMsg) OwnerPageID() string           { return "ecs" }
func (taskLogTargetsLoadedMsg) OwnerPageID() string  { return "ecs" }
func (taskLogEventsLoadedMsg) OwnerPageID() string   { return "ecs" }
func (taskLogPollTickMsg) OwnerPageID() string       { return "ecs" }
func (taskDefinitionsLoadedMsg) OwnerPageID() string { return "ecs" }
func (serviceUpdatedMsg) OwnerPageID() string        { return "ecs" }
func (updateSuccessClearMsg) OwnerPageID() string    { return "ecs" }

type ECSPage struct {
	service                    ECSService
	stage                      ecsStage
	tab                        ecsTab
	sessionKey                 string
	loadedFor                  string
	loadingClusters            bool
	clustersErr                string
	clusters                   []domainecs.Cluster
	clusterIndex               int
	selectedCluster            domainecs.Cluster
	searchInput                textinput.Model
	clusterTable               table.Model
	clusterPaginator           paginator.Model
	servicesLoading            bool
	servicesErr                string
	services                   []domainecs.Service
	serviceIndex               int
	serviceTable               table.Model
	servicePaginator           paginator.Model
	selectedService            domainecs.Service
	taskDefinitionsLoading     bool
	taskDefinitionsErr         string
	taskDefinitions            []domainecs.TaskDefinitionSummary
	taskDefinitionIndex        int
	taskDefinitionPaginator    paginator.Model
	updateFamilyPrefix         string
	desiredCountInput          textinput.Model
	updateForceNewDeployment   bool
	updatingService            bool
	updateErr                  string
	updateSuccess              string
	updateSuccessSeq           int
	updateCancel               context.CancelFunc
	tasksLoading               bool
	tasksErr                   string
	tasks                      []domainecs.Task
	taskIndex                  int
	taskTable                  table.Model
	taskPaginator              paginator.Model
	selectedTask               domainecs.Task
	taskDetailTab              taskDetailTab
	logTargetsByTaskDefinition map[string][]domainecs.LogTarget
	logTargetsLoading          bool
	logTargetsErr              string
	logTargets                 []domainecs.LogTarget
	logContainerIndex          int
	logEventsLoading           bool
	logEventsErr               string
	logEvents                  []domainecs.LogEvent
	logSeenEventIDs            map[string]struct{}
	logNextToken               string
	logStreaming               bool
	logViewport                viewport.Model
	spinner                    spinner.Model
	clustersCancel             context.CancelFunc
	servicesCancel             context.CancelFunc
	tasksCancel                context.CancelFunc
	logsCancel                 context.CancelFunc
}

func NewECSPage(service ECSService) *ECSPage {
	search := textinput.New()
	search.Prompt = "Search: "
	search.Placeholder = "filter"
	search.CharLimit = 256
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = styles.StatusStyle
	ct := table.New(table.WithColumns(clusterColumns()), table.WithHeight(9))
	ct.SetStyles(tableStyles())
	st := table.New(table.WithColumns(serviceColumns()), table.WithHeight(9))
	st.SetStyles(tableStyles())
	tt := table.New(table.WithColumns(taskColumns()), table.WithHeight(9))
	tt.SetStyles(tableStyles())
	cp := paginator.New(paginator.WithPerPage(8))
	cp.Type = paginator.Arabic
	sp := paginator.New(paginator.WithPerPage(8))
	sp.Type = paginator.Arabic
	tp := paginator.New(paginator.WithPerPage(8))
	tp.Type = paginator.Arabic
	dp := paginator.New(paginator.WithPerPage(8))
	dp.Type = paginator.Arabic
	desired := textinput.New()
	desired.Prompt = "Desired tasks: "
	desired.Placeholder = "1"
	desired.CharLimit = 6
	vp := viewport.New(80, 12)
	return &ECSPage{service: service, stage: ecsStageClusters, searchInput: search, desiredCountInput: desired, spinner: spin, clusterTable: ct, clusterPaginator: cp, serviceTable: st, servicePaginator: sp, taskTable: tt, taskPaginator: tp, taskDefinitionPaginator: dp, logTargetsByTaskDefinition: map[string][]domainecs.LogTarget{}, logSeenEventIDs: map[string]struct{}{}, logViewport: vp}
}

func (*ECSPage) ID() string          { return "ecs" }
func (*ECSPage) Title() string       { return "ECS" }
func (*ECSPage) Description() string { return "Browse ECS clusters, services, and tasks." }
func (p *ECSPage) HasFocusedInput() bool {
	return p.searchInput.Focused() || p.desiredCountInput.Focused()
}
