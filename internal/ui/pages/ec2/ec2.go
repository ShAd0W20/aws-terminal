package ec2

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"

	domainec2 "aws-terminal/internal/domain/ec2"
	"aws-terminal/internal/ui/styles"
)

type EC2Service interface {
	ListInstances(ctx context.Context, profileName, region string) ([]domainec2.Instance, error)
	StopInstance(ctx context.Context, input domainec2.StopInstanceInput) (domainec2.StopInstanceResult, error)
	TerminateInstance(ctx context.Context, input domainec2.TerminateInstanceInput) (domainec2.TerminateInstanceResult, error)
}

type ec2Stage int

const (
	ec2StageInstances ec2Stage = iota
	ec2StageInstanceDetail
	ec2StageStopReview
	ec2StageStopping
	ec2StageTerminateConfirm
	ec2StageTerminating
)

type instancesLoadedMsg struct {
	sessionKey string
	instances  []domainec2.Instance
	err        error
}

type instanceStoppedMsg struct {
	sessionKey string
	instanceID string
	result     domainec2.StopInstanceResult
	err        error
}

type instanceTerminatedMsg struct {
	sessionKey string
	instanceID string
	result     domainec2.TerminateInstanceResult
	err        error
}

func (instancesLoadedMsg) OwnerPageID() string    { return "ec2" }
func (instanceStoppedMsg) OwnerPageID() string    { return "ec2" }
func (instanceTerminatedMsg) OwnerPageID() string { return "ec2" }
func (*EC2Page) ID() string                       { return "ec2" }
func (*EC2Page) Title() string                    { return "EC2" }
func (*EC2Page) Description() string              { return "Browse and manage EC2 instances." }
func (p *EC2Page) HasFocusedInput() bool          { return p.search.Focused() || p.terminateInput.Focused() }

type EC2Page struct {
	service        EC2Service
	stage          ec2Stage
	sessionKey     string
	loadedFor      string
	loading        bool
	loadErr        string
	instances      []domainec2.Instance
	instanceIndex  int
	selected       domainec2.Instance
	search         textinput.Model
	terminateInput textinput.Model
	table          table.Model
	spinner        spinner.Model
	loadCancel     context.CancelFunc
	actionCancel   context.CancelFunc
	stopping       bool
	terminating    bool
	actionErr      string
	actionMessage  string
}

func NewEC2Page(service EC2Service) *EC2Page {
	search := textinput.New()
	search.Prompt = "Search: "
	search.Placeholder = "name, instance id, state, ip"
	search.CharLimit = 256

	confirm := textinput.New()
	confirm.Prompt = "Type instance ID: "
	confirm.Placeholder = "i-..."
	confirm.CharLimit = 64

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = styles.StatusStyle

	instanceTable := table.New(table.WithColumns(instanceColumns()), table.WithHeight(8))
	instanceTable.SetStyles(instanceTableStyles())

	return &EC2Page{service: service, stage: ec2StageInstances, search: search, terminateInput: confirm, table: instanceTable, spinner: spin}
}
