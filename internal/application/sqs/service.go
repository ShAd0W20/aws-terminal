package sqs

import (
	"context"
	"fmt"
	"sort"
	"strings"

	domainsqs "aws-terminal/internal/domain/sqs"
)

type Service struct {
	queues QueueAPI
}

func NewService(queues QueueAPI) *Service {
	return &Service{queues: queues}
}

func (s *Service) ListQueues(ctx context.Context, profileName, region string) ([]domainsqs.Queue, error) {
	profileName, region, err := validateProfileRegion(profileName, region)
	if err != nil {
		return nil, err
	}

	queues, err := s.queues.ListQueues(ctx, profileName, region)
	if err != nil {
		return nil, err
	}
	sort.Slice(queues, func(i, j int) bool {
		return strings.ToLower(queues[i].Name) < strings.ToLower(queues[j].Name)
	})
	return queues, nil
}

func (s *Service) ReceiveMessages(ctx context.Context, input QueueActionInput) ([]domainsqs.Message, error) {
	input, err := validateQueueActionInput(input)
	if err != nil {
		return nil, err
	}
	if input.MaxCount <= 0 || input.MaxCount > 10 {
		input.MaxCount = 10
	}
	return s.queues.ReceiveMessages(ctx, input)
}

func (s *Service) PurgeQueue(ctx context.Context, input QueueActionInput) error {
	input, err := validateQueueActionInput(input)
	if err != nil {
		return err
	}
	return s.queues.PurgeQueue(ctx, input)
}

func validateQueueActionInput(input QueueActionInput) (QueueActionInput, error) {
	profileName, region, err := validateProfileRegion(input.Profile, input.Region)
	if err != nil {
		return QueueActionInput{}, err
	}
	input.Profile = profileName
	input.Region = region
	input.Queue.Name = strings.TrimSpace(input.Queue.Name)
	input.Queue.URL = strings.TrimSpace(input.Queue.URL)
	if input.Queue.Name == "" {
		return QueueActionInput{}, fmt.Errorf("queue name is required")
	}
	if input.Queue.URL == "" {
		return QueueActionInput{}, fmt.Errorf("queue URL is required")
	}
	return input, nil
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
