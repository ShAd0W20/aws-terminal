package s3

import (
	"aws-terminal/internal/ui/pageapi"
	"aws-terminal/internal/ui/workflow"
)

func (p *S3Page) PageStatus(pageapi.State) pageapi.Status {
	return workflow.FirstStatus(
		workflow.Error(p.bucketErr),
		workflow.Error(p.sourceErr),
		workflow.Error(p.planErr),
		workflow.Error(p.syncErr),
		workflow.Activity(p.syncMessage != "", p.syncMessage),
		workflow.Activity(p.loadingBuckets, "S3: loading buckets..."),
		workflow.Activity(p.inspectingSource, "S3: inspecting source..."),
		workflow.Activity(p.planning, "S3: building sync plan..."),
		workflow.Activity(p.syncing, "S3: sync running..."),
	)
}
