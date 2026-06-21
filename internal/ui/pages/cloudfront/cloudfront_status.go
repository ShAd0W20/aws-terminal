package cloudfront

import (
	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/workflow"
)

func (p *CloudFrontPage) PageStatus(pageapi.State) pageapi.Status {
	return workflow.FirstStatus(
		workflow.Error(p.loadErr),
		workflow.Error(p.createErr),
		workflow.Activity(p.copiedMessage != "", p.copiedMessage),
		workflow.Activity(p.loading, "CloudFront: loading distributions..."),
		workflow.Activity(p.creating && p.invalidation == nil, "CloudFront: creating invalidation..."),
		workflow.Activity(p.creating, "CloudFront: waiting for invalidation completion..."),
	)
}
