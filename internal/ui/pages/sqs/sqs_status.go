package sqs

import (
	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/workflow"
)

func (p *SQSPage) PageStatus(pageapi.State) pageapi.Status {
	return workflow.FirstStatus(
		workflow.Error(p.loadErr),
		workflow.Error(p.messagesErr),
		workflow.Error(p.purgeErr),
		workflow.Activity(p.purgeMessage != "", p.purgeMessage),
		workflow.Activity(p.loading, "Loading SQS queues..."),
		workflow.Activity(p.messagesLoading, "Pulling SQS messages..."),
		workflow.Activity(p.purging, "Purging SQS queue..."),
	)
}
