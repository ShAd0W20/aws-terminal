package ecs

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	domainecs "aws-terminal/internal/domain/ecs"
)

type Service struct{ api API }

func NewService(api API) *Service { return &Service{api: api} }

func (s *Service) ListClusters(ctx context.Context, profileName, region string) ([]domainecs.Cluster, error) {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return nil, fmt.Errorf("profile name is required")
	}
	clusters, err := s.api.ListClusters(ctx, profileName, strings.TrimSpace(region))
	if err != nil {
		return nil, err
	}
	SortClusters(clusters)
	return clusters, nil
}

func (s *Service) ListServices(ctx context.Context, profileName, region, clusterARN string) ([]domainecs.Service, error) {
	profileName = strings.TrimSpace(profileName)
	clusterARN = strings.TrimSpace(clusterARN)
	if profileName == "" {
		return nil, fmt.Errorf("profile name is required")
	}
	if clusterARN == "" {
		return nil, fmt.Errorf("cluster ARN is required")
	}
	services, err := s.api.ListServices(ctx, profileName, strings.TrimSpace(region), clusterARN)
	if err != nil {
		return nil, err
	}
	SortServices(services)
	return services, nil
}

func (s *Service) ListTaskDefinitions(ctx context.Context, profileName, region, familyPrefix string) ([]domainecs.TaskDefinitionSummary, error) {
	profileName = strings.TrimSpace(profileName)
	familyPrefix = strings.TrimSpace(familyPrefix)
	if profileName == "" {
		return nil, fmt.Errorf("profile name is required")
	}
	if familyPrefix == "" {
		return nil, fmt.Errorf("task definition family is required")
	}
	definitions, err := s.api.ListTaskDefinitions(ctx, profileName, strings.TrimSpace(region), familyPrefix)
	if err != nil {
		return nil, err
	}
	SortTaskDefinitions(definitions)
	return definitions, nil
}

func (s *Service) ListTasks(ctx context.Context, profileName, region, clusterARN string) ([]domainecs.Task, error) {
	profileName = strings.TrimSpace(profileName)
	clusterARN = strings.TrimSpace(clusterARN)
	if profileName == "" {
		return nil, fmt.Errorf("profile name is required")
	}
	if clusterARN == "" {
		return nil, fmt.Errorf("cluster ARN is required")
	}
	tasks, err := s.api.ListTasks(ctx, profileName, strings.TrimSpace(region), clusterARN)
	if err != nil {
		return nil, err
	}
	filtered := tasks[:0]
	for _, task := range tasks {
		if !strings.EqualFold(strings.TrimSpace(task.LastStatus), "STOPPED") {
			filtered = append(filtered, task)
		}
	}
	SortTasks(filtered)
	return filtered, nil
}

func (s *Service) UpdateService(ctx context.Context, input domainecs.UpdateServiceInput) (domainecs.UpdateServiceResult, error) {
	input.ProfileName = strings.TrimSpace(input.ProfileName)
	input.Region = strings.TrimSpace(input.Region)
	input.ClusterARN = strings.TrimSpace(input.ClusterARN)
	input.Service = strings.TrimSpace(input.Service)
	input.TaskDefinitionARN = strings.TrimSpace(input.TaskDefinitionARN)
	if input.ProfileName == "" {
		return domainecs.UpdateServiceResult{}, fmt.Errorf("profile name is required")
	}
	if input.ClusterARN == "" {
		return domainecs.UpdateServiceResult{}, fmt.Errorf("cluster ARN is required")
	}
	if input.Service == "" {
		return domainecs.UpdateServiceResult{}, fmt.Errorf("service is required")
	}
	if input.DesiredCount != nil && *input.DesiredCount < 0 {
		return domainecs.UpdateServiceResult{}, fmt.Errorf("desired count must be non-negative")
	}
	if input.TaskDefinitionARN == "" && input.DesiredCount == nil && !input.ForceNewDeployment {
		return domainecs.UpdateServiceResult{}, fmt.Errorf("at least one service update is required")
	}
	return s.api.UpdateService(ctx, input)
}

func (s *Service) DescribeTaskLogTargets(ctx context.Context, profileName, region, taskDefinitionARN, taskID string) ([]domainecs.LogTarget, error) {
	profileName = strings.TrimSpace(profileName)
	taskDefinitionARN = strings.TrimSpace(taskDefinitionARN)
	taskID = strings.TrimSpace(taskID)
	if profileName == "" {
		return nil, fmt.Errorf("profile name is required")
	}
	if taskDefinitionARN == "" {
		return nil, fmt.Errorf("task definition ARN is required")
	}
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	return s.api.DescribeTaskLogTargets(ctx, profileName, strings.TrimSpace(region), taskDefinitionARN, taskID)
}

func (s *Service) FetchTaskLogEvents(ctx context.Context, profileName, region string, target domainecs.LogTarget, nextToken string, lookback time.Duration, limit int32) (domainecs.LogEventsPage, error) {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return domainecs.LogEventsPage{}, fmt.Errorf("profile name is required")
	}
	if !target.Supported {
		return domainecs.LogEventsPage{}, fmt.Errorf("log target is not supported")
	}
	if strings.TrimSpace(target.LogGroup) == "" {
		return domainecs.LogEventsPage{}, fmt.Errorf("log group is required")
	}
	if strings.TrimSpace(target.LogStream) == "" {
		return domainecs.LogEventsPage{}, fmt.Errorf("log stream is required")
	}
	if lookback <= 0 {
		lookback = 15 * time.Minute
	}
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	return s.api.FetchTaskLogEvents(ctx, profileName, strings.TrimSpace(region), target, strings.TrimSpace(nextToken), lookback, limit)
}

func SortClusters(clusters []domainecs.Cluster) {
	sort.SliceStable(clusters, func(i, j int) bool {
		ia, ja := strings.EqualFold(clusters[i].Status, "ACTIVE"), strings.EqualFold(clusters[j].Status, "ACTIVE")
		if ia != ja {
			return ia
		}
		return strings.ToLower(clusters[i].Name) < strings.ToLower(clusters[j].Name)
	})
}

func SortServices(services []domainecs.Service) {
	sort.SliceStable(services, func(i, j int) bool {
		ia, ja := strings.EqualFold(services[i].Status, "ACTIVE"), strings.EqualFold(services[j].Status, "ACTIVE")
		if ia != ja {
			return ia
		}
		return strings.ToLower(services[i].Name) < strings.ToLower(services[j].Name)
	})
}

func SortTaskDefinitions(definitions []domainecs.TaskDefinitionSummary) {
	sort.SliceStable(definitions, func(i, j int) bool {
		if !strings.EqualFold(definitions[i].Family, definitions[j].Family) {
			return strings.ToLower(definitions[i].Family) < strings.ToLower(definitions[j].Family)
		}
		return definitions[i].Revision > definitions[j].Revision
	})
}

func SortTasks(tasks []domainecs.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		ir, jr := strings.EqualFold(tasks[i].LastStatus, "RUNNING"), strings.EqualFold(tasks[j].LastStatus, "RUNNING")
		if ir != jr {
			return !ir
		}
		if !tasks[i].CreatedAt.Equal(tasks[j].CreatedAt) {
			return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
		}
		return tasks[i].ID < tasks[j].ID
	})
}
