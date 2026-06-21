package awssqs

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqssdk "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	appsqs "aws-terminal/internal/application/sqs"
	domainsqs "aws-terminal/internal/domain/sqs"
	"aws-terminal/internal/infrastructure/awsclients"
)

type sqsClient interface {
	ListQueues(ctx context.Context, params *awssqssdk.ListQueuesInput, optFns ...func(*awssqssdk.Options)) (*awssqssdk.ListQueuesOutput, error)
	GetQueueAttributes(ctx context.Context, params *awssqssdk.GetQueueAttributesInput, optFns ...func(*awssqssdk.Options)) (*awssqssdk.GetQueueAttributesOutput, error)
	ReceiveMessage(ctx context.Context, params *awssqssdk.ReceiveMessageInput, optFns ...func(*awssqssdk.Options)) (*awssqssdk.ReceiveMessageOutput, error)
	PurgeQueue(ctx context.Context, params *awssqssdk.PurgeQueueInput, optFns ...func(*awssqssdk.Options)) (*awssqssdk.PurgeQueueOutput, error)
}

type Service struct {
	clients *awsclients.Factory
	client  sqsClient
}

func NewService() *Service {
	return NewServiceWithFactory(awsclients.Default())
}

func NewServiceWithFactory(clients *awsclients.Factory) *Service {
	if clients == nil {
		clients = awsclients.Default()
	}
	return &Service{clients: clients}
}

func newServiceWithClient(client sqsClient) *Service {
	return &Service{client: client}
}

func (s *Service) ListQueues(ctx context.Context, profileName, region string) ([]domainsqs.Queue, error) {
	if s.client == nil {
		ctxWithTimeout, cancel := awsclients.WithTimeout(ctx, s.clients.OperationTimeout())
		defer cancel()
		ctx = ctxWithTimeout
	}

	client, err := s.sqsClient(ctx, profileName, region)
	if err != nil {
		return nil, err
	}

	urls, err := listQueueURLs(ctx, client)
	if err != nil {
		return nil, err
	}

	queues := make([]domainsqs.Queue, 0, len(urls))
	for _, queueURL := range urls {
		queue, err := queueFromAttributes(ctx, client, queueURL)
		if err != nil {
			return nil, err
		}
		queues = append(queues, queue)
	}
	return queues, nil
}

func (s *Service) ReceiveMessages(ctx context.Context, input appsqs.QueueActionInput) ([]domainsqs.Message, error) {
	if s.client == nil {
		ctxWithTimeout, cancel := awsclients.WithTimeout(ctx, s.clients.OperationTimeout())
		defer cancel()
		ctx = ctxWithTimeout
	}
	client, err := s.sqsClient(ctx, input.Profile, input.Region)
	if err != nil {
		return nil, err
	}
	maxCount := input.MaxCount
	if maxCount <= 0 || maxCount > 10 {
		maxCount = 10
	}
	output, err := client.ReceiveMessage(ctx, &awssqssdk.ReceiveMessageInput{
		QueueUrl:            aws.String(input.Queue.URL),
		MaxNumberOfMessages: maxCount,
		MessageSystemAttributeNames: []sqstypes.MessageSystemAttributeName{
			sqstypes.MessageSystemAttributeNameSentTimestamp,
		},
	})
	if err != nil {
		return nil, err
	}
	messages := make([]domainsqs.Message, 0, len(output.Messages))
	for _, message := range output.Messages {
		messages = append(messages, domainsqs.Message{
			ID:            aws.ToString(message.MessageId),
			Body:          aws.ToString(message.Body),
			ReceiptHandle: aws.ToString(message.ReceiptHandle),
			SentAt:        parseUnixMilliseconds(message.Attributes[string(sqstypes.MessageSystemAttributeNameSentTimestamp)]),
		})
	}
	return messages, nil
}

func (s *Service) PurgeQueue(ctx context.Context, input appsqs.QueueActionInput) error {
	if s.client == nil {
		ctxWithTimeout, cancel := awsclients.WithTimeout(ctx, s.clients.OperationTimeout())
		defer cancel()
		ctx = ctxWithTimeout
	}
	client, err := s.sqsClient(ctx, input.Profile, input.Region)
	if err != nil {
		return err
	}
	_, err = client.PurgeQueue(ctx, &awssqssdk.PurgeQueueInput{QueueUrl: aws.String(input.Queue.URL)})
	return err
}

func (s *Service) sqsClient(ctx context.Context, profileName, region string) (sqsClient, error) {
	if s.client != nil {
		return s.client, nil
	}
	client, err := s.clients.SQS(ctx, profileName, region)
	if err != nil {
		return nil, fmt.Errorf("load SQS client: %w", err)
	}
	return client, nil
}

func listQueueURLs(ctx context.Context, client sqsClient) ([]string, error) {
	var urls []string
	var nextToken *string
	for {
		page, err := client.ListQueues(ctx, &awssqssdk.ListQueuesInput{MaxResults: aws.Int32(1000), NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		urls = append(urls, page.QueueUrls...)
		if page.NextToken == nil || aws.ToString(page.NextToken) == "" {
			break
		}
		nextToken = page.NextToken
	}
	return urls, nil
}

func queueFromAttributes(ctx context.Context, client sqsClient, queueURL string) (domainsqs.Queue, error) {
	output, err := client.GetQueueAttributes(ctx, &awssqssdk.GetQueueAttributesInput{
		QueueUrl: aws.String(queueURL),
		AttributeNames: []sqstypes.QueueAttributeName{
			sqstypes.QueueAttributeNameQueueArn,
			sqstypes.QueueAttributeNameApproximateNumberOfMessages,
			sqstypes.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
		},
	})
	if err != nil {
		return domainsqs.Queue{}, err
	}
	attrs := output.Attributes
	arn := attrs[string(sqstypes.QueueAttributeNameQueueArn)]
	return domainsqs.Queue{
		Name:              queueName(queueURL, arn),
		URL:               queueURL,
		ARN:               arn,
		AvailableMessages: parseAttributeInt(attrs[string(sqstypes.QueueAttributeNameApproximateNumberOfMessages)]),
		InFlightMessages:  parseAttributeInt(attrs[string(sqstypes.QueueAttributeNameApproximateNumberOfMessagesNotVisible)]),
	}, nil
}

func parseUnixMilliseconds(value string) time.Time {
	millis := parseAttributeInt(value)
	if millis <= 0 {
		return time.Time{}
	}
	return time.Unix(0, millis*int64(time.Millisecond))
}

func parseAttributeInt(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func queueName(queueURL, arn string) string {
	if arn = strings.TrimSpace(arn); arn != "" {
		parts := strings.Split(arn, ":")
		if len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) != "" {
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	parsed, err := url.Parse(strings.TrimSpace(queueURL))
	if err == nil {
		path := strings.Trim(parsed.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) != "" {
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	queueURL = strings.TrimRight(strings.TrimSpace(queueURL), "/")
	if index := strings.LastIndex(queueURL, "/"); index >= 0 && index < len(queueURL)-1 {
		return queueURL[index+1:]
	}
	return queueURL
}
