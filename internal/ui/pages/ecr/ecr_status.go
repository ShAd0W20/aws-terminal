package ecr

import "aws-terminal/internal/ui/pageapi"

import "aws-terminal/internal/ui/workflow"

func (p *ECRPage) PageStatus(state pageapi.State) pageapi.Status {
	return workflow.FirstStatus(
		workflow.Error(p.repositoryErr), workflow.Error(p.imagesErr), workflow.Error(p.localErr), workflow.Error(p.planErr), workflow.Error(p.pushErr),
		workflow.Activity(p.loadingRepositories, "Loading ECR repositories..."), workflow.Activity(p.imagesLoading, "Loading ECR images..."), workflow.Activity(p.localLoading, "Loading local Docker images..."), workflow.Activity(p.pushing, "Pushing Docker image to ECR..."), workflow.Activity(p.pushMessage != "", p.pushMessage),
	)
}
