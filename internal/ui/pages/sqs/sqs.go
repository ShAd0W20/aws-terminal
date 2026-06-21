package sqs

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"

	appsqs "aws-terminal/internal/application/sqs"
	domainsqs "aws-terminal/internal/domain/sqs"
	"aws-terminal/internal/ui/styles"
)

type SQSService interface {
	ListQueues(ctx context.Context, profileName, region string) ([]domainsqs.Queue, error)
	ReceiveMessages(ctx context.Context, input appsqs.QueueActionInput) ([]domainsqs.Message, error)
	PurgeQueue(ctx context.Context, input appsqs.QueueActionInput) error
}

type sqsStage int

const (
	sqsStageQueues sqsStage = iota
	sqsStageQueueActions
	sqsStageMessages
	sqsStagePurgeConfirm
)

type queuesLoadedMsg struct {
	sessionKey string
	queues     []domainsqs.Queue
	err        error
}

type messagesLoadedMsg struct {
	queueName string
	messages  []domainsqs.Message
	err       error
}

type queuePurgedMsg struct {
	queueName string
	err       error
}

func (queuesLoadedMsg) OwnerPageID() string   { return "sqs" }
func (messagesLoadedMsg) OwnerPageID() string { return "sqs" }
func (queuePurgedMsg) OwnerPageID() string    { return "sqs" }

type SQSPage struct {
	service       SQSService
	stage         sqsStage
	sessionKey    string
	loadedFor     string
	loading       bool
	loadErr       string
	queues        []domainsqs.Queue
	queueIndex    int
	selectedQueue domainsqs.Queue
	search        textinput.Model
	table         table.Model
	spinner       spinner.Model
	cancel        context.CancelFunc

	messagesLoading bool
	messagesErr     string
	messages        []domainsqs.Message
	messageIndex    int
	messagesCancel  context.CancelFunc

	purging      bool
	purgeErr     string
	purgeMessage string
	purgeInput   textinput.Model
	purgeCancel  context.CancelFunc
}

func NewSQSPage(service SQSService) *SQSPage {
	search := textinput.New()
	search.Prompt = "Search: "
	search.Placeholder = "queue name"
	search.CharLimit = 256

	purgeInput := textinput.New()
	purgeInput.Prompt = "Type queue name: "
	purgeInput.Placeholder = "queue-name"
	purgeInput.CharLimit = 256

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = styles.StatusStyle

	queueTable := table.New(
		table.WithColumns(queueTableColumns()),
		table.WithHeight(8),
	)
	queueTable.SetStyles(queueTableStyles())

	return &SQSPage{service: service, stage: sqsStageQueues, search: search, purgeInput: purgeInput, spinner: spin, table: queueTable}
}

func (*SQSPage) ID() string    { return "sqs" }
func (*SQSPage) Title() string { return "SQS" }
func (*SQSPage) Description() string {
	return "Visualize SQS queues and approximate available/in-flight messages."
}
