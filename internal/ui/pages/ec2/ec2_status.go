package ec2

import (
	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/workflow"
)

func (p *EC2Page) PageStatus(pageapi.State) pageapi.Status {
	return workflow.FirstStatus(
		workflow.Error(p.loadErr),
		workflow.Error(p.actionErr),
		workflow.Activity(p.actionMessage != "", p.actionMessage),
		workflow.Activity(p.loading, "Loading EC2 instances..."),
		workflow.Activity(p.stopping, "Stopping EC2 instance..."),
		workflow.Activity(p.terminating, "Terminating EC2 instance..."),
	)
}
