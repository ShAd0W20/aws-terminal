package sqs

import (
	"context"

	domainsqs "aws-terminal/internal/domain/sqs"
)

type QueueAPI interface {
	ListQueues(ctx context.Context, profileName, region string) ([]domainsqs.Queue, error)
	ReceiveMessages(ctx context.Context, input QueueActionInput) ([]domainsqs.Message, error)
	PurgeQueue(ctx context.Context, input QueueActionInput) error
}

type QueueActionInput struct {
	Profile  string
	Region   string
	Queue    domainsqs.Queue
	MaxCount int32
}
