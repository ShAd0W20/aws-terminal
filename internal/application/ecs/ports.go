package ecs

import (
	"context"
	"time"

	domainecs "aws-terminal/internal/domain/ecs"
)

type API interface {
	ListClusters(ctx context.Context, profileName, region string) ([]domainecs.Cluster, error)
	ListServices(ctx context.Context, profileName, region, clusterARN string) ([]domainecs.Service, error)
	ListTaskDefinitions(ctx context.Context, profileName, region, familyPrefix string) ([]domainecs.TaskDefinitionSummary, error)
	ListTasks(ctx context.Context, profileName, region, clusterARN string) ([]domainecs.Task, error)
	UpdateService(ctx context.Context, input domainecs.UpdateServiceInput) (domainecs.UpdateServiceResult, error)
	StopTask(ctx context.Context, input domainecs.StopTaskInput) (domainecs.StopTaskResult, error)
	DescribeTaskLogTargets(ctx context.Context, profileName, region, taskDefinitionARN, taskID string) ([]domainecs.LogTarget, error)
	FetchTaskLogEvents(ctx context.Context, profileName, region string, target domainecs.LogTarget, nextToken string, lookback time.Duration, limit int32) (domainecs.LogEventsPage, error)
}
