package ec2

import (
	"context"
	"fmt"
	"sort"
	"strings"

	domainec2 "aws-terminal/internal/domain/ec2"
)

type Service struct{ api API }

func NewService(api API) *Service { return &Service{api: api} }

func (s *Service) ListInstances(ctx context.Context, profileName, region string) ([]domainec2.Instance, error) {
	profileName, region, err := validateProfileRegion(profileName, region)
	if err != nil {
		return nil, err
	}
	instances, err := s.api.ListInstances(ctx, profileName, region)
	if err != nil {
		return nil, err
	}
	filtered := instances[:0]
	for _, instance := range instances {
		if !strings.EqualFold(strings.TrimSpace(instance.State), "terminated") {
			filtered = append(filtered, instance)
		}
	}
	SortInstances(filtered)
	return filtered, nil
}

func (s *Service) StopInstance(ctx context.Context, input domainec2.StopInstanceInput) (domainec2.StopInstanceResult, error) {
	var err error
	input.ProfileName, input.Region, err = validateProfileRegion(input.ProfileName, input.Region)
	if err != nil {
		return domainec2.StopInstanceResult{}, err
	}
	input.InstanceID = strings.TrimSpace(input.InstanceID)
	if input.InstanceID == "" {
		return domainec2.StopInstanceResult{}, fmt.Errorf("instance ID is required")
	}
	return s.api.StopInstance(ctx, input)
}

func (s *Service) TerminateInstance(ctx context.Context, input domainec2.TerminateInstanceInput) (domainec2.TerminateInstanceResult, error) {
	var err error
	input.ProfileName, input.Region, err = validateProfileRegion(input.ProfileName, input.Region)
	if err != nil {
		return domainec2.TerminateInstanceResult{}, err
	}
	input.InstanceID = strings.TrimSpace(input.InstanceID)
	if input.InstanceID == "" {
		return domainec2.TerminateInstanceResult{}, fmt.Errorf("instance ID is required")
	}
	return s.api.TerminateInstance(ctx, input)
}

func SortInstances(instances []domainec2.Instance) {
	sort.SliceStable(instances, func(i, j int) bool {
		ir, jr := strings.EqualFold(instances[i].State, "running"), strings.EqualFold(instances[j].State, "running")
		if ir != jr {
			return ir
		}
		if !instances[i].LaunchTime.Equal(instances[j].LaunchTime) {
			return instances[i].LaunchTime.After(instances[j].LaunchTime)
		}
		in, jn := strings.ToLower(instanceLabel(instances[i])), strings.ToLower(instanceLabel(instances[j]))
		if in != jn {
			return in < jn
		}
		return instances[i].ID < instances[j].ID
	})
}

func validateProfileRegion(profileName, region string) (string, string, error) {
	profileName = strings.TrimSpace(profileName)
	region = strings.TrimSpace(region)
	if profileName == "" {
		return "", "", fmt.Errorf("profile name is required")
	}
	if region == "" {
		return "", "", fmt.Errorf("region is required")
	}
	return profileName, region, nil
}

func instanceLabel(instance domainec2.Instance) string {
	if strings.TrimSpace(instance.Name) != "" {
		return instance.Name
	}
	return instance.ID
}
